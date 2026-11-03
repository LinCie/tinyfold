package users

import "time"

type UserRole string

var (
	UserRoleAdmin  UserRole = "admin"
	UserRoleEditor UserRole = "editor"
	UserRoleAuthor UserRole = "author"
)

type UserStatus string

var (
	UserStatusInvited  UserStatus = "invited"
	UserStatusActive   UserStatus = "active"
	UserStatusDisabled UserStatus = "disabled"
)

type User struct {
	ID          string     `db:"id"`
	Username    string     `db:"username"`
	Email       string     `db:"email"`
	DisplayName string     `db:"display_name"`
	Bio         string     `db:"bio"`
	Role        UserRole   `db:"role"`
	Status      UserStatus `db:"status"`
	CreatedAt   time.Time  `db:"created_at"`
	UpdatedAt   time.Time  `db:"updated_at"`
}
