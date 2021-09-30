package utils

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	api "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"runtime"
	"strings"
	"time"
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

// MemUsageMiBString memory-info
func MemUsageMiBString() map[string]string {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	result := make(map[string]string)
	result["allocated"] = fmt.Sprintf("%v MiB", bToMb(m.Alloc))
	result["totalAllocated"] = fmt.Sprintf("%v MiB", bToMb(m.TotalAlloc))
	result["system"] = fmt.Sprintf("%v MiB", bToMb(m.Sys))
	result["numGC"] = fmt.Sprintf("%v", m.NumGC)
	return result
}

// MemUsageMiB memory-info
func MemUsageMiB() map[string]uint64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	result := make(map[string]uint64)
	result["allocated"] = bToMb(m.Alloc)
	result["totalAllocated"] = bToMb(m.TotalAlloc)
	result["system"] = bToMb(m.Sys)
	result["numGC"] = uint64(m.NumGC)
	return result
}

func bToMb(b uint64) uint64 {
	return b / 1024 / 1024
}

// VerifySignature verifies sha256 hash
func VerifySignature(secret string, expectedHash256 string, body []byte) error {
	hash := hmac.New(sha256.New, []byte(secret))
	if _, err := hash.Write(body); err != nil {
		return err
	}
	actualHash := hex.EncodeToString(hash.Sum(nil))
	if actualHash != expectedHash256 {
		return fmt.Errorf("failed to match '%s' sha256 with '%s'", actualHash, expectedHash256)
	}
	return nil
}

// ParseStartDateTime parses start date
func ParseStartDateTime(s string) time.Time {
	start := time.Unix(0, 0)
	if d, err := time.Parse("2006-01-02T15:04:05-0700", s); err == nil {
		return d
	} else if d, err := time.Parse("2006-01-02", s); err == nil {
		return time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, d.Location())
	}
	return start
}

// ParseEndDateTime parses end date
func ParseEndDateTime(s string) time.Time {
	end := time.Now()
	if d, err := time.Parse("2006-01-02T15:04:05-0700", s); err == nil {
		return d
	} else if d, err := time.Parse("2006-01-02", s); err == nil {
		return time.Date(d.Year(), d.Month(), d.Day(), 23, 59, 59, 0, d.Location())
	}
	return end
}
