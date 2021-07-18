package types

import (
	"errors"
	"fmt"
	"plexobject.com/formicary/internal/crypto"
	"time"
)

// UserToken defines JWT tokens to access the API.
// Note: The JWT token is not directly stored in the database, just its hash and expiration.
// Also, this can be used to revoke API tokens.
type UserToken struct {
	//gorm.Model
	// ID defines UUID for primary key
	ID string `json:"id" gorm:"primary_key"`
	// UserID `defines foreign key
	UserID string `json:"user_id"`
	// OrganizationID defines foreign key
	OrganizationID string `json:"organization_id"`
	// TokenName defines name of token
	TokenName string `json:"token_name"`
	// SHA256 defines sha of token
	SHA256 string `json:"sha256"`
	// Active is used to soft delete token
	Active bool `json:"-"`
	// ExpiresAt expiration time
	ExpiresAt time.Time `json:"expires_at"`
	// CreatedAt created time
	CreatedAt time.Time `json:"created_at"`
	APIToken  string    `json:"-" gorm:"-"`
}

// NewUserToken creates new instance of user token
func NewUserToken(
	userID string,
	orgID string,
	tokenName string) *UserToken {
	return &UserToken{
		UserID:         userID,
		OrganizationID: orgID,
		TokenName:      tokenName,
		Active:         true,
		CreatedAt:      time.Now(),
	}
}

// TableName overrides default table name
func (UserToken) TableName() string {
	return "formicary_user_tokens"
}

// String token
func (u *UserToken) String() string {
	return fmt.Sprintf("%s %s", u.TokenName, u.ExpiresAt)
}

// Validate validates token
func (u *UserToken) Validate() (err error) {
	if u.UserID == "" {
		return errors.New("user-id is not specified")
	}
	if u.TokenName == "" {
		return errors.New("token-name is not specified")
	}
	if u.APIToken == "" {
		return errors.New("api token is not specified")
	}
	u.SHA256 = crypto.SHA256(u.APIToken)
	now := time.Now()
	if u.ExpiresAt.Unix() < now.Unix() {
		return errors.New("expires-at is already expired")
	}

	return
}
