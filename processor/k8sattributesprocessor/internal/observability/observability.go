// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package observability // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sattributesprocessor/internal/observability"

import (
	"context"

	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
)

// TODO: re-think if processor should register it's own telemetry views or if some other
// mechanism should be used by the collector to discover views from all components

func init() {
	_ = view.Register(
		viewPodsUpdated,
		viewPodsAdded,
		viewPodsDeleted,
		viewIPLookupMiss,
		viewPodTableSize,
		viewNamespacesAdded,
		viewNamespacesUpdated,
		viewNamespacesDeleted,
	)
}

var (
	mPodsUpdated        = stats.Int64("otelsvc/k8s/pod_updated", "Number of pod update events received", "1")
	mPodsAdded          = stats.Int64("otelsvc/k8s/pod_added", "Number of pod add events received", "1")
	mPodsDeleted        = stats.Int64("otelsvc/k8s/pod_deleted", "Number of pod delete events received", "1")
	mPodTableSize       = stats.Int64("otelsvc/k8s/pod_table_size", "Size of table containing pod info", "1")
	mIPLookupMiss       = stats.Int64("otelsvc/k8s/ip_lookup_miss", "Number of times pod by IP lookup failed.", "1")
	mNamespacesUpdated  = stats.Int64("otelsvc/k8s/namespace_updated", "Number of namespace update events received", "1")
	mNamespacesAdded    = stats.Int64("otelsvc/k8s/namespace_added", "Number of namespace add events received", "1")
	mNamespacesDeleted  = stats.Int64("otelsvc/k8s/namespace_deleted", "Number of namespace delete events received", "1")
	mReplicaSetsUpdated = stats.Int64("otelsvc/k8s/replicaset_updated", "Number of ReplicaSet update events received", "1")
	mReplicaSetsAdded   = stats.Int64("otelsvc/k8s/replicaset_added", "Number of ReplicaSet add events received", "1")
	mReplicaSetsDeleted = stats.Int64("otelsvc/k8s/replicaset_deleted", "Number of ReplicaSet delete events received", "1")
)

var viewPodsUpdated = &view.View{
	Name:        mPodsUpdated.Name(),
	Description: mPodsUpdated.Description(),
	Measure:     mPodsUpdated,
	Aggregation: view.Sum(),
}

var viewPodsAdded = &view.View{
	Name:        mPodsAdded.Name(),
	Description: mPodsAdded.Description(),
	Measure:     mPodsAdded,
	Aggregation: view.Sum(),
}

var viewPodsDeleted = &view.View{
	Name:        mPodsDeleted.Name(),
	Description: mPodsDeleted.Description(),
	Measure:     mPodsDeleted,
	Aggregation: view.Sum(),
}

var viewIPLookupMiss = &view.View{
	Name:        mIPLookupMiss.Name(),
	Description: mIPLookupMiss.Description(),
	Measure:     mIPLookupMiss,
	Aggregation: view.Sum(),
}

var viewPodTableSize = &view.View{
	Name:        mPodTableSize.Name(),
	Description: mPodTableSize.Description(),
	Measure:     mPodTableSize,
	Aggregation: view.LastValue(),
}

var viewNamespacesUpdated = &view.View{
	Name:        mNamespacesUpdated.Name(),
	Description: mNamespacesUpdated.Description(),
	Measure:     mNamespacesUpdated,
	Aggregation: view.Sum(),
}

var viewNamespacesAdded = &view.View{
	Name:        mNamespacesAdded.Name(),
	Description: mNamespacesAdded.Description(),
	Measure:     mNamespacesAdded,
	Aggregation: view.Sum(),
}

var viewNamespacesDeleted = &view.View{
	Name:        mNamespacesDeleted.Name(),
	Description: mNamespacesDeleted.Description(),
	Measure:     mNamespacesDeleted,
	Aggregation: view.Sum(),
}

// RecordPodUpdated increments the metric that records pod update events received.
func RecordPodUpdated() {
	stats.Record(context.Background(), mPodsUpdated.M(int64(1)))
}

// RecordPodAdded increments the metric that records pod add events receiver.
func RecordPodAdded() {
	stats.Record(context.Background(), mPodsAdded.M(int64(1)))
}

// RecordPodDeleted increments the metric that records pod events deleted.
func RecordPodDeleted() {
	stats.Record(context.Background(), mPodsDeleted.M(int64(1)))
}

// RecordIPLookupMiss increments the metric that records Pod lookup by IP misses.
func RecordIPLookupMiss() {
	stats.Record(context.Background(), mIPLookupMiss.M(int64(1)))
}

// RecordPodTableSize store size of pod table field in WatchClient
func RecordPodTableSize(podTableSize int64) {
	stats.Record(context.Background(), mPodTableSize.M(podTableSize))
}

// RecordNamespaceUpdated increments the metric that records namespace update events received.
func RecordNamespaceUpdated() {
	stats.Record(context.Background(), mNamespacesUpdated.M(int64(1)))
}

// RecordNamespaceAdded increments the metric that records namespace add events receiver.
func RecordNamespaceAdded() {
	stats.Record(context.Background(), mNamespacesAdded.M(int64(1)))
}

// RecordNamespaceDeleted increments the metric that records namespace events deleted.
func RecordNamespaceDeleted() {
	stats.Record(context.Background(), mNamespacesDeleted.M(int64(1)))
}

// RecordReplicaSetUpdated increments the metric that records ReplicaSet update events received.
func RecordReplicaSetUpdated() {
	stats.Record(context.Background(), mReplicaSetsUpdated.M(int64(1)))
}

// RecordReplicaSetAdded increments the metric that records ReplicaSet add events receiver.
func RecordReplicaSetAdded() {
	stats.Record(context.Background(), mReplicaSetsAdded.M(int64(1)))
}

// RecordReplicaSetDeleted increments the metric that records ReplicaSet events deleted.
func RecordReplicaSetDeleted() {
	stats.Record(context.Background(), mReplicaSetsDeleted.M(int64(1)))
}
