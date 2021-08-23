package repository

import (
	common "plexobject.com/formicary/internal/types"
	types "plexobject.com/formicary/queen/types"
	"time"
)

// ArtifactRepository defines data access methods for artifacts
type ArtifactRepository interface {
	// GetResourceUsage - Finds usage between time
	GetResourceUsage(
		qc *common.QueryContext,
		ranges []types.DateRange) ([]types.ResourceUsage, error)
	// ExpiredArtifacts finds expired artifact
	ExpiredArtifacts(
		qc *common.QueryContext,
		expiration time.Duration,
		limit int) (arts []*common.Artifact, err error)
	// Query finds artifact by parameters
	Query(
		qc *common.QueryContext,
		params map[string]interface{},
		page int,
		pageSize int,
		order []string) (arts []*common.Artifact, total int64, err error)
	// GetBySHA256 - Finds Artifact by sha256
	GetBySHA256(
		qc *common.QueryContext,
		sha256 string) (*common.Artifact, error)
	// Get - Finds Artifact by id
	Get(
		qc *common.QueryContext,
		id string) (*common.Artifact, error)
	// Delete artifact by id
	Delete(
		qc *common.QueryContext,
		id string) error
	// Update artifact
	Update(
		qc *common.QueryContext,
		art *common.Artifact) (*common.Artifact, error)
	// Save - Saves artifact
	Save(
		art *common.Artifact) (*common.Artifact, error)
	// Clear for testing
	Clear()
}
