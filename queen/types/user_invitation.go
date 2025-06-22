package types

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/oklog/ulid/v2"
	"math/big"
	common "plexobject.com/formicary/internal/types"
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
	// OrgUnit defines org-unit
	OrgUnit string `json:"org_unit"`
	// InvitedByUserID defines foreign key
	InvitedByUserID string `json:"invited_by_user_id"`
	// ExpiresAt expiration time
	AcceptedAt *time.Time `json:"-"`
	// ExpiresAt expiration time
	ExpiresAt time.Time `json:"expires_at"`
	// CreatedAt created time
	CreatedAt time.Time         `json:"created_at"`
	Errors    map[string]string `yaml:"-" json:"-" gorm:"-"`
}

// NewUserInvitation creates new instance of user invitation
func NewUserInvitation(
	email string,
	byUser *common.User,
) (inv *UserInvitation) {
	inv = &UserInvitation{
		Email:           email,
		InvitationCode:  randomString(20),
		InvitedByUserID: byUser.ID,
		OrganizationID:  byUser.OrganizationID,
		ExpiresAt:       time.Now().Add(time.Hour * 24 * 3),
		CreatedAt:       time.Now(),
	}
	if byUser.HasOrganization() {
		inv.OrgUnit = byUser.Organization.OrgUnit
	} else {
		inv.OrgUnit = byUser.OrgUnit
	}
	return
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
		err = errors.New("org is not specified")
		u.Errors["OrganizationID"] = err.Error()
	}
	if u.InvitedByUserID == "" {
		err = errors.New("invited-by-user is not specified")
		u.Errors["InvitedByUserID"] = err.Error()
	}
	if u.OrgUnit == "" {
		err = errors.New("org-unit is not specified")
		u.Errors["OrgUnit"] = err.Error()
	}
	if u.ExpiresAt.Unix() < time.Now().Unix() {
		u.ExpiresAt = time.Now().Add(time.Hour * 24 * 3)
	}

	return
}

// ValidateBeforeSave validation
func (u *UserInvitation) ValidateBeforeSave() (err error) {
	if err = u.Validate(); err != nil {
		return err
	}
	u.ID = ulid.Make().String()
	u.CreatedAt = time.Now()
	u.AcceptedAt = nil
	u.InvitationCode = randomString(12)
	return nil
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
		if n, err := rand.Int(rand.Reader, big.NewInt(int64(len(letters)))); err == nil {
			s[i] = letters[n.Int64()]
		}
	}
	return string(s)
}
