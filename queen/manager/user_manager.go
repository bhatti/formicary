package manager

import (
	"fmt"
	"github.com/sirupsen/logrus"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/config"
	"plexobject.com/formicary/queen/notify"
	"plexobject.com/formicary/queen/repository"
	"plexobject.com/formicary/queen/security"
	"plexobject.com/formicary/queen/types"
	"sort"
	"strings"
	"time"
)

// UserManager for managing state of request and its execution
type UserManager struct {
	serverCfg                   *config.ServerConfig
	auditRecordRepository       repository.AuditRecordRepository
	userRepository              repository.UserRepository
	orgRepository               repository.OrganizationRepository
	orgConfigRepository         repository.OrganizationConfigRepository
	invRepository               repository.InvitationRepository
	emailVerificationRepository repository.EmailVerificationRepository
	subscriptionRepository      repository.SubscriptionRepository
	jobExecRepository           repository.JobExecutionRepository
	artifactRepository          repository.ArtifactRepository
	notifier                    notify.Notifier
}

// NewUserManager manages job request, definition and execution
func NewUserManager(
	serverCfg *config.ServerConfig,
	auditRecordRepository repository.AuditRecordRepository,
	userRepository repository.UserRepository,
	orgRepository repository.OrganizationRepository,
	orgConfigRepository repository.OrganizationConfigRepository,
	invRepository repository.InvitationRepository,
	emailVerificationRepository repository.EmailVerificationRepository,
	subscriptionRepository repository.SubscriptionRepository,
	jobExecRepository repository.JobExecutionRepository,
	artifactRepository repository.ArtifactRepository,
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
	if orgConfigRepository == nil {
		return nil, fmt.Errorf("org-config-repository is not specified")
	}
	if invRepository == nil {
		return nil, fmt.Errorf("invitation-repository is not specified")
	}
	if jobExecRepository == nil {
		return nil, fmt.Errorf("jobExecution-repository is not specified")
	}
	if artifactRepository == nil {
		return nil, fmt.Errorf("artifact-repository is not specified")
	}
	if notifier == nil {
		return nil, fmt.Errorf("notifier is not specified")
	}

	return &UserManager{
		serverCfg:                   serverCfg,
		auditRecordRepository:       auditRecordRepository,
		userRepository:              userRepository,
		orgRepository:               orgRepository,
		orgConfigRepository:         orgConfigRepository,
		invRepository:               invRepository,
		emailVerificationRepository: emailVerificationRepository,
		subscriptionRepository:      subscriptionRepository,
		jobExecRepository:           jobExecRepository,
		artifactRepository:          artifactRepository,
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

// AddStickyMessageForEmail updates sticky message for email failure
func (m *UserManager) AddStickyMessageForEmail(
	qc *common.QueryContext,
	user *common.User,
	err error) error {
	if user != nil && user.StickyMessage == "" {
		user.StickyMessage = fmt.Sprintf("email-error: %s", err)
		return m.UpdateStickyMessage(qc, user)
	}
	return nil
}

// ClearStickyMessageForEmail updates sticky message for email success
func (m *UserManager) ClearStickyMessageForEmail(
	qc *common.QueryContext,
	user *common.User,
) error {
	if user != nil && strings.Contains(user.StickyMessage, "email-error") {
		user.StickyMessage = ""
		return m.UpdateStickyMessage(qc, user)
	}
	return nil
}

// AddStickyMessageForSlack updates sticky message for slack failure
func (m *UserManager) AddStickyMessageForSlack(
	qc *common.QueryContext,
	user *common.User,
	err error) error {
	if user != nil && user.StickyMessage == "" {
		user.StickyMessage = fmt.Sprintf("slack-error: Slack messages could not be sent due to '%s'", err)
		return m.UpdateStickyMessage(qc, user)
	}
	return nil
}

// ClearStickyMessageForSlack updates sticky message for slack success
func (m *UserManager) ClearStickyMessageForSlack(
	qc *common.QueryContext,
	user *common.User,
) error {
	if user != nil && strings.Contains(user.StickyMessage, "slack-error") {
		user.StickyMessage = ""
		return m.UpdateStickyMessage(qc, user)
	}
	return nil
}

// UpdateStickyMessage updates sticky message
func (m *UserManager) UpdateStickyMessage(
	qc *common.QueryContext,
	user *common.User,
) error {
	err := m.orgRepository.UpdateStickyMessage(qc, user)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"Component":     "UserManager",
			"User":          user,
			"Organization":  user.Organization,
			"StickyMessage": user.StickyMessage,
			"Error":         err,
		}).Warnf("failed to set sticky message")
	} else {
		logrus.WithFields(logrus.Fields{
			"Component":     "UserManager",
			"User":          user,
			"Organization":  user.Organization,
			"StickyMessage": user.StickyMessage,
		}).Infof("updated sticky message")
	}
	return err
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
	token string,
) (*types.UserToken, error) {
	tok := types.NewUserToken(qc.User, token)
	strTok, expiration, err := security.BuildToken(
		qc.User,
		m.serverCfg.Auth.JWTSecret,
		m.serverCfg.Auth.TokenMaxAge)
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

	if _, verifyErr := m.CreateEmailVerification(qc, types.NewEmailVerification(user.Email, user)); verifyErr != nil {
		logrus.WithFields(logrus.Fields{
			"Component": "UserManager",
			"User":      user,
			"Email":     user.Email,
			"Error":     verifyErr,
		}).Errorf("failed to send email verification")
	}

	if subscription, err := m.subscriptionRepository.Create(
		qc,
		common.NewFreemiumSubscription(saved)); err == nil {
		logrus.WithFields(logrus.Fields{
			"Component":    "UserManager",
			"Subscription": subscription,
		}).Info("created Subscription")
		_, _ = m.auditRecordRepository.Save(types.NewAuditRecordFromSubscription(subscription, qc))
	} else {
		logrus.WithFields(logrus.Fields{
			"Component":    "UserManager",
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

	subscription := common.NewFreemiumSubscription(user)
	if subscription, err = m.subscriptionRepository.Create(
		qc,
		subscription); err == nil {
		logrus.WithFields(logrus.Fields{
			"Component":    "UserManager",
			"Subscription": subscription,
		}).Info("created Subscription")
		_, _ = m.auditRecordRepository.Save(types.NewAuditRecordFromSubscription(subscription, qc))
	} else {
		logrus.WithFields(logrus.Fields{
			"Component":    "UserManager",
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

// GetSlackToken returns slack token
func (m *UserManager) GetSlackToken(
	qc *common.QueryContext,
	org *common.Organization,
) (token string, err error) {
	if !qc.HasOrganization() {
		return "", nil
	}
	if org != nil {
		return org.GetConfigString(types.SlackToken), nil
	}
	recs, _, err := m.orgConfigRepository.Query(
		qc,
		map[string]interface{}{"name": types.SlackToken},
		0,
		1,
		make([]string, 0),
	)
	if err != nil {
		return "", err
	}
	if len(recs) == 0 {
		return "", nil
	}
	return recs[0].Value, nil
}

// UpdateUserNotification updates user settings for notification
func (m *UserManager) UpdateUserNotification(
	qc *common.QueryContext,
	id string,
	email string,
	slackChannel string,
	slackToken string,
	when string,
) (user *common.User, err error) {
	user, err = m.GetUser(qc, id)
	if err != nil {
		return nil, err
	}
	slackChannel = strings.TrimSpace(slackChannel)
	email = strings.TrimSpace(email)
	notifyWhen := common.NotifyWhen(strings.TrimSpace(when))
	if email == "" && slackChannel == "" {
		return user, common.NewValidationError("no email or slack channel specified")
	}
	user.Notify = make(map[common.NotifyChannel]common.JobNotifyConfig)

	if email != "" {
		err = user.SetNotifyEmail(email, notifyWhen)
		if err != nil {
			return user, err
		}
	}
	if slackChannel != "" {
		err = user.SetNotifyChannel(slackChannel, notifyWhen)
		if err != nil {
			return user, err
		}
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

	if slackToken != "" && qc.HasOrganization() {
		cfg, err := common.NewOrganizationConfig(
			qc.GetOrganizationID(),
			types.SlackToken,
			slackToken,
			true)
		if err != nil {
			return saved, err
		}
		cfg, err = m.orgConfigRepository.Save(qc, cfg)
		if err != nil {
			return saved, err
		}
		_, _ = m.auditRecordRepository.Save(types.NewAuditRecordFromOrganizationConfig(cfg, qc))
	}

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
) (err error) {
	if user == nil {
		return fmt.Errorf("failed to find user in session for invitation")
	}
	if !user.HasOrganization() {
		return fmt.Errorf("user does not belong to organization")
	}
	inv.InvitedByUserID = user.ID
	inv.OrganizationID = user.OrganizationID
	inv.OrgUnit = user.Organization.OrgUnit

	if err = m.invRepository.Create(inv); err != nil {
		return err
	}
	err = m.notifier.EmailUserInvitation(
		qc,
		user,
		inv)
	if err != nil {
		_ = m.invRepository.Delete(inv.ID)
		return err
	}
	_, _ = m.auditRecordRepository.Save(types.NewAuditRecordFromInvite(inv, qc))
	logrus.WithFields(logrus.Fields{
		"Component":  "UserManager",
		"Admin":      user.IsAdmin(),
		"User":       user,
		"Org":        user.Organization,
		"Invitation": inv,
	}).Infof("user invited")
	return nil
}

// GetInvitation get invitation
func (m *UserManager) GetInvitation(
	id string) (*types.UserInvitation, error) {
	return m.invRepository.Get(id)
}

// QueryInvitations query invitations
func (m *UserManager) QueryInvitations(
	qc *common.QueryContext,
	params map[string]interface{},
	page int,
	pageSize int,
	order []string,
) ([]*types.UserInvitation, int64, error) {
	return m.invRepository.Query(qc, params, page, pageSize, order)
}

// BuildOrgWithInvitation checks existing when signing up
func (m *UserManager) BuildOrgWithInvitation(
	user *common.User) (org *common.Organization, err error) {
	if !user.HasOrganizationOrInvitationCode() {
		return
	}
	qc := common.NewQueryContext(nil, "")
	if user.InvitationCode != "" {
		if inv, err := m.invRepository.Accept(user.Email, user.InvitationCode); err == nil {
			org, err = m.orgRepository.Get(qc, inv.OrganizationID)
			if err != nil {
				return nil, fmt.Errorf("failed to find organization in invitation %s due to %w", inv.OrganizationID, err)
			}
			user.OrganizationID = org.ID
			user.BundleID = org.BundleID
			user.OrgUnit = org.OrgUnit
			logrus.WithFields(logrus.Fields{
				"Component":  "UserManager",
				"Org":        org,
				"User":       user,
				"Invitation": inv,
			}).Infof("accepted invitation")
		} else {
			logrus.WithFields(logrus.Fields{
				"Component": "UserManager",
				"Org":       org,
				"User":      user,
				"Error":     err,
			}).Warnf("failed to accept invitation")
			err = fmt.Errorf("invitation-code is not valid, please contact admin of your organization to re-invite you to the organization")
			user.Errors["OrgUnit"] = err.Error()
			return nil, err
		}
	} else {
		org, _ = m.orgRepository.GetByUnit(qc, user.OrgUnit)
		if org != nil {
			err = fmt.Errorf("organization already exists, please contact admin of your organization to invite you to the organization")
			user.Errors["OrgUnit"] = err.Error()
			return nil, err
		}
		if user.BundleID == "" {
			err = fmt.Errorf("bundleID is not specified")
			user.Errors["BundleID"] = err.Error()
			return nil, err
		}
		org = common.NewOrganization(user.ID, user.OrgUnit, user.BundleID)
	}
	return
}

// GetCPUResourcesByOrgUser returns cpu usage by org/user
func (m *UserManager) GetCPUResourcesByOrgUser(
	ranges []types.DateRange,
	limit int,
) ([]types.ResourceUsage, error) {
	return m.jobExecRepository.GetResourceUsageByOrgUser(ranges, limit)
}

// GetStorageResourcesByOrgUser returns disk usage by org/user
func (m *UserManager) GetStorageResourcesByOrgUser(
	ranges []types.DateRange,
	limit int,
) ([]types.ResourceUsage, error) {
	return m.artifactRepository.GetResourceUsageByOrgUser(ranges, limit)
}

// CombinedResourcesByOrgUser returns combined disk/cpu usage by org/user
func (m *UserManager) CombinedResourcesByOrgUser(
	from time.Time,
	to time.Time,
	_ int,
) []types.CombinedResourceUsage {
	ranges := []types.DateRange{
		{
			StartDate: from,
			EndDate:   to,
		},
	}

	usageLookup := make(map[string]types.CombinedResourceUsage)
	if cpuUsage, err := m.GetCPUResourcesByOrgUser(
		ranges, 10000); err == nil {
		for _, usage := range cpuUsage {
			addUsageRecord(usage, types.CPUResource, usageLookup)
		}
	}

	if storageUsage, err := m.GetStorageResourcesByOrgUser(
		ranges, 10000); err == nil {
		for _, usage := range storageUsage {
			addUsageRecord(usage, types.DiskResource, usageLookup)
		}
	}
	combinedUsage := make([]types.CombinedResourceUsage, len(usageLookup))
	i := 0
	for _, usage := range usageLookup {
		combinedUsage[i] = usage
		i++
	}
	sort.Slice(combinedUsage, func(i, j int) bool { return combinedUsage[i].CPUResource.Value > combinedUsage[j].CPUResource.Value })
	return combinedUsage
}

func addUsageRecord(
	usage types.ResourceUsage,
	resourceType types.ResourceUsageType,
	usageLookup map[string]types.CombinedResourceUsage) {
	var record types.CombinedResourceUsage
	if usage.OrganizationID == "" {
		record = usageLookup[usage.UserID]
	} else {
		record = usageLookup[usage.OrganizationID]
	}
	record.UserID = usage.UserID
	record.OrganizationID = usage.OrganizationID
	if resourceType == types.CPUResource {
		record.CPUResource = usage
	} else {
		record.DiskResource = usage
	}
	if usage.OrganizationID == "" {
		usageLookup[usage.UserID] = record
	} else {
		usageLookup[usage.OrganizationID] = record
	}
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
	user *common.User,
) map[string]bool {
	return m.emailVerificationRepository.GetVerifiedEmails(qc, user)
}

// CreateEmailVerification adds email verifications
func (m *UserManager) CreateEmailVerification(
	qc *common.QueryContext,
	emailVerification *types.EmailVerification) (*types.EmailVerification, error) {
	user, err := m.GetUser(qc, emailVerification.UserID)
	if err != nil {
		return nil, err
	}
	saved, err := m.emailVerificationRepository.Create(emailVerification)
	if err != nil {
		return nil, err
	}
	err = m.notifier.SendEmailVerification(
		qc,
		user,
		saved)
	if err != nil {
		_ = m.emailVerificationRepository.Delete(qc, emailVerification.ID)
		return nil, err
	}
	_, _ = m.auditRecordRepository.Save(types.NewAuditRecordFromEmailVerification(saved, types.EmailVerificationCreated, qc))

	logrus.WithFields(logrus.Fields{
		"Component":         "UserManager",
		"EmailVerification": saved,
		"Code":              saved.EmailCode,
		"Verified":          saved.VerifiedAt,
		"Expires":           saved.ExpiresAt,
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
	rec, err = m.emailVerificationRepository.Verify(qc, user, code)
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
	} else {
		logrus.WithFields(logrus.Fields{
			"Component":         "UserManager",
			"User":              user,
			"EmailVerification": rec,
			"Error":             err,
		}).Warnf("failed to verify email")
	}
	return
}

/////////////////////////////////////////// RESOURCES METHODS ////////////////////////////////////////////

// GetCPUResourceUsage usage
func (m *UserManager) GetCPUResourceUsage(
	qc *common.QueryContext,
	ranges []types.DateRange) ([]types.ResourceUsage, error) {
	return m.jobExecRepository.GetResourceUsage(qc, ranges)
}

// GetStorageResourceUsage usage
func (m *UserManager) GetStorageResourceUsage(
	qc *common.QueryContext,
	ranges []types.DateRange) ([]types.ResourceUsage, error) {
	return m.artifactRepository.GetResourceUsage(qc, ranges)
}
