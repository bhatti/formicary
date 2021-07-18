package artifacts

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"net/url"
	"plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/internal/utils"
	"time"
)

type stubArtifact struct {
	*types.Artifact
	data []byte
}

type stub struct {
	storage map[string]*stubArtifact
}

type nopCloser struct {
	io.Reader
}

func (nopCloser) Close() error { return nil }

// NewStub creates new stub
func NewStub(_ *types.S3Config) (Service, error) {
	return &stub{
		storage: make(map[string]*stubArtifact),
	}, nil
}

// Get downloads artifact
func (s *stub) Get(
	_ context.Context,
	id string) (io.ReadCloser, error) {
	art := s.storage[id]
	if art == nil {
		return nil, fmt.Errorf("not found")
	}
	return nopCloser{Reader: bytes.NewReader(art.data)}, nil
}

// SaveFile saves artifact
func (s *stub) SaveFile(
	_ context.Context,
	prefix string,
	artifact *types.Artifact,
	filePath string) error {
	artifact.Bucket = "test-mock-bucket"
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return err
	}
	if len(data) == 0 {
		return fmt.Errorf("no data for %s", filePath)
	}
	sum256 := sha256.Sum256(data)
	hex256 := hex.EncodeToString(sum256[:])
	artifact.SHA256 = hex256
	artifact.ContentLength = int64(len(data))
	if artifact.ID == "" {
		artifact.ID = utils.NormalizePrefix(prefix) + hex256
	}
	if err := artifact.Validate(); err != nil {
		return err
	}

	s.storage[artifact.ID] = &stubArtifact{Artifact: artifact, data: data}
	return nil
}

// SaveBytes saves artifact
func (s *stub) SaveBytes(
	ctx context.Context,
	prefix string,
	name string,
	data []byte) (*types.Artifact, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("no data for %s", name)
	}

	sum256 := sha256.Sum256(data)
	hex256 := hex.EncodeToString(sum256[:])

	artifact := &types.Artifact{
		SHA256:        hex256,
		Name:          name,
		Bucket:        "test",
		ContentLength: int64(len(data)),
		ContentType:   "application/octet-stream",
		Metadata:      map[string]string{},
		Tags:          map[string]string{},
	}

	if err := s.SaveData(ctx, prefix, artifact, bytes.NewReader(data)); err != nil {
		return nil, err
	}
	return artifact, nil
}

// SaveData saves artifact
func (s *stub) SaveData(
	_ context.Context,
	prefix string,
	artifact *types.Artifact,
	reader io.Reader) error {
	if artifact.ID == "" {
		artifact.ID = utils.NormalizePrefix(prefix) + artifact.SHA256
	}
	artifact.Bucket = "test-mock-bucket"
	if err := artifact.Validate(); err != nil {
		return err
	}

	data, err := ioutil.ReadAll(reader)
	if err != nil {
		return err
	}
	if len(data) == 0 {
		return fmt.Errorf("no data for %s", artifact.Name)
	}
	s.storage[artifact.ID] = &stubArtifact{Artifact: artifact, data: data}
	return nil
}

// Delete deletes artifact
func (s *stub) Delete(
	_ context.Context,
	id string) error {
	delete(s.storage, id)
	return nil
}

// PresignedGetURL creates presigned URL for downloading
func (s *stub) PresignedGetURL(
	_ context.Context,
	id string,
	_ string,
	_ time.Duration) (*url.URL, error) {
	return url.Parse("http://localhost:9000/" + id)
}

// PresignedPutURL creates presigned URL for uploading
func (s *stub) PresignedPutURL(
	_ context.Context,
	id string,
	_ time.Duration) (*url.URL, error) {
	return url.Parse("http://localhost:9000/" + id)
}

// List - lists artifacts
func (s *stub) List(context.Context, string) ([]*types.Artifact, error) {
	artifacts := make([]*types.Artifact, 0)
	for _, art := range s.storage {
		artifacts = append(artifacts, art.Artifact)
	}
	return artifacts, nil
}
