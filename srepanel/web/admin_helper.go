package web

import (
	"context"
	"fmt"

	"go.mkw.re/ghidra-panel/common"
)

// isSuperAdmin checks if a user is a super admin
// Checks config-based super_admins, panel_admins table, and legacy is_super_admin column
func (s *Server) isSuperAdmin(ctx context.Context, ident *common.Identity) bool {
	// Check config-based super admins (highest priority)
	// Format is now "provider:id" (e.g. "github:1234") or just "1234" for legacy Discord fallback
	identStr := fmt.Sprintf("%s:%d", ident.Provider, ident.ID)
	for _, idStr := range s.Config.SuperAdmins {
		if idStr == identStr {
			return true
		}
		// Legacy support: if the user just put a naked ID, assume it's for their current provider
		// (this retains backwards compatibility while allowing exact matching going forward)
		if idStr == fmt.Sprintf("%d", ident.ID) {
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
