package controller

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
	"plexobject.com/formicary/internal/acl"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/internal/web"
	"plexobject.com/formicary/queen/repository"
	"plexobject.com/formicary/queen/types"
)

// SubscriptionController structure
type SubscriptionController struct {
	subscriptionRepository repository.SubscriptionRepository
	userRepository         repository.UserRepository
	orgRepository          repository.OrganizationRepository
	auditRecordRepository  repository.AuditRecordRepository
	webserver              web.Server
}

// NewSubscriptionController instantiates controller for updating subscriptions
func NewSubscriptionController(
	subscriptionRepository repository.SubscriptionRepository,
	userRepository repository.UserRepository,
	orgRepository repository.OrganizationRepository,
	auditRecordRepository repository.AuditRecordRepository,
	webserver web.Server) *SubscriptionController {
	ctr := &SubscriptionController{
		subscriptionRepository: subscriptionRepository,
		userRepository:         userRepository,
		orgRepository:          orgRepository,
		auditRecordRepository:  auditRecordRepository,
		webserver:              webserver,
	}
	webserver.GET("/api/subscriptions", ctr.querySubscriptions, acl.NewPermission(acl.Subscription, acl.Query)).Name = "query_subscriptions"
	webserver.GET("/api/subscriptions/:id", ctr.getSubscription, acl.NewPermission(acl.Subscription, acl.View)).Name = "get_subscription"
	webserver.POST("/api/subscriptions", ctr.postSubscription, acl.NewPermission(acl.Subscription, acl.Create)).Name = "create_subscription"
	webserver.PUT("/api/subscriptions/:id", ctr.putSubscription, acl.NewPermission(acl.Subscription, acl.Update)).Name = "update_subscription"
	webserver.DELETE("/api/subscriptions/:id", ctr.deleteSubscription, acl.NewPermission(acl.Subscription, acl.Delete)).Name = "delete_subscription"
	return ctr
}

// ********************************* HTTP Handlers ***********************************

// swagger:route GET /api/subscriptions subscriptions querySubscriptions
// Queries system subscriptions.
// `This requires admin access`
// responses:
//
//	200: subscriptionQueryResponse
func (cc *SubscriptionController) querySubscriptions(c web.APIContext) error {
	qc := web.BuildQueryContext(c)
	params, order, page, pageSize, _, _ := ParseParams(c)
	recs, total, err := cc.subscriptionRepository.Query(qc, params, page, pageSize, order)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, NewPaginatedResult(recs, total, page, pageSize))
}

// swagger:route POST /api/subscriptions subscriptions postSubscription
// Creates new system subscription based on request body.
// `This requires admin access`
// responses:
//
//	200: subscriptionResponse
func (cc *SubscriptionController) postSubscription(c web.APIContext) (err error) {
	qc := common.NewQueryContext(nil, "").WithAdmin()
	subscription, err := cc.buildSubscription(c)
	if err != nil {
		return err
	}
	var saved *common.Subscription
	status := http.StatusCreated
	if subscription.ID != "" {
		saved, err = cc.subscriptionRepository.Update(qc, subscription)
		status = http.StatusOK
		logrus.WithFields(logrus.Fields{
			"Component":    "SubscriptionController",
			"Subscription": subscription,
		}).Info("updated Subscription")
	} else {
		saved, err = cc.subscriptionRepository.Create(qc, subscription)
		logrus.WithFields(logrus.Fields{
			"Component":    "SubscriptionController",
			"Subscription": subscription,
		}).Info("created Subscription")
	}
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"Component":    "SubscriptionController",
			"Subscription": subscription,
			"Error":        err,
		}).Warnf("failed to create Subscription")
		return err
	}
	_, _ = cc.auditRecordRepository.Save(types.NewAuditRecordFromSubscription(saved, qc))
	return c.JSON(status, saved)
}

// swagger:route PUT /api/subscriptions/{id} subscriptions putSubscription
// Updates an existing system subscription based on request body.
// `This requires admin access`
// responses:
//
//	200: subscriptionResponse
func (cc *SubscriptionController) putSubscription(c web.APIContext) error {
	qc := web.BuildQueryContext(c)
	subscription, err := cc.buildSubscription(c)
	if err != nil {
		return err
	}
	subscription.ID = c.Param("id")
	saved, err := cc.subscriptionRepository.Update(qc, subscription)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"Component":    "SubscriptionController",
			"Subscription": subscription,
			"Error":        err,
		}).Warnf("failed to update Subscription")
		return err
	}

	logrus.WithFields(logrus.Fields{
		"Component":    "SubscriptionController",
		"Subscription": subscription,
	}).Info("Updated Subscription")
	_, _ = cc.auditRecordRepository.Save(types.NewAuditRecordFromSubscription(saved, qc))
	return c.JSON(http.StatusOK, saved)
}

// swagger:route GET /api/subscriptions/{id} subscriptions getSubscription
// Finds an existing system subscription based on id.
// `This requires admin access`
// responses:
//
//	200: subscriptionResponse
func (cc *SubscriptionController) getSubscription(c web.APIContext) error {
	qc := web.BuildQueryContext(c)
	subscription, err := cc.subscriptionRepository.Get(qc, c.Param("id"))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, subscription)
}

// swagger:route DELETE /api/subscriptions/{id} subscriptions deleteSubscription
// Deletes an existing system subscription based on id.
// `This requires admin access`
// responses:
//
//	200: emptyResponse
func (cc *SubscriptionController) deleteSubscription(c web.APIContext) error {
	qc := web.BuildQueryContext(c)
	err := cc.subscriptionRepository.Delete(qc, c.Param("id"))
	if err != nil {
		return err
	}
	return c.NoContent(http.StatusOK)
}

// ********************************* Swagger types ***********************************

// swagger:parameters querySubscriptions
// The params for querying subscriptions
type subscriptionsQueryParams struct {
	// in:query
	Page     int `json:"page"`
	PageSize int `json:"page_size"`
	// Scope defines scope such as default or org-unit
	Scope string `json:"scope"`
	// Kind defines kind of subscription property
	Kind string `json:"kind"`
	// Name defines name of subscription property
	Name string `json:"name"`
}

// Query results of subscriptions
// swagger:response subscriptionQueryResponse
type subscriptionsQueryResponseBody struct {
	// in:body
	Body struct {
		Records      []*common.Subscription
		TotalRecords int64
		Page         int
		PageSize     int
		TotalPages   int64
	}
}

// swagger:parameters postSubscription
// The params for system-subscription
type subscriptionCreateParams struct {
	// in:body
	Body common.Subscription
}

// swagger:parameters putSubscription
// The params for system-subscription
type subscriptionUpdateParams struct {
	// in:path
	ID string `json:"id"`
	// in:body
	Body common.Subscription
}

// Subscription body for update
// swagger:response subscriptionResponse
type subscriptionResponseBody struct {
	// in:body
	Body common.Subscription
}

// swagger:parameters deleteSubscription getSubscription
// The parameters for finding system-subscription by id
type subscriptionIDParams struct {
	// in:path
	ID string `json:"id"`
}

func (cc *SubscriptionController) buildSubscription(c web.APIContext) (*common.Subscription, error) {
	qc := common.NewQueryContext(nil, "").WithAdmin()
	subscription := &common.Subscription{
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	err := json.NewDecoder(c.Request().Body).Decode(subscription)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"Component":    "SubscriptionController",
			"Subscription": subscription,
			"Error":        err,
		}).Warnf("failed to deserialize subscription")
		return nil, err
	}
	if subscription.UserID == "" && subscription.OrganizationID == "" {
		return nil, common.NewValidationError("user_id or organization_id is not specified")
	}
	var oldSubscription *common.Subscription
	if subscription.OrganizationID != "" {
		org, err := cc.orgRepository.Get(qc, subscription.OrganizationID)
		if err != nil {
			return nil, err
		}
		if org.Subscription != nil && !org.Subscription.Expired() {
			oldSubscription = org.Subscription
		}
	} else if subscription.UserID != "" {
		user, err := cc.userRepository.Get(qc, subscription.UserID)
		if err != nil {
			return nil, err
		}
		if user.Subscription != nil && !user.Subscription.Expired() {
			oldSubscription = user.Subscription
		}
	}
	freeSubscription := common.NewFreemiumSubscription(&common.User{})
	// recent subscription
	if oldSubscription != nil && oldSubscription.StartedAt.Unix() > time.Now().Add(time.Hour*-48).Unix() {
		subscription.ID = oldSubscription.ID
		if oldSubscription.CreatedAt.IsZero() {
			subscription.CreatedAt = oldSubscription.StartedAt
		} else {
			subscription.CreatedAt = oldSubscription.CreatedAt
		}
		if subscription.Kind == "" {
			subscription.Kind = oldSubscription.Kind
		}
		if subscription.Policy == "" {
			subscription.Policy = oldSubscription.Policy
		}
		if subscription.Period == "" {
			subscription.Period = oldSubscription.Period
		}
		if subscription.Price == 0 {
			subscription.Price = oldSubscription.Price
		}
		if subscription.StartedAt.IsZero() {
			subscription.StartedAt = oldSubscription.StartedAt
		}
		if subscription.EndedAt.IsZero() {
			subscription.EndedAt = oldSubscription.EndedAt
		}

		if subscription.CPUQuota == 0 {
			subscription.CPUQuota = oldSubscription.CPUQuota + freeSubscription.CPUQuota
		}

		if subscription.DiskQuota == 0 {
			subscription.DiskQuota = oldSubscription.DiskQuota + freeSubscription.DiskQuota
		}
	} else {
		if subscription.Kind == "" {
			subscription.Kind = freeSubscription.Kind
		}
		if subscription.Policy == "" {
			subscription.Policy = freeSubscription.Policy
		}
		if subscription.Period == "" {
			subscription.Period = freeSubscription.Period
		}
		if subscription.Price == 0 {
			subscription.Price = freeSubscription.Price
		}
		if subscription.StartedAt.IsZero() {
			subscription.StartedAt = freeSubscription.StartedAt
		}
		if subscription.EndedAt.IsZero() {
			subscription.EndedAt = freeSubscription.EndedAt
		}
		if subscription.CPUQuota == 0 {
			subscription.CPUQuota += freeSubscription.CPUQuota
		}
		if subscription.DiskQuota == 0 {
			subscription.DiskQuota += freeSubscription.DiskQuota
		}
	}
	return subscription, nil
}
