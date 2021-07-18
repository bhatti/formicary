package github

import "time"

// Commit git commit
type Commit struct {
	ID        string    `json:"id"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
	URL       string    `json:"url"`
	Author    Author    `json:"author"`
	Committer Author    `json:"committer"`
	Added     []string  `json:"added"`    // list of files
	Removed   []string  `json:"removed"`  // list of files
	Modified  []string  `json:"modified"` // list of files
}
