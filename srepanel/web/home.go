package web

import (
	"log"
	"net/http"
	"sort"
	"strings"

	"go.mkw.re/ghidra-panel/common"
	"go.mkw.re/ghidra-panel/database"
	"go.mkw.re/ghidra-panel/ghidra"
	"google.golang.org/protobuf/types/known/emptypb"
)

type HomeState struct {
	*State
	ACL                []common.UserRepoAccessDisplay
	GhidraUsername     string
	GhidraVersion      string
	Repos              []string
	LinkedAccounts     []database.LinkedAccount
	AvailableProviders []*common.ProviderMetadata
}

func (s *Server) handleHome(wr http.ResponseWriter, req *http.Request) {
	if req.URL.Path != "/" {
		http.NotFound(wr, req)
		return
	}

	state := HomeState{State: s.stateWithNav(req, Nav{Route: "/", Name: "Ghidra"})}
	if !s.authenticateState(wr, req, state.State) {
		return
	}
	// If admin mode is disabled, treat as regular user for repository listing
	viewAsAdmin := state.SuperAdmin && !state.AdminModeDisabled

	if s.Config.Dev {
		log.Printf("Home page: User %s (ID: %d, Provider: %s) - SuperAdmin: %v, ViewAsAdmin: %v",
			state.Identity.Username, state.Identity.ID, state.Identity.Provider, state.SuperAdmin, viewAsAdmin)
	}

	// Fetch linked accounts
	linkedAccounts, err := s.DB.GetLinkedAccounts(req.Context(), state.Identity.ID, state.Identity.Provider)
	if err != nil {
		log.Println("Warning: Failed to fetch linked accounts:", err)
	}
	state.LinkedAccounts = linkedAccounts

	// Determine available providers to link
	linkedMap := make(map[string]bool)
	linkedMap[state.Identity.Provider] = true // Don't allow linking the primary provider
	for _, l := range linkedAccounts {
		linkedMap[l.Provider] = true
	}

	for name := range s.Providers {
		if !linkedMap[name] {
			metadata := s.ProviderMetadata[name]
			if metadata == nil {
				metadata = &common.ProviderMetadata{
					Name:        name,
					DisplayName: name,
				}
			}
			state.AvailableProviders = append(state.AvailableProviders, metadata)
		}
	}
	// Sort available providers
	sort.Slice(state.AvailableProviders, func(i, j int) bool {
		return state.AvailableProviders[i].Name < state.AvailableProviders[j].Name
	})

	// Fetch repository and user information from Ghidra
	reply, err := s.Client.GetRepositories(req.Context(), &emptypb.Empty{})
	if err != nil {
		// Log the error but continue - allow access to panel features without Ghidra server
		log.Println("Warning: Failed to fetch repositories (Ghidra server may be offline):", err)
		state.GhidraVersion = "Unavailable (Ghidra server offline)"
		// Render home page with empty repository list
		// Render home page with empty repository list
		s.renderTemplate(wr, req, homePage, state)
		return
	}

	// Store Ghidra version
	state.GhidraVersion = reply.Version.GhidraVersion

	// Check if there's a matching legacy Ghidra account
	for _, u := range reply.Users {
		if strings.EqualFold(u.Username, state.UserState.Username) {
			state.GhidraUsername = u.Username
			break
		}
	}

	// Query for repository access
	state.ACL = make([]common.UserRepoAccessDisplay, 0)
	for _, r := range reply.Repositories {
		acl := common.UserRepoAccessDisplay{
			Repo:    r.Name,
			Perm:    ghidra.Permission_NONE,
			IsAdmin: viewAsAdmin,
		}
		// Copy stats if available
		if r.Stats != nil {
			acl.Stats = &common.RepositoryStats{
				SizeBytes:        r.Stats.SizeBytes,
				FileCount:        r.Stats.FileCount,
				UserCount:        r.Stats.UserCount,
				CreatedTime:      r.Stats.CreatedTime,
				LastModifiedTime: r.Stats.LastModifiedTime,
			}
		}
		for _, u := range r.Users {
			if strings.EqualFold(u.User.Username, state.UserState.Username) {
				acl.Perm = u.Permission
				if u.Permission == ghidra.Permission_ADMIN {
					acl.IsAdmin = true
				}
				break
			}
		}
		// If viewing as admin, show all repos. Otherwise only show repos with access
		if viewAsAdmin || acl.Perm != ghidra.Permission_NONE {
			state.ACL = append(state.ACL, acl)
		}
	}
	sort.Slice(state.ACL, func(i, j int) bool { return lessCaseInsensitive(state.ACL[i].Repo, state.ACL[j].Repo) })

	// Query for repository list
	state.Repos = make([]string, len(reply.Repositories))
	for i, v := range reply.Repositories {
		state.Repos[i] = v.Name
	}
	sort.Slice(state.Repos, func(i, j int) bool { return lessCaseInsensitive(state.Repos[i], state.Repos[j]) })

	s.renderTemplate(wr, req, homePage, state)
}
