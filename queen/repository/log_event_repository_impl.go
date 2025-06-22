package repository

import (
	"fmt"
	"github.com/sirupsen/logrus"
	common "plexobject.com/formicary/internal/types"
	"time"

	"github.com/oklog/ulid/v2"
	"gorm.io/gorm"
	"plexobject.com/formicary/internal/events"
)

var _ LogEventRepository = &LogEventRepositoryImpl{}

// LogEventRepositoryImpl implements LogEventRepository using gorm O/R mapping
type LogEventRepositoryImpl struct {
	db *gorm.DB
}

// NewLogEventRepositoryImpl creates new instance for audit-record-repository
func NewLogEventRepositoryImpl(db *gorm.DB) (*LogEventRepositoryImpl, error) {
	return &LogEventRepositoryImpl{db: db}, nil
}

// clear - for testing
func (l *LogEventRepositoryImpl) clear() {
	clearDB(l.db)
}

// Save persists audit-record
func (l *LogEventRepositoryImpl) Save(
	record *events.LogEvent) (*events.LogEvent, error) {
	err := record.Validate()
	if err != nil {
		return nil, common.NewValidationError(err)
	}
	err = l.db.Transaction(func(tx *gorm.DB) error {
		var res *gorm.DB
		record.ID = ulid.Make().String()
		record.CreatedAt = time.Now()
		res = tx.Create(record)
		if res.Error != nil {
			return res.Error
		}
		return nil
	})
	return record, err
}

// DeleteByRequestID delete all logs by request-id
func (l *LogEventRepositoryImpl) DeleteByRequestID(requestID string) (int64, error) {
	res := l.db.Where("job_request_id = ?", requestID).Delete(&events.LogEvent{})
	if res.Error != nil {
		return 0, res.Error
	}
	return res.RowsAffected, nil
}

// DeleteByJobExecutionID delete all logs by job-execution-id
func (l *LogEventRepositoryImpl) DeleteByJobExecutionID(jobExecutionID string) (int64, error) {
	res := l.db.Where("job_execution_id = ?", jobExecutionID).Delete(&events.LogEvent{})
	if res.Error != nil {
		return 0, res.Error
	}
	return res.RowsAffected, nil
}

// DeleteByTaskExecutionID delete all logs by task-execution-id
func (l *LogEventRepositoryImpl) DeleteByTaskExecutionID(taskExecutionID string) (int64, error) {
	res := l.db.Where("task_execution_id = ?", taskExecutionID).Delete(&events.LogEvent{})
	if res.Error != nil {
		return 0, res.Error
	}
	return res.RowsAffected, nil
}

// Query finds matching audit-records by parameters
func (l *LogEventRepositoryImpl) Query(
	params map[string]interface{},
	page int,
	pageSize int,
	order []string) (records []*events.LogEvent, totalRecords int64, err error) {
	records = make([]*events.LogEvent, 0)
	tx := l.db.Limit(pageSize).Offset(page * pageSize)
	if len(order) > 0 {
		for _, ord := range order {
			tx = tx.Order(ord)
		}
	}
	tx = l.addQuery(params, tx)

	res := tx.Find(&records)
	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		logrus.WithFields(logrus.Fields{
			"Component": "LogEventRepositoryImpl",
			"Query":     res.Statement.SQL,
			"Vars":      res.Statement.Vars,
			"Error":     res.Error,
			"Affected":  res.RowsAffected,
			"Records":   len(records),
			"Params":    params,
			"PageSize":  pageSize,
			"Page":      page,
		}).Debugf("queried log events")
	}

	if res.Error != nil {
		err = res.Error
		return nil, 0, err
	}

	for _, r := range records {
		r.AfterLoad()
	}

	totalRecords, _ = l.Count(params)
	return
}

// ExpireLogEvents delete old logs
func (l *LogEventRepositoryImpl) ExpireLogEvents(
	qc *common.QueryContext,
	expiration time.Duration) (int64, error) {
	res := qc.AddUserWhere(l.db.Model(&events.LogEvent{}), false).
		Where("created_at < ?", time.Now().Add(-expiration)).Delete(&events.LogEvent{})
	if res.Error != nil {
		return 0, res.Error
	}
	return res.RowsAffected, nil
}

// Count counts records by query
func (l *LogEventRepositoryImpl) Count(
	params map[string]interface{}) (totalRecords int64, err error) {
	tx := l.db.Model(&events.LogEvent{})
	tx = l.addQuery(params, tx)
	res := tx.Count(&totalRecords)
	if res.Error != nil {
		err = res.Error
		return 0, err
	}
	return
}

func (l *LogEventRepositoryImpl) addQuery(params map[string]interface{}, tx *gorm.DB) *gorm.DB {
	q := params["q"]
	if q != nil {
		qs := fmt.Sprintf("%%%s%%", q)
		tx = tx.Where("user_id LIKE ? OR encoded_message LIKE ? OR job_request_id = ?",
			qs, qs, q)
	}
	return addQueryParamsWhere(filterParams(params, "q"), tx)
}
