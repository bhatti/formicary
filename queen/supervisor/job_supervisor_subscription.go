package supervisor

import (
	"context"
	"github.com/sirupsen/logrus"
	common "plexobject.com/formicary/internal/types"
	"time"
)

func (js *JobSupervisor) startTickerToUpdateRequestTimestamp(ctx context.Context) *time.Ticker {
	ticker := time.NewTicker(js.serverCfg.Jobs.OrphanRequestsUpdateInterval)
	go func() {
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				return
			case <-ticker.C:
				if err := js.jobStateMachine.UpdateJobRequestTimestampAndCheckQuota(ctx); err != nil {
					switch err.(type) {
					case *common.QuotaExceededError:
						js.cancel()
						js.jobStateMachine.JobExecution.ErrorMessage = err.Error()
						js.jobStateMachine.JobExecution.ErrorCode = common.ErrorQuotaExceeded
						logrus.WithFields(js.jobStateMachine.LogFields("JobSupervisor", err)).
							Warnf("received quota error while executing, cancelling the job")
						ticker.Stop()
						// publish event
						js.jobStateMachine.JobManager.CancelJobRequest(
							js.jobStateMachine.QueryContext(),
							js.jobStateMachine.Request.GetID())
						return
					}
				}
			}
		}
	}()
	return ticker
}
