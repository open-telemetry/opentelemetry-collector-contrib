// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package extractors

import (
	"bytes"
	"encoding/json"
	"testing"

	cinfo "github.com/google/cadvisor/info/v1"
	"github.com/stretchr/testify/assert"

	. "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/containerinsight"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/cadvisor/testutils"
)

func TestFSStats(t *testing.T) {
	result := testutils.LoadContainerInfo(t, "./testdata/CurInfoContainer.json")
	// container type
	containerType := TypeContainer
	extractor := NewFileSystemMetricExtractor(nil)

	var cMetrics []*CAdvisorMetric
	if extractor.HasValue(result[0]) {
		cMetrics = extractor.GetValue(result[0], nil, containerType)
	}

	expectedFields := map[string]interface{}{
		"container_filesystem_usage":       uint64(25661440),
		"container_filesystem_capacity":    uint64(21462233088),
		"container_filesystem_available":   uint64(0),
		"container_filesystem_utilization": float64(0.11956556381986117),
	}
	expectedTags := map[string]string{
		"device": "/dev/xvda1",
		"fstype": "vfs",
		"Type":   "ContainerFS",
	}
	AssertContainsTaggedField(t, cMetrics[0], expectedFields, expectedTags)

	// pod type
	containerType = TypePod
	extractor = NewFileSystemMetricExtractor(nil)

	if extractor.HasValue(result[0]) {
		cMetrics = extractor.GetValue(result[0], nil, containerType)
	}

	assert.Equal(t, len(cMetrics), 0)

	// node type for eks

	result2 := testutils.LoadContainerInfo(t, "./testdata/CurInfoNode.json")
	containerType = TypeNode
	extractor = NewFileSystemMetricExtractor(nil)

	if extractor.HasValue(result2[0]) {
		cMetrics = extractor.GetValue(result2[0], nil, containerType)
	}

	expectedFields = map[string]interface{}{
		"node_filesystem_available":   uint64(67108864),
		"node_filesystem_capacity":    uint64(67108864),
		"node_filesystem_inodes":      uint64(2052980),
		"node_filesystem_inodes_free": uint64(2052979),
		"node_filesystem_usage":       uint64(0),
		"node_filesystem_utilization": float64(0),
	}
	expectedTags = map[string]string{
		"device": "/dev/shm",
		"fstype": "vfs",
		"Type":   "NodeFS",
	}
	AssertContainsTaggedField(t, cMetrics[0], expectedFields, expectedTags)

	expectedFields = map[string]interface{}{
		"node_filesystem_available":   uint64(6925574144),
		"node_filesystem_capacity":    uint64(21462233088),
		"node_filesystem_inodes":      uint64(10484672),
		"node_filesystem_inodes_free": uint64(10387672),
		"node_filesystem_usage":       uint64(14536658944),
		"node_filesystem_utilization": float64(67.73134409824186),
	}
	expectedTags = map[string]string{
		"device": "/dev/xvda1",
		"fstype": "vfs",
		"Type":   "NodeFS",
	}
	AssertContainsTaggedField(t, cMetrics[1], expectedFields, expectedTags)

	expectedFields = map[string]interface{}{
		"node_filesystem_available":   uint64(10682417152),
		"node_filesystem_capacity":    uint64(10726932480),
		"node_filesystem_inodes":      uint64(5242880),
		"node_filesystem_inodes_free": uint64(5242877),
		"node_filesystem_usage":       uint64(44515328),
		"node_filesystem_utilization": float64(0.4149865591397849),
	}
	expectedTags = map[string]string{
		"device": "/dev/xvdce",
		"fstype": "vfs",
		"Type":   "NodeFS",
	}
	AssertContainsTaggedField(t, cMetrics[2], expectedFields, expectedTags)
}

func TestAllowList(t *testing.T) {
	extractor := NewFileSystemMetricExtractor(nil)
	assert.Equal(t, true, extractor.allowListRegexP.MatchString("/dev/shm"))
	assert.Equal(t, true, extractor.allowListRegexP.MatchString("tmpfs"))
	assert.Equal(t, true, extractor.allowListRegexP.MatchString("overlay"))
	assert.Equal(t, false, extractor.allowListRegexP.MatchString("overlaytest"))
	assert.Equal(t, false, extractor.allowListRegexP.MatchString("/dev"))
}

func TestFSStatsWithAllowList(t *testing.T) {
	var result []*cinfo.ContainerInfo
	containerInfos := testutils.LoadContainerInfo(t, "./testdata/FileSystemStat.json")
	result = append(result, containerInfos...)

	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	assert.NoError(t, enc.Encode(result))
	containerType := TypeContainer
	extractor := NewFileSystemMetricExtractor(nil)

	var cMetrics []*CAdvisorMetric
	if extractor.HasValue(result[0]) {
		cMetrics = extractor.GetValue(result[0], nil, containerType)
	}

	// There are 3 valid device names which pass the allowlist in testAllowList json.
	assert.Equal(t, 3, len(cMetrics))
	assert.Equal(t, "tmpfs", cMetrics[0].tags["device"])
	assert.Equal(t, "/dev/xvda1", cMetrics[1].tags["device"])
	assert.Equal(t, "overlay", cMetrics[2].tags["device"])

}
