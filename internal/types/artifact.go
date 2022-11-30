package types

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ArtifactKindLogs for logs
const ArtifactKindLogs = "LOGS"

// ArtifactKindTask for task
const ArtifactKindTask = "TASK_ARTIFACTS"

// ArtifactKindUser for user-uploaded
const ArtifactKindUser = "USER_UPLOADED"

// ArtifactKindCache for cached directory
const ArtifactKindCache = "CACHE"

// Artifact defines metadata of artifact that is uploaded by a job such as task logs, task results, etc.
// The metadata defines properties to associate artifact with a task or job and can be used to query artifacts
// related for a job or an organization.
type Artifact struct {
	//gorm.Model
	// ID defines UUID for primary key
	ID string `json:"id" gorm:"primary_key"`
	// Bucket defines bucket where artifact is stored
	Bucket string `json:"bucket"`
	// Name defines name of artifact for display
	Name string `json:"name"`
	// OrganizationID defines org who submitted the job
	OrganizationID string `json:"organization_id"`
	// UserID defines user who submitted the job
	UserID string `json:"user_id"`
	// Group of artifact
	Group string `json:"group"`
	// Kind of artifact
	Kind string `json:"kind"`
	// ETag stores ETag from underlying storage such as Minio/S3
	ETag string `json:"etag"`
	// ArtifactOrder of artifact in group
	ArtifactOrder int `json:"artifact_order"`
	// JobRequestID refers to request-id being processed
	JobRequestID uint64 `json:"job_request_id"`
	// JobExecutionID refers to job-execution-id being processed
	JobExecutionID string `json:"job_execution_id"`
	// TaskExecutionID refers to task-execution-id being processed
	TaskExecutionID string `json:"task_execution_id"`
	// TaskType defines type of task
	TaskType string `yaml:"task_type" json:"task_type"`
	// SHA256 defines hash of the contents using SHA-256 algorithm
	SHA256 string `json:"sha256"`
	// ContentType refers to content-type of artifact
	ContentType string `json:"content_type"`
	// ContentLength refers to content-length of artifact
	ContentLength int64 `json:"content_length"`
	// Permissions of artifact
	Permissions int32 `json:"permissions"`
	// MetadataSerialized of artifact
	MetadataSerialized string `json:"-"`
	// TagsSerialized of artifact
	TagsSerialized string `json:"-"`
	// ExpiresAt - expiration time
	ExpiresAt time.Time `json:"expires_at"`
	// CreatedAt job creation time
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt job update time
	UpdatedAt time.Time `json:"updated_at"`
	// Active is used to softly delete artifact
	Active bool `yaml:"-" json:"-"`
	// MetadataMap - transient map of properties - deserialized from MetadataSerialized
	Metadata map[string]string `json:"metadata" gorm:"-"`
	Tags     map[string]string `json:"tags" gorm:"-"`
	URL      string            `json:"url" gorm:"-"`
}

// TableName overrides default table name
func (Artifact) TableName() string {
	return "formicary_artifacts"
}

// NewArtifact creates new instance of artifact
func NewArtifact(
	bucket string,
	name string,
	group string,
	kind string,
	reqID uint64,
	sha256 string,
	length int64) *Artifact {
	return &Artifact{
		Bucket:        bucket,
		Name:          name,
		Group:         group,
		Kind:          kind,
		JobRequestID:  reqID,
		SHA256:        sha256,
		ContentLength: length,
		CreatedAt:     time.Now(),
		Metadata:      make(map[string]string),
		Tags:          make(map[string]string),
	}
}

// AddMetadata adds metadata
func (a *Artifact) AddMetadata(name string, value string) {
	lowerName := strings.ToLower(name)
	if a.ContentType == "" && len(lowerName) <= 12 &&
		strings.HasPrefix(lowerName, "content") && strings.HasSuffix(lowerName, "type") {
		a.ContentType = value
	} else if lowerName == "kind" {
		a.Kind = value
	} else if lowerName == "name" {
		a.Name = value
	} else if lowerName == "group" {
		a.Group = value
	} else if lowerName == "user_id" {
		a.UserID = value
	} else if lowerName == "organization_id" || lowerName == "org" || lowerName == "org_id" {
		a.OrganizationID = value
	} else {
		if a.Metadata == nil {
			a.Metadata = make(map[string]string)
		}
		a.Metadata[name] = value
	}
}

// ShortUserID short user id
func (a *Artifact) ShortUserID() string {
	if len(a.UserID) > 8 {
		return a.UserID[0:8] + "..."
	}
	return a.UserID
}

// AddTag adds tag
func (a *Artifact) AddTag(name string, value string) {
	a.Tags[name] = value
}

// String
func (a *Artifact) String() string {
	return fmt.Sprintf("Name=%s Kind=%s Group=%s Task=%s, Metadata=%v",
		a.Name, a.Kind, a.Group, a.TaskExecutionID, a.Metadata)
}

// Validate validates artifact
func (a *Artifact) Validate() error {
	if a.Name == "" {
		return fmt.Errorf("artifact name is not specified")
	}
	if a.Bucket == "" {
		return fmt.Errorf("artifact bucket is not specified")
	}
	if a.ID == "" {
		return errors.New("artifact id is not specified")
	}
	if a.SHA256 == "" {
		return errors.New("artifact sha256 is not specified")
	}
	if a.ContentLength == 0 {
		return errors.New("artifact content-length is not specified")
	}
	return nil
}

// ValidateBeforeSave validates artifact
func (a *Artifact) ValidateBeforeSave() error {
	if err := a.Validate(); err != nil {
		return err
	}

	if a.ExpiresAt.IsZero() {
		return errors.New("artifact expiration is not specified")
	}
	if len(a.Metadata) > 0 {
		b, err := json.Marshal(a.Metadata)
		if err != nil {
			return err
		}
		a.MetadataSerialized = string(b)
	}
	if len(a.Tags) > 0 {
		b, err := json.Marshal(a.Tags)
		if err != nil {
			return err
		}
		a.TagsSerialized = string(b)
	}
	return nil
}

// AfterLoad populates artifact
func (a *Artifact) AfterLoad() error {
	if len(a.MetadataSerialized) > 0 {
		err := json.Unmarshal([]byte(a.MetadataSerialized), &a.Metadata)
		if err != nil {
			return fmt.Errorf("failed to parse '%v' due to %w", a.MetadataSerialized, err)
		}
	}
	if len(a.TagsSerialized) > 0 {
		err := json.Unmarshal([]byte(a.TagsSerialized), &a.Tags)
		if err != nil {
			return fmt.Errorf("failed to parse '%v' due to %w", a.TagsSerialized, err)
		}
	}
	return nil
}

// DashboardURL link to download artifact
func (a *Artifact) DashboardURL() string {
	return strings.ReplaceAll(a.URL, "/api/", "/dashboard/")
}

// DashboardRawURL link to download artifact
func (a *Artifact) DashboardRawURL() string {
	return strings.ReplaceAll(a.URL, "/api/", "/dashboard/") + "/raw"
}

// Digest hash
func (a *Artifact) Digest() uint64 {
	n, _ := strconv.ParseUint(a.SHA256, 16, 64)
	return n
}

// LengthString / 1024 * 1024
func (a *Artifact) LengthString() string {
	if a.ContentLength > 1024*1024 {
		return fmt.Sprintf("%d MiB",
			a.ContentLength/1024/1024)
	} else if a.ContentLength > 1024 {
		return fmt.Sprintf("%d KiB",
			a.ContentLength/1024)
	} else {
		return fmt.Sprintf("%d B",
			a.ContentLength)
	}
}
