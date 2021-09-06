package controller

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/internal/web"
	"plexobject.com/formicary/queen/repository"
)

func Test_InitializeSwaggerStructsForSubscriptionController(t *testing.T) {
	_ = subscriptionsQueryParams{}
	_ = subscriptionsQueryResponseBody{}
	_ = subscriptionCreateParams{}
	_ = subscriptionUpdateParams{}
	_ = subscriptionResponseBody{}
	_ = subscriptionIDParams{}
}

func Test_ShouldQuerySubscriptions(t *testing.T) {
	// GIVEN subscription controller
	subscriptionRepository, err := repository.NewTestSubscriptionRepository()
	require.NoError(t, err)
	userRepository, err := repository.NewTestUserRepository()
	require.NoError(t, err)
	orgRepository, err := repository.NewTestOrganizationRepository()
	require.NoError(t, err)
	auditRepository, err := repository.NewTestAuditRecordRepository()
	require.NoError(t, err)
	subscription := common.NewFreemiumSubscription("user", "org")
	_, err = subscriptionRepository.Create(subscription)
	require.NoError(t, err)
	webServer := web.NewStubWebServer()
	ctrl := NewSubscriptionController(subscriptionRepository, userRepository, orgRepository, auditRepository, webServer)

	// WHEN querying subscription
	reader := io.NopCloser(strings.NewReader(""))
	req := &http.Request{Body: reader, URL: &url.URL{}}
	ctx := web.NewStubContext(req)
	err = ctrl.querySubscriptions(ctx)

	// THEN it should return valid configs
	require.NoError(t, err)
	recs := ctx.Result.(*PaginatedResult).Records.([]*common.Subscription)
	require.NotEqual(t, 0, len(recs))
}

func Test_ShouldCreateAndGetSubscription(t *testing.T) {
	// GIVEN subscription controller
	subscriptionRepository, err := repository.NewTestSubscriptionRepository()
	require.NoError(t, err)
	userRepository, err := repository.NewTestUserRepository()
	require.NoError(t, err)
	orgRepository, err := repository.NewTestOrganizationRepository()
	require.NoError(t, err)
	auditRepository, err := repository.NewTestAuditRecordRepository()
	webServer := web.NewStubWebServer()
	ctrl := NewSubscriptionController(subscriptionRepository, userRepository, orgRepository, auditRepository, webServer)
	user, err := userRepository.Create(common.NewUser("", "user@formicary.io", "name", "", false))
	if err != nil {
		user, err = userRepository.GetByUsername(common.NewQueryContext("", "", ""), "user@formicary.io")
	}
	require.NoError(t, err)
	subscription := common.NewFreemiumSubscription(user.ID, "")
	b, err := json.Marshal(subscription)
	require.NoError(t, err)
	reader := io.NopCloser(bytes.NewReader(b))
	ctx := web.NewStubContext(&http.Request{Body: reader})

	// WHEN creating subscription
	err = ctrl.postSubscription(ctx)

	// THEN it should return subscription
	require.NoError(t, err)
	updatedSubscription := ctx.Result.(*common.Subscription)
	require.NotEqual(t, "", updatedSubscription.ID)

	// WHEN getting subscription
	ctx.Params["id"] = updatedSubscription.ID
	err = ctrl.getSubscription(ctx)

	// THEN it should not fail
	require.NoError(t, err)
}

func Test_ShouldUpdateAndGetSubscription(t *testing.T) {
	// GIVEN subscription controller
	subscriptionRepository, err := repository.NewTestSubscriptionRepository()
	require.NoError(t, err)
	userRepository, err := repository.NewTestUserRepository()
	require.NoError(t, err)
	orgRepository, err := repository.NewTestOrganizationRepository()
	require.NoError(t, err)
	auditRepository, err := repository.NewTestAuditRecordRepository()
	webServer := web.NewStubWebServer()
	ctrl := NewSubscriptionController(subscriptionRepository, userRepository, orgRepository, auditRepository, webServer)
	user, err := userRepository.Create(common.NewUser("", "user@formicary.io", "name", "", false))
	if err != nil {
		user, err = userRepository.GetByUsername(common.NewQueryContext("", "", ""), "user@formicary.io")
	}
	require.NoError(t, err)

	// WHEN updating subscription
	subscription := common.NewFreemiumSubscription(user.ID, "")
	b, err := json.Marshal(subscription)
	require.NoError(t, err)
	reader := io.NopCloser(bytes.NewReader(b))
	ctx := web.NewStubContext(&http.Request{Body: reader})

	// WHEN creating subscription
	err = ctrl.postSubscription(ctx)
	require.NoError(t, err)
	updatedSubscription := ctx.Result.(*common.Subscription)

	b, err = json.Marshal(subscription)
	reader = io.NopCloser(bytes.NewReader(b))
	ctx = web.NewStubContext(&http.Request{Body: reader})
	ctx.Params["id"] = updatedSubscription.ID
	err = ctrl.putSubscription(ctx)

	// THEN it should return subscription
	require.NoError(t, err)
	updatedSubscription = ctx.Result.(*common.Subscription)
	require.NotEqual(t, "", updatedSubscription.ID)

	// WHEN getting subscription
	ctx.Params["id"] = updatedSubscription.ID
	err = ctrl.getSubscription(ctx)
	// THEN it should not fail
	require.NoError(t, err)
}

func Test_ShouldAddAndDeleteSubscription(t *testing.T) {
	// GIVEN subscription controller
	subscriptionRepository, err := repository.NewTestSubscriptionRepository()
	require.NoError(t, err)
	userRepository, err := repository.NewTestUserRepository()
	require.NoError(t, err)
	orgRepository, err := repository.NewTestOrganizationRepository()
	require.NoError(t, err)
	auditRepository, err := repository.NewTestAuditRecordRepository()
	webServer := web.NewStubWebServer()
	ctrl := NewSubscriptionController(subscriptionRepository, userRepository, orgRepository, auditRepository, webServer)
	user, err := userRepository.Create(common.NewUser("", "user@formicary.io", "name", "", false))
	if err != nil {
		user, err = userRepository.GetByUsername(common.NewQueryContext("", "", ""), "user@formicary.io")
	}
	require.NoError(t, err)

	// WHEN adding subscription
	subscription := common.NewFreemiumSubscription(user.ID, "")
	b, err := json.Marshal(subscription)
	require.NoError(t, err)
	reader := io.NopCloser(bytes.NewReader(b))
	ctx := web.NewStubContext(&http.Request{Body: reader})
	err = ctrl.postSubscription(ctx)

	// THEN it should return subscription
	require.NoError(t, err)
	updatedSubscription := ctx.Result.(*common.Subscription)
	require.NotEqual(t, subscription.ID, updatedSubscription.ID)

	// WHEN deleting subscription
	ctx.Params["id"] = updatedSubscription.ID
	err = ctrl.deleteSubscription(ctx)
	require.NoError(t, err)
}
