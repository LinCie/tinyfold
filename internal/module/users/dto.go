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
