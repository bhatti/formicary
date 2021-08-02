package types

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func Test_ShouldVerifyOrganizationTable(t *testing.T) {
	u := NewOrganization("owner", "unit", "bundle")
	require.Equal(t, "formicary_orgs", u.TableName())
}

func Test_ShouldStringifyOrganization(t *testing.T) {
	u := NewOrganization("owner", "unit", "bundle")
	err := u.AfterLoad([]byte("key"))
	require.NoError(t, err)
	require.NotEqual(t, "", u.String())
	require.NoError(t, u.ValidateBeforeSave([]byte("key")))
}

func Test_ShouldVerifyEqualForOrganization(t *testing.T) {
	u1 := NewOrganization("owner", "unit1", "bundle")
	u2 := NewOrganization("owner", "unit1", "bundle")
	u3 := NewOrganization("owner", "unit2", "bundle")
	require.NoError(t, u1.Equals(u2))
	require.Error(t, u1.Equals(u3))
	require.Error(t, u1.Equals(nil))
}

func Test_ShouldAddDeleteConfigForOrganization(t *testing.T) {
	u := NewOrganization("owner", "unit1", "bundle")
	u.AddConfig("name1", "value1", false)
	u.AddConfig("name2", "value2", true)
	require.Equal(t, "name1=value1,name2=value2,", u.ConfigString())
	u.DeleteConfig("name1")
	require.Equal(t, "value2", u.GetConfig("name2").Value)
	require.Nil(t, u.GetConfigByID("name2"))
}
