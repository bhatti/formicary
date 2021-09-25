package types

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"plexobject.com/formicary/internal/acl"
)

const publicEmailExt = `@(aim.com|alice.it|aliceadsl.fr|aol.com|arcor.de|att.net|bellsouth.net|bigpond.com|bigpond.net.au|bluewin.ch|blueyonder.co.uk|bol.com.br|centurytel.net|charter.net|chello.nl|club-internet.fr|comcast.net|cox.net|earthlink.net|facebook.com|free.fr|freenet.de|frontiernet.net|gmail.com|gmx.de|gmx.net|googlemail.com|hetnet.nl|home.nl|hotmail.co.uk|hotmail.com|hotmail.de|hotmail.es|hotmail.fr|hotmail.it|icloud.com|ig.com.br|inbox.com|juno.com|laposte.net|libero.it|live.ca|live.co.uk|live.com.au|live.com|live.fr|live.it|live.nl|mac.com|mail.com|mail.ru|me.com|msn.com|neuf.fr|ntlworld.com|optonline.net|optusnet.com.au|orange.fr|outlook.com|planet.nl|qq.com|rambler.ru|rediffmail.com|rocketmail.com|sbcglobal.net|sfr.fr|shaw.ca|sky.com|skynet.be|sympatico.ca|t-online.de|telenet.be|terra.com.br|tin.it|tiscali.co.uk|tiscali.it|uol.com.br|verizon.net|virgilio.it|voila.fr|wanadoo.fr|web.de|windstream.net|yahoo.ca|yahoo.co.id|yahoo.co.in|yahoo.co.jp|yahoo.co.uk|yahoo.com.ar|yahoo.com.au|yahoo.com.br|yahoo.com.mx|yahoo.com.sg|yahoo.com|yahoo.de|yahoo.es|yahoo.fr|yahoo.in|yahoo.it|yandex.ru|ymail.com|zonnet.nl)`
const emailRegex = "^[a-zA-Z0-9.!#$%&'*+/=?^_`{|}~-]+@[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$"

// User represents a user of the system with multi-tenancy support.
type User struct {
	//gorm.Model
	// ID defines UUID for primary key
	ID string `json:"id" gorm:"primary_key"`
	// Name of user
	Name string `json:"name"`
	// Username defines username
	Username string `json:"username"`
	// Email defines email
	Email string `json:"email"`
	// URL defines url
	URL string `json:"url"`
	// PictureURL defines URL for picture
	PictureURL string `json:"picture_url"`

	// OrganizationID defines foreign key for Organization
	OrganizationID string        `json:"organization_id"`
	Organization   *Organization `gorm:"foreignKey:OrganizationID" gorm:"auto_preload" gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`

	// AuthID defines id from external auth provider
	AuthID string `json:"auth_id" gorm:"auth_id"`
	// AuthProvider defines provider for external auth provider
	AuthProvider string `json:"auth_provider" gorm:"auth_provider"`
	// MaxConcurrency defines max number of jobs that can be run concurrently by org
	MaxConcurrency int `yaml:"max_concurrency,omitempty" json:"max_concurrency"`
	// NotifySerialized serialized notification
	NotifySerialized string `yaml:"-,omitempty" json:"-" gorm:"notify_serialized"`
	// StickyMessage defines an error message that needs user attention
	StickyMessage string `json:"sticky_message" gorm:"sticky_message"`
	// BundleID defines package or bundle
	BundleID string `json:"bundle_id"`
	// SerializedPerms defines permissions
	SerializedPerms string `json:"-"`
	// SerializedRoles defines roles
	SerializedRoles string `json:"-"`
	// Salt for password
	Salt string `json:"salt"`
	// Subscription defines quota limits and usage period
	Subscription *Subscription `json:"subscription" gorm:"ForeignKey:UserID" gorm:"auto_preload" gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`
	// EmailVerified for email
	EmailVerified bool `json:"-"`
	// Locked account
	Locked bool `json:"-"`
	// Active is used to softly delete user
	Active bool `json:"-"`
	// CreatedAt created time
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt update time
	UpdatedAt time.Time         `json:"updated_at"`
	Errors    map[string]string `yaml:"-" json:"-" gorm:"-"`
	// OrgUnit defines org-unit
	OrgUnit string `json:"-" gorm:"-"`
	// InvitationCode defines code for invitation
	InvitationCode string `json:"-" gorm:"-"`
	// AgreeTerms defines code for invitation
	AgreeTerms bool                              `json:"-" gorm:"-"`
	Notify     map[NotifyChannel]JobNotifyConfig `yaml:"notify,omitempty" json:"notify" gorm:"-"`

	// permissions defines ACL permissions
	permissions *acl.Permissions `gorm:"-"`
	// roles defines ACL roles
	roles *acl.Roles `gorm:"-"`
}

// NewUser creates new instance of user
func NewUser(
	orgID string,
	username string,
	name string,
	email string,
	roles *acl.Roles) *User {
	if email == "" && strings.Contains(username, "@") {
		email = username
	}
	user := &User{
		OrganizationID:  orgID,
		Username:        username,
		Email:           strings.ToLower(email),
		Name:            name,
		Active:          true,
		SerializedPerms: acl.DefaultPermissionsString(),
		SerializedRoles: roles.String(),
		Notify:          make(map[NotifyChannel]JobNotifyConfig),
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}
	return user
}

// TableName overrides default table name
func (User) TableName() string {
	return "formicary_users"
}

// String provides short summary of user
func (u *User) String() string {
	return fmt.Sprintf("ID=%s Org=%s Username=%s", u.ID, u.OrganizationID, u.Username)
}

// Equals compares other job-resource for equality
func (u *User) Equals(other *User) error {
	if other == nil {
		return fmt.Errorf("found nil other job")
	}

	if u.OrganizationID != other.OrganizationID {
		return fmt.Errorf("expected jobType %v but was %v", u.OrganizationID, other.OrganizationID)
	}
	if u.Username != other.Username {
		return fmt.Errorf("expected jobType %v but was %v", u.Username, other.Username)
	}
	return nil
}

// GetUnverifiedNotificationEmails returns unverified emails
func (u *User) GetUnverifiedNotificationEmails() (res []string) {
	lookup := make(map[string]bool)
	if u.Email != "" && !u.EmailVerified {
		lookup[strings.ToLower(u.Email)] = true
	}
	for _, r := range u.Notify[EmailChannel].Recipients {
		lr := strings.ToLower(r)
		if !lookup[lr] {
			lookup[lr] = true
		}
	}
	res = make([]string, 0)
	for email := range lookup {
		res = append(res, email)
	}
	return
}

// AfterLoad initializes user
func (u *User) AfterLoad() error {
	if u.NotifySerialized != "" {
		u.Notify = make(map[NotifyChannel]JobNotifyConfig)
		if err := json.Unmarshal([]byte(u.NotifySerialized), &u.Notify); err != nil {
			return err
		}
	} else {
		if cfg, err := JobNotifyConfigWithEmail(u.Email, NotifyWhenOnFailure); err == nil {
			u.Notify = map[NotifyChannel]JobNotifyConfig{EmailChannel: cfg}
		}
	}
	if u.Organization != nil {
		u.OrgUnit = u.Organization.OrgUnit
	}
	return nil
}

// NotifyWhen returns when
func (u *User) NotifyWhen() NotifyWhen {
	for _, v := range u.Notify {
		return v.When
	}
	return NotifyWhenOnFailure
}

// NotifyEmail returns notify email
func (u *User) NotifyEmail() string {
	emailCfg := u.Notify[EmailChannel]
	return strings.Join(emailCfg.Recipients, ",")
}

// SetNotifyEmail sets notify email
func (u *User) SetNotifyEmail(email string, when NotifyWhen) error {
	notifyCfg, err := JobNotifyConfigWithEmail(email, when)
	if err != nil {
		return err
	}
	u.Notify[EmailChannel] = notifyCfg
	return nil
}

// NotifyChannel returns slack channel
func (u *User) NotifyChannel() string {
	slackCfg := u.Notify[SlackChannel]
	return strings.Join(slackCfg.Recipients, ",")
}

// SetNotifyChannel sets slack channel
func (u *User) SetNotifyChannel(channel string, when NotifyWhen) error {
	notifyCfg, err := JobNotifyConfigWithChannel(channel, when)
	if err != nil {
		return err
	}
	u.Notify[SlackChannel] = notifyCfg
	return nil
}

// Validate validates job-resource
func (u *User) Validate() (err error) {
	u.Errors = make(map[string]string)
	if u.Username == "" {
		err = errors.New("username is not specified")
		u.Errors["Username"] = err.Error()
	}

	if u.Name == "" {
		err = errors.New("name is not specified")
		u.Errors["Name"] = err.Error()
	}
	if len(u.Name) > 100 {
		err = errors.New("name is too long")
		u.Errors["Name"] = err.Error()
	}

	if u.Email == "" {
		err = errors.New("email is not specified")
		u.Errors["Email"] = err.Error()
	}
	if len(u.Email) > 100 {
		err = errors.New("email is too long")
		u.Errors["Email"] = err.Error()
	}
	re := regexp.MustCompile(emailRegex)
	if !re.MatchString(u.Email) {
		err = errors.New("email is not valid")
		u.Errors["Email"] = err.Error()
	}

	if len(u.OrgUnit) > 100 {
		err = errors.New("org-unit is too long")
		u.Errors["OrgUnit"] = err.Error()
	}
	if u.MaxConcurrency == 0 {
		u.MaxConcurrency = 1
	}
	for source, notify := range u.Notify {
		if source == EmailChannel {
			if err = notify.ValidateEmail(); err != nil {
				u.Errors["Notify"] = err.Error()
				return err
			}
		}
	}
	return
}

// ValidateBeforeSave validates job-resource
func (u *User) ValidateBeforeSave() error {
	if err := u.Validate(); err != nil {
		return err
	}
	if len(u.Notify) > 0 {
		if b, err := json.Marshal(u.Notify); err == nil {
			u.NotifySerialized = string(b)
		} else {
			return err
		}
	}
	u.Email = strings.ToLower(strings.TrimSpace(u.Email))
	return nil
}

// UsesCommonEmail checks for common email extensions
func (u *User) UsesCommonEmail() bool {
	return CommonEmailExtension(u.Email)
}

// HasOrganization - association to org
func (u *User) HasOrganization() bool {
	return u.OrganizationID != "" || u.OrgUnit != ""
}

// HasOrganizationOrInvitationCode - returns true if invitation or organization is populated
func (u *User) HasOrganizationOrInvitationCode() bool {
	return u.HasOrganization() || u.InvitationCode != ""
}

// CommonEmailExtension checks for common email extensions
func CommonEmailExtension(email string) bool {
	if strings.HasSuffix(email, ".edu") {
		return true
	}
	match, _ := regexp.MatchString(publicEmailExt, email)
	return match
}

// GetPermissions getter
func (u *User) GetPermissions() *acl.Permissions {
	if u.permissions == nil {
		u.permissions = acl.NewPermissions(u.SerializedPerms)
	}
	return u.permissions
}

// PermissionList getter
func (u *User) PermissionList() []*acl.Permission {
	return acl.UnmarshalPermissions(u.SerializedPerms)
}

// HasPermission getter
func (u *User) HasPermission(resource acl.Resource, action int) bool {
	return u.IsAdmin() || u.GetPermissions().Has(resource, action)
}

// GetRoles getter
func (u *User) GetRoles() *acl.Roles {
	if u.roles == nil {
		u.roles = acl.NewRoles(u.SerializedRoles)
	}
	return u.roles
}

// RolesList getter
func (u *User) RolesList() []*acl.Role {
	return acl.UnmarshalRoles(u.SerializedRoles)
}

// HasRole getter
func (u *User) HasRole(roleType acl.RoleType, scope ...string) bool {
	return u.GetRoles().HasRole(roleType, scope...)
}

// CopyRolesPermissions copies roles/permissions
func (u *User) CopyRolesPermissions(other *User) {
	u.SerializedPerms = other.SerializedPerms
	u.SerializedRoles = other.SerializedRoles
	u.roles = nil
	u.permissions = nil
}

// IsAdmin getter
func (u *User) IsAdmin() bool {
	return u.GetRoles().IsAdmin()
}

// IsReadAdmin getter
func (u *User) IsReadAdmin() bool {
	return u.GetRoles().IsReadAdmin()
}

// HasInvitationCode getter
func (u *User) HasInvitationCode() bool {
	return u.InvitationCode != ""
}
