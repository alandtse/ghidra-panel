package oauth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"strconv"

	"github.com/coreos/go-oidc/v3/oidc"
	"go.mkw.re/ghidra-panel/common"
	"go.mkw.re/ghidra-panel/csrf"
	"golang.org/x/oauth2"
)

// OIDCProvider implements a generic OpenID Connect provider
// This works for Google, GitLab, and any other OIDC-compliant provider
type OIDCProvider struct {
	name     string
	config   oauth2.Config
	verifier *oidc.IDTokenVerifier
	prot     *csrf.OneTime
}

// NewOIDCProvider creates a new OIDC provider
// issuerURL should be the OIDC issuer URL (e.g., "https://accounts.google.com" for Google)
// The provider will automatically discover endpoints via .well-known/openid-configuration
func NewOIDCProvider(ctx context.Context, name, issuerURL, clientID, clientSecret, redirectURL string) (*OIDCProvider, error) {
	provider, err := oidc.NewProvider(ctx, issuerURL)
	if err != nil {
		return nil, err
	}

	config := oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint:     provider.Endpoint(),
		RedirectURL:  redirectURL,
		Scopes:       []string{oidc.ScopeOpenID, "profile", "email"},
	}

	verifier := provider.Verifier(&oidc.Config{ClientID: clientID})

	return &OIDCProvider{
		name:     name,
		config:   config,
		verifier: verifier,
		prot:     csrf.NewOneTime(),
	}, nil
}

func (p *OIDCProvider) Name() string {
	return p.name
}

func (p *OIDCProvider) AuthURL() string {
	return p.config.AuthCodeURL(p.prot.Issue(), oauth2.AccessTypeOnline)
}

func (p *OIDCProvider) HandleRedirect(wr http.ResponseWriter, req *http.Request) (*common.Identity, error) {
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

	// Exchange code for token
	oauth2Token, err := p.config.Exchange(ctx, code)
	if err != nil {
		return nil, err
	}

	// Extract ID Token from OAuth2 token
	rawIDToken, ok := oauth2Token.Extra("id_token").(string)
	if !ok {
		return nil, errors.New("no id_token field in OAuth2 token")
	}

	// Verify ID Token
	idToken, err := p.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return nil, err
	}

	// Extract claims
	var claims struct {
		Sub           string `json:"sub"`
		Name          string `json:"name"`
		PreferredName string `json:"preferred_username"`
		Email         string `json:"email"`
		Picture       string `json:"picture"`
	}
	if err := idToken.Claims(&claims); err != nil {
		return nil, err
	}

	// Determine username (prefer preferred_username, fallback to name, then email)
	username := claims.PreferredName
	if username == "" {
		username = claims.Name
	}
	if username == "" {
		username = claims.Email
	}
	if username == "" || claims.Sub == "" {
		return nil, errors.New("insufficient claims in ID token")
	}

	// Convert subject to uint64 ID by hashing (OIDC subjects are strings)
	id := hashSubjectToID(claims.Sub)

	// Prevent CSRF token reuse
	if err := p.prot.Consume(csrfID); err != nil {
		return nil, err
	}

	return &common.Identity{
		ID:         id,
		Username:   username,
		AvatarHash: claims.Picture,
		Provider:   p.Name(),
	}, nil
}

// hashSubjectToID converts an OIDC subject string to a deterministic uint64
// This is necessary because OIDC subjects are strings, but our Identity.ID is uint64
func hashSubjectToID(subject string) uint64 {
	hash := sha256.Sum256([]byte(subject))
	// Take first 16 hex characters (64 bits) of the hash
	hexStr := hex.EncodeToString(hash[:])[:16]
	id, err := strconv.ParseUint(hexStr, 16, 64)
	if err != nil {
		panic("invalid hex slice from sha256: " + err.Error())
	}
	return id
}
