package github

import (
	"strings"
)

// WebhookEvent callback event
type WebhookEvent struct {
	Ref        string            `json:"ref"`
	Before     string            `json:"before"`
	After      string            `json:"after"`
	Repository Repository        `json:"repository"`
	Pusher     Author            `json:"pusher"`
	Sender     Author            `json:"sender"`
	Compare    string            `json:"compare"`
	Commits    []Commit          `json:"commits"`
	HeadCommit Commit            `json:"head_commit"`
	Headers    map[string]string `json:"-"`
}

// Branch of head
func (e *WebhookEvent) Branch() string {
	if strings.HasPrefix(e.Ref, "refs/heads/") {
		return e.Ref[11:]
	}
	return e.Ref
}
