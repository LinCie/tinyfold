package users

import (
	"context"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"tinyfold/internal/model"
	"tinyfold/internal/pkg/validation"
)

//go:generate mockgen -source=service.go -destination=mock/service.go -package=mock
type Service interface {
	Create(ctx context.Context, user *model.User) (*model.User, error)
	GetMany(ctx context.Context) ([]model.User, error)
	GetByID(ctx context.Context, id string) (*model.User, error)
	Update(ctx context.Context, user *model.User) (*model.User, error)
	Delete(ctx context.Context, id string) error
}

type service struct {
	repo Repository
}

func NewService(repo Repository) Service {
	return &service{repo: repo}
}

var (
	usernamePattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
	emailPattern    = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)
)

// Create implements [Service].
func (s service) Create(ctx context.Context, user *model.User) (*model.User, error) {
	if user == nil {
		return nil, invalid("user", "is required")
	}

	username, err := validateUsername(user.Username)
	if err != nil {
		return nil, err
	}
	email, err := validateEmail(user.Email)
	if err != nil {
		return nil, err
	}
	displayName, err := validateDisplayName(user.DisplayName)
	if err != nil {
		return nil, err
	}
	if len(user.Bio) > MaxBioLength {
		return nil, invalid("bio", "is too long")
	}

	role := user.Role
	if role == "" {
		role = model.UserRoleAuthor
	}
	if !validRole(role) {
		return nil, invalid("role", "is invalid")
	}

	status := user.Status
	if status == "" {
		status = model.UserStatusInvited
	}
	if status != model.UserStatusInvited {
		return nil, invalid("status", "must be invited when creating a user")
	}

	created := &model.User{
		PasswordHash: user.PasswordHash,
		Username:     username, Email: email, DisplayName: displayName, Bio: user.Bio,
		Role: role, Status: status,
	}
	return s.repo.Create(ctx, created)
}

// GetByID implements [Service].
func (s service) GetByID(ctx context.Context, id string) (*model.User, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, invalid("id", "is required")
	}

	user, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrNotFound
	}
	return user, nil
}

// GetMany implements [Service].
func (s service) GetMany(ctx context.Context) ([]model.User, error) {
	users, err := s.repo.GetMany(ctx)
	if err != nil {
		return nil, err
	}
	if users == nil {
		users = []model.User{}
	}
	return users, nil
}

// Update implements [Service].
func (s service) Update(ctx context.Context, user *model.User) (*model.User, error) {
	if user == nil {
		return nil, invalid("user", "is required")
	}

	id := strings.TrimSpace(user.ID)
	if id == "" {
		return nil, invalid("id", "is required")
	}
	username, err := validateUsername(user.Username)
	if err != nil {
		return nil, err
	}
	email, err := validateEmail(user.Email)
	if err != nil {
		return nil, err
	}
	displayName, err := validateDisplayName(user.DisplayName)
	if err != nil {
		return nil, err
	}
	if len(user.Bio) > MaxBioLength {
		return nil, invalid("bio", "is too long")
	}
	if !validRole(user.Role) {
		return nil, invalid("role", "is invalid")
	}
	if !validStatus(user.Status) {
		return nil, invalid("status", "is invalid")
	}

	current, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, ErrNotFound
	}
	if !validTransition(current.Status, user.Status) {
		return nil, invalid("status", "transition is not allowed")
	}

	now := time.Now().UTC()
	updated := &model.User{
		ID: id, Username: username, Email: email, DisplayName: displayName, Bio: user.Bio,
		Role: user.Role, Status: user.Status,
		CreatedAt: current.CreatedAt, UpdatedAt: now,
	}

	return s.repo.Update(ctx, updated)
}

// Delete implements [Service].
func (s service) Delete(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return invalid("id", "is required")
	}
	return s.repo.Delete(ctx, id)
}

func validateUsername(username string) (string, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return "", invalid("username", "is required")
	}
	if len(username) > MaxUsernameLength || !usernamePattern.MatchString(username) {
		return "", invalid("username", "must contain lowercase letters, numbers, and single hyphens only")
	}
	return username, nil
}

func validateEmail(email string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return "", invalid("email", "is required")
	}
	if len(email) > MaxEmailLength || !emailPattern.MatchString(email) {
		return "", invalid("email", "must be a valid email address")
	}
	return email, nil
}

func validateDisplayName(displayName string) (string, error) {
	displayName = strings.TrimSpace(displayName)
	if displayName == "" {
		return "", invalid("display_name", "is required")
	}
	if utf8.RuneCountInString(displayName) > MaxDisplayNameLength {
		return "", invalid("display_name", "is too long")
	}
	return displayName, nil
}

func validRole(role model.UserRole) bool {
	return role == model.UserRoleAdmin || role == model.UserRoleEditor || role == model.UserRoleAuthor
}

func validStatus(status model.UserStatus) bool {
	return status == model.UserStatusInvited || status == model.UserStatusActive || status == model.UserStatusDisabled
}

func validTransition(from, to model.UserStatus) bool {
	switch from {
	case model.UserStatusInvited:
		return validStatus(to)
	case model.UserStatusActive:
		return to == model.UserStatusActive || to == model.UserStatusDisabled
	case model.UserStatusDisabled:
		return to == model.UserStatusDisabled || to == model.UserStatusActive
	default:
		return false
	}
}

func invalid(field, reason string) error {
	return validation.New(ErrInvalidUser, field, reason)
}
