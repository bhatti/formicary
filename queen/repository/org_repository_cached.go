package repository

import (
	"github.com/karlseguin/ccache/v2"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/config"
	"plexobject.com/formicary/queen/types"
)

// OrganizationRepositoryCached implements OrgRepository with caching support
type OrganizationRepositoryCached struct {
	serverConf *config.ServerConfig
	adapter    OrganizationRepository
	cache      *ccache.Cache
}

// NewOrganizationRepositoryCached creates new instance for job-definition-repository
func NewOrganizationRepositoryCached(
	serverConf *config.ServerConfig,
	adapter OrganizationRepository) (OrganizationRepository, error) {
	var cache = ccache.New(ccache.Configure().MaxSize(serverConf.Jobs.DBObjectCacheSize).ItemsToPrune(100))
	return &OrganizationRepositoryCached{
		adapter:    adapter,
		serverConf: serverConf,
		cache:      cache,
	}, nil
}

// Get method finds Organization by id
func (orc *OrganizationRepositoryCached) Get(
	qc *common.QueryContext,
	id string) (*common.Organization, error) {
	item, err := orc.cache.Fetch("Org:"+id,
		orc.serverConf.Jobs.DBObjectCache, func() (interface{}, error) {
			return orc.adapter.Get(qc, id)
		})
	if err != nil {
		return nil, err
	}
	return item.Value().(*common.Organization), nil
}

// GetByUnit method finds Organization by unit
func (orc *OrganizationRepositoryCached) GetByUnit(
	qc *common.QueryContext,
	unit string) (*common.Organization, error) {
	item, err := orc.cache.Fetch("OrgUnit:"+unit,
		orc.serverConf.Jobs.DBObjectCache, func() (interface{}, error) {
			return orc.adapter.GetByUnit(qc, unit)
		})
	if err != nil {
		return nil, err
	}
	return item.Value().(*common.Organization), nil
}

// GetByParentID method finds Organization by parent-id
func (orc *OrganizationRepositoryCached) GetByParentID(
	qc *common.QueryContext,
	parentID string) (recs []*common.Organization, err error) {
	return orc.adapter.GetByParentID(qc, parentID)
}

// Delete org
func (orc *OrganizationRepositoryCached) Delete(
	qc *common.QueryContext,
	id string) error {
	org, err := orc.Get(qc, id)
	if err != nil {
		return err
	}
	err = orc.adapter.Delete(qc, id)
	if err != nil {
		return err
	}
	orc.cache.DeletePrefix("Org:" + id)
	orc.cache.DeletePrefix("OrgUnit:" + org.OrgUnit)
	return nil
}

// Create persists org
func (orc *OrganizationRepositoryCached) Create(
	qc *common.QueryContext,
	org *common.Organization) (*common.Organization, error) {
	saved, err := orc.adapter.Create(qc, org)
	if err != nil {
		return nil, err
	}
	orc.cache.DeletePrefix("Org:" + saved.ID)
	orc.cache.DeletePrefix("OrgUnit:" + saved.OrgUnit)
	return saved, nil
}

// UpdateStickyMessage updates sticky message for user and org
func (orc *OrganizationRepositoryCached) UpdateStickyMessage(
	qc *common.QueryContext,
	user *common.User,
	org *common.Organization) error {
	return orc.adapter.UpdateStickyMessage(qc, user, org)
}

// Update persists org
func (orc *OrganizationRepositoryCached) Update(
	qc *common.QueryContext,
	org *common.Organization) (*common.Organization, error) {
	saved, err := orc.adapter.Update(qc, org)
	if err != nil {
		return nil, err
	}
	orc.cache.DeletePrefix("Org:" + saved.ID)
	orc.cache.DeletePrefix("OrgUnit:" + saved.OrgUnit)
	return saved, nil
}

// Query finds matching configs
func (orc *OrganizationRepositoryCached) Query(
	qc *common.QueryContext,
	params map[string]interface{},
	page int,
	pageSize int,
	order []string) (recs []*common.Organization, totalRecords int64, err error) {
	return orc.adapter.Query(qc, params, page, pageSize, order)
}

// Count counts records by query
func (orc *OrganizationRepositoryCached) Count(
	qc *common.QueryContext,
	params map[string]interface{}) (totalRecords int64, err error) {
	return orc.adapter.Count(qc, params)
}

// AddInvitation adds invitation
func (orc *OrganizationRepositoryCached) AddInvitation(invitation *types.UserInvitation) error {
	return orc.adapter.AddInvitation(invitation)
}

// GetInvitation finds invitation
func (orc *OrganizationRepositoryCached) GetInvitation(id string) (*types.UserInvitation, error) {
	return orc.adapter.GetInvitation(id)
}

// FindInvitation finds invitation
func (orc *OrganizationRepositoryCached) FindInvitation(email string, code string) (*types.UserInvitation, error) {
	return orc.adapter.FindInvitation(email, code)
}

// AcceptInvitation accepts invitation
func (orc *OrganizationRepositoryCached) AcceptInvitation(email string, code string) (*types.UserInvitation, error) {
	return orc.adapter.AcceptInvitation(email, code)
}

// Clear removes cache
func (orc *OrganizationRepositoryCached) Clear() {
	orc.cache.DeletePrefix("Org")
	orc.adapter.Clear()
}
