package web

import (
	"context"
	"net/http"
	"strings"

	"go.mkw.re/ghidra-panel/common"
	"go.mkw.re/ghidra-panel/database"
)

// Helper to extract client IP from request
func getClientIP(req *http.Request) string {
	// Check X-Forwarded-For header first (if behind proxy)
	forwarded := req.Header.Get("X-Forwarded-For")
	if forwarded != "" {
		// X-Forwarded-For can contain multiple IPs, take the first one
		ips := strings.Split(forwarded, ",")
		return strings.TrimSpace(ips[0])
	}

	// Check X-Real-IP header
	realIP := req.Header.Get("X-Real-IP")
	if realIP != "" {
		return realIP
	}

	// Fall back to RemoteAddr
	ip := req.RemoteAddr
	// Remove port if present
	if idx := strings.LastIndex(ip, ":"); idx != -1 {
		ip = ip[:idx]
	}
	return ip
}

// Helper to log audit events with context from request
func (s *Server) logAudit(ctx context.Context, req *http.Request, action string, resourceType, resourceName string, success bool, details map[string]interface{}) {
	var userID *uint64
	var username string

	// Try to get identity from context
	if ident, ok := ctx.Value("identity").(*common.Identity); ok && ident != nil {
		userID = &ident.ID
		username = ident.Username
	}

	// Get user state if available
	if state, ok := ctx.Value("userState").(*common.UserState); ok && state != nil && username == "" {
		username = state.Username
	}

	ipAddress := getClientIP(req)
	userAgent := req.UserAgent()

	err := s.DB.CreateAuditLogWithDetails(
		ctx, userID, username, action, resourceType, resourceName,
		ipAddress, userAgent, success, details,
	)
	if err != nil {
		// Log error but don't fail the request
		s.Logger.Error("Failed to create audit log", "error", err)
	}
}

// Simplified audit log for cases where we already have the user info
func (s *Server) logAuditSimple(ctx context.Context, userID *uint64, username, action, resourceType, resourceName, ipAddress, userAgent string, success bool) {
	err := s.DB.CreateAuditLog(ctx, &database.AuditLogEntry{
		UserID:       userID,
		Username:     username,
		Action:       action,
		ResourceType: resourceType,
		ResourceName: resourceName,
		IPAddress:    ipAddress,
		UserAgent:    userAgent,
		Success:      success,
	})
	if err != nil {
		s.Logger.Error("Failed to create audit log", "error", err)
	}
}
