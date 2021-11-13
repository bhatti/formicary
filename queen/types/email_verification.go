package types

import (
	"errors"
	"fmt"
	"github.com/twinj/uuid"
	common "plexobject.com/formicary/internal/types"
	"regexp"
	"time"
)

// EmailVerification represents verified email
type EmailVerification struct {
	//gorm.Model
	// ID defines UUID for primary key
	ID string `json:"id" gorm:"primary_key"`
	// Email defines invitee
	Email string `json:"email"`
	// EmailCode defines code
	EmailCode string `json:"email_code"`
	// UserID defines foreign key
	UserID string `json:"user_id"`
	// OrganizationID defines org who submitted the job
	OrganizationID string `json:"organization_id"`
	// ExpiresAt expiration time
	ExpiresAt time.Time `json:"expires_at"`
	// VerifiedAt verification time
	VerifiedAt *time.Time `json:"verified_at"`
	// CreatedAt created time
	CreatedAt time.Time         `json:"created_at"`
	Errors    map[string]string `yaml:"-" json:"-" gorm:"-"`
}

// NewEmailVerification creates new instance of user email
func NewEmailVerification(
	email string,
	user *common.User,
) *EmailVerification {
	return &EmailVerification{
		Email:          email,
		EmailCode:      randomString(20),
		UserID:         user.ID,
		OrganizationID: user.OrganizationID,
		ExpiresAt:      time.Now().Add(time.Hour * 24 * 1),
		CreatedAt:      time.Now(),
	}
}

// TableName overrides default table name
func (EmailVerification) TableName() string {
	return "formicary_email_verifications"
}

// Validate validates properties
func (u *EmailVerification) Validate() (err error) {
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

	if u.UserID == "" {
		err = errors.New("user-id is not specified")
		u.Errors["UserID"] = err.Error()
	}
	if u.ExpiresAt.Unix() < time.Now().Unix() {
		u.ExpiresAt = time.Now().Add(time.Hour * 24 * 3)
	}

	return
}

// ValidateBeforeSave validation
func (u *EmailVerification) ValidateBeforeSave() (err error) {
	if err = u.Validate(); err != nil {
		return err
	}
	u.EmailCode = randomString(12)
	u.ID = uuid.NewV4().String()
	u.CreatedAt = time.Now()
	u.VerifiedAt = nil
	return nil
}

// String provides short summary of email
func (u *EmailVerification) String() string {
	return fmt.Sprintf("Email=%s User=%s", u.Email, u.UserID)
}
