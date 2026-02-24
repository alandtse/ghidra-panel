package web

import (
	"embed"
	"fmt"
	"html/template"
	"log"
	"log/slog"
	"net/http"
	"net/url"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/gorilla/csrf"
	"github.com/oschwald/geoip2-golang"
	"go.mkw.re/ghidra-panel/common"
	"go.mkw.re/ghidra-panel/database"
	"go.mkw.re/ghidra-panel/discord"
	"go.mkw.re/ghidra-panel/ghidra"
	"go.mkw.re/ghidra-panel/oauth"
	"go.mkw.re/ghidra-panel/token"
	"google.golang.org/protobuf/types/known/emptypb"
)

var (
	//go:embed templates/*
	templates embed.FS

	//go:embed assets/*
	assets embed.FS
)

var (
	homePage        *template.Template
	loginPage       *template.Template
	repoPage        *template.Template
	credentialsPage *template.Template
	adminPage       *template.Template
	errorPage       *template.Template
)

func init() {
	templates, err := template.New("").
		Funcs(template.FuncMap{
			"permColor":      ghidra.PermColorHex,
			"permDisplay":    ghidra.PermDisplay,
			"formatSize":     formatSize,
			"formatDate":     formatDate,
			"formatTime":     formatTime,
			"formatUptime":   formatUptime,
			"formatDetails":  formatDetails,
			"actionDesc":     database.GetActionDescription,
			"formatLocation": formatLocation,
		}).
		ParseFS(templates, "templates/*.gohtml")
	if err != nil {
		panic(err)
	}
	homePage = templates.Lookup("home.gohtml")
	loginPage = templates.Lookup("login.gohtml")
	repoPage = templates.Lookup("repo.gohtml")
	credentialsPage = templates.Lookup("credentials.gohtml")
	adminPage = templates.Lookup("admin.gohtml")
	errorPage = templates.Lookup("error.gohtml")
}

type Config struct {
	CommunityName     string
	BaseURL           string
	GhidraEndpoint    *common.GhidraEndpoint
	Links             []common.Link
	DiscordApp        *discord.Application
	DiscordWebhookURL string
	Dev               bool // developer mode
	SuperAdmins       []string
	FirstUserIsAdmin  bool
	GeoIPDatabase     string // path to GeoLite2-City.mmdb (optional)
	MaxMindAccountID  string
	MaxMindLicenseKey string
}

type Server struct {
	Config           *Config
	DB               *database.DB
	Auth             *discord.Auth // deprecated, kept for backwards compatibility
	Providers        map[string]oauth.Provider
	ProviderMetadata map[string]*common.ProviderMetadata
	Issuer           *token.Issuer
	Client           ghidra.GhidraClient
	useSecureCookie  bool           // whether to set Secure flag on cookies
	StartTime        time.Time      // Panel start time for uptime tracking
	GeoIPDB          *geoip2.Reader // GeoIP database for location lookups (optional)
	Logger           *slog.Logger
}

func NewServer(
	config *Config,
	db *database.DB,
	auth *discord.Auth,
	issuer *token.Issuer,
	client ghidra.GhidraClient,
) (*Server, error) {
	// Determine if we should use secure cookies based on base URL
	useSecure := true
	if baseURL, err := url.Parse(config.BaseURL); err == nil {
		// Only use secure cookies if the base URL is HTTPS
		useSecure = baseURL.Scheme == "https"
	}

	server := &Server{
		Config:           config,
		DB:               db,
		Auth:             auth,
		Providers:        make(map[string]oauth.Provider),
		ProviderMetadata: make(map[string]*common.ProviderMetadata),
		Issuer:           issuer,
		Client:           client,
		useSecureCookie:  useSecure,
		StartTime:        time.Now(),
		Logger:           slog.Default(),
	}
	return server, nil
}

// AddProvider registers an OAuth provider
func (s *Server) AddProvider(provider oauth.Provider) {
	s.Providers[provider.Name()] = provider
}

// AddProviderWithMetadata registers an OAuth provider with display metadata
func (s *Server) AddProviderWithMetadata(provider oauth.Provider, metadata *common.ProviderMetadata) {
	s.Providers[provider.Name()] = provider
	s.ProviderMetadata[provider.Name()] = metadata
}

func (s *Server) RegisterRoutes(mux *http.ServeMux) http.Handler {
	mux.HandleFunc("GET /", s.handleHome)
	mux.HandleFunc("GET /login", s.handleLogin)
	mux.HandleFunc("POST /login", s.handleLogin)
	mux.HandleFunc("GET /redirect", s.handleOAuthRedirect)
	mux.HandleFunc("GET /oauth/{provider}", s.handleOAuthProvider)
	mux.HandleFunc("GET /logout", s.handleLogout)
	mux.HandleFunc("GET /repos/{repo}", s.handleRepo)

	mux.HandleFunc("GET /credentials", s.handleCredentials)
	mux.HandleFunc("POST /create_account", s.handleCreateAccount)
	mux.HandleFunc("POST /update_account", s.handleUpdateAccount)
	mux.HandleFunc("POST /request_access", s.handleRequestAccess)
	mux.HandleFunc("POST /set_user_access", s.handleSetUserAccess)
	mux.HandleFunc("POST /update_repo", s.handleUpdateRepo)
	mux.HandleFunc("POST /delete_repo", s.handleDeleteRepo)
	mux.HandleFunc("POST /unlink_account", s.handleUnlinkAccount)
	mux.HandleFunc("POST /delete_account", s.handleDeleteAccount)
	mux.HandleFunc("GET /admin", s.handleAdmin)
	mux.HandleFunc("POST /admin/delete_user", s.handleAdminDeleteUser)
	mux.HandleFunc("POST /admin/bulk_user_action", s.handleAdminBulkUserAction)
	mux.HandleFunc("POST /admin/delete_ghidra_user", s.handleAdminDeleteGhidraUser)
	mux.HandleFunc("POST /admin/bulk_ghidra_user_action", s.handleAdminBulkGhidraUserAction)
	mux.HandleFunc("POST /admin/delete_repository", s.handleAdminDeleteRepository)
	mux.HandleFunc("POST /admin/bulk_repository_action", s.handleAdminBulkRepositoryAction)

	// CSRF Protection
	var trustedOrigins []string
	if baseURL, err := url.Parse(s.Config.BaseURL); err == nil && baseURL.Host != "" {
		trustedOrigins = append(trustedOrigins, baseURL.Host)
	}

	csrfMiddleware := csrf.Protect(
		s.Issuer.Secret,
		csrf.Secure(s.useSecureCookie),
		csrf.Path("/"),
		csrf.TrustedOrigins(trustedOrigins),
	)

	// Combine middlewares
	handler := s.securityMiddleware(csrfMiddleware(mux))

	// Create file server for assets (exempt from CSRF but needs security headers)
	// Note: We apply security headers to assets too just to be safe/consistent
	assetHandler := s.securityMiddleware(http.FileServer(http.FS(assets)))
	mux.Handle("GET /assets/", assetHandler)

	return handler
}

func (s *Server) renderTemplate(wr http.ResponseWriter, req *http.Request, tmpl *template.Template, data interface{}) {
	// Inject CSRF token into data if it's a map or struct
	// This is a simplified approach; in production you'd use a base context struct

	// For now, we'll use a wrapper struct that includes CSRF data
	type TemplateData struct {
		Data      interface{}
		CSRFField template.HTML
		CSRFToken string
	}

	tmplData := TemplateData{
		Data:      data,
		CSRFField: csrf.TemplateField(req),
		CSRFToken: csrf.Token(req),
	}

	if err := tmpl.Execute(wr, tmplData); err != nil {
		s.Logger.Error("Failed to render template", "error", err)
	}
}

// State holds server-side web page state.
type State struct {
	Identity          *common.Identity // current user, null if unauthenticated
	UserState         *common.UserState
	Nav               []Nav         // navigation bar
	Links             []common.Link // footer links
	Ghidra            *common.GhidraEndpoint
	Status            string
	GhidraOnline      bool   // whether Ghidra server is reachable
	GhidraVersion     string // Ghidra server version (if online)
	CommunityName     string // Community/server name for display
	SuperAdmin        bool   // whether current user is a super admin
	AdminModeDisabled bool   // whether the user has toggled off admin view
	HasGeoIP          bool   // whether GeoIP database is loaded
}

type Nav struct {
	Route string
	Name  string
}

func (s *Server) stateWithNav(req *http.Request, nav ...Nav) *State {
	// Check Ghidra server status
	ghidraOnline := false
	ghidraVersion := ""

	if s.Config.Dev {
		log.Printf("Checking Ghidra server at %s:%d", s.Config.GhidraEndpoint.Hostname, s.Config.GhidraEndpoint.Port)
	}

	reply, err := s.Client.GetRepositories(req.Context(), &emptypb.Empty{})
	if err != nil {
		if s.Config.Dev {
			log.Printf("Ghidra server connection failed: %v", err)
		}
	} else if reply != nil && reply.Version != nil {
		ghidraOnline = true
		ghidraVersion = reply.Version.GhidraVersion
		if s.Config.Dev {
			log.Printf("Ghidra server online: version %s", ghidraVersion)
		}
	}

	communityName := s.Config.CommunityName
	if communityName == "" {
		communityName = "Ghidra"
	}

	// Check if admin mode is disabled via query parameter
	adminModeParam := req.URL.Query().Get("admin_mode")

	return &State{
		Ghidra:            s.Config.GhidraEndpoint,
		Nav:               nav,
		Links:             s.Config.Links,
		Status:            req.URL.Query().Get("status"),
		GhidraOnline:      ghidraOnline,
		GhidraVersion:     ghidraVersion,
		CommunityName:     communityName,
		AdminModeDisabled: adminModeParam == "0",
		HasGeoIP:          s.GeoIPDB != nil,
	}
}

func (s *Server) authenticateState(wr http.ResponseWriter, req *http.Request, state *State) bool {
	ident, ok := s.checkAuth(req)
	if !ok {
		http.SetCookie(wr, &http.Cookie{
			Name:   "token",
			Value:  "",
			Path:   "/",
			MaxAge: 0,
		})
		s.redirectLogin(wr, req, true)
		return false
	}

	// Populate provider from database (don't trust JWT token for provider info)
	if ident.Provider == "" {
		provider, username, err := s.DB.GetOAuthIdentity(req.Context(), ident.ID)
		if err != nil {
			log.Printf("Warning: Failed to get OAuth identity for user %d: %v", ident.ID, err)
		} else if provider != "" {
			ident.Provider = provider
			if username != "" && ident.Username == "" {
				ident.Username = username
			}
		}
	}

	state.Identity = ident

	userState, err := s.DB.GetUserState(req.Context(), ident)
	if err != nil {
		log.Println("Failed to get user state:", err)
		s.renderError(wr, req, http.StatusInternalServerError, "Failed to get user state.", state)
		return false
	}
	state.UserState = userState

	// Set SuperAdmin flag
	state.SuperAdmin = s.isSuperAdmin(req.Context(), ident)

	return true
}

// lessCaseInsensitive compares s, t without allocating
func lessCaseInsensitive(s, t string) bool {
	for {
		if len(t) == 0 {
			return false
		}
		if len(s) == 0 {
			return true
		}
		c, sizec := utf8.DecodeRuneInString(s)
		d, sized := utf8.DecodeRuneInString(t)

		lowerc := unicode.ToLower(c)
		lowerd := unicode.ToLower(d)

		if lowerc < lowerd {
			return true
		}
		if lowerc > lowerd {
			return false
		}

		s = s[sizec:]
		t = t[sized:]
	}
}

// redirectUrl Generates a redirect back to the original resource with added query parameters.
func redirectUrl(req *http.Request, params map[string]string) string {
	out := req.Header.Get("Referer")
	if out == "" {
		out = "/"
	}
	u, err := url.Parse(out)
	if err != nil {
		u, _ = url.Parse("/")
	}
	var q = url.Values{}
	for k, v := range params {
		q.Set(k, v)
	}
	u.RawQuery = q.Encode()
	return u.String()
}

type UserPermissionResult struct {
	Username   string
	Permission ghidra.Permission
}

// fetchUserPermission fetches the user's permission for a repository.
func (s *Server) fetchUserPermission(req *http.Request, ident *common.Identity, repo string) (*UserPermissionResult, error) {
	// Fetch user state from the database
	userState, err := s.DB.GetUserState(req.Context(), ident)
	if err != nil {
		return nil, fmt.Errorf("failed to get user state: %w", err)
	}

	// Fetch the user's repository permission by Ghidra username
	repoUser, err := s.Client.GetRepositoryUser(req.Context(), &ghidra.GetRepositoryUserRequest{
		Username:   userState.Username,
		Repository: repo,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get repository user: %w", err)
	}

	permission := ghidra.Permission_NONE
	if repoUser.Result != nil {
		permission = repoUser.Result.Permission
	}
	return &UserPermissionResult{
		Username:   userState.Username,
		Permission: permission,
	}, nil
}
