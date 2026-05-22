package artifacts

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	internaltypes "plexobject.com/formicary/internal/types"
)

// newTestAdapter creates a real Adapter backed by a minimal httptest S3-compatible server.
// It handles the subset of S3 API used by the artifact service.
func newTestAdapter(t *testing.T) (*Adapter, *httptest.Server) {
	t.Helper()
	store := make(map[string][]byte)

	// Use a plain handler (not ServeMux) to avoid Go's automatic 301 redirect from
	// /bucket → /bucket/ that would be returned by ServeMux for HeadBucket requests.
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/formicary-test")
		path = strings.TrimPrefix(path, "/")

		switch r.Method {
		case http.MethodHead:
			if path == "" {
				// HeadBucket — bucket exists
				w.WriteHeader(http.StatusOK)
				return
			}
			if _, ok := store[path]; ok {
				w.WriteHeader(http.StatusOK)
			} else {
				w.WriteHeader(http.StatusNotFound)
			}
		case http.MethodPut:
			if path == "" {
				// CreateBucket
				w.WriteHeader(http.StatusOK)
				return
			}
			body, _ := io.ReadAll(r.Body)
			store[path] = body
			w.Header().Set("ETag", `"test-etag"`)
			w.WriteHeader(http.StatusOK)
		case http.MethodGet:
			// ListObjectsV2 has ?list-type=2
			if strings.Contains(r.URL.RawQuery, "list-type") {
				w.Header().Set("Content-Type", "application/xml")
				_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?><ListBucketResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/"><IsTruncated>false</IsTruncated></ListBucketResult>`))
				return
			}
			data, ok := store[path]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "application/octet-stream")
			_, _ = w.Write(data)
		case http.MethodDelete:
			delete(store, path)
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	conf := &internaltypes.S3Config{
		Endpoint:        strings.TrimPrefix(srv.URL, "http://"),
		AccessKeyID:     "test-key",
		SecretAccessKey: "test-secret",
		Region:          "us-east-1",
		Bucket:          "formicary-test",
		Prefix:          "",
	}
	require.NoError(t, conf.Validate())

	client, presign, err := newS3Client(conf)
	require.NoError(t, err)

	adapter := &Adapter{
		conf:          conf,
		prefix:        "",
		client:        client,
		presignClient: presign,
	}
	return adapter, srv
}

func Test_AdapterShouldSaveAndGetBytes(t *testing.T) {
	adapter, _ := newTestAdapter(t)
	ctx := context.Background()

	data := []byte("hello seaweedfs")
	artifact, err := adapter.SaveBytes(ctx, "prefix/", "test.txt", data)
	require.NoError(t, err)
	require.NotEmpty(t, artifact.ID)
	require.NotEmpty(t, artifact.SHA256)
	require.Equal(t, int64(len(data)), artifact.ContentLength)

	reader, err := adapter.Get(ctx, artifact.ID)
	require.NoError(t, err)
	defer func() { _ = reader.Close() }()

	got, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.Equal(t, data, got)
}

func Test_AdapterShouldSaveAndDeleteBytes(t *testing.T) {
	adapter, _ := newTestAdapter(t)
	ctx := context.Background()

	artifact, err := adapter.SaveBytes(ctx, "", "todelete.txt", []byte("bye"))
	require.NoError(t, err)

	err = adapter.Delete(ctx, artifact.ID)
	require.NoError(t, err)

	_, err = adapter.Get(ctx, artifact.ID)
	require.Error(t, err)
}

func Test_AdapterShouldSaveDataAndGet(t *testing.T) {
	adapter, _ := newTestAdapter(t)
	ctx := context.Background()

	payload := []byte("streaming data")
	artifact := &internaltypes.Artifact{
		Name:          "stream.bin",
		Bucket:        "formicary-test",
		SHA256:        "abc123",
		ContentLength: int64(len(payload)),
		ContentType:   "application/octet-stream",
		Metadata:      map[string]string{},
		Tags:          map[string]string{},
	}

	err := adapter.SaveData(ctx, "pfx/", artifact, bytes.NewReader(payload))
	require.NoError(t, err)
	require.NotEmpty(t, artifact.ID)

	reader, err := adapter.Get(ctx, artifact.ID)
	require.NoError(t, err)
	defer func() { _ = reader.Close() }()
	got, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.Equal(t, payload, got)
}

func Test_AdapterPresignedGetURLReturnsURL(t *testing.T) {
	adapter, _ := newTestAdapter(t)
	ctx := context.Background()

	u, err := adapter.PresignedGetURL(ctx, "some/key", "file.zip", 5*time.Minute)
	require.NoError(t, err)
	require.NotNil(t, u)
	require.Contains(t, u.String(), "some/key")
}

func Test_AdapterPresignedPutURLReturnsURL(t *testing.T) {
	adapter, _ := newTestAdapter(t)
	ctx := context.Background()

	u, err := adapter.PresignedPutURL(ctx, "upload/key", 5*time.Minute)
	require.NoError(t, err)
	require.NotNil(t, u)
	require.Contains(t, u.String(), "upload/key")
}

func Test_NewFactoryShouldFailWithEmptyConfig(t *testing.T) {
	_, _, err := New(&internaltypes.S3Config{})
	require.Error(t, err)
}

func Test_NewFactoryShouldSucceedWithValidExternalConfig(t *testing.T) {
	conf := &internaltypes.S3Config{
		Endpoint:        "localhost:9999", // won't be contacted at construction time
		AccessKeyID:     "k",
		SecretAccessKey: "s",
		Bucket:          "b",
		Region:          "us-east-1",
	}
	svc, closer, err := New(conf)
	require.NoError(t, err)
	require.NotNil(t, svc)
	require.NotNil(t, closer)
	_ = closer.Close()
}
