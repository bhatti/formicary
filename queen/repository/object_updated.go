package repository

import common "plexobject.com/formicary/internal/types"

// UpdateKind defines enum for kind of update
type UpdateKind string

// Types of update kind
const (
	// ObjectUpdated type
	ObjectUpdated UpdateKind = "ObjectUpdated"
	// ObjectDeleted type
	ObjectDeleted UpdateKind = "ObjectDeleted"
)

// ObjectUpdatedHandler - callback for update notification
type ObjectUpdatedHandler func(
	qc *common.QueryContext,
	id string,
	kind UpdateKind,
	obj interface{})
