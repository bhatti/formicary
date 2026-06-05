// SPDX-License-Identifier: AGPL-3.0-or-later

package trigger

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"plexobject.com/formicary/internal/tracing"
	internaltypes "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/repository"
	"plexobject.com/formicary/queen/types"
)

// S3Poller periodically lists S3 objects and fires a trigger for new ones.
type S3Poller struct {
	s3Client         *s3.Client
	evaluator        *Evaluator
	submitter        *Submitter
	triggerStateRepo repository.TriggerStateRepository
	jobDef           *types.JobDefinition
	trigger          *types.TriggerDefinition
	defaultInterval  time.Duration
	stopCh           chan struct{}
	wg               sync.WaitGroup
}

// NewS3Poller creates and starts an S3Poller.
func NewS3Poller(
	ctx context.Context,
	s3Cfg *internaltypes.S3Config,
	evaluator *Evaluator,
	submitter *Submitter,
	triggerStateRepo repository.TriggerStateRepository,
	jobDef *types.JobDefinition,
	trigger *types.TriggerDefinition,
	defaultInterval time.Duration,
) (*S3Poller, error) {
	s3Client, err := newS3Client(ctx, s3Cfg)
	if err != nil {
		return nil, err
	}
	interval := trigger.PollInterval
	if interval <= 0 {
		interval = defaultInterval
	}
	if interval <= 0 {
		interval = 60 * time.Second
	}
	p := &S3Poller{
		s3Client:         s3Client,
		evaluator:        evaluator,
		submitter:        submitter,
		triggerStateRepo: triggerStateRepo,
		jobDef:           jobDef,
		trigger:          trigger,
		defaultInterval:  interval,
		stopCh:           make(chan struct{}),
	}
	p.start(ctx)
	return p, nil
}

// Stop halts the polling goroutine.
func (p *S3Poller) Stop() {
	close(p.stopCh)
	p.wg.Wait()
}

func (p *S3Poller) start(ctx context.Context) {
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		ticker := time.NewTicker(p.defaultInterval)
		defer ticker.Stop()
		// Poll immediately on first start.
		p.poll(ctx)
		for {
			select {
			case <-p.stopCh:
				return
			case <-ticker.C:
				p.poll(ctx)
			}
		}
	}()
}

func (p *S3Poller) poll(ctx context.Context) {
	ctx, span := tracing.Tracer("formicary.trigger").Start(ctx, "trigger.s3_poll",
		trace.WithAttributes(
			attribute.String("trigger.name", p.trigger.Name),
			attribute.String("job.type", p.jobDef.JobType),
			attribute.String("s3.bucket", p.trigger.Bucket),
			attribute.String("s3.prefix", p.trigger.Prefix),
		),
	)
	defer func() { span.End() }()

	state, err := p.triggerStateRepo.FindByJobAndTrigger(p.jobDef.ID, p.trigger.Name)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return
	}
	// On first run (no state), initialize marker to now so we don't backfill all historical objects.
	if state == nil {
		state = &types.TriggerState{
			JobDefinitionID: p.jobDef.ID,
			TriggerName:     p.trigger.Name,
			LastSeenTime:    time.Now(),
		}
		if _, err = p.triggerStateRepo.Upsert(state); err != nil {
			logrus.WithFields(logrus.Fields{
				"Component":   "S3Poller",
				"JobType":     p.jobDef.JobType,
				"TriggerName": p.trigger.Name,
			}).Warnf("failed to initialize trigger state: %v", err)
		}
		return
	}

	input := &s3.ListObjectsV2Input{
		Bucket: aws.String(p.trigger.Bucket),
	}
	if p.trigger.Prefix != "" {
		input.Prefix = aws.String(p.trigger.Prefix)
	}
	if state.LastSeenKey != "" {
		input.StartAfter = aws.String(state.LastSeenKey)
	}

	paginator := s3.NewListObjectsV2Paginator(p.s3Client, input)
	var lastKey string
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			logrus.WithFields(logrus.Fields{
				"Component":   "S3Poller",
				"JobType":     p.jobDef.JobType,
				"TriggerName": p.trigger.Name,
			}).Errorf("S3 list failed: %v", err)
			return
		}
		for _, obj := range page.Contents {
			key := aws.ToString(obj.Key)
			if p.trigger.Suffix != "" && !strings.HasSuffix(key, p.trigger.Suffix) {
				lastKey = key
				continue
			}
			etag := ""
			if obj.ETag != nil {
				etag = strings.Trim(*obj.ETag, `"`)
			}
			objData := map[string]interface{}{
				"Key":          key,
				"Bucket":       p.trigger.Bucket,
				"Size":         aws.ToInt64(obj.Size),
				"ETag":         etag,
				"LastModified": obj.LastModified,
			}
			data := map[string]interface{}{
				"Object": objData,
			}
			result, err := p.evaluator.Evaluate(ctx, &TriggerEvent{
				JobDefinition: p.jobDef,
				Trigger:       p.trigger,
				Data:          data,
			})
			if err != nil {
				// Don't advance the cursor — this object must be retried on the next poll.
				logrus.WithFields(logrus.Fields{
					"Component":   "S3Poller",
					"JobType":     p.jobDef.JobType,
					"TriggerName": p.trigger.Name,
					"Key":         key,
				}).Errorf("evaluator error: %v", err)
				continue
			}
			if result.Passed {
				if _, err = p.submitter.Submit(ctx, p.jobDef, p.trigger.Name, result); err != nil {
					logrus.WithFields(logrus.Fields{
						"Component":   "S3Poller",
						"JobType":     p.jobDef.JobType,
						"TriggerName": p.trigger.Name,
						"Key":         key,
					}).Errorf("submit failed: %v", err)
					continue
				}
			}
			lastKey = key
		}
	}

	if lastKey != "" && lastKey != state.LastSeenKey {
		state.LastSeenKey = lastKey
		state.LastSeenTime = time.Now()
		if _, err = p.triggerStateRepo.Upsert(state); err != nil {
			logrus.WithFields(logrus.Fields{
				"Component":   "S3Poller",
				"JobType":     p.jobDef.JobType,
				"TriggerName": p.trigger.Name,
			}).Warnf("failed to update trigger state: %v", err)
		}
	}
}

func newS3Client(ctx context.Context, cfg *internaltypes.S3Config) (*s3.Client, error) {
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(cfg.Region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			cfg.AccessKeyID, cfg.SecretAccessKey, "",
		)),
	)
	if err != nil {
		return nil, err
	}
	return s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.UsePathStyle = true
		if cfg.Endpoint != "" {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
		}
	}), nil
}
