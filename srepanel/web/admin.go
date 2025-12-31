package web

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

	"go.mkw.re/ghidra-panel/database"
	"go.mkw.re/ghidra-panel/ghidra"
	"google.golang.org/protobuf/types/known/emptypb"
)

type AdminDashboardState struct {
	*State
	Stats          *PanelStats
	RecentActivity []*AuditLogWithLocation
	Users          []*UserSummary
	FilterAction   string
	FilterUser     string
}

// AuditLogWithLocation combines audit log entry with GeoIP lookup
type AuditLogWithLocation struct {
	*database.AuditLogEntry
	Location *GeoLocation
}

type PanelStats struct {
	TotalUsers       int
	TotalRepos       int
	TotalDiskUsage   int64
	ActiveSessions   int
	FailedLogins24h  int
	TotalAuditLogs   int
	PanelUptime      time.Duration
}

type UserSummary struct {
	UserID      uint64
	Username    string
	Provider    string
	ProfileURL  string
	CreatedAt   int64
	RepoCount   int
	LastLoginAt *int64
}

// Provider profile URL templates
var providerProfileURLs = map[string]string{
	"discord": "https://discord.com/users/%d",
	"github":  "https://github.com/%s",
	"gitlab":  "https://gitlab.com/%s",
}

// getProviderProfileURL generates a profile URL based on provider type
func getProviderProfileURL(provider string, userID uint64, username string) string{
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

	// Check if user is super admin
	if !s.isSuperAdmin(req.Context(), state.Identity) {
		http.Error(wr, "Forbidden", http.StatusForbidden)
		return
	}

	// Get filter parameters
	state.FilterAction = req.URL.Query().Get("action")
	state.FilterUser = req.URL.Query().Get("user")

	// Gather panel statistics
	stats, err := s.getPanelStats(req)
	if err != nil {
		log.Println("Failed to get panel stats:", err)
		s.renderError(wr, http.StatusInternalServerError, "Failed to load statistics.", state.State)
		return
	}
	state.Stats = stats

	// Get recent activity (with optional filtering)
	activity, err := s.getRecentActivity(req, 50, state.FilterAction, state.FilterUser)
	if err != nil {
		log.Println("Failed to get recent activity:", err)
		s.renderError(wr, http.StatusInternalServerError, "Failed to load activity.", state.State)
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
		s.renderError(wr, http.StatusInternalServerError, "Failed to load users.", state.State)
		return
	}
	state.Users = users

	err = adminPage.Execute(wr, state)
	if err != nil {
		log.Println("Failed to render admin page:", err)
		s.renderError(wr, http.StatusInternalServerError, "Failed to render page.", state.State)
	}
}

func (s *Server) getPanelStats(req *http.Request) (*PanelStats, error) {
	stats := &PanelStats{
		PanelUptime: time.Since(s.StartTime),
	}

	// Total users
	count, err := s.DB.CountAccounts(req.Context())
	if err != nil {
		return nil, err
	}
	stats.TotalUsers = count

	// Get repositories from Ghidra
	reply, err := s.Client.GetRepositories(req.Context(), &emptypb.Empty{})
	if err != nil {
		return nil, err
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
		return nil, err
	}
	stats.ActiveSessions = activeSessions

	// Failed logins in last 24 hours
	failedLogins, err := s.DB.GetFailedLoginCount(req.Context(), since)
	if err != nil {
		return nil, err
	}
	stats.FailedLogins24h = failedLogins

	// Total audit logs (approximate - could be expensive on large DBs)
	var totalLogs int
	err = s.DB.QueryRowContext(req.Context(), "SELECT COUNT(*) FROM audit_logs").Scan(&totalLogs)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	stats.TotalAuditLogs = totalLogs

	return stats, nil
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
		`SELECT p.id, p.username, p.provider, strftime('%s', p.updated_at) * 1000, COALESCE(o.username, '')
		 FROM passwords p
		 LEFT JOIN oauth_identities o ON p.id = o.id AND p.provider = o.provider
		 ORDER BY p.updated_at DESC`)
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
		
		// Generate profile URL based on provider
		user.ProfileURL = getProviderProfileURL(user.Provider, user.UserID, oauthUsername)

		// Get last login time
		lastLogin, err := s.DB.GetLastLoginByUser(req.Context(), user.UserID)
		if err != nil {
			log.Printf("Warning: Failed to get last login for user %s: %v", user.Username, err)
		}
		user.LastLoginAt = lastLogin

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
