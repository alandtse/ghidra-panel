package web

import (
	"log"
	"net/http"
	"net/url"
	"sort"

	"go.mkw.re/ghidra-panel/common"
)

func (s *Server) handleLogin(wr http.ResponseWriter, req *http.Request) {
	if _, ok := s.checkAuth(req); ok {
		s.redirectHome(wr, req)
		return
	}

	switch req.Method {
	case http.MethodGet:
		state := s.stateWithNav(
			req,
			Nav{Route: "/", Name: "Ghidra"},
			Nav{Route: "/", Name: "Login"},
		)

		// Pass available providers to the template
		type LoginState struct {
			*State
			Providers []*common.ProviderMetadata
		}

		providerList := make([]*common.ProviderMetadata, 0, len(s.Providers))
		for name := range s.Providers {
			metadata := s.ProviderMetadata[name]
			if metadata == nil {
				// Fallback if no metadata registered
				metadata = &common.ProviderMetadata{
					Name:        name,
					DisplayName: name,
				}
			}
			providerList = append(providerList, metadata)
		}

		// Support legacy Auth for backwards compatibility
		if s.Auth != nil && len(providerList) == 0 {
			providerList = append(providerList, &common.ProviderMetadata{
				Name:        "discord",
				DisplayName: "Discord",
			})
		}

		// Sort providers by name for consistent display order
		sort.Slice(providerList, func(i, j int) bool {
			return providerList[i].Name < providerList[j].Name
		})

		loginState := &LoginState{
			State:     state,
			Providers: providerList,
		}

		s.renderTemplate(wr, req, loginPage, loginState)
	case http.MethodPost:
		if s.Config.Dev {
			ident := &common.Identity{
				ID:       1,
				Username: "testuser",
				Provider: "dev",
			}
			token, exp := s.Issuer.Issue(ident)
			log.Printf("Dev mode login: Setting token cookie (length=%d, secure=%v)", len(token), s.useSecureCookie)
			http.SetCookie(wr, &http.Cookie{
				Name:     "token",
				Value:    token,
				Path:     "/",
				Expires:  exp,
				HttpOnly: true,
				Secure:   s.useSecureCookie,
			})
			s.redirectHome(wr, req)
			return
		}
		// Legacy support: redirect to Discord auth if Auth is set
		if s.Auth != nil {
			http.Redirect(wr, req, s.Auth.AuthURL(), http.StatusSeeOther)
		} else {
			http.Error(wr, "No OAuth providers configured", http.StatusInternalServerError)
		}
	default:
		http.Error(wr, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleOAuthProvider initiates OAuth flow for a specific provider
func (s *Server) handleOAuthProvider(wr http.ResponseWriter, req *http.Request) {
	providerName := req.PathValue("provider")

	provider, ok := s.Providers[providerName]
	if !ok {
		http.Error(wr, "Unknown provider", http.StatusNotFound)
		return
	}

	http.Redirect(wr, req, provider.AuthURL(), http.StatusSeeOther)
}

func (s *Server) handleOAuthRedirect(wr http.ResponseWriter, req *http.Request) {
	var ident *common.Identity
	var err error

	// Try each provider until one successfully handles the redirect
	// The provider that issued the state token will succeed
	handled := false

	// First try new providers
	for _, provider := range s.Providers {
		ident, err = provider.HandleRedirect(wr, req)
		if err == nil && ident != nil {
			handled = true
			break
		}
		// Reset error if this provider couldn't handle it (CSRF mismatch)
		// Another provider might be able to
		if err != nil {
			continue
		}
		// If ident is nil but no error, the provider handled it (e.g., redirect to login)
		if ident == nil {
			return
		}
	}

	// Fallback to legacy Auth if no provider handled it
	if !handled && s.Auth != nil {
		ident, err = s.Auth.HandleRedirect(wr, req)
		if err != nil {
			log.Println("Redirect request failed:", err)
			http.Error(wr, "Authorization failed", http.StatusUnauthorized)
			return
		}
		if ident == nil {
			return
		}
		// Set provider for legacy auth
		ident.Provider = "discord"
	}

	if !handled && s.Auth == nil {
		log.Println("No OAuth provider handled the redirect")
		http.Error(wr, "Authorization failed", http.StatusUnauthorized)
		return
	}

	// Record OAuth login in database (source of truth for provider)
	if err := s.DB.RecordOAuthLogin(req.Context(), ident); err != nil {
		log.Printf("Warning: Failed to record OAuth login: %v", err)
	}

	// Check if this is the first user and should be granted panel admin
	if s.Config.FirstUserIsAdmin {
		count, err := s.DB.CountPanelAdmins(req.Context())
		if err != nil {
			log.Printf("Warning: Failed to count panel admins: %v", err)
		} else if count == 0 {
			// First OAuth user gets panel admin privileges
			log.Printf("Granting panel admin to first OAuth user: %s (ID: %d, Provider: %s)", ident.Username, ident.ID, ident.Provider)
			if err := s.DB.GrantPanelAdmin(req.Context(), ident.ID, ident.Provider); err != nil {
				log.Printf("Warning: Failed to grant panel admin: %v", err)
			} else {
				log.Printf("✓ Panel admin granted - user can manage all repositories")
			}
		}
	}

	token, exp := s.Issuer.Issue(ident)
	http.SetCookie(wr, &http.Cookie{
		Name:     "token",
		Value:    token,
		Path:     "/",
		Expires:  exp,
		HttpOnly: true,
		Secure:   s.useSecureCookie,
	})

	// Log successful login
	s.logAuditSimple(req.Context(), &ident.ID, ident.Username, "login", "session", "", getClientIP(req), req.UserAgent(), true)

	s.redirectHome(wr, req)
}

func (s *Server) checkAuth(req *http.Request) (*common.Identity, bool) {
	cookie, err := req.Cookie("token")
	if err != nil || cookie == nil {
		return nil, false
	}

	// Debug: log cookie value in dev mode
	if s.Config.Dev {
		log.Printf("checkAuth: cookie value length=%d, value=%q, isEmpty=%v", len(cookie.Value), cookie.Value, cookie.Value == "")
	}

	// Skip verification if cookie is empty or just whitespace (was cleared)
	if len(cookie.Value) == 0 {
		if s.Config.Dev {
			log.Print("checkAuth: skipping verification, cookie is empty")
		}
		return nil, false
	}

	ident, err := s.Issuer.Verify(cookie.Value)
	if err != nil {
		// Only log errors in development mode
		if s.Config.Dev {
			log.Print("failed to verify token: ", err)
		}
		return nil, false
	}
	return ident, true
}

func (s *Server) handleLogout(wr http.ResponseWriter, req *http.Request) {
	// Try to get identity before clearing cookie
	ident, _ := s.checkAuth(req)

	http.SetCookie(wr, &http.Cookie{
		Name:   "token",
		Value:  "",
		Path:   "/",
		MaxAge: 0,
	})

	// Log logout if we had a valid session
	if ident != nil {
		s.logAuditSimple(req.Context(), &ident.ID, ident.Username, "logout", "session", "", getClientIP(req), req.UserAgent(), true)
	}

	s.redirectLogin(wr, req, false)
}

// redirectHome redirects to the home page or a stored redirect target.
func (s *Server) redirectHome(wr http.ResponseWriter, req *http.Request) {
	if toUrl := fetchRedirect(wr, req); toUrl != nil {
		http.Redirect(wr, req, toUrl.String(), http.StatusSeeOther)
	} else {
		http.Redirect(wr, req, "/", http.StatusSeeOther)
	}
}

// fetchRedirect fetches the redirect URL from the request cookies.
func fetchRedirect(wr http.ResponseWriter, req *http.Request) *url.URL {
	cookie, err := req.Cookie("redirect")
	if err != nil {
		return nil
	}
	// Clear the redirect cookie
	http.SetCookie(wr, &http.Cookie{
		Name:   "redirect",
		Value:  "",
		Path:   "/",
		MaxAge: 0,
	})
	toUrl, err := url.Parse(cookie.Value)
	if err != nil {
		return nil
	}
	return toUrl
}

// redirectLogin redirects to the login page, optionally storing the current URL as a redirect target.
func (s *Server) redirectLogin(wr http.ResponseWriter, req *http.Request, store bool) {
	if store {
		http.SetCookie(wr, &http.Cookie{
			Name:  "redirect",
			Value: req.RequestURI,
			Path:  "/",
		})
	}
	http.Redirect(wr, req, "/login", http.StatusSeeOther)
}
