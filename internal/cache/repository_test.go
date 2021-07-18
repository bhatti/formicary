package cache

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func newStub() (Repository, error) {
	//return New(&types.RedisConfig{Host: "localhost", Port: 6379, PoolSize: 5, TTLMinutes: 5})
	return NewStub()
}


func Test_ShouldReadPropertiesAfterWrite(t *testing.T) {
	cache, err := newStub()
	require.NoError(t, err)
	err = cache.Save("students", "name", []byte("jake"))
	require.NoError(t, err)
	err = cache.Save("students", "grade", []byte("A"))
	require.NoError(t, err)
	m, err := cache.GetAll("students")
	require.NoError(t, err)
	require.Equal(t, 2, len(m))
	m, err = cache.Get("students", "name", "grade")
	require.NoError(t, err)
	require.Equal(t, "jake", string(m["name"]))
	require.Equal(t, "A", string(m["grade"]))
}

func Test_ShouldNotFindPropertiesAfterDelete(t *testing.T) {
	cache, err := newStub()
	require.NoError(t, err)
	err = cache.Save("students", "name", []byte("jake"))
	require.NoError(t, err)
	err = cache.Save("students", "grade", []byte("A"))
	require.NoError(t, err)
	m, err := cache.GetAll("students")
	require.NoError(t, err)
	require.Equal(t, 2, len(m))
	err = cache.Delete("students", "grade")
	require.NoError(t, err)
	m, err = cache.GetAll("students")
	require.NoError(t, err)
	require.Equal(t, 1, len(m))
}
