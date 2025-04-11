package metricgroup

import (
	"encoding/binary"
	"hash/fnv"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/datapoints"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/elasticsearch"
)

type HashKey struct {
	resourceHash uint32
	scopeHash    uint32
	dpHash       uint32
}

// DataPointHasher is an interface for hashing data points by their identity,
// for grouping into a single document.
type DataPointHasher interface {
	UpdateResource(pcommon.Resource)
	UpdateScope(pcommon.InstrumentationScope)
	UpdateDataPoint(datapoints.DataPoint)
	HashKey() HashKey
}

type hashableDataPoint interface {
	Timestamp() pcommon.Timestamp
	StartTimestamp() pcommon.Timestamp
	Metric() pmetric.Metric
	Attributes() pcommon.Map
}

type (
	// ECSDataPointHasher solely relies on hashCombined because data point attributes overwrite resource attributes on merge.
	ECSDataPointHasher struct {
		resource pcommon.Resource
		dp       hashableDataPoint
	}
	// OTelDataPointHasher does not use hashCombined as resource, scope and data points are independent.
	OTelDataPointHasher struct {
		resourceHash uint32
		scopeHash    uint32
		dpHash       uint32
	}
)

func (h *ECSDataPointHasher) UpdateResource(resource pcommon.Resource) {
	h.resource = resource
}

func (h *ECSDataPointHasher) UpdateScope(_ pcommon.InstrumentationScope) {
}

func (h *ECSDataPointHasher) UpdateDataPoint(dp datapoints.DataPoint) {
	h.dp = dp
}

func (h *ECSDataPointHasher) HashKey() HashKey {
	merged := pcommon.NewMap()
	merged.EnsureCapacity(h.resource.Attributes().Len() + h.dp.Attributes().Len())
	h.resource.Attributes().CopyTo(merged)
	// scope attributes are ignored in ECS mode
	for k, v := range h.dp.Attributes().All() {
		v.CopyTo(merged.PutEmpty(k))
	}

	hasher := fnv.New32a()

	timestampBuf := make([]byte, 8)
	binary.LittleEndian.PutUint64(timestampBuf, uint64(h.dp.Timestamp()))
	hasher.Write(timestampBuf)

	mapHashSortedExcludeReservedAttrs(hasher, merged)

	return HashKey{
		dpHash: hasher.Sum32(),
	}
}

func (h *OTelDataPointHasher) UpdateResource(resource pcommon.Resource) {
	hasher := fnv.New32a()
	// There is special handling to merge geo attributes during serialization,
	// but we can hash them as if they are separate now.
	mapHashSortedExcludeReservedAttrs(hasher, resource.Attributes(), elasticsearch.MappingHintsAttrKey)
	h.resourceHash = hasher.Sum32()
}

func (h *OTelDataPointHasher) UpdateScope(scope pcommon.InstrumentationScope) {
	hasher := fnv.New32a()
	hasher.Write([]byte(scope.Name()))
	// There is special handling to merge geo attributes during serialization,
	// but we can hash them as if they are separate now.
	mapHashSortedExcludeReservedAttrs(hasher, scope.Attributes(), elasticsearch.MappingHintsAttrKey)
	h.scopeHash = hasher.Sum32()
}

func (h *OTelDataPointHasher) UpdateDataPoint(dp datapoints.DataPoint) {
	hasher := fnv.New32a()

	timestampBuf := make([]byte, 8)
	binary.LittleEndian.PutUint64(timestampBuf, uint64(dp.Timestamp()))
	hasher.Write(timestampBuf)

	binary.LittleEndian.PutUint64(timestampBuf, uint64(dp.StartTimestamp()))
	hasher.Write(timestampBuf)

	hasher.Write([]byte(dp.Metric().Unit()))

	// There is special handling to merge geo attributes during serialization,
	// but we can hash them as if they are separate now.
	mapHashSortedExcludeReservedAttrs(hasher, dp.Attributes(), elasticsearch.MappingHintsAttrKey)

	h.dpHash = hasher.Sum32()
}

func (h *OTelDataPointHasher) HashKey() HashKey {
	return HashKey{
		resourceHash: h.resourceHash,
		scopeHash:    h.scopeHash,
		dpHash:       h.dpHash,
	}
}
