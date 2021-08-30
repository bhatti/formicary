package web

import common "plexobject.com/formicary/internal/types"

// PostWebhookHandler is invoked after webhook is called
type PostWebhookHandler func(
	qc *common.QueryContext,
	org *common.Organization,
	jobType string,
	jobVersion string,
	params map[string]string,
	hash256 string,
	body []byte,
) error
