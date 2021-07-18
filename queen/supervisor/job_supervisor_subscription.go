package supervisor

import (
	"context"
	"github.com/sirupsen/logrus"
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
				// TODO accounting here
				if err := js.jobStateMachine.UpdateJobRequestTimestamp(ctx); err != nil {
					if logrus.IsLevelEnabled(logrus.DebugLevel) {
						logrus.WithFields(js.jobStateMachine.LogFields("JobSupervisor", err)).
							Debug("failed to update request timestamp")
					}
					ticker.Stop()
					return
				}
			}
		}
	}()
	return ticker
}
