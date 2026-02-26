package token

import (
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"go.mkw.re/ghidra-panel/common"
)

// TODO Integrate BitRing for token expiry

type Issuer struct {
	Secret   []byte
	Validity time.Duration
}

func NewIssuer(secret []byte, validity time.Duration) Issuer {
	if validity <= 0 {
		validity = 90 * 24 * time.Hour
	}
	return Issuer{secret, validity}
}

type Claims struct {
	jwt.RegisteredClaims
	Name       string `json:"name,omitempty"`
	AvatarHash string `json:"avatar,omitempty"`
	Provider   string `json:"provider,omitempty"`
}

func (iss Issuer) Issue(ident *common.Identity) (string, time.Time) {
	iat := time.Now()
	exp := iat.Add(iss.Validity)

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   strconv.FormatUint(ident.ID, 10),
			IssuedAt:  jwt.NewNumericDate(iat),
			ExpiresAt: jwt.NewNumericDate(exp),
		},
		Name:       ident.Username,
		AvatarHash: ident.AvatarHash,
		Provider:   ident.Provider,
	})

	tokenString, err := token.SignedString(iss.Secret)
	if err != nil {
		log.Panicf("jwt signing failed: %v", err)
	}
	return tokenString, exp
}

func (iss Issuer) Verify(tokenString string) (ident *common.Identity, err error) {
	// Parse and validate token
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return iss.Secret, nil
	})
	if err != nil {
		return nil, err
	}

	// Parse claims
	claims, ok := token.Claims.(*Claims)
	if !ok {
		return nil, fmt.Errorf("invalid token claims")
	}
	id, err := strconv.ParseUint(claims.Subject, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse subject: %v", err)
	}

	// Reconstruct identity
	return &common.Identity{
		ID:         id,
		Username:   claims.Name,
		AvatarHash: claims.AvatarHash,
		Provider:   claims.Provider,
	}, nil
}
