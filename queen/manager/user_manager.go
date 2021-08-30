package manager

import (
	"context"
	"fmt"
	"github.com/sirupsen/logrus"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/config"
	"plexobject.com/formicary/queen/notify"
	"plexobject.com/formicary/queen/repository"
	"plexobject.com/formicary/queen/security"
	"plexobject.com/formicary/queen/types"
	"strings"
)

// UserManager for managing state of request and its execution
type UserManager struct {
	serverCfg                   *config.ServerConfig
	auditRecordRepository       repository.AuditRecordRepository
	userRepository              repository.UserRepository
	orgRepository               repository.OrganizationRepository
	emailVerificationRepository repository.EmailVerificationRepository
	subscriptionRepository      repository.SubscriptionRepository
	notifier                    notify.Notifier
}

// NewUserManager manages job request, definition and execution
func NewUserManager(
	serverCfg *config.ServerConfig,
	auditRecordRepository repository.AuditRecordRepository,
	userRepository repository.UserRepository,
	orgRepository repository.OrganizationRepository,
	emailVerificationRepository repository.EmailVerificationRepository,
	subscriptionRepository repository.SubscriptionRepository,
	notifier notify.Notifier) (*UserManager, error) {
	if serverCfg == nil {
		return nil, fmt.Errorf("server-config is not specified")
	}
	if auditRecordRepository == nil {
		return nil, fmt.Errorf("audit-repository is not specified")
	}
	if userRepository == nil {
		return nil, fmt.Errorf("user-repository is not specified")
	}
	if emailVerificationRepository == nil {
		return nil, fmt.Errorf("email-verification-repository is not specified")
	}
	if subscriptionRepository == nil {
		return nil, fmt.Errorf("subscription-repository is not specified")
	}
	if orgRepository == nil {
		return nil, fmt.Errorf("org-repository is not specified")
	}
	if notifier == nil {
		return nil, fmt.Errorf("notifier is not specified")
	}

	return &UserManager{
		serverCfg:                   serverCfg,
		auditRecordRepository:       auditRecordRepository,
		userRepository:              userRepository,
		orgRepository:               orgRepository,
		emailVerificationRepository: emailVerificationRepository,
		subscriptionRepository:      subscriptionRepository,
		notifier:                    notifier,
	}, nil
}

/////////////////////////////////////////// USER METHODS ////////////////////////////////////////////

// QueryUsers find users
func (m *UserManager) QueryUsers(
	qc *common.QueryContext,
	params map[string]interface{},
	page int,
	pageSize int,
	order []string) (recs []*common.User, totalRecords int64, err error) {
	return m.userRepository.Query(qc, params, page, pageSize, order)
}

// SaveAudit - save persists audit-record
func (m *UserManager) SaveAudit(
	record *types.AuditRecord) (*types.AuditRecord, error) {
	return m.auditRecordRepository.Save(record)
}

// CreateEmailNotification starts process for email verification
func (m *UserManager) CreateEmailNotification(
	ev *types.EmailVerification) (*types.EmailVerification, error) {
	return m.emailVerificationRepository.Create(ev)
}

// UpdateStickyMessage updates sticky message
func (m *UserManager) UpdateStickyMessage(
	qc *common.QueryContext,
	user *common.User,
	org *common.Organization) error {
	return m.orgRepository.UpdateStickyMessage(qc, user, org)
}

// GetUser find user by id
func (m *UserManager) GetUser(
	qc *common.QueryContext,
	userID string,
) (*common.User, error) {
	return m.userRepository.Get(qc, userID)
}

// DeleteUser deletes user by id
func (m *UserManager) DeleteUser(
	qc *common.QueryContext,
	userID string,
) error {
	return m.userRepository.Delete(qc, userID)
}

// GetUserTokens finds tokens
func (m *UserManager) GetUserTokens(
	qc *common.QueryContext,
	userID string,
) ([]*types.UserToken, error) {
	return m.userRepository.GetTokens(qc, userID)
}

// RevokeUserToken revokes given token-id
func (m *UserManager) RevokeUserToken(
	qc *common.QueryContext,
	userID string,
	token string,
) error {
	return m.userRepository.RevokeToken(qc, userID, token)
}

// CreateUserToken adds new token
func (m *UserManager) CreateUserToken(
	qc *common.QueryContext,
	user *common.User,
	token string,
) (*types.UserToken, error) {
	tok := types.NewUserToken(qc.UserID, qc.OrganizationID, token)
	strTok, expiration, err := security.BuildToken(user, m.serverCfg.Auth.JWTSecret, m.serverCfg.Auth.TokenMaxAge)
	if err != nil {
		return nil, err
	}
	tok.APIToken = strTok
	tok.ExpiresAt = expiration
	err = m.userRepository.AddToken(tok)
	if err != nil {
		return nil, err
	}
	_, _ = m.auditRecordRepository.Save(types.NewAuditRecordFromToken(tok, qc))
	return tok, nil
}

// CreateUser creates new user
func (m *UserManager) CreateUser(
	qc *common.QueryContext,
	user *common.User) (*common.User, error) {
	saved, err := m.userRepository.Create(user)
	if err != nil {
		return nil, err
	}
	_, _ = m.auditRecordRepository.Save(types.NewAuditRecordFromUser(saved, types.UserUpdated, qc))

	subscription := common.NewFreemiumSubscription(saved.ID, saved.OrganizationID)
	if subscription, err = m.subscriptionRepository.Create(subscription); err == nil {
		logrus.WithFields(logrus.Fields{
			"Component":    "SubscriptionController",
			"Subscription": subscription,
		}).Info("created Subscription")
		_, _ = m.auditRecordRepository.Save(types.NewAuditRecordFromSubscription(subscription, qc))
	} else {
		logrus.WithFields(logrus.Fields{
			"Component":    "SubscriptionController",
			"Subscription": subscription,
			"Error":        err,
		}).Errorf("failed to create Subscription")
	}
	return saved, nil
}

// PostSignup handles post signup
func (m *UserManager) PostSignup(
	qc *common.QueryContext,
	user *common.User) (err error) {
	_, _ = m.auditRecordRepository.Save(types.NewAuditRecordFromUser(user, types.UserSignup, qc))

	subscription := common.NewFreemiumSubscription(user.ID, user.OrganizationID)
	if subscription, err = m.subscriptionRepository.Create(subscription); err == nil {
		logrus.WithFields(logrus.Fields{
			"Component":    "UserAdminController",
			"Subscription": subscription,
		}).Info("created Subscription")
		_, _ = m.auditRecordRepository.Save(types.NewAuditRecordFromSubscription(subscription, qc))
	} else {
		logrus.WithFields(logrus.Fields{
			"Component":    "UserAdminController",
			"Subscription": subscription,
			"Error":        err,
		}).Errorf("failed to create Subscription")
	}
	return
}

// UpdateUser updates existing user
func (m *UserManager) UpdateUser(
	qc *common.QueryContext,
	user *common.User) (*common.User, error) {
	saved, err := m.userRepository.Update(qc, user)
	if err != nil {
		return nil, err
	}
	_, _ = m.auditRecordRepository.Save(types.NewAuditRecordFromUser(saved, types.UserUpdated, qc))
	return saved, nil
}

// UpdateUserNotification updates user settings for notification
func (m *UserManager) UpdateUserNotification(
	qc *common.QueryContext,
	id string,
	email string,
	when string,
) (user *common.User, err error) {
	user, err = m.GetUser(qc, id)
	if err != nil {
		return nil, err
	}
	user.NotifyEmail = strings.TrimSpace(email)
	user.NotifyWhen = common.NotifyWhen(strings.TrimSpace(when))
	if user.NotifyEmail == "" {
		return user, common.NewValidationError("no email specified")
	}
	var notifyCfg common.JobNotifyConfig
	notifyCfg, err = common.JobNotifyConfigWithEmail(user.NotifyEmail, user.NotifyWhen)
	if err != nil {
		return user, err
	}
	user.Notify = map[common.NotifyChannel]common.JobNotifyConfig{
		common.EmailChannel: notifyCfg,
	}
	err = user.Validate()
	if err != nil {
		return user, err
	}
	saved, err := m.userRepository.Update(qc, user)
	if err != nil {
		return user, err
	}

	_, _ = m.auditRecordRepository.Save(types.NewAuditRecordFromUser(user, types.UserUpdated, qc))
	return saved, nil
}

/////////////////////////////////////////// ORG METHODS ////////////////////////////////////////////

// GetOrganization find org by id
func (m *UserManager) GetOrganization(
	qc *common.QueryContext,
	id string,
) (*common.Organization, error) {
	return m.orgRepository.Get(qc, id)
}

// DeleteOrganization deletes org by id
func (m *UserManager) DeleteOrganization(
	qc *common.QueryContext,
	id string,
) error {
	return m.orgRepository.Delete(qc, id)
}

// QueryOrgs find orgs
func (m *UserManager) QueryOrgs(
	qc *common.QueryContext,
	params map[string]interface{},
	page int,
	pageSize int,
	order []string) (recs []*common.Organization, totalRecords int64, err error) {
	return m.orgRepository.Query(qc, params, page, pageSize, order)
}

// CreateOrg adds new org
func (m *UserManager) CreateOrg(
	qc *common.QueryContext,
	org *common.Organization,
) (*common.Organization, error) {
	saved, err := m.orgRepository.Create(qc, org)
	if err != nil {
		return nil, err
	}
	_, _ = m.auditRecordRepository.Save(types.NewAuditRecordFromOrganization(saved, qc))
	return saved, nil
}

// UpdateOrg updates existing org
func (m *UserManager) UpdateOrg(
	qc *common.QueryContext,
	org *common.Organization,
) (*common.Organization, error) {
	saved, err := m.orgRepository.Update(qc, org)
	if err != nil {
		return nil, err
	}
	_, _ = m.auditRecordRepository.Save(types.NewAuditRecordFromOrganization(saved, qc))
	return saved, nil
}

// InviteUser invites user to organization
func (m *UserManager) InviteUser(
	qc *common.QueryContext,
	user *common.User,
	inv *types.UserInvitation,
) error {
	if user == nil {
		return fmt.Errorf("failed to find user in session for invitation")
	}
	orgID := user.OrganizationID
	if orgID == "" {
		return fmt.Errorf("organization is not available for invitation")
	}
	inv.InvitedByUserID = user.ID
	inv.OrganizationID = orgID
	if err := m.orgRepository.AddInvitation(inv); err != nil {
		return err
	}
	_, _ = m.auditRecordRepository.Save(types.NewAuditRecordFromInvite(inv, qc))
	logrus.WithFields(logrus.Fields{
		"Component":  "OrganizationAdminController",
		"Admin":      user.Admin,
		"Org":        orgID,
		"User":       user,
		"Invitation": inv,
	}).Infof("user invited")
	return nil
}

// BuildOrgWithInvitation checks existing when signing up
func (m *UserManager) BuildOrgWithInvitation(
	user *common.User) (org *common.Organization, err error) {
	qc := common.NewQueryContext("", "", "")
	if user.OrgUnit != "" {
		org, _ = m.orgRepository.GetByUnit(qc, user.OrgUnit)
		if org != nil {
			needInvitation := true
			if user.InvitationCode != "" {
				if inv, err := m.orgRepository.AcceptInvitation(user.Email, user.InvitationCode); err == nil {
					org, err = m.orgRepository.Get(qc, inv.OrganizationID)
					if err != nil {
						return nil, fmt.Errorf("failed to find organization in invitation %s due to %s",
							inv.OrganizationID, err.Error())
					}
					needInvitation = false
					user.OrganizationID = org.ID
				}
			}
			if needInvitation {
				user.Errors["OrgUnit"] = "Organization already exists, please contact admin of your organization to invite you to this organization."
				return nil, fmt.Errorf("organization already exists, please contact admin of your organization to invite you to this organization")
			}
		} else {
			org = common.NewOrganization(user.ID, user.OrgUnit, user.BundleID)
		}
	}
	return
}

/////////////////////////////////////////// USER METHODS ////////////////////////////////////////////

// QueryEmailVerifications finds email verifications
func (m *UserManager) QueryEmailVerifications(
	qc *common.QueryContext,
	params map[string]interface{},
	page int,
	pageSize int,
	order []string) (recs []*types.EmailVerification, totalRecords int64, err error) {
	return m.emailVerificationRepository.Query(qc, params, page, pageSize, order)
}

// GetVerifiedEmailByID finds verified email record by id
func (m *UserManager) GetVerifiedEmailByID(
	qc *common.QueryContext,
	userID string,
) (*types.EmailVerification, error) {
	return m.emailVerificationRepository.Get(qc, userID)
}

// GetVerifiedEmails finds verified emails
func (m *UserManager) GetVerifiedEmails(
	qc *common.QueryContext,
	userID string,
) map[string]bool {
	return m.emailVerificationRepository.GetVerifiedEmails(qc, userID)
}

// CreateEmailVerifications adds email verifications
func (m *UserManager) CreateEmailVerifications(
	qc *common.QueryContext,
	emailVerification *types.EmailVerification) (*types.EmailVerification, error) {
	user, err := m.userRepository.Get(qc, emailVerification.UserID)
	if err != nil {
		return nil, err
	}
	saved, err := m.emailVerificationRepository.Create(emailVerification)
	if err != nil {
		return nil, err
	}
	_, err = m.notifier.SendEmailVerification(context.Background(), user, saved)
	if err != nil {
		_ = m.emailVerificationRepository.Delete(qc, emailVerification.ID)
		return nil, err
	}
	_, _ = m.auditRecordRepository.Save(types.NewAuditRecordFromEmailVerification(saved, types.EmailVerificationCreated, qc))

	logrus.WithFields(logrus.Fields{
		"Component":         "UserManager",
		"EmailVerification": saved,
	}).Infof("created email verification")
	return saved, nil
}

// VerifyEmail verify email verifications
func (m *UserManager) VerifyEmail(
	qc *common.QueryContext,
	userID string,
	code string,
) (rec *types.EmailVerification, err error) {
	user, err := m.userRepository.Get(qc, userID)
	if err != nil {
		return nil, err
	}
	rec, err = m.emailVerificationRepository.Verify(qc, userID, code)
	if err == nil {
		_, _ = m.auditRecordRepository.Save(types.NewAuditRecordFromEmailVerification(rec, types.EmailVerificationVerified, qc))
		if !user.EmailVerified && user.Email == rec.Email {
			user.EmailVerified = true
			user, err = m.userRepository.Update(qc, user)
		}
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"Component":         "UserManager",
				"User":              user,
				"EmailVerification": rec,
				"Error":             err,
			}).Warnf("verified email verification but failed to update user")
		} else {
			logrus.WithFields(logrus.Fields{
				"Component":         "UserManager",
				"User":              user,
				"EmailVerification": rec,
			}).Infof("verified email verification")
		}
	}
	return
}
