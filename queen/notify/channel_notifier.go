package notify

import (
	"fmt"
	"io/ioutil"
	"sync"

	"github.com/sirupsen/logrus"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/config"
	"plexobject.com/formicary/queen/repository"
	"plexobject.com/formicary/queen/types"
	"plexobject.com/formicary/queen/utils"
)

// Notifier defines operations to notify job results
type Notifier interface {
	NotifyJob(
		qc *common.QueryContext,
		user *common.User,
		org *common.Organization,
		job *types.JobDefinition,
		request types.IJobRequest,
		lastRequestState common.RequestState) error
	SendEmailVerification(
		qc *common.QueryContext,
		user *common.User,
		org *common.Organization,
		ev *types.EmailVerification,
	) error
	EmailUserInvitation(
		qc *common.QueryContext,
		user *common.User,
		org *common.Organization,
		inv *types.UserInvitation,
	) error
}

// DefaultNotifier defines operations to send email
type DefaultNotifier struct {
	cfg                    *config.ServerConfig
	senders                map[common.NotifyChannel]types.Sender
	emailRepository        repository.EmailVerificationRepository
	jobsTemplates          map[string]string
	verifyEmailTemplate    string
	userInvitationTemplate string
	lock                   sync.RWMutex
}

// New constructor
func New(
	cfg *config.ServerConfig,
	emailRepository repository.EmailVerificationRepository,
) (*DefaultNotifier, error) {
	n := &DefaultNotifier{
		cfg:             cfg,
		senders:         make(map[common.NotifyChannel]types.Sender),
		emailRepository: emailRepository,
		jobsTemplates:   make(map[string]string),
	}
	if b, err := loadTemplate(cfg.Notify.VerifyEmailTemplateFile, cfg.PublicDir); err == nil {
		n.verifyEmailTemplate = string(b)
	} else {
		return nil, err
	}
	if b, err := loadTemplate(cfg.Notify.UserInvitationTemplateFile, cfg.PublicDir); err == nil {
		n.userInvitationTemplate = string(b)
	} else {
		return nil, err
	}

	return n, nil
}

// AddSender adds sender for channel
func (n *DefaultNotifier) AddSender(channel common.NotifyChannel, sender types.Sender) {
	n.senders[channel] = sender
}

// SendEmailVerification sends email with code to verify
func (n *DefaultNotifier) SendEmailVerification(
	qc *common.QueryContext,
	user *common.User,
	org *common.Organization,
	ev *types.EmailVerification) (err error) {
	if user == nil {
		return fmt.Errorf("user is not specified")
	}
	params := map[string]interface{}{
		"UserID":    ev.UserID,
		"Name":      user.Name,
		"User":      user,
		"URLPrefix": n.cfg.CommonConfig.ExternalBaseURL,
		"Email":     ev.Email,
		"EmailCode": ev.EmailCode,
		"VerifyID":  ev.ID,
		"Title":     "Email Verification",
	}

	msg, err := utils.ParseTemplate(n.verifyEmailTemplate, params)
	if err != nil {
		return err
	}

	sender := n.senders[common.EmailChannel]
	if sender == nil {
		logrus.WithFields(logrus.Fields{
			"Component":         "DefaultNotifier",
			"EmailVerification": ev,
			"User":              user,
			"Recipients":        ev.Email,
		}).Warnf("no email setup, ignoring sending email verification")
		return nil
	}
	if err = sender.SendMessage(
		qc,
		user,
		org,
		[]string{ev.Email},
		"Email Verification",
		msg,
		make(map[string]interface{})); err != nil {
		return err
	}

	logrus.WithFields(logrus.Fields{
		"Component":         "DefaultNotifier",
		"EmailVerification": ev,
		"User":              user,
		"Recipients":        ev.Email,
	}).Infof("sent email code for verification")
	return
}

// EmailUserInvitation sends email with invitation
func (n *DefaultNotifier) EmailUserInvitation(
	qc *common.QueryContext,
	user *common.User,
	org *common.Organization,
	inv *types.UserInvitation,
) error {
	if user == nil {
		return fmt.Errorf("user is not specified")
	}
	params := map[string]interface{}{
		"UserID":         user.ID,
		"Name":           user.Name,
		"User":           user,
		"URLPrefix":      n.cfg.CommonConfig.ExternalBaseURL,
		"Email":          inv.Email,
		"InvitationCode": inv.InvitationCode,
		"ID":             inv.ID,
		"Title":          "User Invitation",
	}

	msg, err := utils.ParseTemplate(n.userInvitationTemplate, params)
	if err != nil {
		return err
	}

	sender := n.senders[common.EmailChannel]
	if sender == nil {
		logrus.WithFields(logrus.Fields{
			"Component":      "DefaultNotifier",
			"UserID":         user.ID,
			"Name":           user.Name,
			"User":           user,
			"Email":          inv.Email,
			"InvitationCode": inv.InvitationCode,
			"ID":             inv.ID,
		}).Warnf("no email setup, ignoring sending user-invitation")
		return nil
	}
	if err = sender.SendMessage(
		qc,
		user,
		org,
		[]string{inv.Email},
		"Your are invited to the Formicary",
		msg,
		make(map[string]interface{})); err != nil {
		return err
	}

	logrus.WithFields(logrus.Fields{
		"Component":      "DefaultNotifier",
		"UserID":         user.ID,
		"Name":           user.Name,
		"User":           user,
		"Email":          inv.Email,
		"InvitationCode": inv.InvitationCode,
		"ID":             inv.ID,
	}).Infof("sent user invitation")
	return nil
}

// NotifyJob sends message to recipients
func (n *DefaultNotifier) NotifyJob(
	qc *common.QueryContext,
	user *common.User,
	org *common.Organization,
	job *types.JobDefinition,
	request types.IJobRequest,
	lastRequestState common.RequestState) (err error) {
	prefix := ""
	if request.GetJobState().Completed() && lastRequestState.Failed() {
		prefix = "Fixed "
	}
	subject := fmt.Sprintf("%sJob %s - %d %s", prefix, job.JobType, request.GetID(), request.GetJobState())

	link := fmt.Sprintf("%s/dashboard/jobs/requests/%d", n.cfg.CommonConfig.ExternalBaseURL, request.GetID())
	params := map[string]interface{}{
		"Job":       request,
		"URLPrefix": n.cfg.CommonConfig.ExternalBaseURL,
		"Title":     subject,
		"Link":      link,
	}

	if user != nil {
		params["User"] = user
	}
	opts := map[string]interface{}{
		types.Color: request.GetJobState().SlackColor(),
		types.Link:  link,
		types.Emoji: request.GetJobState().Emoji(),
	}

	var recipients []string
	var unverified []string
	var failed []string
	var senders []common.NotifyChannel
	total := 0
	jobNotify := job.Notify
	if len(jobNotify) == 0 && user != nil {
		jobNotify = user.Notify
	}
	var verifiedEmails map[string]bool

	whens := make([]common.NotifyWhen, 0)
	for k, v := range jobNotify {
		sender := n.senders[k]
		if sender == nil {
			return fmt.Errorf("no sender for %s", sender)
		}
		if len(v.Recipients) == 0 {
			continue
		}
		whens = append(whens, v.When)
		senders = append(senders, k)
		if v.When.Accept(request.GetJobState(), lastRequestState) {
			tmpl, err := n.loadJobsTemplate(sender)
			if err != nil {
				return err
			}
			msg, err := utils.ParseTemplate(tmpl, params)
			if err != nil {
				return err
			}

			for _, recipient := range v.Recipients {
				if k == common.EmailChannel && user != nil {
					if recipient != user.Email {
						if verifiedEmails == nil {
							verifiedEmails = n.emailRepository.GetVerifiedEmails(
								common.NewQueryContext("", "", "").WithAdmin(),
								user.ID)
						}
						if !verifiedEmails[recipient] {
							unverified = append(unverified, recipient)
							continue
						}
					}
				}
				if sendErr := sender.SendMessage(
					qc,
					user,
					org,
					[]string{recipient},
					subject,
					msg,
					opts); sendErr != nil {
					err = sendErr
					failed = append(failed, recipient)
				} else {
					recipients = append(recipients, recipient)
					total++
				}
			}
		}
	}

	logrus.WithFields(logrus.Fields{
		"Component":        "DefaultNotifier",
		"Senders":          senders,
		"Request":          request.GetID(),
		"LastRequestState": lastRequestState,
		"State":            request.GetJobState(),
		"Unverified":       unverified,
		"Failed":           failed,
		"Recipients":       recipients,
		"Whens":            whens,
		"Subject":          subject,
		"Total":            total,
		"Error":            err,
	}).Infof("notified job")
	return
}

func (n *DefaultNotifier) loadJobsTemplate(sender types.Sender) (string, error) {
	n.lock.Lock()
	defer n.lock.Unlock()
	body := n.jobsTemplates[sender.JobNotifyTemplateFile()]
	if body == "" {
		if b, err := loadTemplate(sender.JobNotifyTemplateFile(), n.cfg.PublicDir); err == nil {
			body = string(b)
			n.jobsTemplates[sender.JobNotifyTemplateFile()] = body
		} else {
			return "", err
		}
	}
	return body, nil
}

func loadTemplate(name string, dir string) ([]byte, error) {
	b, err := ioutil.ReadFile(name)
	if err != nil {
		b, err = ioutil.ReadFile(dir + name)
	}
	if err != nil {
		return nil, fmt.Errorf("error loading template: '%s' due to %s", name, err)
	}
	return b, nil
}
