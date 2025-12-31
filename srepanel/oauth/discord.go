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

var discordEndpoint = oauth2.Endpoint{
	AuthURL:   "https://discord.com/oauth2/authorize",
	TokenURL:  "https://discord.com/api/oauth2/token",
	AuthStyle: oauth2.AuthStyleInParams,
}

type DiscordProvider struct {
	config oauth2.Config
	prot   *csrf.OneTime
}

func NewDiscordProvider(clientID, clientSecret, redirectURL string) *DiscordProvider {
	return &DiscordProvider{
		config: oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			Endpoint:     discordEndpoint,
			RedirectURL:  redirectURL,
			Scopes:       []string{"identify"},
		},
		prot: csrf.NewOneTime(),
	}
}

func (p *DiscordProvider) Name() string {
	return "discord"
}

func (p *DiscordProvider) AuthURL() string {
	return p.config.AuthCodeURL(p.prot.Issue(), oauth2.AccessTypeOnline)
}

func (p *DiscordProvider) HandleRedirect(wr http.ResponseWriter, req *http.Request) (*common.Identity, error) {
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

	// Request authorization token from Discord
	token, err := p.config.Exchange(ctx, code)
	if err != nil {
		return nil, err
	}

	// Ask Discord for user ID/username associated with token
	ident, err := p.getDiscordIdentity(ctx, token)
	if err != nil {
		return nil, err
	}

	// Prevent CSRF token reuse
	err = p.prot.Consume(csrfID)
	return ident, err
}

func (p *DiscordProvider) getDiscordIdentity(ctx context.Context, token *oauth2.Token) (*common.Identity, error) {
	meReq, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://discord.com/api/oauth2/@me", nil)
	if err != nil {
		return nil, err
	}

	res, err := p.config.Client(ctx, token).Do(meReq)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	var info struct {
		User struct {
			ID       uint64 `json:"id,string"`
			Username string `json:"username"`
			Avatar   string `json:"avatar"`
		} `json:"user"`
	}
	if err := json.NewDecoder(res.Body).Decode(&info); err != nil {
		return nil, err
	}

	if info.User.ID == 0 || info.User.Username == "" {
		return nil, errors.New("invalid response")
	}

	return &common.Identity{
		ID:         info.User.ID,
		Username:   info.User.Username,
		AvatarHash: info.User.Avatar,
		Provider:   p.Name(),
	}, nil
}
