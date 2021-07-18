package artifacts

import (
	"context"
	"io"
	"net/url"
	"plexobject.com/formicary/internal/types"
	"time"
)

// Service provides access to artifacts
type Service interface {
	// Get Finds artifacts by id
	Get(
		ctx context.Context,
		id string) (io.ReadCloser, error)
	// SaveFile uploads artifact with given value
	SaveFile(
		ctx context.Context,
		prefix string,
		artifact *types.Artifact,
		filePath string) error
	// SaveBytes upload artifact with given byte value
	SaveBytes(
		ctx context.Context,
		prefix string,
		name string,
		data []byte) (*types.Artifact, error)
	// SaveData upload artifacts with given reader
	SaveData(
		ctx context.Context,
		prefix string,
		artifact *types.Artifact,
		reader io.Reader) error
	// Delete artifacts by id
	Delete(
		ctx context.Context,
		id string) error
	// PresignedGetURL - temporary URL for downloading artifact
	PresignedGetURL(
		ctx context.Context,
		id string,
		fileName string,
		expires time.Duration) (*url.URL, error)
	// PresignedPutURL - temporary URL for uploading artifact
	PresignedPutURL(
		ctx context.Context,
		id string,
		expires time.Duration) (*url.URL, error)
	// List - lists artifacts
	List(
		ctx context.Context,
		prefix string) ([]*types.Artifact, error)
}
