package auth

import (
	"context"
	"errors"
	"time"

	"tinyfold/internal/config"
	"tinyfold/internal/model"
	"tinyfold/internal/pkg/jwt"
)

var (
	ErrInvalidRefreshToken = errors.New("invalid refresh token")
	ErrInvalidCredentials  = errors.New("invalid user ID or password")
	ErrInactiveUser        = errors.New("user is not active")
)

type UserService interface {
	Create(ctx context.Context, user *model.User) (*model.User, error)
	GetByID(ctx context.Context, id string) (*model.User, error)
}

type PasswordService interface {
	Hash(ctx context.Context, password string) (string, error)
	Verify(ctx context.Context, password, hash string) (bool, error)
}

type Service interface {
	Register(ctx context.Context, user *model.User, password string) (*model.User, error)
	Login(ctx context.Context, userID, password string) (jwt.Pair, error)
	Issue(ctx context.Context, userID string) (jwt.Pair, error)
	Refresh(ctx context.Context, refreshToken string) (jwt.Pair, error)
	Logout(ctx context.Context, refreshToken string) error
}

type service struct {
	repo      Repository
	users     UserService
	passwords PasswordService
	tokens    *jwt.Manager
}

func NewService(repo Repository, userService UserService, passwords PasswordService, cfg *config.Config) (Service, error) {
	tokens, err := jwt.NewManager(cfg.JWTAccessSecret, cfg.JWTRefreshSecret)
	if err != nil {
		return nil, err
	}
	return &service{repo: repo, users: userService, passwords: passwords, tokens: tokens}, nil
}

func (s *service) Register(ctx context.Context, user *model.User, password string) (*model.User, error) {
	if user == nil || password == "" {
		return nil, ErrInvalidCredentials
	}
	passwordHash, err := s.passwords.Hash(ctx, password)
	if err != nil {
		return nil, err
	}
	created := *user
	created.PasswordHash = passwordHash
	return s.users.Create(ctx, &created)
}

func (s *service) Login(ctx context.Context, userID, password string) (jwt.Pair, error) {
	user, err := s.users.GetByID(ctx, userID)
	if err != nil || user == nil {
		return jwt.Pair{}, ErrInvalidCredentials
	}
	valid, err := s.passwords.Verify(ctx, password, user.PasswordHash)
	if err != nil || !valid {
		return jwt.Pair{}, ErrInvalidCredentials
	}
	if user.Status != model.UserStatusActive {
		return jwt.Pair{}, ErrInactiveUser
	}
	return s.Issue(ctx, user.ID)
}

func (s *service) Issue(ctx context.Context, userID string) (jwt.Pair, error) {
	pair, err := s.tokens.NewPair(userID)
	if err != nil {
		return jwt.Pair{}, err
	}
	if err := s.repo.CreateRefreshToken(ctx, pair.RefreshID, userID, pair.RefreshExpiresAt); err != nil {
		return jwt.Pair{}, err
	}
	return pair, nil
}

func (s *service) Refresh(ctx context.Context, refreshToken string) (jwt.Pair, error) {
	claims, err := s.tokens.ParseRefresh(refreshToken)
	if err != nil {
		return jwt.Pair{}, ErrInvalidRefreshToken
	}

	pair, err := s.tokens.NewPair(claims.Subject)
	if err != nil {
		return jwt.Pair{}, err
	}
	if err := s.repo.RotateRefreshToken(ctx, claims.ID, claims.Subject, pair.RefreshID, pair.RefreshExpiresAt, time.Now().UTC()); err != nil {
		return jwt.Pair{}, err
	}
	return pair, nil
}

func (s *service) Logout(ctx context.Context, refreshToken string) error {
	claims, err := s.tokens.ParseRefresh(refreshToken)
	if err != nil {
		return ErrInvalidRefreshToken
	}
	return s.repo.RevokeRefreshToken(ctx, claims.ID, claims.Subject, time.Now().UTC())
}
