package config

import (
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	"os"
	"testing"
)

func Test_ShouldLoadConfig(t *testing.T) {
	viper.AddConfigPath("../..")
	viper.SetConfigName(".formicary-queen")
	viper.SetConfigType("yaml")
	os.Setenv("COMMON_AUTH_GOOGLE_CLIENT_ID", "my-client")
	os.Setenv("COMMON_AUTH_GOOGLE_CLIENT_SECRET", "my-secret")
	cfg, err := NewServerConfig("id")
	require.NoError(t, err)
	require.Equal(t, "id", cfg.ID)
	require.Equal(t, "my-client", cfg.Auth.GoogleClientID)
	require.Equal(t, "my-secret", cfg.Auth.GoogleClientSecret)
}
