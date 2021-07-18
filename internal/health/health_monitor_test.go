package health

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func Test_ShouldParseDockerURL(t *testing.T) {
	host, port, err := parseURL("localhost:2375")
	require.NoError(t, err)
	require.Equal(t, "localhost", host)
	require.Equal(t, 2375, port)
}

func Test_ShouldParsePulsarURL(t *testing.T) {
	host, port, err := parseURL("pulsar://localhost:6650")
	require.NoError(t, err)
	require.Equal(t, "localhost", host)
	require.Equal(t, 6650, port)
}

func Test_ShouldParseS3URL(t *testing.T) {
	host, port, err := parseURL("8105ca6a73c5.ngrok.io:8080")
	require.NoError(t, err)
	require.Equal(t, "8105ca6a73c5.ngrok.io", host)
	require.Equal(t, 8080, port)
}
