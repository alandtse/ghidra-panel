package web

import (
	"fmt"
	"log"
	"net/http"
	"net/url"

	"go.mkw.re/ghidra-panel/passphrase"
)

func (s *Server) handleUpdateAccount(wr http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(wr, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ident, ok := s.checkAuth(req)
	if !ok {
		http.Error(wr, "Not authorized", http.StatusUnauthorized)
		return
	}

	// Auto-generate new secure passphrase
	pass, err := passphrase.Generate()
	if err != nil {
		log.Println("Failed to generate passphrase:", err)
		http.Redirect(wr, req, redirectUrl(req, map[string]string{"status": "internal_error"}), http.StatusSeeOther)
		return
	}

	// Update password in database
	if err := s.DB.UpdatePassword(req.Context(), ident.ID, pass, ident.Provider); err != nil {
		log.Println("Failed to update password for user:", err)
		http.Redirect(wr, req, redirectUrl(req, map[string]string{"status": "internal_error"}), http.StatusSeeOther)
		return
	}

	// Get username from database
	userState, err := s.DB.GetUserState(req.Context(), ident)
	if err != nil {
		log.Println("Failed to get user state:", err)
		http.Redirect(wr, req, redirectUrl(req, map[string]string{"status": "internal_error"}), http.StatusSeeOther)
		return
	}

	log.Printf("Regenerated password for user %s (OAuth: %s/%d)", userState.Username, ident.Provider, ident.ID)

	// Log password reset
	s.logAudit(req.Context(), req, "password_reset", "user", userState.Username, true, nil)

	// Redirect to credentials page with new passphrase
	credentialsUrl := fmt.Sprintf("/credentials?username=%s&password=%s&regenerated=1",
		url.QueryEscape(userState.Username), url.QueryEscape(pass))
	http.Redirect(wr, req, credentialsUrl, http.StatusSeeOther)
}
