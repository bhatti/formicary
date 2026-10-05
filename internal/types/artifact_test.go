package types

import (
	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

// Verify table names for artifact and config
func Test_ShouldArtifactTableNames(t *testing.T) {
	art := NewArtifact("bucket", "name", "group", "kind", ulid.Make().String(), "sha", 54)
	require.Equal(t, "formicary_artifacts", art.TableName())
}

// Validate valid artifact
func Test_ShouldCreateArtifact(t *testing.T) {
	art := NewArtifact("bucket", "name", "group", "kind", ulid.Make().String(), "sha", 54)
	art.AddMetadata("n1", "v1")
	art.AddMetadata("n2", "v2")
	art.AddMetadata("n3", "1")
	art.AddTag("t1", "v1")
	art.AddTag("t2", "v2")
	art.AddTag("t3", "1")
	art.ExpiresAt = time.Now().Add(time.Hour)
	art.ID = ulid.Make().String()
	err := art.ValidateBeforeSave()
	require.NoError(t, err)
	err = art.AfterLoad()
	require.NoError(t, err)
}

// Validate artifact with invalid metadata
func Test_ShouldNotValidateArtifactWithInvalidMetadata(t *testing.T) {
	// GIVEN an artifact
	// WHEN it's instantiated with invalid metadata
	art := NewArtifact("bucket", "name", "group", "kind", ulid.Make().String(), "sha", 54)
	art.MetadataSerialized = "xxxx"
	err := art.AfterLoad()
	// THEN it should fail
	require.Error(t, err)
}

// Validate artifact without bucket
func Test_ShouldArtifactWithoutBucket(t *testing.T) {
	// GIVEN an artifact
	// WHEN it's instantiated without bucket name
	art := NewArtifact("", "name", "group", "kind", ulid.Make().String(), "sha", 54)
	err := art.ValidateBeforeSave()
	// THEN it should fail
	require.Error(t, err)
}

// Validate artifact without name
func Test_ShouldArtifactWithoutName(t *testing.T) {
	// GIVEN an artifact
	// WHEN it's instantiated without name
	art := NewArtifact("bucket", "", "group", "kind", ulid.Make().String(), "sha", 54)
	err := art.ValidateBeforeSave()
	// THEN it should fail
	require.Error(t, err)
}

// Validate artifact without id
func Test_ShouldArtifactWithoutID(t *testing.T) {
	// GIVEN an artifact
	// WHEN it's instantiated without id
	art := NewArtifact("bucket", "name", "", "kind", ulid.Make().String(), "sha", 54)
	art.AddMetadata("n1", "v1")
	art.AddMetadata("n2", "v2")
	art.AddMetadata("n3", "1")
	art.AddTag("t1", "v1")
	art.AddTag("t2", "v2")
	art.AddTag("t3", "1")
	err := art.ValidateBeforeSave()
	// THEN it should fail
	require.Error(t, err)
}

// Validate artifact without sha256
func Test_ShouldArtifactWithoutSHA256(t *testing.T) {
	// GIVEN an artifact
	// WHEN it's instantiated without sha256 hash
	art := NewArtifact("bucket", "name", "group", "kind", ulid.Make().String(), "", 54)
	art.ID = ulid.Make().String()
	err := art.ValidateBeforeSave()
	// THEN it should fail
	require.Error(t, err)
}

// Validate artifact without content-length
func Test_ShouldArtifactWithoutContentLength(t *testing.T) {
	// GIVEN an artifact
	// WHEN it's instantiated without content-length
	art := NewArtifact("bucket", "name", "group", "kind", ulid.Make().String(), "sha", 0)
	art.ID = ulid.Make().String()
	err := art.ValidateBeforeSave()
	// THEN it should fail
	require.Error(t, err)
	require.Equal(t, "", art.DashboardURL())
	require.Equal(t, "0 B", art.LengthString())
	art.ContentLength = 1024 * 1024 * 1024
	require.Equal(t, "1024 MiB", art.LengthString())
	art.ContentLength = 1024 * 1024
	require.Equal(t, "1024 KiB", art.LengthString())
	art.ContentLength = 1024
	require.Equal(t, "1024 B", art.LengthString())
}

func Test_ShouldRoundTripReportFiles(t *testing.T) {
	art := NewArtifact("bucket", "name", "group", "kind", ulid.Make().String(), "sha256abc", 100)
	art.ID = ulid.Make().String()
	art.UserID = ulid.Make().String()
	art.ContentType = "application/zip"
	art.ExpiresAt = time.Now().Add(24 * time.Hour)
	art.ReportFiles = []ReportFile{
		{Path: "reports/index.html", Title: "My Report", MIMEType: "text/html"},
		{Path: "reports/README.md", Title: "README.md", MIMEType: "text/markdown"},
	}

	// WHEN ValidateBeforeSave serializes ReportFiles
	require.NoError(t, art.ValidateBeforeSave())
	require.NotEmpty(t, art.ReportFilesSerialized)

	// Simulate DB round-trip: clear in-memory slice, reload from serialized
	art.ReportFiles = nil
	require.NoError(t, art.AfterLoad())

	// THEN ReportFiles should be restored
	require.Len(t, art.ReportFiles, 2)
	require.Equal(t, "reports/index.html", art.ReportFiles[0].Path)
	require.Equal(t, "My Report", art.ReportFiles[0].Title)
	require.Equal(t, "text/html", art.ReportFiles[0].MIMEType)
	require.True(t, art.HasReports())
}

func Test_ShouldHaveNoReportsWhenEmpty(t *testing.T) {
	art := NewArtifact("bucket", "name", "group", "kind", ulid.Make().String(), "sha256abc", 100)
	require.False(t, art.HasReports())
}

func Test_ShouldBuildRawReportURL(t *testing.T) {
	art := NewArtifact("bucket", "name", "group", "kind", ulid.Make().String(), "sha256abc", 100)
	art.JobRequestID = "job-abc-123"

	u := art.RawReportURL("reports/index.html")
	require.Equal(t, "/dashboard/artifacts/by-job/job-abc-123/download/raw?file=reports%2Findex.html", u)
}

func Test_ShouldReturnEmptyRawReportURLWhenNoJobID(t *testing.T) {
	// Create artifact with empty JobRequestID by clearing it after construction
	art := NewArtifact("bucket", "name", "group", "kind", "", "sha256abc", 100)
	require.Equal(t, "", art.RawReportURL("reports/index.html"))
}
