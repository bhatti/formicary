package notify

import (
	"context"
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
	NotifyJob(
		ctx context.Context,
		user *common.User,
		job *types.JobDefinition,
		request types.IJobRequest,
		lastRequestState common.RequestState) (string, error)
}

// DefaultJobNotifier defines operations to send email
type DefaultJobNotifier struct {
	cfg      *config.ServerConfig
	senders  map[common.NotifyChannel]types.Sender
	template string
}

// New constructor
func New(
	cfg *config.ServerConfig,
	senders map[common.NotifyChannel]types.Sender,
) (JobNotifier, error) {
	b, err := ioutil.ReadFile(cfg.Email.JobsTemplateFile)
	if err != nil {
		b, err = ioutil.ReadFile(cfg.PublicDir + cfg.Email.JobsTemplateFile)
		if err != nil {
			return nil, fmt.Errorf("error loading jobs_template_file: '%s' due to %s", cfg.Email.JobsTemplateFile, err)
		}
	}

	return &DefaultJobNotifier{
		cfg:      cfg,
		senders:  senders,
		template: string(b),
	}, nil
}

// NotifyJob sends message to recipients
func (n *DefaultJobNotifier) NotifyJob(
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

	msg, err = utils.ParseTemplate(n.template, params)
	if err != nil {
		return "", err
	}

	var recipients []string
	var senders []common.NotifyChannel
	total := 0
	notify := job.Notify
	if len(notify) == 0 && user != nil {
		notify = user.Notify
	}

	whens := make([]common.NotifyWhen, 0)
	for k, v := range notify {
		sender := n.senders[k]
		if sender == nil {
			return "", fmt.Errorf("no sender for %s", sender)
		}
		whens = append(whens, v.When)
		senders = append(senders, k)
		if v.When.Accept(request.GetJobState(), lastRequestState) {
			if err = sender.SendMessage(v.Recipients, subject, msg); err != nil {
				return "", err
			}
			total++
			for _, recipient := range v.Recipients {
				recipients = append(recipients, recipient)
			}
		}
	}

	logrus.WithFields(logrus.Fields{
		"Component":        "DefaultJobNotifier",
		"Senders":          senders,
		"Request":          request.GetID(),
		"LastRequestState": lastRequestState,
		"State":            request.GetJobState(),
		"Recipients":       recipients,
		"Whens":            whens,
		"Subject":          subject,
		"Total":            total,
	}).Infof("notified job")
	return
}
