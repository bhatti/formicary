package security

import common "plexobject.com/formicary/internal/types"

// GithubPostWebhookHandler is invoked after webhook is called
type GithubPostWebhookHandler func(
	qc *common.QueryContext,
	jobType string,
	jobVersion string,
	params map[string]string,
	hash256 string,
	body []byte,
) error
