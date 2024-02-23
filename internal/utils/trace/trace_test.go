package trace

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"testing"
)

func Test_ShouldWriteWithTrace(t *testing.T) {
	count := 0
	jobTrace, err := NewJobTrace(func(b []byte, tags string) {
		count += len(b)
	}, 10000, []string{"secret", "token", "password"})
	require.NoError(t, err)

	n, err := jobTrace.Writeln("1: test line", "")
	require.NoError(t, err)
	require.Equal(t, 14, n)

	n, err = jobTrace.Write([]byte("2: test line secret"), "")
	require.NoError(t, err)
	require.Equal(t, 21, n)

	n, err = jobTrace.Write([]byte("3: test line secret\napi token\n"), "")
	require.NoError(t, err)
	require.Equal(t, 32, n)

	out, err := jobTrace.Finish()
	require.NoError(t, err)
	require.Contains(t, fmt.Sprintf("%s", out), "[****]")
	require.Equal(t, 68, len(out))
	require.Equal(t, 68, count)
	jobTrace.Close()
}
