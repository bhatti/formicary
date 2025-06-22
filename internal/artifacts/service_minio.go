package artifacts

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/minio/minio-go/v7/pkg/encrypt"
	"github.com/oklog/ulid/v2"
	"io"
	"net/url"
	"os"
	"plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/internal/utils"
	"strings"
	"sync"
	"time"
)

// Adapter captures client for Minio
type Adapter struct {
	conf           *types.S3Config
	prefix         string
	minioClient    *minio.Client
	lock           sync.RWMutex
	verifiedBucket bool
}

// New creates new adapter for artifacts service
func New(conf *types.S3Config) (Service, error) {
	if err := conf.Validate(); err != nil {
		return nil, err
	}
	minioClient, err := minio.New(conf.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(conf.AccessKeyID, conf.SecretAccessKey, conf.Token),
		Secure: conf.UseSSL,
	})
	if err != nil {
		return nil, err
	}
	return &Adapter{
		conf:        conf,
		prefix:      utils.NormalizePrefix(conf.Prefix),
		minioClient: minioClient,
	}, nil
}

// Get downloads artifact
func (a *Adapter) Get(
	ctx context.Context,
	id string) (io.ReadCloser, error) {
	opts := minio.GetObjectOptions{}

	if a.conf.UseSSL {
		encryption := encrypt.DefaultPBKDF([]byte(a.conf.EncryptionPassword), []byte(a.conf.Bucket+id))
		opts.ServerSideEncryption = encryption
	}

	return a.minioClient.GetObject(
		ctx,
		a.conf.Bucket,
		id,
		opts)
}

// SaveFile saves artifact from file
func (a *Adapter) SaveFile(
	ctx context.Context,
	_ string,
	artifact *types.Artifact,
	filePath string) error {
	// checking file size
	fi, err := os.Stat(filePath)
	if err != nil {
		return err
	}
	if fi.Size() == 0 {
		return fmt.Errorf("no data for %s", filePath)
	}

	f, err := os.Open(filePath)
	if err != nil {
		return err
	}

	defer func() {
		_ = f.Close()
	}()
	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
		return nil
	}
	sum256 := hasher.Sum(nil)
	hex256 := hex.EncodeToString(sum256[:])
	defaultPrefix := utils.NormalizePrefix(a.prefix)
	if artifact.ID == "" {
		artifact.ID = defaultPrefix + ulid.Make().String()
	} else if defaultPrefix != "" && !strings.HasPrefix(artifact.ID, defaultPrefix) {
		artifact.ID = defaultPrefix + artifact.ID
	}

	artifact.SHA256 = hex256
	artifact.Bucket = a.conf.Bucket
	artifact.ContentLength = fi.Size()

	if err := artifact.Validate(); err != nil {
		return err
	}

	// check if bucket exists
	if err := a.checkBucket(ctx); err != nil {
		return err
	}

	opts := a.buildPutOptions(artifact)

	uploadedInfo, err := a.minioClient.FPutObject(
		ctx,
		artifact.Bucket,
		artifact.ID,
		filePath,
		opts,
	)
	if err != nil {
		return fmt.Errorf("uploaded failed for file=%s to bucket=%s id=%s size=%d due to %s",
			filePath, artifact.Bucket, artifact.ID, artifact.ContentLength, err)
	}

	if uploadedInfo.Size != artifact.ContentLength {
		return fmt.Errorf("expecting content length to be %v but was %v",
			artifact.ContentLength, uploadedInfo.Size)
	}
	artifact.ETag = uploadedInfo.ETag
	return nil
}

// SaveBytes saves artifact from bytes
func (a *Adapter) SaveBytes(
	ctx context.Context,
	prefix string,
	name string,
	data []byte) (*types.Artifact, error) {
	if data == nil || len(data) == 0 {
		return nil, fmt.Errorf("no data for %s", name)
	}
	sum256 := sha256.Sum256(data)
	hex256 := hex.EncodeToString(sum256[:])

	// create artifact
	artifact := &types.Artifact{
		Name:          name,
		Bucket:        a.conf.Bucket,
		SHA256:        hex256,
		ContentLength: int64(len(data)),
		ContentType:   "application/octet-stream",
		Metadata:      make(map[string]string),
		Tags:          make(map[string]string),
	}
	// calling save data that supports general purpose reader
	if err := a.SaveData(ctx, prefix, artifact, bytes.NewReader(data)); err != nil {
		return nil, err
	}
	return artifact, nil
}

// SaveData saves artifact from reader
func (a *Adapter) SaveData(
	ctx context.Context,
	prefix string,
	artifact *types.Artifact,
	reader io.Reader) error {
	defaultPrefix := utils.NormalizePrefix(a.prefix)
	if artifact.ID == "" {
		artifact.ID = utils.NormalizePrefix(a.prefix) + utils.NormalizePrefix(prefix) + artifact.SHA256
	} else if defaultPrefix != "" && !strings.HasPrefix(artifact.ID, defaultPrefix) {
		artifact.ID = defaultPrefix + artifact.ID
	}
	if err := artifact.Validate(); err != nil {
		return err
	}
	if err := a.checkBucket(ctx); err != nil {
		return err
	}
	// we need SHA256 as ID, so we can't calculate it on the fly
	//hasher := sha256.New()
	//tee := io.TeeReader(reader, hasher)
	//sum256 := hasher.Sum(nil)
	artifact.Bucket = a.conf.Bucket
	opts := a.buildPutOptions(artifact)

	// Upload artifact
	if uploadedInfo, err := a.minioClient.PutObject(
		ctx,
		artifact.Bucket,
		artifact.ID,
		reader,
		artifact.ContentLength,
		opts,
	); err != nil {
		return err
	} else if uploadedInfo.Size != artifact.ContentLength {
		return fmt.Errorf("expecting content length to be %v but was %v",
			artifact.ContentLength, uploadedInfo.Size)
	} else {
		artifact.ETag = uploadedInfo.ETag
	}
	return nil
}

// Delete deletes artifact
func (a *Adapter) Delete(
	ctx context.Context,
	id string) error {
	opts := minio.RemoveObjectOptions{
		GovernanceBypass: true,
	}
	return a.minioClient.RemoveObject(ctx, a.conf.Bucket, id, opts)
}

// PresignedGetURL creates presigned URL for downloading
func (a *Adapter) PresignedGetURL(
	ctx context.Context,
	id string,
	fileName string,
	expires time.Duration) (*url.URL, error) {
	reqParams := make(url.Values)
	attachment := fmt.Sprintf("attachment; filename=\"%s\"", fileName)
	reqParams.Set("response-content-disposition", attachment)

	return a.minioClient.PresignedGetObject(
		ctx,
		a.conf.Bucket,
		id,
		expires,
		reqParams)
}

// PresignedPutURL creates presigned URL for uploading
func (a *Adapter) PresignedPutURL(
	ctx context.Context,
	id string,
	expires time.Duration) (*url.URL, error) {
	if err := a.checkBucket(ctx); err != nil {
		return nil, err
	}
	return a.minioClient.PresignedPutObject(
		ctx,
		a.conf.Bucket,
		id,
		expires)
}

// List - lists artifacts
func (a *Adapter) List(
	ctx context.Context,
	prefix string) ([]*types.Artifact, error) {
	opts := minio.ListObjectsOptions{Prefix: a.prefix + prefix, Recursive: true}
	artifacts := make([]*types.Artifact, 0)
	for object := range a.minioClient.ListObjects(ctx, a.conf.Bucket, opts) {
		if object.Err != nil {
			return nil, object.Err
		}
		artifacts = append(artifacts, &types.Artifact{
			Bucket:        a.conf.Bucket,
			ID:            object.UserMetadata["ID"],
			SHA256:        object.UserMetadata["SHA256"],
			ContentLength: object.Size,
			ContentType:   object.ContentType,
			Metadata:      object.UserMetadata,
			Tags:          object.UserTags,
			ExpiresAt:     object.Expiration,
		})
	}
	return artifacts, nil
}

/////////////////////////////////////////// PRIVATE METHODS ////////////////////////////////////////////

func (a *Adapter) checkBucket(ctx context.Context) error {
	a.lock.Lock()
	defer a.lock.Unlock()
	if a.verifiedBucket {
		return nil
	}
	exists, err := a.minioClient.BucketExists(ctx, a.conf.Bucket)
	if err != nil {
		return fmt.Errorf("failed to check bucket '%s' due to %w", a.conf.Bucket, err)
	}
	if !exists {
		err = a.minioClient.MakeBucket(ctx, a.conf.Bucket, minio.MakeBucketOptions{Region: a.conf.Region})
		if err != nil {
			return fmt.Errorf("failed to create bucket '%s' due to %w (%s)", a.conf.Bucket, err, a.conf.Endpoint)
		}
	}
	a.verifiedBucket = true
	return nil
}

func (a *Adapter) buildPutOptions(artifact *types.Artifact) minio.PutObjectOptions {
	opts := minio.PutObjectOptions{
		ContentType:  artifact.ContentType,
		UserMetadata: artifact.Metadata,
	}
	if artifact.Tags != nil {
		opts.UserTags = artifact.Tags
	}
	if a.conf.UseSSL {
		encryption := encrypt.DefaultPBKDF([]byte(a.conf.EncryptionPassword), []byte(artifact.Bucket+artifact.ID))
		opts.ServerSideEncryption = encryption
	}
	opts.UserMetadata["ID"] = artifact.ID
	opts.UserMetadata["SHA256"] = artifact.SHA256
	if artifact.ExpiresAt.Unix() > time.Now().Unix() {
		// See https://docs.minio.io/docs/minio-bucket-object-lock-guide.html
		// TODO fix it, commenting because it causes: Bucket is missing ObjectLockConfiguration
		//opts.RetainUntilDate = artifact.ExpiresAt
	}
	return opts
}
