// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package observability // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sattributesprocessor/internal/observability"

import (
	"context"

	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opentelemetry.io/collector/featuregate"
)

var renameInternalMetrics = featuregate.GlobalRegistry().MustRegister(
	"processor.k8sattributes.RenameInternalMetrics",
	featuregate.StageAlpha,
	featuregate.WithRegisterDescription("When enabled, the k8sattributes processor will"+
		" be renamed to new metric names"),
)

var (
	mPodsUpdated        *stats.Int64Measure
	mPodsAdded          *stats.Int64Measure
	mPodsDeleted        *stats.Int64Measure
	mPodTableSize       *stats.Int64Measure
	mIPLookupMiss       *stats.Int64Measure
	mNamespacesUpdated  *stats.Int64Measure
	mNamespacesAdded    *stats.Int64Measure
	mNamespacesDeleted  *stats.Int64Measure
	mReplicaSetsUpdated *stats.Int64Measure
	mReplicaSetsAdded   *stats.Int64Measure
	mReplicaSetsDeleted *stats.Int64Measure
)

// TODO: re-think if processor should register it's own telemetry views or if some other
// mechanism should be used by the collector to discover views from all components

func Init() {

	mPodsUpdatedName := "otelsvc/k8s/pod_updated"
	mPodsAddedName := "otelsvc/k8s/pod_added"
	mPodsDeletedName := "otelsvc/k8s/pod_deleted"
	mPodTableSizeName := "otelsvc/k8s/pod_table_size"
	mIPLookupMissName := "otelsvc/k8s/ip_lookup_miss"
	mNamespacesUpdatedName := "otelsvc/k8s/namespace_updated"
	mNamespacesAddedName := "otelsvc/k8s/namespace_added"
	mNamespacesDeletedName := "otelsvc/k8s/namespace_deleted"
	mReplicaSetsUpdatedName := "otelsvc/k8s/replicaset_updated"
	mReplicaSetsAddedName := "otelsvc/k8s/replicaset_added"
	mReplicaSetsDeletedName := "otelsvc/k8s/replicaset_deleted"

	if renameInternalMetrics.IsEnabled() {
		mPodsUpdatedName = "processor.k8sattributes.pods.updated"
		mPodsAddedName = "processor.k8sattributes.pods.added"
		mPodsDeletedName = "processor.k8sattributes.pods.deleted"
		mPodTableSizeName = "processor.k8sattributes.pods.table_size"
		mIPLookupMissName = "processor.k8sattributes.ip_lookup_misses"
		mNamespacesUpdatedName = "processor.k8sattributes.namespaces.updated"
		mNamespacesAddedName = "processor.k8sattributes.namespaces.added"
		mNamespacesDeletedName = "processor.k8sattributes.namespaces.deleted"
		mReplicaSetsUpdatedName = "processor.k8sattributes.replicasets.updated"
		mReplicaSetsAddedName = "processor.k8sattributes.replicasets.added"
		mReplicaSetsDeletedName = "processor.k8sattributes.replicasets.deleted"
	}

	mPodsUpdated = stats.Int64(mPodsUpdatedName, "Number of pod update events received", "1")
	mPodsAdded = stats.Int64(mPodsAddedName, "Number of pod add events received", "1")
	mPodsDeleted = stats.Int64(mPodsDeletedName, "Number of pod delete events received", "1")
	mPodTableSize = stats.Int64(mPodTableSizeName, "Size of table containing pod info", "1")
	mIPLookupMiss = stats.Int64(mIPLookupMissName, "Number of times pod by IP lookup failed.", "1")
	mNamespacesUpdated = stats.Int64(mNamespacesUpdatedName, "Number of namespace update events received", "1")
	mNamespacesAdded = stats.Int64(mNamespacesAddedName, "Number of namespace add events received", "1")
	mNamespacesDeleted = stats.Int64(mNamespacesDeletedName, "Number of namespace delete events received", "1")
	mReplicaSetsUpdated = stats.Int64(mReplicaSetsUpdatedName, "Number of ReplicaSet update events received", "1")
	mReplicaSetsAdded = stats.Int64(mReplicaSetsAddedName, "Number of ReplicaSet add events received", "1")
	mReplicaSetsDeleted = stats.Int64(mReplicaSetsDeletedName, "Number of ReplicaSet delete events received", "1")

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

	var viewReplicasetsUpdated = &view.View{
		Name:        mReplicaSetsUpdated.Name(),
		Description: mReplicaSetsUpdated.Description(),
		Measure:     mReplicaSetsUpdated,
		Aggregation: view.Sum(),
	}

	var viewReplicasetsAdded = &view.View{
		Name:        mReplicaSetsAdded.Name(),
		Description: mReplicaSetsAdded.Description(),
		Measure:     mReplicaSetsAdded,
		Aggregation: view.Sum(),
	}

	var viewReplicasetsDeleted = &view.View{
		Name:        mReplicaSetsDeleted.Name(),
		Description: mReplicaSetsDeleted.Description(),
		Measure:     mReplicaSetsDeleted,
		Aggregation: view.Sum(),
	}

	_ = view.Register(
		viewPodsUpdated,
		viewPodsAdded,
		viewPodsDeleted,
		viewIPLookupMiss,
		viewPodTableSize,
		viewNamespacesAdded,
		viewNamespacesUpdated,
		viewNamespacesDeleted,
		viewReplicasetsUpdated,
		viewReplicasetsAdded,
		viewReplicasetsDeleted,
	)
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
