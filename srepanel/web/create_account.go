package web

import (
	"fmt"
	"log"
	"net/http"
	"net/url"

	"go.mkw.re/ghidra-panel/ghidra"
	"go.mkw.re/ghidra-panel/passphrase"
	"go.mkw.re/ghidra-panel/username"
)

func (s *Server) handleCreateAccount(wr http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(wr, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ident, ok := s.checkAuth(req)
	if !ok {
		http.Error(wr, "Not authorized", http.StatusUnauthorized)
		return
	}

	// Try to use the exact sanitized OAuth username first
	baseUser := username.Sanitize(ident.Username)
	user := baseUser

	// Check for collision and append incremental numeric suffix if needed
	for i := 0; i < 100; i++ { // limit to 100 attempts to prevent infinite loops
		exists, err := s.DB.UsernameExists(req.Context(), user)
		if err != nil {
			log.Println("Failed to check if username exists:", err)
			http.Redirect(wr, req, redirectUrl(req, map[string]string{"status": "internal_error"}), http.StatusSeeOther)
			return
		}

		if !exists {
			break // Found a unique username
		}

		// Collision occurred, append a suffix (e.g., baseUser_1)
		user = fmt.Sprintf("%s_%d", baseUser, i+1)
	}

	// Auto-generate secure passphrase
	pass, err := passphrase.Generate()
	if err != nil {
		log.Println("Failed to generate passphrase:", err)
		http.Redirect(wr, req, redirectUrl(req, map[string]string{"status": "internal_error"}), http.StatusSeeOther)
		return
	}

	// Check if Ghidra account already exists (shouldn't happen with auto-generated usernames)
	request := ghidra.AuthenticateUserRequest{
		Username: user,
		Password: pass,
	}
	auth, err := s.Client.AuthenticateUser(req.Context(), &request)
	if err != nil {
		log.Println("Failed to authenticate user:", err)
		http.Redirect(wr, req, redirectUrl(req, map[string]string{"status": "internal_error"}), http.StatusSeeOther)
		return
	}
	// If account exists, this is unexpected (collision in username generation)
	if auth.Username != "" {
		log.Printf("Unexpected: Ghidra account %s already exists", user)
		http.Redirect(wr, req, redirectUrl(req, map[string]string{"status": "internal_error"}), http.StatusSeeOther)
		return
	}

	// Create the account in the database
	// Check if this is the first user and should be made admin
	count, err := s.DB.CountAccounts(req.Context())
	if err != nil {
		log.Println("Failed to count accounts:", err)
		http.Redirect(wr, req, redirectUrl(req, map[string]string{"status": "internal_error"}), http.StatusSeeOther)
		return
	}

	log.Printf("Creating Ghidra account for %s (provider: %s, ID: %d). Existing accounts: %d", ident.Username, ident.Provider, ident.ID, count)

	if count == 0 && s.Config.FirstUserIsAdmin {
		// First user gets admin privileges
		log.Printf("→ Granting super admin privileges (first user)")
		if err := s.DB.CreateAccountAsSuperAdmin(req.Context(), ident.ID, user, pass, ident.Provider); err != nil {
			log.Println("Failed to create admin account for user:", err)
			http.Redirect(wr, req, redirectUrl(req, map[string]string{"status": "internal_error"}), http.StatusSeeOther)
			return
		}
		log.Printf("✓ Super admin account created: %s", user)
	} else {
		if err := s.DB.CreateAccount(req.Context(), ident.ID, user, pass, ident.Provider); err != nil {
			log.Println("Failed to create account for user:", err)
			http.Redirect(wr, req, redirectUrl(req, map[string]string{"status": "internal_error"}), http.StatusSeeOther)
			return
		}
	}

	// Create the account in Ghidra
	_, err = s.Client.AddUser(req.Context(), &ghidra.AddUserRequest{Username: user})
	if err != nil {
		log.Println("Failed to create Ghidra account:", err)
		http.Redirect(wr, req, redirectUrl(req, map[string]string{"status": "internal_error"}), http.StatusSeeOther)
		return
	}

	log.Printf("Created account for user %s (OAuth: %s/%d)", user, ident.Provider, ident.ID)

	// Log account creation
	s.logAudit(req.Context(), req, "account_created", "user", user, true, map[string]interface{}{
		"provider": ident.Provider,
		"is_admin": count == 0 && s.Config.FirstUserIsAdmin,
	})

	// Redirect to credentials page with auto-generated username and passphrase
	credentialsUrl := fmt.Sprintf("/credentials?username=%s&password=%s&first_time=1",
		url.QueryEscape(user), url.QueryEscape(pass))
	http.Redirect(wr, req, credentialsUrl, http.StatusSeeOther)
}
