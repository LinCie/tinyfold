package jwt

import (
	"errors"
	"time"

	jwtlib "github.com/golang-jwt/jwt/v5"
	"github.com/segmentio/ksuid"
)

const (
	AccessToken  = "access"
	RefreshToken = "refresh"
)

var ErrInvalidToken = errors.New("invalid token")

type Claims struct {
	TokenUse string `json:"token_use"`
	jwtlib.RegisteredClaims
}

type Pair struct {
	RefreshID        string    `json:"-"`
	AccessToken      string    `json:"access_token"`
	AccessExpiresAt  time.Time `json:"access_expires_at"`
	RefreshToken     string    `json:"refresh_token"`
	RefreshExpiresAt time.Time `json:"refresh_expires_at"`
}

type Manager struct {
	accessSecret  []byte
	refreshSecret []byte
	accessTTL     time.Duration
	refreshTTL    time.Duration
}

func NewManager(accessSecret, refreshSecret string) (*Manager, error) {
	if len(accessSecret) < 32 || len(refreshSecret) < 32 {
		return nil, errors.New("JWT secrets must each be at least 32 characters")
	}
	return &Manager{
		accessSecret: []byte(accessSecret), refreshSecret: []byte(refreshSecret),
		accessTTL: 15 * time.Minute, refreshTTL: 7 * 24 * time.Hour,
	}, nil
}

func (m *Manager) NewPair(userID string) (Pair, error) {
	if userID == "" {
		return Pair{}, errors.New("user ID is required")
	}
	now := time.Now().UTC()
	refreshID := ksuid.New().String()
	access, err := m.sign(userID, AccessToken, "", now, m.accessTTL, m.accessSecret)
	if err != nil {
		return Pair{}, err
	}
	refresh, err := m.sign(userID, RefreshToken, refreshID, now, m.refreshTTL, m.refreshSecret)
	if err != nil {
		return Pair{}, err
	}
	return Pair{
		RefreshID:   refreshID,
		AccessToken: access, AccessExpiresAt: now.Add(m.accessTTL),
		RefreshToken: refresh, RefreshExpiresAt: now.Add(m.refreshTTL),
	}, nil
}

func (m *Manager) sign(userID, use, tokenID string, now time.Time, ttl time.Duration, secret []byte) (string, error) {
	claims := Claims{TokenUse: use, RegisteredClaims: jwtlib.RegisteredClaims{
		Subject: userID, ID: tokenID, IssuedAt: jwtlib.NewNumericDate(now),
		ExpiresAt: jwtlib.NewNumericDate(now.Add(ttl)),
	}}
	return jwtlib.NewWithClaims(jwtlib.SigningMethodHS256, claims).SignedString(secret)
}

func (m *Manager) ParseAccess(raw string) (*Claims, error) {
	return m.parse(raw, AccessToken, m.accessSecret)
}

func (m *Manager) ParseRefresh(raw string) (*Claims, error) {
	return m.parse(raw, RefreshToken, m.refreshSecret)
}

func (m *Manager) parse(raw, use string, secret []byte) (*Claims, error) {
	claims := new(Claims)
	token, err := jwtlib.ParseWithClaims(raw, claims, func(*jwtlib.Token) (any, error) {
		return secret, nil
	}, jwtlib.WithValidMethods([]string{jwtlib.SigningMethodHS256.Alg()}))
	if err != nil || token == nil || !token.Valid || claims.TokenUse != use || claims.Subject == "" {
		return nil, ErrInvalidToken
	}
	if use == RefreshToken && claims.ID == "" {
		return nil, ErrInvalidToken
	}
	return claims, nil
}
