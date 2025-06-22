package manager

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/oklog/ulid/v2"
	"plexobject.com/formicary/internal/artifacts"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/config"
	"plexobject.com/formicary/queen/repository"
	"plexobject.com/formicary/queen/types"
)

// ArtifactManager  for managing artifacts
type ArtifactManager struct {
	serverCfg          *config.ServerConfig
	artifactRepository repository.ArtifactRepository
	logEventRepository repository.LogEventRepository
	artifactService    artifacts.Service
}

// NewArtifactManager manages artifacts
func NewArtifactManager(
	serverCfg *config.ServerConfig,
	logEventRepository repository.LogEventRepository,
	artifactRepository repository.ArtifactRepository,
	artifactService artifacts.Service) (*ArtifactManager, error) {
	if artifactRepository == nil {
		return nil, fmt.Errorf("artifact-repository is not specified")
	}
	if artifactService == nil {
		return nil, fmt.Errorf("artifact-service is not specified")
	}
	return &ArtifactManager{
		serverCfg:          serverCfg,
		logEventRepository: logEventRepository,
		artifactRepository: artifactRepository,
		artifactService:    artifactService,
	}, nil
}

// ExpireArtifacts - expire and removes artifact
func (am *ArtifactManager) ExpireArtifacts(
	ctx context.Context,
	qc *common.QueryContext,
	expiration time.Duration,
	limit int,
) (expired int, size int64, err error) {
	if _, err = am.logEventRepository.ExpireLogEvents(qc, expiration); err != nil {
		logrus.WithFields(
			logrus.Fields{
				"Component":  "ArtifactManager",
				"QC":         qc,
				"Expiration": expiration,
				"Error":      err,
			}).Warnf("failed to expire log events")
	}

	for i := 0; i < limit; i += 10000 {
		records, err := am.artifactRepository.ExpiredArtifacts(qc, expiration, 10000)
		if err != nil {
			return 0, 0, err
		}
		expired += len(records)
		for _, rec := range records {
			size += rec.ContentLength
			if err = am.DeleteArtifact(ctx, qc, rec.ID); err != nil {
				return 0, 0, err
			}
			logrus.WithFields(
				logrus.Fields{
					"Component":          "ArtifactManager",
					"QC":                 qc,
					"ID":                 rec.ID,
					"ArtifactExpiration": rec.ExpiresAt,
					"ArtifactUpdated":    rec.UpdatedAt,
					"ArtifactSize":       rec.ContentLength,
					"Expiration":         expiration,
					"ArtifactName":       rec.Name,
					"RequestID":          rec.JobRequestID,
				}).Infof("expired artifact")
		}
	}
	logrus.WithFields(
		logrus.Fields{
			"Component":  "ArtifactManager",
			"QC":         qc,
			"Expiration": expiration,
			"Total":      expired,
			"Size":       size,
		}).Infof("total expired artifacts")
	return
}

// QueryArtifacts - queries artifact
func (am *ArtifactManager) QueryArtifacts(
	ctx context.Context,
	qc *common.QueryContext,
	params map[string]interface{},
	page int,
	pageSize int,
	order []string) (arts []*common.Artifact, total int64, err error) {
	records, total, err := am.artifactRepository.Query(qc, params, page, pageSize, order)
	if err != nil {
		return nil, 0, err
	}
	for _, art := range records {
		am.UpdateURL(ctx, art)
	}
	return records, total, nil
}

// UploadArtifact - artifacts
func (am *ArtifactManager) UploadArtifact(
	ctx context.Context,
	qc *common.QueryContext,
	body io.ReadCloser,
	params map[string]string) (*common.Artifact, error) {
	tmpFile, err := ioutil.TempFile(os.TempDir(), ulid.Make().String())
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file due to %w", err)
	}
	if body == nil {
		return nil, fmt.Errorf("artifact body is nil")
	}
	defer func() {
		_ = tmpFile.Close()
		_ = os.Remove(tmpFile.Name())
	}()

	dst, err := os.Create(tmpFile.Name())
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file due to %w", err)
	}
	_, err = io.Copy(dst, body)
	ioutil.NopCloser(dst)
	if err != nil {
		return nil, fmt.Errorf("failed to copy file due to %w", err)
	}

	artifact := &common.Artifact{
		Name:           ulid.Make().String(),
		Metadata:       params,
		Kind:           common.ArtifactKindUser,
		Tags:           make(map[string]string),
		UserID:         qc.GetUserID(),
		OrganizationID: qc.GetOrganizationID(),
		ExpiresAt:      time.Now().Add(am.serverCfg.DefaultArtifactExpiration),
	}

	if err = am.artifactService.SaveFile(
		ctx,
		artifact.UserID,
		artifact,
		tmpFile.Name()); err != nil {
		return nil, fmt.Errorf("failed to upload file due to %w", err)
	}

	if _, err = am.artifactRepository.Save(artifact); err != nil {
		return nil, fmt.Errorf("failed to save file due to %w", err)
	}
	return artifact, nil
}

// DownloadArtifactBySHA256 - download artifact by sha256
func (am *ArtifactManager) DownloadArtifactBySHA256(
	ctx context.Context,
	qc *common.QueryContext,
	sha256 string) (io.ReadCloser, string, string, error) {
	art, err := am.artifactRepository.GetBySHA256(qc, sha256)
	if err != nil {
		return nil, "", "", err
	}
	reader, err := am.artifactService.Get(
		ctx,
		art.ID)
	return reader, art.Name, art.ContentType, err
}

// GetArtifact - finds artifact by id
func (am *ArtifactManager) GetArtifact(
	ctx context.Context,
	qc *common.QueryContext,
	id string) (*common.Artifact, error) {
	art, err := am.artifactRepository.Get(qc, id)
	if err != nil {
		return nil, err
	}
	am.UpdateURL(ctx, art)
	return art, nil
}

// UpdateArtifact - updates artifact
func (am *ArtifactManager) UpdateArtifact(
	ctx context.Context,
	qc *common.QueryContext,
	artifact *common.Artifact) (saved *common.Artifact, err error) {
	am.UpdateURL(ctx, artifact)
	return am.artifactRepository.Update(qc, artifact)
}

// DeleteArtifact - deletes artifact by id
func (am *ArtifactManager) DeleteArtifact(
	ctx context.Context,
	qc *common.QueryContext,
	id string) error {
	svcErr := am.artifactService.Delete(ctx, id)
	dbErr := am.artifactRepository.Delete(qc, id)

	logrus.WithFields(
		logrus.Fields{
			"Component": "ArtifactManager",
			"QC":        qc,
			"ID":        id,
		}).Infof("deleted artifact")
	if svcErr != nil {
		return svcErr
	}
	return dbErr
}

// UpdateURL - using presigned or external api
func (am *ArtifactManager) UpdateURL(
	ctx context.Context,
	art *common.Artifact) {
	if am.serverCfg.Common.ExternalBaseURL == "" {
		if url, err := am.artifactService.PresignedGetURL(
			ctx,
			art.ID,
			art.Name,
			am.serverCfg.URLPresignedExpirationMinutes*time.Minute); err == nil {
			art.URL = url.String()
		}
	} else {
		art.URL = am.serverCfg.Common.ExternalBaseURL + "/api/artifacts/" + art.SHA256 + "/download"
	}
}

// GetResourceUsage - Finds usage between time
func (am *ArtifactManager) GetResourceUsage(
	qc *common.QueryContext,
	ranges []types.DateRange) ([]types.ResourceUsage, error) {
	return am.artifactRepository.GetResourceUsage(qc, ranges)
}
