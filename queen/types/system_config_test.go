package types

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func Test_ShouldVerifySystemConfigTableNames(t *testing.T) {
	sysError := NewSystemConfig("scope", "kind", "name", "value")
	require.Equal(t, "formicary_system_config", sysError.TableName())
}

func Test_ShouldVerifySystemConfigString(t *testing.T) {
	sysError := NewSystemConfig("scope", "kind", "name", "value")
	require.Contains(t, sysError.String(), "name=value")
}

func Test_ShouldVerifySystemConfigShortID(t *testing.T) {
	sysError := NewSystemConfig("scope", "kind", "name", "value")
	sysError.ID = "1234567890"
	require.Contains(t, sysError.ShortID(), "123")
	sysError.ID = "123"
	require.Equal(t, sysError.ShortID(), "123")
}

// Validate error-execution with proper initialization
func Test_ShouldSystemConfigHappyPath(t *testing.T) {
	// GIVEN system config
	sysError := NewSystemConfig("scope", "kind", "name", "value")
	// WHEN validating a valid config
	err := sysError.ValidateBeforeSave()
	// THEN it should not fail
	require.NoError(t, err)
}

// Test validate without scope
func Test_ShouldSystemConfigWithoutScope(t *testing.T) {
	// GIVEN system config
	sysError := NewSystemConfig("", "kind", "name", "value")
	// WHEN validating a config without scope
	err := sysError.ValidateBeforeSave()
	// THEN it should fail
	require.Error(t, err)
	require.Contains(t, err.Error(), "scope is not specified")
}

// Test validate without kind
func Test_ShouldSystemConfigWithoutKind(t *testing.T) {
	// GIVEN system config
	sysError := NewSystemConfig("scope", "", "name", "value")
	// WHEN validating a config without kind
	err := sysError.ValidateBeforeSave()
	// THEN it should fail
	require.Error(t, err)
	require.Contains(t, err.Error(), "kind is not specified")
}

// Test validate without name
func Test_ShouldSystemConfigWithoutName(t *testing.T) {
	// GIVEN system config
	sysError := NewSystemConfig("scope", "kind", "", "value")
	// WHEN validating a config without name
	err := sysError.ValidateBeforeSave()
	// THEN it should fail
	require.Error(t, err)
	require.Contains(t, err.Error(), "name is not specified")
}

// Test validate without value
func Test_ShouldSystemConfigWithoutValue(t *testing.T) {
	// GIVEN system config
	sysError := NewSystemConfig("scope", "kind", "name", "")
	// WHEN validating a config without value
	err := sysError.ValidateBeforeSave()
	// THEN it should fail
	require.Error(t, err)
	require.Contains(t, err.Error(), "value is not specified")
}

func Test_ShouldEqualSystemConfig(t *testing.T) {
	sysConfig1 := NewSystemConfig("scope", "kind", "name", "v")
	sysConfig2 := NewSystemConfig("scope", "kind", "name", "v")
	sysConfig3 := NewSystemConfig("nscope", "kind", "name", "v")
	sysConfig4 := NewSystemConfig("scope", "nkind", "name", "v")
	sysConfig5 := NewSystemConfig("scope", "kind", "nname", "v")
	sysConfig6 := NewSystemConfig("scope", "kind", "name", "nv")
	require.Error(t, sysConfig1.Equals(nil))
	require.Error(t, sysConfig1.Equals(sysConfig3))
	require.Error(t, sysConfig1.Equals(sysConfig4))
	require.Error(t, sysConfig1.Equals(sysConfig5))
	require.Error(t, sysConfig1.Equals(sysConfig6))
	require.NoError(t, sysConfig1.Equals(sysConfig1))
	require.NoError(t, sysConfig1.Equals(sysConfig2))
}

