package notify

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"io/ioutil"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/config"
	"plexobject.com/formicary/queen/types"
	"plexobject.com/formicary/queen/utils"
)

// JobNotifier defines operations to notify job results
type JobNotifier interface {
	NotifyJob(user *common.User, job *types.JobDefinition, request types.IJobRequest) (string, error)
}

// DefaultJobNotifier defines operations to send email
type DefaultJobNotifier struct {
	cfg      *config.ServerConfig
	senders  map[string]types.Sender
	template string
}

// New constructor
func New(
	cfg *config.ServerConfig,
	senders map[string]types.Sender,
) (JobNotifier, error) {
	b, err := ioutil.ReadFile(cfg.Email.JobsTemplateFile)
	if err != nil {
		return nil, fmt.Errorf("error loading jobs_template_file: '%s' due to %s", cfg.Email.JobsTemplateFile, err)
	}

	return &DefaultJobNotifier{
		cfg:      cfg,
		senders:  senders,
		template: string(b),
	}, nil
}

// NotifyJob sends message to recipients
func (n *DefaultJobNotifier) NotifyJob(
	user *common.User,
	job *types.JobDefinition,
	request types.IJobRequest) (msg string, err error) {
	params := map[string]interface{}{
		"Job":       request,
		"URLPrefix": n.cfg.CommonConfig.ExternalBaseURL,
	}
	if user != nil {
		params["User"] = user
	}
	msg, err = utils.ParseTemplate(n.template, params)
	if err != nil {
		return "", err
	}
	subject := fmt.Sprintf("Formicary Job %s - %d %s", job.JobType, request.GetID(), request.GetJobState())
	var recipients []string
	var senders []string
	total := 0
	notify := job.Notify
	if len(notify) == 0 && user != nil {
		notify = user.Notify
	}
	for k, v := range notify {
		sender := n.senders[k]
		if sender == nil {
			return "", fmt.Errorf("no sender for %s", sender)
		}
		if v.When.Accept(request.GetJobState()) {
			if err = sender.SendMessage(v.Recipients, subject, msg); err != nil {
				return "", err
			}
			total++
			for _, recipient := range v.Recipients {
				recipients = append(recipients, recipient)
			}
			senders = append(senders, k)
		}
	}

	logrus.WithFields(logrus.Fields{
		"Component":  "DefaultJobNotifier",
		"Senders":    senders,
		"Request":    request.GetID(),
		"State":      request.GetJobState(),
		"Recipients": recipients,
		"Subject":    subject,
		"Total":      total,
	}).Infof("notified job")
	return
}
