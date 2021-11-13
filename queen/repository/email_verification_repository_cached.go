package repository

import (
	"github.com/karlseguin/ccache/v2"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/config"
	"plexobject.com/formicary/queen/types"
)

// EmailVerificationRepositoryCached implements EmailVerificationRepository with caching support
type EmailVerificationRepositoryCached struct {
	serverConf *config.ServerConfig
	adapter    EmailVerificationRepository
	cache      *ccache.Cache
}

// NewEmailVerificationRepositoryCached creates new instance for user-repository
func NewEmailVerificationRepositoryCached(
	serverConf *config.ServerConfig,
	adapter EmailVerificationRepository) (EmailVerificationRepository, error) {
	var cache = ccache.New(ccache.Configure().MaxSize(serverConf.Jobs.DBObjectCacheSize).ItemsToPrune(1000))
	return &EmailVerificationRepositoryCached{
		adapter:    adapter,
		serverConf: serverConf,
		cache:      cache,
	}, nil
}

// Delete deletes record
func (urc *EmailVerificationRepositoryCached) Delete(
	qc *common.QueryContext,
	id string) (err error) {
	err = urc.adapter.Delete(qc, id)
	if err == nil {
		urc.cache.DeletePrefix("VerifiedEmails:" + id)
	}
	return
}

// Get fetches record by id
func (urc *EmailVerificationRepositoryCached) Get(
	qc *common.QueryContext,
	id string) (*types.EmailVerification, error) {
	return urc.adapter.Get(qc, id)
}

// Create - creates new email verification
func (urc *EmailVerificationRepositoryCached) Create(
	ev *types.EmailVerification) (*types.EmailVerification, error) {
	return urc.adapter.Create(ev)
}

// GetVerifiedEmails returns verified emails
func (urc *EmailVerificationRepositoryCached) GetVerifiedEmails(
	qc *common.QueryContext,
	user *common.User,
) map[string]bool {
	item, err := urc.cache.Fetch("VerifiedEmails:"+user.ID,
		urc.serverConf.Jobs.DBObjectCache, func() (interface{}, error) {
			return urc.adapter.GetVerifiedEmails(qc, user), nil
		})
	if err != nil {
		return make(map[string]bool)
	}
	return item.Value().(map[string]bool)
}

// Query returns matching email verification records
func (urc *EmailVerificationRepositoryCached) Query(
	qc *common.QueryContext,
	params map[string]interface{},
	page int,
	pageSize int,
	order []string) (recs []*types.EmailVerification, totalRecords int64, err error) {
	return urc.adapter.Query(qc, params, page, pageSize, order)
}

// Verify updates database for email verification
func (urc *EmailVerificationRepositoryCached) Verify(
	qc *common.QueryContext,
	user *common.User,
	code string) (recs *types.EmailVerification, err error) {
	recs, err = urc.adapter.Verify(qc, user, code)
	if err == nil {
		urc.cache.DeletePrefix("VerifiedEmails:" + user.ID)
	}
	return
}
