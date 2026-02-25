package oauth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"go.mkw.re/ghidra-panel/common"
	"go.mkw.re/ghidra-panel/csrf"
	"golang.org/x/oauth2"
)

// OAuth2Config contains endpoints and field mappings for a generic OAuth2 provider
type OAuth2Config struct {
	Name           string   // Provider name (e.g., "discord", "github")
	AuthURL        string   // Authorization endpoint
	TokenURL       string   // Token endpoint
	UserInfoURL    string   // User info endpoint
	Scopes         []string // OAuth scopes to request
	AuthStyle      string   // "params" or "header" (default: "header")
	UserIDField    string   // JSON field for user ID (default: "id")
	UsernameField  string   // JSON field for username (default: "username")
	AvatarField    string   // JSON field for avatar (optional)
	UserIDIsString bool     // Whether user ID is a string that needs parsing
}

// GenericOAuth2Provider implements a configurable OAuth2 provider
type GenericOAuth2Provider struct {
	name     string
	config   oauth2.Config
	userInfo OAuth2Config
	prot     *csrf.OneTime
}

// NewGenericOAuth2Provider creates a new generic OAuth2 provider
func NewGenericOAuth2Provider(
	clientID, clientSecret, redirectURL string,
	providerConfig OAuth2Config,
) *GenericOAuth2Provider {
	authStyle := oauth2.AuthStyleAutoDetect
	if providerConfig.AuthStyle == "params" {
		authStyle = oauth2.AuthStyleInParams
	} else if providerConfig.AuthStyle == "header" {
		authStyle = oauth2.AuthStyleInHeader
	}

	// Set defaults
	if providerConfig.UserIDField == "" {
		providerConfig.UserIDField = "id"
	}
	if providerConfig.UsernameField == "" {
		providerConfig.UsernameField = "username"
	}

	return &GenericOAuth2Provider{
		name: providerConfig.Name,
		config: oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			Endpoint: oauth2.Endpoint{
				AuthURL:   providerConfig.AuthURL,
				TokenURL:  providerConfig.TokenURL,
				AuthStyle: authStyle,
			},
			RedirectURL: redirectURL,
			Scopes:      providerConfig.Scopes,
		},
		userInfo: providerConfig,
		prot:     csrf.NewOneTime(),
	}
}

func (p *GenericOAuth2Provider) Name() string {
	return p.name
}

func (p *GenericOAuth2Provider) AuthURL() string {
	return p.config.AuthCodeURL(p.prot.Issue(), oauth2.AccessTypeOnline)
}

func (p *GenericOAuth2Provider) HandleRedirect(wr http.ResponseWriter, req *http.Request) (*common.Identity, error) {
	ctx := req.Context()

	errID := req.FormValue("error")
	errDescription := req.FormValue("error_description")
	if errID != "" {
		if errID == "access_denied" {
			http.Redirect(wr, req, "/login", http.StatusSeeOther)
			return nil, nil
		}
		http.Error(wr, errDescription, http.StatusUnauthorized)
		return nil, nil
	}

	query := req.URL.Query()
	code := query.Get("code")
	state := query.Get("state")

	// Check CSRF token validity
	csrfID, err := p.prot.Check(state)
	if err != nil {
		return nil, err
	}

	// Exchange code for token
	token, err := p.config.Exchange(ctx, code)
	if err != nil {
		return nil, err
	}

	// Get user identity
	ident, err := p.getUserIdentity(ctx, token)
	if err != nil {
		return nil, err
	}

	// Prevent CSRF token reuse
	err = p.prot.Consume(csrfID)
	return ident, err
}

func (p *GenericOAuth2Provider) getUserIdentity(ctx context.Context, token *oauth2.Token) (*common.Identity, error) {
	userReq, err := http.NewRequestWithContext(ctx, http.MethodGet, p.userInfo.UserInfoURL, nil)
	if err != nil {
		return nil, err
	}

	res, err := p.config.Client(ctx, token).Do(userReq)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	// Decode response into generic map
	var data map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&data); err != nil {
		return nil, err
	}

	// Extract user ID
	var userID uint64
	if p.userInfo.UserIDIsString {
		// ID is a string (e.g., Discord)
		idStr, ok := getNestedString(data, p.userInfo.UserIDField)
		if !ok {
			return nil, fmt.Errorf("user ID field '%s' not found or not a string", p.userInfo.UserIDField)
		}
		userID, err = strconv.ParseUint(idStr, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("failed to parse user ID: %w", err)
		}
	} else {
		// ID is a number
		idFloat, ok := getNestedFloat(data, p.userInfo.UserIDField)
		if !ok {
			return nil, fmt.Errorf("user ID field '%s' not found or not a number", p.userInfo.UserIDField)
		}
		userID = uint64(idFloat)
	}

	// Extract username
	username, ok := getNestedString(data, p.userInfo.UsernameField)
	if !ok {
		return nil, fmt.Errorf("username field '%s' not found", p.userInfo.UsernameField)
	}

	if userID == 0 || username == "" {
		return nil, errors.New("invalid user info response")
	}

	// Extract avatar (optional)
	avatar := ""
	if p.userInfo.AvatarField != "" {
		avatar, _ = getNestedString(data, p.userInfo.AvatarField)
	}

	return &common.Identity{
		ID:         userID,
		Username:   username,
		AvatarHash: avatar,
		Provider:   p.Name(),
	}, nil
}

// Helper to get nested string value from map (supports "user.username" syntax)
func getNestedString(data map[string]interface{}, field string) (string, bool) {
	val := getNestedValue(data, field)
	if val == nil {
		return "", false
	}
	str, ok := val.(string)
	return str, ok
}

// Helper to get nested float value from map
func getNestedFloat(data map[string]interface{}, field string) (float64, bool) {
	val := getNestedValue(data, field)
	if val == nil {
		return 0, false
	}
	// JSON numbers are decoded as float64
	num, ok := val.(float64)
	return num, ok
}

// Helper to get nested value from map (supports "user.id" syntax)
func getNestedValue(data map[string]interface{}, field string) interface{} {
	// Split field by dots
	parts := splitField(field)

	// Traverse nested maps
	current := interface{}(data)
	for _, part := range parts {
		if currentMap, ok := current.(map[string]interface{}); ok {
			if val, ok := currentMap[part]; ok {
				current = val
			} else {
				return nil
			}
		} else {
			return nil
		}
	}

	return current
}

func splitField(field string) []string {
	var parts []string
	current := ""

	for i := 0; i < len(field); i++ {
		if field[i] == '.' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(field[i])
		}
	}

	if current != "" {
		parts = append(parts, current)
	}

	return parts
}
