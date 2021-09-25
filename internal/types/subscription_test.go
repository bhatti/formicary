package types

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func Test_ShouldCreateSubscription(t *testing.T) {
	subs := NewSubscription(IndividualSubscription, Weekly)
	require.NotNil(t, subs)
	require.Equal(t, "formicary_subscriptions", subs.TableName())
	require.NotEqual(t, "", subs.String())
}

func Test_ShouldCreateFreemiumSubscription(t *testing.T) {
	subs := NewFreemiumSubscription(&User{ID: "123", OrganizationID: "456"})
	require.NotNil(t, subs)
	require.NoError(t, subs.Validate())
	require.NotEqual(t, "", subs.StartedString())
	require.NotEqual(t, "", subs.EndedString())
	require.False(t, subs.Expired())
}

