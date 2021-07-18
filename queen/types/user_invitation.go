package types

import (
	"encoding/hex"
	"errors"
	"fmt"
	"math/rand"
	"regexp"
	"time"
)

// UserInvitation represents a user session
type UserInvitation struct {
	//gorm.Model
	// ID defines UUID for primary key
	ID string `json:"id" gorm:"primary_key"`
	// Email defines invitee
	Email string `json:"email"`
	// InvitationCode defines code
	InvitationCode string `json:"invitation_code"`
	// OrganizationID defines foreign key
	OrganizationID string `json:"organization_id"`
	// InvitedByUserID defines foreign key
	InvitedByUserID string `json:"invited_by_user_id"`
	// ExpiresAt expiration time
	AcceptedAt *time.Time `json:"accepted_at"`
	// ExpiresAt expiration time
	ExpiresAt time.Time `json:"expires_at"`
	// CreatedAt created time
	CreatedAt time.Time         `json:"created_at"`
	Errors    map[string]string `yaml:"-" json:"-" gorm:"-"`
}

// NewUserInvitation creates new instance of user invitation
func NewUserInvitation(
	username string,
	userID string,
	orgID string) *UserInvitation {
	return &UserInvitation{
		Email:           username,
		InvitationCode:  randomString(20),
		InvitedByUserID: userID,
		OrganizationID:  orgID,
		ExpiresAt:       time.Now().Add(time.Hour * 24 * 3),
		CreatedAt:       time.Now(),
	}
}

// TableName overrides default table name
func (UserInvitation) TableName() string {
	return "formicary_user_invitations"
}

// Validate validates session
func (u *UserInvitation) Validate() (err error) {
	u.Errors = make(map[string]string)
	if u.Email == "" {
		err = errors.New("email is not specified")
		u.Errors["Email"] = err.Error()
	}

	re := regexp.MustCompile("^[a-zA-Z0-9.!#$%&'*+/=?^_`{|}~-]+@[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$")
	if !re.MatchString(u.Email) {
		err = errors.New("email is not valid")
		u.Errors["Email"] = err.Error()
	}

	if u.OrganizationID == "" {
		return errors.New("org is not specified")
	}
	if u.InvitedByUserID == "" {
		err = errors.New("invited-by-user is not specified")
		u.Errors["InvitedByUserID"] = err.Error()
	}
	if u.ExpiresAt.Unix() < time.Now().Unix() {
		u.ExpiresAt = time.Now().Add(time.Hour * 24 * 3)
	}
	if u.InvitationCode == "" {
		u.InvitationCode = randomString(20)
	}

	return
}

// String provides short summary of invitation
func (u *UserInvitation) String() string {
	return fmt.Sprintf("Email=%s Org=%s", u.Email, u.OrganizationID)
}

func randomString(n int) string {
	bytes := make([]byte, n)
	if _, err := rand.Read(bytes); err == nil {
		return hex.EncodeToString(bytes)
	}
	var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
	s := make([]rune, n)
	for i := range s {
		s[i] = letters[rand.Intn(len(letters))]
	}
	return string(s)
}
