package email

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"net/smtp"
	"plexobject.com/formicary/queen/config"
	"plexobject.com/formicary/queen/types"
	"strings"
)

// DefaultEmailSender defines operations to send email
type DefaultEmailSender struct {
	cfg  *config.SMTPConfig
	auth smtp.Auth
}

// New constructor
func New(
	cfg *config.SMTPConfig,
) (types.Sender, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	d := &DefaultEmailSender{
		cfg: cfg,
	}
	if cfg.APIKey == "" {
		d.auth = smtp.PlainAuth("", cfg.FromEmail, cfg.Password, cfg.Host)
	} else {
		return nil, fmt.Errorf("api not supported")
	}
	return d, nil
}

// SendMessage sends email to recipients
func (d *DefaultEmailSender) SendMessage(to []string, subject string, body string) error {
	hostPort := fmt.Sprintf("%s:%d", d.cfg.Host, d.cfg.Port)
	from := d.cfg.FromName + "<" + d.cfg.FromEmail + ">"
	logrus.WithFields(logrus.Fields{
		"Component": "DefaultEmailSender",
		"Host":      hostPort,
		"From":      from,
		"To":        to,
		"Size":      len(body),
	}).Infof("sending email")

	// The msg parameter should be an RFC 822-style email with headers
	// first, a blank line, and then the message body. The lines of msg
	// should be CRLF terminated. The msg headers should usually include
	// fields such as "From", "To", "Subject", and "Cc".  Sending "Bcc"
	// messages is accomplished by including an email address in the to
	// parameter but not including it in the msg headers.
	var msg strings.Builder
	if strings.HasPrefix(body, "<") {
		msg.WriteString("Content-Type: text/html; charset=\"UTF-8\"\r\n")
		msg.WriteString("Content-Transfer-Encoding: quoted-printable\r\n")
	} else {
		msg.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
		msg.WriteString("Content-Transfer-Encoding: 7bit\r\n")
	}
	msg.WriteString("From: " + from + "\r\n")
	msg.WriteString("Subject: " + subject + "\r\n")
	msg.WriteString("\r\n")
	msg.WriteString(body + "\r\n")

	return smtp.SendMail(hostPort, d.auth, d.cfg.FromEmail, to, []byte(msg.String()))
}
