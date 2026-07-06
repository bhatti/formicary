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

// NewPersonalOrg must produce a valid org: unique non-empty BundleID, OrgUnit = username.
func Test_ShouldCreatePersonalOrgWithValidBundleID(t *testing.T) {
	org := NewPersonalOrg("owner-1", "user@example.com")
	require.True(t, org.IsPersonal)
	require.Equal(t, "user@example.com", org.OrgUnit)
	require.NotEmpty(t, org.BundleID, "BundleID must not be empty")
	require.Contains(t, org.BundleID, ".formicary.io", "BundleID must have formicary.io suffix")
	require.NoError(t, org.Validate(), "personal org must pass validation")
}

// Personal org with empty OrgUnit and BundleID must auto-generate defaults on Validate.
func Test_ShouldAutoGenerateFieldsForPersonalOrgOnValidate(t *testing.T) {
	org := &Organization{IsPersonal: true}
	require.NoError(t, org.Validate())
	require.NotEmpty(t, org.OrgUnit, "OrgUnit must be auto-generated for personal org")
	require.NotEmpty(t, org.BundleID, "BundleID must be auto-generated for personal org")
	require.Contains(t, org.BundleID, ".formicary.io")
}

// Non-personal org must still require OrgUnit and BundleID.
func Test_ShouldRequireOrgUnitAndBundleIDForNonPersonalOrg(t *testing.T) {
	org := &Organization{IsPersonal: false}
	err := org.Validate()
	require.Error(t, err)
}

// Two personal orgs for different users must get different BundleIDs.
func Test_ShouldCreateUniquePersonalOrgBundleIDs(t *testing.T) {
	org1 := NewPersonalOrg("owner-1", "alice@example.com")
	org2 := NewPersonalOrg("owner-2", "bob@example.com")
	require.NotEqual(t, org1.BundleID, org2.BundleID, "personal orgs must have unique BundleIDs")
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
