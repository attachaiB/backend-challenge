package model

import "time"

type CreateUserRequest struct {
	Name     string `json:"name" binding:"required,max=100"`
	Email    string `json:"email" binding:"required,email,max=254"`
	Password string `json:"password" binding:"required,min=8,max=72"`
}

// UpdateUserRequest uses pointers so the service can tell "not sent" from "sent".
type UpdateUserRequest struct {
	Name  *string `json:"name" binding:"omitempty,max=100"`
	Email *string `json:"email" binding:"omitempty,email,max=254"`
}

type UserResponse struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

type UserListResponse struct {
	Users []UserResponse `json:"users"`
	Total int            `json:"total"`
}
