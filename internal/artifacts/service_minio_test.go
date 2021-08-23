package artifacts

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/stretchr/testify/require"
	"github.com/twinj/uuid"
	"io/ioutil"
	"os"
	"plexobject.com/formicary/internal/types"
	"testing"
	"time"
)

func Test_ShouldUploadZipFile(t *testing.T) {
	// GIVEN a zip file
	tmpFile, err := ioutil.TempFile(os.TempDir(), "artifact.zip")
	require.NoError(t, err)

	pathsNames := make([]string, 0)
	defer func() {
		_ = os.Remove(tmpFile.Name())
		for _, p := range pathsNames {
			_ = os.Remove(p)
		}
	}()

	for i := 0; i < 10; i++ {
		f, _, _, err := createFile(fmt.Sprintf("test file %d", i))
		require.NoError(t, err)
		_ = f.Close()
		pathsNames = append(pathsNames, f.Name())
	}

	err = ZipFiles(tmpFile, pathsNames)
	require.NoError(t, err)
	_ = tmpFile.Close()

	// AND an artifact-service client
	//cli, err := New(s3Config())
	cli, err := NewStub(s3Config())
	require.NoError(t, err)
	expiration := time.Now().Add(1 * time.Hour)
	artifact := &types.Artifact{
		ID:            uuid.NewV4().String(),
		Bucket:        "test-bucket",
		Name:          "name",
		SHA256:        "sha256",
		ContentLength: 123,
		ContentType:   "application/zip",
		Metadata:      map[string]string{"type": "test"},
		Tags:          map[string]string{"label": "test"},
		ExpiresAt:     expiration,
	}

	// WHEN zip file is uploaded to artifact-service
	err = cli.SaveFile(
		context.Background(),
		"prefix",
		artifact,
		tmpFile.Name())
	// THEN it should not fail
	require.NoError(t, err)
}

func Test_ShouldUploadAndDownloadFile(t *testing.T) {
	// GIVEN a text file
	data := "Hello again"
	f, hash, size, err := createFile(data)
	require.NoError(t, err)
	defer func() {
		_ = os.Remove(f.Name())
	}()

	// AND client for artifact-service
	//cli, err := New(s3Config())
	cli, err := NewStub(s3Config())
	require.NoError(t, err)

	artifact := &types.Artifact{
		ID:            uuid.NewV4().String(),
		Bucket:        "test-bucket",
		Name:          "name",
		SHA256:        hash,
		ContentLength: int64(size),
		ContentType:   "text/plain",
		Metadata:      map[string]string{"type": "test"},
		Tags:          map[string]string{"label": "test"},
	}

	// WHEN file is uploaded
	err = cli.SaveFile(
		context.Background(),
		"prefix",
		artifact,
		f.Name())
	require.NoError(t, err)

	// AND is then downloaded
	reader, err := cli.Get(
		context.Background(),
		artifact.ID)

	// THEN no error occur
	require.NoError(t, err)

	defer func() {
		_ = reader.Close()
	}()

	// AND file should be valid after download
	b, err := ioutil.ReadAll(reader)
	require.NoError(t, err)
	require.Equal(t, data, string(b))
}

func Test_ShouldUploadAndDownloadBytes(t *testing.T) {
	// GIVEN an array of bytes
	data := "Hello again"
	b := []byte(data)

	// AND a client for artifact-service
	//cli, err := New(s3Config())
	cli, err := NewStub(s3Config())
	require.NoError(t, err)

	// WHEN byte-array is saved/uploaded
	artifact, err := cli.SaveBytes(
		context.Background(),
		"prefix",
		"name",
		b,
	)
	require.NoError(t, err)

	// AND downloaded
	reader, err := cli.Get(
		context.Background(),
		artifact.ID)
	require.NoError(t, err)

	defer func() {
		_ = reader.Close()
	}()

	// THEN file should be valid
	b, err = ioutil.ReadAll(reader)
	require.NoError(t, err)
	require.Equal(t, data, string(b))
}

func s3Config() *types.S3Config {
	return &types.S3Config{
		Endpoint:        "localhost:9000",
		AccessKeyID:     "admin",
		SecretAccessKey: "password",
		Region:          "US-WEST-2",
		Bucket:          "test-bucket",
		UseSSL:          false,
	}
}

func createFile(data string) (*os.File, string, int, error) {
	b := []byte(data)
	f, err := ioutil.TempFile(os.TempDir(), "test-")
	if err != nil {
		return nil, "", 0, err
	}

	defer func() {
		_ = f.Close()
	}()
	_, err = f.WriteString(data)
	if err != nil {
		return nil, "", 0, err
	}
	hash := sha256.Sum256(b)
	return f, hex.EncodeToString(hash[:]), len(b), nil
}

