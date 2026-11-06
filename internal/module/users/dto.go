package users

import (
	"time"

	"tinyfold/internal/model"
)

type UserResponse struct {
	ID          string           `json:"id"`
	Username    string           `json:"username"`
	Email       string           `json:"email"`
	DisplayName string           `json:"display_name"`
	Bio         string           `json:"bio"`
	Role        model.UserRole   `json:"role"`
	Status      model.UserStatus `json:"status"`
	CreatedAt   time.Time        `json:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at"`
}

func ToUserResponse(user model.User) UserResponse {
	return UserResponse{
		ID: user.ID, Username: user.Username, Email: user.Email,
		DisplayName: user.DisplayName, Bio: user.Bio, Role: user.Role,
		Status: user.Status, CreatedAt: user.CreatedAt, UpdatedAt: user.UpdatedAt,
	}
}
