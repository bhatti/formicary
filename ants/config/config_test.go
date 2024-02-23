package config

import (
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	"plexobject.com/formicary/internal/types"
	"testing"
)

func Test_ShouldNotCreateAntConfigWithoutConfigFile(t *testing.T) {
	_, err := NewAntConfig("id")
	require.Error(t, err)
}

func Test_ShouldCreateValidAntConfigFromPath(t *testing.T) {
	// GIVEN viper is setup with path
	viper.AddConfigPath("../..")
	viper.SetConfigName(".formicary-ant")
	viper.SetConfigType("yaml")

	// WHEN ant config is created
	cfg, err := NewAntConfig("id")

	// THEN it should create valid config
	require.NoError(t, err)
	require.Equal(t, "id", cfg.Common.ID)
}

func Test_ShouldCreateValidateAntConfig(t *testing.T) {
	// GIVEN a valid config
	cfg := newTestConfig()
	cfg.Methods = []types.TaskMethod{types.HTTPPostJSON}
	// WHEN Validate method is called
	// THEN it should not fail
	err := cfg.Validate()
	require.NoError(t, err)
}

func Test_IDForNewAntRegistration(t *testing.T) {
	// GIVEN a valid config
	cfg := newTestConfig()
	// WHEN new ant registration is created
	// THEN its ant-id should match config-id
	require.Equal(t, cfg.Common.ID, cfg.NewAntRegistration().AntID)
}

func Test_HaveNonZeroShutdownTimeout(t *testing.T) {
	// GIVEN a valid config
	cfg := newTestConfig()
	// WHEN shutdown-timeout is called
	// THEN it should return non-zero value
	require.NotEqual(t, 0, cfg.GetShutdownTimeout())
}

func Test_HaveNonZeroAwaitRunningPeriod(t *testing.T) {
	// GIVEN a valid config
	cfg := newTestConfig()
	// WHEN await-running-period is called
	// THEN it should return non-zero value
	require.NotEqual(t, 0, cfg.GetAwaitRunningPeriod())
}

func Test_HaveNonZeroPollAttempts(t *testing.T) {
	// GIVEN a valid config
	cfg := newTestConfig()
	// WHEN poll-attempts is called
	// THEN it should return non-zero value
	require.NotEqual(t, 0, cfg.GetPollAttempts())
}

func Test_HaveNonZeroPollTimeout(t *testing.T) {
	// GIVEN a valid config
	cfg := newTestConfig()
	// WHEN poll-timeout is called
	// THEN it should return non-zero value
	require.NotEqual(t, 0, cfg.GetPollTimeout())
}

func Test_HaveNonZeroPollInterval(t *testing.T) {
	// GIVEN a valid config
	cfg := newTestConfig()
	// WHEN poll-interval is called
	// THEN it should return non-zero value
	require.NotEqual(t, 0, cfg.GetPollInterval())
}

func Test_ShouldFailValidateAntConfigWithoutMethods(t *testing.T) {
	cfg := newTestConfig()
	err := cfg.Validate()
	require.Error(t, err)
}

func newTestConfig() AntConfig {
	cfg := AntConfig{}
	cfg.Common.ID = "test-id"
	cfg.Common.S3.AccessKeyID = "admin"
	cfg.Common.S3.SecretAccessKey = "password"
	cfg.Common.S3.Bucket = "test-bucket"
	cfg.Common.Pulsar.URL = "test"
	cfg.Common.Redis.Host = "test"
	return cfg
}
