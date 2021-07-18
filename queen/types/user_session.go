package types

import (
	"errors"
	common "plexobject.com/formicary/internal/types"
	"time"
)

// UserSession represents a user session
type UserSession struct {
	//gorm.Model
	// ID defines UUID for primary key
	ID string `json:"id" gorm:"primary_key"`
	// SessionID defines session
	SessionID string `json:"session_id"`
	// UserID `defines foreign key
	UserID string `json:"user_id"`
	// Username
	Username string `json:"username"`
	// Email address
	Email string `json:"email"`
	// PictureURL address
	PictureURL string `json:"picture_url"`
	// AuthProvider defines provider for external oauth provider
	AuthProvider string `json:"auth_provider"`
	// IPAddress address
	IPAddress string `json:"ip_address"`
	// Data defines additional data
	Data string `json:"data"`
	// CreatedAt created time
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt update time
	UpdatedAt time.Time `json:"updated_at"`
}

// NewUserSession creates new instance of user session
func NewUserSession(
	user *common.User,
	sessionID string) *UserSession {
	return &UserSession{
		UserID:     user.ID,
		Username:   user.Username,
		Email:      user.Email,
		PictureURL: user.PictureURL,
		SessionID:  sessionID,
		Data:       "",
		CreatedAt:  time.Now(),
	}
}

// TableName overrides default table name
func (UserSession) TableName() string {
	return "formicary_user_sessions"
}

// Validate validates session
func (u *UserSession) Validate() (err error) {
	if u.SessionID == "" {
		return errors.New("session-id is not specified")
	}
	if u.Username == "" {
		return errors.New("username is not specified")
	}
	if u.UserID == "" {
		return errors.New("user-id is not specified")
	}

	return
}
