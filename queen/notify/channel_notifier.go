package notify

import (
	"context"
	"fmt"
	"github.com/sirupsen/logrus"
	"io/ioutil"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/config"
	"plexobject.com/formicary/queen/repository"
	"plexobject.com/formicary/queen/types"
	"plexobject.com/formicary/queen/utils"
)

// Notifier defines operations to notify job results
type Notifier interface {
	NotifyJob(
		ctx context.Context,
		user *common.User,
		job *types.JobDefinition,
		request types.IJobRequest,
		lastRequestState common.RequestState) (string, error)
	SendEmailVerification(
		ctx context.Context,
		user *common.User,
		ev *types.EmailVerification,
	) (string, error)
}

// DefaultNotifier defines operations to send email
type DefaultNotifier struct {
	cfg                 *config.ServerConfig
	senders             map[common.NotifyChannel]types.Sender
	emailRepository     repository.EmailVerificationRepository
	jobsTemplate        string
	verifyEmailTemplate string
}

// New constructor
func New(
	cfg *config.ServerConfig,
	senders map[common.NotifyChannel]types.Sender,
	emailRepository repository.EmailVerificationRepository,
) (Notifier, error) {
	n := &DefaultNotifier{
		cfg:             cfg,
		senders:         senders,
		emailRepository: emailRepository,
	}
	if b, err := loadTemplate(cfg.Email.JobsTemplateFile, cfg.PublicDir); err == nil {
		n.jobsTemplate = string(b)
	} else {
		return nil, err
	}

	if b, err := loadTemplate(cfg.Email.VerifyEmailTemplateFile, cfg.PublicDir); err == nil {
		n.verifyEmailTemplate = string(b)
	} else {
		return nil, err
	}

	return n, nil
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

// SendEmailVerification sends email with code to verify
func (n *DefaultNotifier) SendEmailVerification(
	_ context.Context,
	user *common.User,
	ev *types.EmailVerification) (msg string, err error) {
	if user == nil {
		return "", fmt.Errorf("user is not specified")
	}
	params := map[string]interface{}{
		"UserID":    ev.UserID,
		"Name":      user.Name,
		"User":      user,
		"Email":     ev.Email,
		"EmailCode": ev.EmailCode,
		"URLPrefix": n.cfg.CommonConfig.ExternalBaseURL,
		"Title":     "Email Verification",
	}

	msg, err = utils.ParseTemplate(n.verifyEmailTemplate, params)
	if err != nil {
		return "", err
	}

	sender := n.senders[common.EmailChannel]
	if sender == nil {
		logrus.WithFields(logrus.Fields{
			"Component":         "DefaultNotifier",
			"EmailVerification": ev,
			"User":              user,
			"Recipients":        ev.Email,
		}).Warnf("no email setup, ignoring sending email verification")
		return "", nil
	}
	if err = sender.SendMessage([]string{ev.Email}, "Email Verification", msg); err != nil {
		return "", err
	}

	logrus.WithFields(logrus.Fields{
		"Component":         "DefaultNotifier",
		"EmailVerification": ev,
		"User":              user,
		"Recipients":        ev.Email,
	}).Infof("sent email code for verification")
	return
}

// NotifyJob sends message to recipients
func (n *DefaultNotifier) NotifyJob(
	_ context.Context,
	user *common.User,
	job *types.JobDefinition,
	request types.IJobRequest,
	lastRequestState common.RequestState) (msg string, err error) {
	prefix := ""
	if request.GetJobState().Completed() && lastRequestState.Failed() {
		prefix = "Fixed "
	}
	subject := fmt.Sprintf("%sJob %s - %d %s", prefix, job.JobType, request.GetID(), request.GetJobState())

	params := map[string]interface{}{
		"Job":       request,
		"URLPrefix": n.cfg.CommonConfig.ExternalBaseURL,
		"Title":     subject,
	}

	if user != nil {
		params["User"] = user
	}

	msg, err = utils.ParseTemplate(n.jobsTemplate, params)
	if err != nil {
		return "", err
	}

	var recipients []string
	var unverified []string
	var failed []string
	var senders []common.NotifyChannel
	total := 0
	notify := job.Notify
	if len(notify) == 0 && user != nil {
		notify = user.Notify
	}
	var verifiedEmails map[string]bool

	whens := make([]common.NotifyWhen, 0)
	for k, v := range notify {
		sender := n.senders[k]
		if sender == nil {
			return "", fmt.Errorf("no sender for %s", sender)
		}
		whens = append(whens, v.When)
		senders = append(senders, k)
		if v.When.Accept(request.GetJobState(), lastRequestState) {
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
				if sendErr := sender.SendMessage([]string{recipient}, subject, msg); sendErr != nil {
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
