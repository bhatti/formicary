package artifacts

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go/transport/http"
	"github.com/oklog/ulid/v2"

	internaltypes "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/internal/utils"
)

// Adapter implements the Service interface using the AWS SDK v2 S3 client.
// It works with AWS S3, SeaweedFS, MinIO, or any S3-compatible store.
type Adapter struct {
	conf           *internaltypes.S3Config
	prefix         string
	client         *s3.Client
	presignClient  *s3.PresignClient
	localServer    *LocalServer // non-nil in local mode; WaitReady called lazily
	lock           sync.RWMutex
	verifiedBucket bool
}

// New creates an artifact Service. When conf.IsLocalMode() is true an embedded
// SeaweedFS subprocess is started; the returned io.Closer must be called on shutdown.
func New(conf *internaltypes.S3Config) (Service, io.Closer, error) {
	if err := conf.Validate(); err != nil {
		return nil, nil, err
	}

	var closer io.Closer = io.NopCloser(nil)

	var localSrv *LocalServer
	if conf.IsLocalMode() {
		srv, err := StartLocalServer(conf)
		if err != nil {
			return nil, nil, err
		}
		// Point the config at the embedded subprocess endpoint.
		// The subprocess starts in the background; WaitReady is called lazily
		// on the first S3 operation so startup doesn't block server init.
		localConf := *conf
		localConf.Endpoint = srv.Endpoint
		localConf.UseSSL = false
		conf = &localConf
		closer = srv
		localSrv = srv
	}

	client, presign, err := newS3Client(conf)
	if err != nil {
		_ = closer.Close()
		return nil, nil, err
	}

	return &Adapter{
		conf:          conf,
		prefix:        utils.NormalizePrefix(conf.Prefix),
		client:        client,
		presignClient: presign,
		localServer:   localSrv,
	}, closer, nil
}

func newS3Client(conf *internaltypes.S3Config) (*s3.Client, *s3.PresignClient, error) {
	cfg, err := awsconfig.LoadDefaultConfig(context.Background(),
		awsconfig.WithRegion(conf.Region),
		awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(conf.AccessKeyID, conf.SecretAccessKey, conf.Token),
		),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("s3: failed to create AWS config: %w", err)
	}

	var endpointURL string
	if conf.Endpoint != "" && conf.Endpoint != "s3.amazonaws.com" {
		scheme := "http"
		if conf.UseSSL {
			scheme = "https"
		}
		if strings.HasPrefix(conf.Endpoint, "http://") || strings.HasPrefix(conf.Endpoint, "https://") {
			endpointURL = conf.Endpoint
		} else {
			endpointURL = scheme + "://" + conf.Endpoint
		}
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.UsePathStyle = true // required for SeaweedFS and path-style S3-compatible stores
		if endpointURL != "" {
			o.BaseEndpoint = aws.String(endpointURL)
		}
	})
	return client, s3.NewPresignClient(client), nil
}

// Get downloads an artifact by its storage ID.
func (a *Adapter) Get(ctx context.Context, id string) (io.ReadCloser, error) {
	out, err := a.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(a.conf.Bucket),
		Key:    aws.String(id),
	})
	if err != nil {
		return nil, err
	}
	return out.Body, nil
}

// SaveFile uploads an artifact from a file path.
func (a *Adapter) SaveFile(ctx context.Context, _ string, artifact *internaltypes.Artifact, filePath string) error {
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
	defer func() { _ = f.Close() }()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
		return err
	}
	hex256 := hex.EncodeToString(hasher.Sum(nil))

	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return err
	}

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
	if err := a.checkBucket(ctx); err != nil {
		return err
	}

	input := a.buildPutInput(artifact, f, fi.Size())
	out, err := a.client.PutObject(ctx, input)
	if err != nil {
		return fmt.Errorf("upload failed for file=%s bucket=%s id=%s size=%d: %w",
			filePath, artifact.Bucket, artifact.ID, artifact.ContentLength, err)
	}
	if out.ETag != nil {
		artifact.ETag = strings.Trim(*out.ETag, `"`)
	}
	return nil
}

// SaveBytes uploads an artifact from a byte slice.
func (a *Adapter) SaveBytes(ctx context.Context, prefix string, name string, data []byte) (*internaltypes.Artifact, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("no data for %s", name)
	}
	sum := sha256.Sum256(data)
	hex256 := hex.EncodeToString(sum[:])

	artifact := &internaltypes.Artifact{
		Name:          name,
		Bucket:        a.conf.Bucket,
		SHA256:        hex256,
		ContentLength: int64(len(data)),
		ContentType:   "application/octet-stream",
		Metadata:      make(map[string]string),
		Tags:          make(map[string]string),
	}
	if err := a.SaveData(ctx, prefix, artifact, bytes.NewReader(data)); err != nil {
		return nil, err
	}
	return artifact, nil
}

// SaveData uploads an artifact from a reader.
func (a *Adapter) SaveData(ctx context.Context, prefix string, artifact *internaltypes.Artifact, reader io.Reader) error {
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
	artifact.Bucket = a.conf.Bucket

	input := a.buildPutInput(artifact, reader, artifact.ContentLength)
	out, err := a.client.PutObject(ctx, input)
	if err != nil {
		return err
	}
	if out.ETag != nil {
		artifact.ETag = strings.Trim(*out.ETag, `"`)
	}
	return nil
}

// Delete removes an artifact by ID.
func (a *Adapter) Delete(ctx context.Context, id string) error {
	_, err := a.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(a.conf.Bucket),
		Key:    aws.String(id),
	})
	return err
}

// PresignedGetURL returns a time-limited download URL.
func (a *Adapter) PresignedGetURL(ctx context.Context, id string, fileName string, expires time.Duration) (*url.URL, error) {
	req, err := a.presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket:                     aws.String(a.conf.Bucket),
		Key:                        aws.String(id),
		ResponseContentDisposition: aws.String(fmt.Sprintf("attachment; filename=%q", fileName)),
	}, s3.WithPresignExpires(expires))
	if err != nil {
		return nil, err
	}
	return url.Parse(req.URL)
}

// PresignedPutURL returns a time-limited upload URL.
func (a *Adapter) PresignedPutURL(ctx context.Context, id string, expires time.Duration) (*url.URL, error) {
	if err := a.checkBucket(ctx); err != nil {
		return nil, err
	}
	req, err := a.presignClient.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(a.conf.Bucket),
		Key:    aws.String(id),
	}, s3.WithPresignExpires(expires))
	if err != nil {
		return nil, err
	}
	return url.Parse(req.URL)
}

// List returns artifacts matching the given prefix.
func (a *Adapter) List(ctx context.Context, prefix string) ([]*internaltypes.Artifact, error) {
	fullPrefix := a.prefix + prefix
	paginator := s3.NewListObjectsV2Paginator(a.client, &s3.ListObjectsV2Input{
		Bucket: aws.String(a.conf.Bucket),
		Prefix: aws.String(fullPrefix),
	})

	var result []*internaltypes.Artifact
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, obj := range page.Contents {
			art := &internaltypes.Artifact{
				Bucket:        a.conf.Bucket,
				ID:            aws.ToString(obj.Key),
				ContentLength: aws.ToInt64(obj.Size),
			}
			if obj.ETag != nil {
				art.ETag = strings.Trim(*obj.ETag, `"`)
			}
			result = append(result, art)
		}
		// Note: ListObjectsV2 does not return user metadata; callers that need
		// SHA256/ContentType must issue individual HeadObject requests.
	}
	return result, nil
}

func (a *Adapter) checkBucket(ctx context.Context) error {
	a.lock.Lock()
	defer a.lock.Unlock()
	if a.verifiedBucket {
		return nil
	}
	// In local mode, block until the SeaweedFS subprocess is ready to accept S3 connections.
	if a.localServer != nil {
		if err := a.localServer.WaitReady(ctx); err != nil {
			return fmt.Errorf("seaweedfs: not ready: %w", err)
		}
	}
	_, err := a.client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(a.conf.Bucket),
	})
	if err != nil {
		if !isBucketNotFound(err) {
			return fmt.Errorf("failed to check bucket '%s': %w", a.conf.Bucket, err)
		}
		// Bucket doesn't exist — create it.
		// AWS S3 requires omitting CreateBucketConfiguration for us-east-1.
		input := &s3.CreateBucketInput{
			Bucket: aws.String(a.conf.Bucket),
		}
		if a.conf.Region != "" && a.conf.Region != "us-east-1" {
			input.CreateBucketConfiguration = &types.CreateBucketConfiguration{
				LocationConstraint: types.BucketLocationConstraint(a.conf.Region),
			}
		}
		if _, err = a.client.CreateBucket(ctx, input); err != nil {
			return fmt.Errorf("failed to create bucket '%s': %w", a.conf.Bucket, err)
		}
	}
	a.verifiedBucket = true
	return nil
}

// isBucketNotFound returns true when err represents an HTTP 404/NoSuchBucket response,
// which means the bucket does not exist (as opposed to auth errors or network failures).
func isBucketNotFound(err error) bool {
	var notFound *types.NotFound
	if errors.As(err, &notFound) {
		return true
	}
	var respErr *http.ResponseError
	if errors.As(err, &respErr) {
		return respErr.HTTPStatusCode() == 404
	}
	return false
}

func (a *Adapter) buildPutInput(artifact *internaltypes.Artifact, body io.Reader, size int64) *s3.PutObjectInput {
	contentType := artifact.ContentType
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	meta := make(map[string]string)
	for k, v := range artifact.Metadata {
		meta[k] = v
	}
	// Use uppercase keys to match the original MinIO implementation for backwards compatibility.
	meta["ID"] = artifact.ID
	meta["SHA256"] = artifact.SHA256

	input := &s3.PutObjectInput{
		Bucket:        aws.String(artifact.Bucket),
		Key:           aws.String(artifact.ID),
		Body:          body,
		ContentLength: aws.Int64(size),
		ContentType:   aws.String(contentType),
		Metadata:      meta,
	}
	// Encode tags as a URL-encoded query string (S3 tagging format).
	if len(artifact.Tags) > 0 {
		var tagParts []string
		for k, v := range artifact.Tags {
			tagParts = append(tagParts, url.QueryEscape(k)+"="+url.QueryEscape(v))
		}
		input.Tagging = aws.String(strings.Join(tagParts, "&"))
	}
	return input
}
