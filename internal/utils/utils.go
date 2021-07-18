package utils

import (
	"fmt"
	api "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"strings"
)

// NormalizePrefix adds trailing slash if needed
func NormalizePrefix(prefix string) string {
	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		return prefix + "/"
	}
	return prefix
}

// CacheArtifactID builds unique id for task cache
func CacheArtifactID(
	prefix string,
	username string,
	jobType string,
	key string) string {
	return NormalizePrefix(prefix) +
		NormalizePrefix(username) +
		NormalizePrefix(jobType) +
		NormalizePrefix(key) +
		"cache.zip"
}

// CreateResourceCost determines cost based on default value
func CreateResourceCost(
	res api.ResourceList,
	def api.ResourceList) float64 {
	resourceQuantityMilliValue := func(q resource.Quantity) int64 {
		return q.MilliValue()
	}
	defValue := resourceQuantityMilliValue(def["cpu"])*1000000000 +
		resourceQuantityMilliValue(def["memory"]) + resourceQuantityMilliValue(def["ephemeral-storage"])
	value := resourceQuantityMilliValue(res["cpu"])*1000000000 +
		resourceQuantityMilliValue(res["memory"]) + resourceQuantityMilliValue(res["ephemeral-storage"])
	return float64(value) / float64(defValue)
}

// CreateResourceList translates cpu and memory strings such as "500m", "50Mi" to resource list
func CreateResourceList(
	cpu string,
	memory string,
	ephemeralStorage string) (api.ResourceList, error) {
	var rCPU, rMem, rStore resource.Quantity
	var err error

	parse := func(s string) (resource.Quantity, error) {
		var q resource.Quantity
		if s == "" {
			return q, nil
		}
		if q, err = resource.ParseQuantity(s); err != nil {
			return q, err
		}
		return q, nil
	}

	if rCPU, err = parse(cpu); err != nil {
		return api.ResourceList{}, fmt.Errorf("failed to parse cpu %s due to %s", cpu, err)
	}

	if rMem, err = parse(memory); err != nil {
		return api.ResourceList{}, fmt.Errorf("failed to parse memory %s due to %s", memory, err)
	}

	if rStore, err = parse(ephemeralStorage); err != nil {
		return api.ResourceList{}, fmt.Errorf("failed to parse ephemeral-storage %s due to %s", ephemeralStorage, err)
	}

	l := make(api.ResourceList)

	q := resource.Quantity{}
	if rCPU != q {
		l[api.ResourceCPU] = rCPU
	}
	if rMem != q {
		l[api.ResourceMemory] = rMem
	}
	if rStore != q {
		l[api.ResourceEphemeralStorage] = rStore
	}

	return l, nil
}
