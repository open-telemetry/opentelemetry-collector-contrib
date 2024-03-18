package resourceutil // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/resourceutil"

import (
	"crypto/sha256"
	"encoding/hex"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"sort"
)

type keyValueLabelPair struct {
	Key   string
	Value string
}

func CalculateResourceAttributesHash(resourceMetrics pcommon.Resource) string {
	var pairs []keyValueLabelPair
	resourceMetrics.Attributes().Range(func(k string, v pcommon.Value) bool {
		pairs = append(pairs, keyValueLabelPair{Key: k, Value: v.AsString()})
		return true
	})

	sort.SliceStable(pairs, func(i, j int) bool {
		return pairs[i].Key < pairs[j].Key
	})

	h := sha256.New()
	for _, pair := range pairs {
		h.Write([]byte(pair.Key))
		h.Write([]byte(pair.Value))
	}
	return hex.EncodeToString(h.Sum(nil))
}
