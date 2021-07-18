package github

import "time"

// Repository for github webhook
type Repository struct {
	ID              int       `json:"id"`
	Name            string    `json:"name"`
	FullName        string    `json:"full_name"`
	Private         bool      `json:"private"`
	Owner           Author    `json:"owner"`
	HTMLURL         string    `json:"html_url"`
	URL             string    `json:"url"`
	CreatedAt       int       `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
	PushedAt        int       `json:"pushed_at"`
	GitURL          string    `json:"git_url"`
	SSHURL          string    `json:"ssh_url"`
	OpenIssuesCount int       `json:"open_issues_count"`
	License         struct {
		Key    string `json:"key"`
		Name   string `json:"name"`
		SpdxID string `json:"spdx_id"`
		URL    string `json:"url"`
		NodeID string `json:"node_id"`
	} `json:"license"`
	Forks         int    `json:"forks"`
	Watchers      int    `json:"watchers"`
	DefaultBranch string `json:"default_branch"`
	MasterBranch  string `json:"master_branch"`
}
