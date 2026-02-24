package web

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

	"go.mkw.re/ghidra-panel/common"
	"go.mkw.re/ghidra-panel/database"
	"go.mkw.re/ghidra-panel/ghidra"
	"google.golang.org/protobuf/types/known/emptypb"
)

type AdminDashboardState struct {
	*State
	Stats          *PanelStats
	RecentActivity []*AuditLogWithLocation
	Users          []*UserSummary
	GhidraUsers    []*GhidraServerUser
	Repositories   []*ghidra.Repository
	FilterAction   string
	FilterUser     string
}

// GhidraServerUser represents a user account direct from the Ghidra server
type GhidraServerUser struct {
	Username    string
	IsLinked    bool // True if this user exists in the panel database
	LinkedIdent string
}

// AuditLogWithLocation combines audit log entry with GeoIP lookup
type AuditLogWithLocation struct {
	*database.AuditLogEntry
	Location *GeoLocation
}

type PanelStats struct {
	TotalUsers      int
	TotalRepos      int
	TotalDiskUsage  int64
	ActiveSessions  int
	FailedLogins24h int
	TotalAuditLogs  int
	PanelUptime     time.Duration
	GeoIPType       string
	GeoIPBuildDate  time.Time
}

type UserSummary struct {
	UserID        uint64
	Username      string // Native Ghidra username (from passwords table)
	OAuthUsername string // Display name from OAuth provider
	Provider      string
	Providers     []string // List of all providers (primary first, then linked)
	ProfileURL    string
	CreatedAt     int64
	RepoCount     int
	LastLoginAt   *int64
	IsAdmin       bool
}

// Provider profile URL templates
var providerProfileURLs = map[string]string{
	"discord": "https://discord.com/users/%d",
	"github":  "https://github.com/%s",
	"gitlab":  "https://gitlab.com/%s",
}

// getProviderProfileURL generates a profile URL based on provider type
func getProviderProfileURL(provider string, userID uint64, username string) string {
	template, ok := providerProfileURLs[provider]
	if !ok || template == "" {
		return ""
	}

	switch provider {
	case "discord":
		return fmt.Sprintf(template, userID)
	case "github", "gitlab":
		if username != "" {
			return fmt.Sprintf(template, username)
		}
		return ""
	default:
		return ""
	}
}

func (s *Server) handleAdmin(wr http.ResponseWriter, req *http.Request) {
	state := &AdminDashboardState{
		State: s.stateWithNav(
			req,
			Nav{Route: "/", Name: "Ghidra"},
			Nav{Route: "/admin", Name: "Admin"},
		),
	}

	if !s.authenticateState(wr, req, state.State) {
		return
	}

	// If the user has disabled admin mode, redirect them to the home page
	if state.AdminModeDisabled {
		http.Redirect(wr, req, "/", http.StatusSeeOther)
		return
	}

	// Check if user is super admin
	if !s.isSuperAdmin(req.Context(), state.Identity) {
		http.Error(wr, "Forbidden", http.StatusForbidden)
		return
	}

	// Get filter parameters
	state.FilterAction = req.URL.Query().Get("action")
	state.FilterUser = req.URL.Query().Get("user")

	// Gather panel statistics and repositories
	stats, repos, err := s.getPanelStats(req)
	if err != nil {
		log.Println("Failed to get panel stats:", err)
		s.renderError(wr, req, http.StatusInternalServerError, "Failed to load statistics.", state.State)
		return
	}
	state.Stats = stats

	// Sort repositories alphabetically
	sort.Slice(repos, func(i, j int) bool {
		return lessCaseInsensitive(repos[i].Name, repos[j].Name)
	})
	state.Repositories = repos

	// Get recent activity (with optional filtering)
	activity, err := s.getRecentActivity(req, 50, state.FilterAction, state.FilterUser)
	if err != nil {
		log.Println("Failed to get recent activity:", err)
		s.renderError(wr, req, http.StatusInternalServerError, "Failed to load activity.", state.State)
		return
	}

	// Add GeoIP lookups to activity
	activityWithLocation := make([]*AuditLogWithLocation, len(activity))
	for i, log := range activity {
		activityWithLocation[i] = &AuditLogWithLocation{
			AuditLogEntry: log,
			Location:      s.LookupIPLocation(log.IPAddress),
		}
	}
	state.RecentActivity = activityWithLocation

	// Get user summaries
	users, err := s.getUserSummaries(req)
	if err != nil {
		log.Println("Failed to get user summaries:", err)
		s.renderError(wr, req, http.StatusInternalServerError, "Failed to load users.", state.State)
		return
	}
	state.Users = users

	// Get Ghidra Server users
	ghidraUsers, err := s.getGhidraServerUsers(req, users)
	if err != nil {
		log.Println("Failed to get Ghidra server users:", err)
		// Don't fail the whole page if this one grpc call fails, just log it
	} else {
		state.GhidraUsers = ghidraUsers
	}

	s.renderTemplate(wr, req, adminPage, state)
}

func (s *Server) getGhidraServerUsers(req *http.Request, panelUsers []*UserSummary) ([]*GhidraServerUser, error) {
	// Try to get all users from Ghidra server
	reply, err := s.Client.GetUsers(req.Context(), &emptypb.Empty{})
	if err != nil {
		return nil, err
	}

	// Create a lookup map of panel users (case-insensitive for safety)
	panelUserMap := make(map[string]*UserSummary)
	for _, pu := range panelUsers {
		if pu.Username != "" {
			panelUserMap[strings.ToLower(pu.Username)] = pu
		}
	}

	var ghidraUsers []*GhidraServerUser
	for _, gu := range reply.Users {
		pu := panelUserMap[strings.ToLower(gu.Username)]
		isLinked := pu != nil
		var linkedIdent string
		if isLinked {
			linkedIdent = fmt.Sprintf("%s/%d", pu.Provider, pu.UserID)
		}

		ghidraUsers = append(ghidraUsers, &GhidraServerUser{
			Username:    gu.Username,
			IsLinked:    isLinked,
			LinkedIdent: linkedIdent,
		})
	}

	// Sort alphabetically
	sort.Slice(ghidraUsers, func(i, j int) bool {
		return lessCaseInsensitive(ghidraUsers[i].Username, ghidraUsers[j].Username)
	})

	return ghidraUsers, nil
}

func (s *Server) getPanelStats(req *http.Request) (*PanelStats, []*ghidra.Repository, error) {
	stats := &PanelStats{
		PanelUptime: time.Since(s.StartTime),
	}

	// Total users
	count, err := s.DB.CountAccounts(req.Context())
	if err != nil {
		return nil, nil, err
	}
	stats.TotalUsers = count

	// Get repositories from Ghidra
	reply, err := s.Client.GetRepositories(req.Context(), &emptypb.Empty{})
	if err != nil {
		return nil, nil, err
	}
	stats.TotalRepos = len(reply.Repositories)

	// Calculate total disk usage from repository stats
	var totalSize int64
	for _, repo := range reply.Repositories {
		if repo.Stats != nil {
			totalSize += repo.Stats.SizeBytes
		}
	}
	stats.TotalDiskUsage = totalSize

	// Active sessions (unique logins in last 24 hours)
	since := time.Now().Add(-24 * time.Hour)
	activeSessions, err := s.DB.GetActiveSessionCount(req.Context(), since)
	if err != nil {
		return nil, nil, err
	}
	stats.ActiveSessions = activeSessions

	// Failed logins in last 24 hours
	failedLogins, err := s.DB.GetFailedLoginCount(req.Context(), since)
	if err != nil {
		return nil, nil, err
	}
	stats.FailedLogins24h = failedLogins

	// Total audit logs (approximate - could be expensive on large DBs)
	var totalLogs int
	err = s.DB.QueryRowContext(req.Context(), "SELECT COUNT(*) FROM audit_logs").Scan(&totalLogs)
	if err != nil && err != sql.ErrNoRows {
		return nil, nil, err
	}
	stats.TotalAuditLogs = totalLogs

	// GeoIP metadata
	if geoMeta := s.GetGeoIPMetadata(); geoMeta != nil {
		stats.GeoIPType = geoMeta.Type
		stats.GeoIPBuildDate = geoMeta.BuildDate
	}

	return stats, reply.Repositories, nil
}

func (s *Server) getRecentActivity(req *http.Request, limit int, filterAction, filterUser string) ([]*database.AuditLogEntry, error) {
	// Apply filters if specified
	if filterAction != "" {
		return s.DB.GetAuditLogsByAction(req.Context(), filterAction, limit)
	}
	// Note: filtering by user would require a different query or joining with passwords table
	// For now, just get all recent activity
	return s.DB.GetAuditLogs(req.Context(), limit, 0)
}

func (s *Server) getUserSummaries(req *http.Request) ([]*UserSummary, error) {
	// Get all users from database with OAuth identity info
	rows, err := s.DB.QueryContext(req.Context(),
		`SELECT 
			COALESCE(p.id, o.id) as uid, 
			COALESCE(p.username, '') as ghidra_username, 
			COALESCE(p.provider, o.provider) as auth_provider, 
			COALESCE(strftime('%s', p.updated_at)*1000, strftime('%s', o.first_login)*1000) as created_at, 
			COALESCE(o.username, '') as oauth_username
		 FROM passwords p
		 LEFT JOIN oauth_identities o ON p.id = o.id AND p.provider = o.provider
		 UNION
		 SELECT 
			id as uid, 
			'' as ghidra_username, 
			provider as auth_provider, 
			strftime('%s', first_login)*1000 as created_at, 
			COALESCE(username, '') as oauth_username
		 FROM oauth_identities
		 WHERE NOT EXISTS (SELECT 1 FROM passwords WHERE passwords.id = oauth_identities.id AND passwords.provider = oauth_identities.provider)
		 ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*UserSummary
	for rows.Next() {
		var user UserSummary
		var oauthUsername string
		err := rows.Scan(&user.UserID, &user.Username, &user.Provider, &user.CreatedAt, &oauthUsername)
		if err != nil {
			return nil, err
		}

		user.OAuthUsername = oauthUsername

		// Compute Admin status dynamically (checks both database and config.yaml)
		user.IsAdmin = s.isSuperAdmin(req.Context(), &common.Identity{
			ID:       user.UserID,
			Provider: user.Provider,
			Username: oauthUsername,
		})

		// Generate profile URL based on provider
		user.ProfileURL = getProviderProfileURL(user.Provider, user.UserID, oauthUsername)

		// Get last login time
		lastLogin, err := s.DB.GetLastLoginByUser(req.Context(), user.UserID)
		if err != nil {
			log.Printf("Warning: Failed to get last login for user %s: %v", user.Username, err)
		}
		user.LastLoginAt = lastLogin

		// Get linked providers
		user.Providers = []string{user.Provider} // Primary first
		if linkedAccounts, err := s.DB.GetLinkedAccounts(req.Context(), user.UserID, user.Provider); err == nil {
			for _, linked := range linkedAccounts {
				user.Providers = append(user.Providers, linked.Provider)
			}
		}

		users = append(users, &user)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	// Get repository counts for each user
	reply, err := s.Client.GetRepositories(req.Context(), &emptypb.Empty{})
	if err != nil {
		return nil, err
	}

	// Count repos per user
	repoCounts := make(map[string]int)
	for _, repo := range reply.Repositories {
		for _, u := range repo.Users {
			if u.Permission != ghidra.Permission_NONE {
				repoCounts[strings.ToLower(u.User.Username)]++
			}
		}
	}

	for _, user := range users {
		user.RepoCount = repoCounts[strings.ToLower(user.Username)]
	}

	// Sort by username
	sort.Slice(users, func(i, j int) bool {
		return lessCaseInsensitive(users[i].Username, users[j].Username)
	})

	return users, nil
}

func (s *Server) handleAdminDeleteUser(wr http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(wr, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ident, ok := s.checkAuth(req)
	if !ok {
		http.Error(wr, "Not authorized", http.StatusUnauthorized)
		return
	}

	if !s.isSuperAdmin(req.Context(), ident) {
		http.Error(wr, "Forbidden", http.StatusForbidden)
		return
	}

	if err := req.ParseForm(); err != nil {
		http.Error(wr, "Invalid form", http.StatusBadRequest)
		return
	}

	targetIDStr := req.FormValue("user_id")
	targetProvider := req.FormValue("provider")
	targetUsername := req.FormValue("username")

	var targetID uint64
	_, err := fmt.Sscanf(targetIDStr, "%d", &targetID)
	if err != nil || targetProvider == "" { // Tolerate empty targetUsername for OAuth-only users
		http.Error(wr, "Invalid parameters", http.StatusBadRequest)
		return
	}

	if targetID == ident.ID && targetProvider == ident.Provider {
		http.Error(wr, "Cannot delete your own account from the admin panel", http.StatusConflict)
		return
	}

	auditTarget := targetUsername
	if auditTarget == "" {
		auditTarget = fmt.Sprintf("%s/%d", targetProvider, targetID)
	}

	log.Printf("Admin %s (%s/%d) initiated account deletion for user %s (%s/%d)", ident.Username, ident.Provider, ident.ID, targetUsername, targetProvider, targetID)

	// 1. Delete from database
	if err := s.DB.DeleteAccount(req.Context(), targetID, targetProvider); err != nil {
		log.Println("Failed to delete account from database:", err)
		s.logAuditSimple(req.Context(), &ident.ID, ident.Username, "admin_delete_user_failed", "user", auditTarget, getClientIP(req), req.UserAgent(), false)
		http.Redirect(wr, req, "/admin?status=internal_error", http.StatusSeeOther)
		return
	}

	// 2. Remove from Ghidra server via gRPC
	if targetUsername != "" {
		if _, err := s.Client.RemoveUser(req.Context(), &ghidra.RemoveUserRequest{Username: targetUsername}); err != nil {
			log.Printf("Warning: Failed to completely remove user %s from Ghidra server (may need manual cleanup): %v", targetUsername, err)
		}
	}

	// 3. Log the audit event
	s.logAuditSimple(req.Context(), &ident.ID, ident.Username, "admin_deleted_user", "user", auditTarget, getClientIP(req), req.UserAgent(), true)

	http.Redirect(wr, req, "/admin", http.StatusSeeOther)
}

func (s *Server) handleAdminDeleteGhidraUser(wr http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(wr, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ident, ok := s.checkAuth(req)
	if !ok {
		http.Error(wr, "Not authorized", http.StatusUnauthorized)
		return
	}

	if !s.isSuperAdmin(req.Context(), ident) {
		http.Error(wr, "Forbidden", http.StatusForbidden)
		return
	}

	if err := req.ParseForm(); err != nil {
		http.Error(wr, "Invalid form", http.StatusBadRequest)
		return
	}

	targetUsername := req.FormValue("username")
	if targetUsername == "" {
		http.Error(wr, "Invalid parameters", http.StatusBadRequest)
		return
	}

	// Double-check they aren't trying to delete themselves in Ghidra if somehow they got orphaned
	if strings.EqualFold(targetUsername, ident.Username) {
		http.Error(wr, "Cannot delete your own Ghidra account", http.StatusConflict)
		return
	}

	log.Printf("Admin %s (%s/%d) initiated Ghidra server account deletion for orphaned user %s", ident.Username, ident.Provider, ident.ID, targetUsername)

	// Remove from Ghidra server via gRPC
	if _, err := s.Client.RemoveUser(req.Context(), &ghidra.RemoveUserRequest{Username: targetUsername}); err != nil {
		log.Printf("Failed to completely remove user %s from Ghidra server: %v", targetUsername, err)
		s.renderError(wr, req, http.StatusInternalServerError, "Failed to remove user from Ghidra server.", s.stateWithNav(req))
		return
	}

	// Log the audit event
	s.logAuditSimple(req.Context(), &ident.ID, ident.Username, "admin_deleted_ghidra_user", "ghidra_user", targetUsername, getClientIP(req), req.UserAgent(), true)

	http.Redirect(wr, req, "/admin", http.StatusSeeOther)
}

func (s *Server) handleAdminBulkUserAction(wr http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(wr, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ident, ok := s.checkAuth(req)
	if !ok || !s.isSuperAdmin(req.Context(), ident) {
		http.Error(wr, "Forbidden", http.StatusForbidden)
		return
	}

	if err := req.ParseForm(); err != nil {
		http.Error(wr, "Invalid form", http.StatusBadRequest)
		return
	}

	action := req.FormValue("action")
	selectedUsers := req.Form["selected_users"]

	if action == "delete" {
		for _, userStr := range selectedUsers {
			parts := strings.Split(userStr, "|")
			if len(parts) != 3 {
				continue
			}

			var targetID uint64
			fmt.Sscanf(parts[0], "%d", &targetID)
			targetProvider := parts[1]
			targetUsername := parts[2]

			if targetID == ident.ID && targetProvider == ident.Provider {
				continue // Can't delete self
			}

			auditTarget := targetUsername
			if auditTarget == "" {
				auditTarget = fmt.Sprintf("%s/%d", targetProvider, targetID)
			}

			log.Printf("Admin %s initiated bulk deletion for %s", ident.Username, auditTarget)
			if err := s.DB.DeleteAccount(req.Context(), targetID, targetProvider); err == nil {
				if targetUsername != "" {
					s.Client.RemoveUser(req.Context(), &ghidra.RemoveUserRequest{Username: targetUsername})
				}
				s.logAuditSimple(req.Context(), &ident.ID, ident.Username, "admin_deleted_user_bulk", "user", auditTarget, getClientIP(req), req.UserAgent(), false)
			} else {
				log.Printf("Bulk delete failed for %s: %v", auditTarget, err)
				s.logAuditSimple(req.Context(), &ident.ID, ident.Username, "admin_delete_user_failed", "user", auditTarget, getClientIP(req), req.UserAgent(), false)
			}
		}
	}

	http.Redirect(wr, req, "/admin", http.StatusSeeOther)
}

func (s *Server) handleAdminBulkGhidraUserAction(wr http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(wr, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ident, ok := s.checkAuth(req)
	if !ok || !s.isSuperAdmin(req.Context(), ident) {
		http.Error(wr, "Forbidden", http.StatusForbidden)
		return
	}

	if err := req.ParseForm(); err != nil {
		http.Error(wr, "Invalid form", http.StatusBadRequest)
		return
	}

	action := req.FormValue("action")
	selectedUsers := req.Form["selected_ghidra_users"]

	if action == "delete" {
		for _, targetUsername := range selectedUsers {
			if strings.EqualFold(targetUsername, ident.Username) {
				continue // Can't delete self
			}

			log.Printf("Admin %s initiated bulk deletion for Ghidra user %s", ident.Username, targetUsername)
			if _, err := s.Client.RemoveUser(req.Context(), &ghidra.RemoveUserRequest{Username: targetUsername}); err == nil {
				s.logAuditSimple(req.Context(), &ident.ID, ident.Username, "admin_deleted_ghidra_user_bulk", "ghidra_user", targetUsername, getClientIP(req), req.UserAgent(), false)
			}
		}
	}

	http.Redirect(wr, req, "/admin", http.StatusSeeOther)
}

func (s *Server) handleAdminDeleteRepository(wr http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(wr, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ident, ok := s.checkAuth(req)
	if !ok {
		http.Error(wr, "Not authorized", http.StatusUnauthorized)
		return
	}

	if !s.isSuperAdmin(req.Context(), ident) {
		http.Error(wr, "Forbidden", http.StatusForbidden)
		return
	}

	if err := req.ParseForm(); err != nil {
		http.Error(wr, "Invalid form", http.StatusBadRequest)
		return
	}

	repoName := req.FormValue("repository")
	if repoName == "" {
		http.Error(wr, "Invalid parameters", http.StatusBadRequest)
		return
	}

	log.Printf("Admin %s initiated Ghidra repository deletion for %s", ident.Username, repoName)

	if _, err := s.Client.DeleteRepository(req.Context(), &ghidra.DeleteRepositoryRequest{Repository: repoName}); err != nil {
		log.Printf("Failed to completely remove repository %s from Ghidra server: %v", repoName, err)
		s.renderError(wr, req, http.StatusInternalServerError, "Failed to remove repository from Ghidra server.", s.stateWithNav(req))
		return
	}

	s.logAuditSimple(req.Context(), &ident.ID, ident.Username, "admin_deleted_repository", "repository", repoName, getClientIP(req), req.UserAgent(), true)

	http.Redirect(wr, req, "/admin", http.StatusSeeOther)
}

func (s *Server) handleAdminBulkRepositoryAction(wr http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(wr, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ident, ok := s.checkAuth(req)
	if !ok || !s.isSuperAdmin(req.Context(), ident) {
		http.Error(wr, "Forbidden", http.StatusForbidden)
		return
	}

	if err := req.ParseForm(); err != nil {
		http.Error(wr, "Invalid form", http.StatusBadRequest)
		return
	}

	action := req.FormValue("action")
	selectedRepos := req.Form["selected_repositories"]

	if action == "delete" {
		for _, repoName := range selectedRepos {
			log.Printf("Admin %s initiated bulk deletion for Ghidra repository %s", ident.Username, repoName)
			if _, err := s.Client.DeleteRepository(req.Context(), &ghidra.DeleteRepositoryRequest{Repository: repoName}); err == nil {
				s.logAuditSimple(req.Context(), &ident.ID, ident.Username, "admin_deleted_repository_bulk", "repository", repoName, getClientIP(req), req.UserAgent(), false)
			}
		}
	}

	http.Redirect(wr, req, "/admin", http.StatusSeeOther)
}
