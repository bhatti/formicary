package utils

import (
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/resource"
	"testing"
)

func Test_ShouldCreateEmptyResourceList(t *testing.T) {
	res, err := CreateResourceList("", "", "")
	require.NoError(t, err)
	require.Equal(t, 0, len(res))
}

func Test_ShouldCreateKResourceList(t *testing.T) {
	resourceQuantityMilliValue := func(q resource.Quantity) int64 {
		return q.MilliValue()
	}
	res, err := CreateResourceList("0.5", "1.2Gi", "1.3k")
	require.NoError(t, err)
	require.Equal(t, 3, len(res))
	require.Equal(t, int64(500), resourceQuantityMilliValue(res["cpu"]))
	require.Equal(t, int64(1288490188800), resourceQuantityMilliValue(res["memory"]))
	require.Equal(t, int64(1300000), resourceQuantityMilliValue(res["ephemeral-storage"]))
}

func Test_ShouldCreateKiResourceList(t *testing.T) {
	resourceQuantityMilliValue := func(q resource.Quantity) int64 {
		return q.MilliValue()
	}
	res, err := CreateResourceList("1.0", "1G", "1.3Ki")
	require.NoError(t, err)
	require.Equal(t, 3, len(res))
	require.Equal(t, int64(1000), resourceQuantityMilliValue(res["cpu"]))
	require.Equal(t, int64(1000000000000), resourceQuantityMilliValue(res["memory"]))
	require.Equal(t, int64(1331200), resourceQuantityMilliValue(res["ephemeral-storage"]))
}

func Test_ShouldCreateDefaultResourceList(t *testing.T) {
	resourceQuantityMilliValue := func(q resource.Quantity) int64 {
		return q.MilliValue()
	}
	res, err := CreateResourceList("0.5", "500M", "500M")
	require.NoError(t, err)
	require.Equal(t, 3, len(res))
	require.Equal(t, int64(500), resourceQuantityMilliValue(res["cpu"]))
	require.Equal(t, int64(500000000000), resourceQuantityMilliValue(res["memory"]))
	require.Equal(t, int64(500000000000), resourceQuantityMilliValue(res["ephemeral-storage"]))
}

func Test_ShouldCalculateCostForResourceList(t *testing.T) {
	def, err := CreateResourceList("0.5", "500M", "500M")
	require.NoError(t, err)
	res, err := CreateResourceList("0.5", "500M", "500M")
	require.NoError(t, err)
	require.Equal(t, float64(1), CreateResourceCost(res, def))
	res, err = CreateResourceList("1.0", "1G", "1G")
	require.NoError(t, err)
	require.Equal(t, float64(2), CreateResourceCost(res, def))
	res, err = CreateResourceList("1.0", "2G", "2G")
	require.NoError(t, err)
	require.Equal(t, 3.3333333333333335, CreateResourceCost(res, def))
}
