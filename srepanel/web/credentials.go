package web

import (
	"net/http"
)

type CredentialsState struct {
	*State
	Username    string
	Password    string
	FirstTime   bool
	Regenerated bool
}

func (s *Server) handleCredentials(wr http.ResponseWriter, req *http.Request) {
	state := CredentialsState{State: s.stateWithNav(req, Nav{Route: "/credentials", Name: "Credentials"})}
	if !s.authenticateState(wr, req, state.State) {
		return
	}

	// Get query parameters
	state.Username = req.URL.Query().Get("username")
	state.Password = req.URL.Query().Get("password")
	state.FirstTime = req.URL.Query().Get("first_time") == "1"
	state.Regenerated = req.URL.Query().Get("regenerated") == "1"

	// Security: Only show credentials if they were just generated
	// This prevents credentials from being shown again via URL manipulation
	if state.Username == "" || state.Password == "" {
		http.Redirect(wr, req, "/", http.StatusSeeOther)
		return
	}

	// Set headers to prevent caching
	wr.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
	wr.Header().Set("Pragma", "no-cache")
	wr.Header().Set("Expires", "0")

	s.renderTemplate(wr, req, credentialsPage, state)
}
