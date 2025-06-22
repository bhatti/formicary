package email

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"net/smtp"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/config"
	"plexobject.com/formicary/queen/manager"
	"plexobject.com/formicary/queen/types"
	"strings"
)

// DefaultEmailSender defines operations to send email
type DefaultEmailSender struct {
	cfg         *config.ServerConfig
	userManager *manager.UserManager
	auth        smtp.Auth
}

// New constructor
func New(
	cfg *config.ServerConfig,
	userManager *manager.UserManager,
) (types.Sender, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	d := &DefaultEmailSender{
		cfg:         cfg,
		userManager: userManager,
	}
	if cfg.SMTP.APIKey == "" {
		d.auth = smtp.PlainAuth("", cfg.SMTP.FromEmail, cfg.SMTP.Password, cfg.SMTP.Host)
	} else {
		return nil, fmt.Errorf("api not supported")
	}
	return d, nil
}

// JobNotifyTemplateFile template file
func (d *DefaultEmailSender) JobNotifyTemplateFile() string {
	return d.cfg.Notify.EmailJobsTemplateFile
}

// SupportsLongReport is supported
func (d *DefaultEmailSender) SupportsLongReport() bool {
	return true
}

// SendMessage sends email to recipients
func (d *DefaultEmailSender) SendMessage(
	qc *common.QueryContext,
	user *common.User,
	to []string,
	subject string,
	body string,
	_ map[string]interface{}) error {
	hostPort := fmt.Sprintf("%s:%d", d.cfg.SMTP.Host, d.cfg.SMTP.Port)
	from := d.cfg.SMTP.FromName + "<" + d.cfg.SMTP.FromEmail + ">"
	logrus.WithFields(logrus.Fields{
		"Component":             "DefaultEmailSender",
		"Host":                  hostPort,
		"From":                  from,
		"To":                    to,
		"JobNotifyTemplateFile": d.JobNotifyTemplateFile(),
		"Size":                  len(body),
	}).Infof("sending email")

	// The msg parameter should be an RFC 822-style email with headers
	// first, a blank line, and then the message body. The lines of msg
	// should be CRLF terminated. The msg headers should usually include
	// fields such as "From", "To", "Subject", and "Cc".  Sending "Bcc"
	// messages is accomplished by including an email address in the "to"
	// parameter but not including it in the msg headers.
	var msg strings.Builder
	if strings.HasPrefix(body, "<") {
		msg.WriteString("Content-Kind: text/html; charset=\"UTF-8\"\r\n")
		msg.WriteString("Content-Transfer-Encoding: quoted-printable\r\n")
	} else {
		msg.WriteString("Content-Kind: text/plain; charset=UTF-8\r\n")
		msg.WriteString("Content-Transfer-Encoding: 7bit\r\n")
	}
	msg.WriteString("From: " + from + "\r\n")
	msg.WriteString("To: " + strings.Join(to, ",") + "\r\n")
	msg.WriteString("Subject: " + subject + "\r\n")
	msg.WriteString("\r\n")
	msg.WriteString(body + "\r\n")

	err := smtp.SendMail(hostPort, d.auth, d.cfg.SMTP.FromEmail, to, []byte(msg.String()))
	if err != nil {
		_ = d.userManager.AddStickyMessageForEmail(
			qc,
			user,
			err)
	} else {
		_ = d.userManager.ClearStickyMessageForSlack(
			qc,
			user,
		)
	}
	return err
}
