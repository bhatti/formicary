package types

// Sender defines operations to send message
type Sender interface {
	SendMessage(to []string, subject string, body string) error
}
