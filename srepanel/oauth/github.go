package oauth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"go.mkw.re/ghidra-panel/common"
	"go.mkw.re/ghidra-panel/csrf"
	"golang.org/x/oauth2"
)

var githubEndpoint = oauth2.Endpoint{
	AuthURL:  "https://github.com/login/oauth/authorize",
	TokenURL: "https://github.com/login/oauth/access_token",
}

type GitHubProvider struct {
	config oauth2.Config
	prot   *csrf.OneTime
}

func NewGitHubProvider(clientID, clientSecret, redirectURL string) *GitHubProvider {
	return &GitHubProvider{
		config: oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			Endpoint:     githubEndpoint,
			RedirectURL:  redirectURL,
			Scopes:       []string{"read:user"},
		},
		prot: csrf.NewOneTime(),
	}
}

func (p *GitHubProvider) Name() string {
	return "github"
}

func (p *GitHubProvider) AuthURL() string {
	return p.config.AuthCodeURL(p.prot.Issue(), oauth2.AccessTypeOnline)
}

func (p *GitHubProvider) HandleRedirect(wr http.ResponseWriter, req *http.Request) (*common.Identity, error) {
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

	// Check CSRF token validity -- do not consume yet
	csrfID, err := p.prot.Check(state)
	if err != nil {
		return nil, err
	}

	// Request authorization token from GitHub
	token, err := p.config.Exchange(ctx, code)
	if err != nil {
		return nil, err
	}

	// Ask GitHub for user info associated with token
	ident, err := p.getGitHubIdentity(ctx, token)
	if err != nil {
		return nil, err
	}

	// Prevent CSRF token reuse
	err = p.prot.Consume(csrfID)
	return ident, err
}

func (p *GitHubProvider) getGitHubIdentity(ctx context.Context, token *oauth2.Token) (*common.Identity, error) {
	userReq, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/user", nil)
	if err != nil {
		return nil, err
	}

	res, err := p.config.Client(ctx, token).Do(userReq)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	var user struct {
		ID        int64  `json:"id"`
		Login     string `json:"login"`
		AvatarURL string `json:"avatar_url"`
	}
	if err := json.NewDecoder(res.Body).Decode(&user); err != nil {
		return nil, err
	}

	if user.ID == 0 || user.Login == "" {
		return nil, errors.New("invalid response from GitHub")
	}

	return &common.Identity{
		ID:         uint64(user.ID),
		Username:   user.Login,
		AvatarHash: user.AvatarURL,
		Provider:   p.Name(),
	}, nil
}
