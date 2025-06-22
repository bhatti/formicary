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
