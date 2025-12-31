package web

import (
	"context"
	"go.mkw.re/ghidra-panel/common"
)

// isSuperAdmin checks if a user is a super admin
// Checks config-based super_admins, panel_admins table, and legacy is_super_admin column
func (s *Server) isSuperAdmin(ctx context.Context, ident *common.Identity) bool {
	// Check config-based super admins (highest priority)
	for _, id := range s.Config.SuperAdmins {
		if id == ident.ID {
			return true
		}
	}

	// Check panel_admins table (new method - works without Ghidra account)
	isAdmin, err := s.DB.IsPanelAdmin(ctx, ident.ID, ident.Provider)
	if err == nil && isAdmin {
		return true
	}

	// Check legacy is_super_admin in passwords table (for backward compatibility)
	isAdmin, err = s.DB.IsSuperAdmin(ctx, ident.ID, ident.Provider)
	if err == nil && isAdmin {
		return true
	}

	return false
}
