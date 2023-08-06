// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package flinkmetricsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/flinkmetricsreceiver"

import (
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/flinkmetricsreceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/flinkmetricsreceiver/internal/models"
)

func (s *flinkmetricsScraper) processJobmanagerMetrics(now pcommon.Timestamp, jobmanagerMetrics *models.JobmanagerMetrics) {
	if jobmanagerMetrics == nil {
		return
	}
	rb := s.mb.NewResourceBuilder()
	rb.SetHostName(jobmanagerMetrics.Host)
	rb.SetFlinkResourceTypeJobmanager()
	rmb := s.mb.ResourceMetricsBuilder(rb.Emit())
	for _, metric := range jobmanagerMetrics.Metrics {
		switch metric.ID {
		case "Status.JVM.CPU.Load":
			_ = rmb.RecordFlinkJvmCPULoadDataPoint(now, metric.Value)
		case "Status.JVM.GarbageCollector.PS_MarkSweep.Time":
			_ = rmb.RecordFlinkJvmGcCollectionsTimeDataPoint(now, metric.Value, metadata.AttributeGarbageCollectorNamePSMarkSweep)
		case "Status.JVM.GarbageCollector.PS_Scavenge.Time":
			_ = rmb.RecordFlinkJvmGcCollectionsTimeDataPoint(now, metric.Value, metadata.AttributeGarbageCollectorNamePSScavenge)
		case "Status.JVM.GarbageCollector.PS_MarkSweep.Count":
			_ = rmb.RecordFlinkJvmGcCollectionsCountDataPoint(now, metric.Value, metadata.AttributeGarbageCollectorNamePSMarkSweep)
		case "Status.JVM.GarbageCollector.PS_Scavenge.Count":
			_ = rmb.RecordFlinkJvmGcCollectionsCountDataPoint(now, metric.Value, metadata.AttributeGarbageCollectorNamePSScavenge)
		case "Status.Flink.Memory.Managed.Used":
			_ = rmb.RecordFlinkMemoryManagedUsedDataPoint(now, metric.Value)
		case "Status.Flink.Memory.Managed.Total":
			_ = rmb.RecordFlinkMemoryManagedTotalDataPoint(now, metric.Value)
		case "Status.JVM.Memory.Mapped.TotalCapacity":
			_ = rmb.RecordFlinkJvmMemoryMappedTotalCapacityDataPoint(now, metric.Value)
		case "Status.JVM.Memory.Mapped.MemoryUsed":
			_ = rmb.RecordFlinkJvmMemoryMappedUsedDataPoint(now, metric.Value)
		case "Status.JVM.CPU.Time":
			_ = rmb.RecordFlinkJvmCPUTimeDataPoint(now, metric.Value)
		case "Status.JVM.Threads.Count":
			_ = rmb.RecordFlinkJvmThreadsCountDataPoint(now, metric.Value)
		case "Status.JVM.Memory.Heap.Committed":
			_ = rmb.RecordFlinkJvmMemoryHeapCommittedDataPoint(now, metric.Value)
		case "Status.JVM.Memory.Metaspace.Committed":
			_ = rmb.RecordFlinkJvmMemoryMetaspaceCommittedDataPoint(now, metric.Value)
		case "Status.JVM.Memory.NonHeap.Max":
			_ = rmb.RecordFlinkJvmMemoryNonheapMaxDataPoint(now, metric.Value)
		case "Status.JVM.Memory.NonHeap.Committed":
			_ = rmb.RecordFlinkJvmMemoryNonheapCommittedDataPoint(now, metric.Value)
		case "Status.JVM.Memory.NonHeap.Used":
			_ = rmb.RecordFlinkJvmMemoryNonheapUsedDataPoint(now, metric.Value)
		case "Status.JVM.Memory.Metaspace.Max":
			_ = rmb.RecordFlinkJvmMemoryMetaspaceMaxDataPoint(now, metric.Value)
		case "Status.JVM.Memory.Direct.MemoryUsed":
			_ = rmb.RecordFlinkJvmMemoryDirectUsedDataPoint(now, metric.Value)
		case "Status.JVM.Memory.Direct.TotalCapacity":
			_ = rmb.RecordFlinkJvmMemoryDirectTotalCapacityDataPoint(now, metric.Value)
		case "Status.JVM.ClassLoader.ClassesLoaded":
			_ = rmb.RecordFlinkJvmClassLoaderClassesLoadedDataPoint(now, metric.Value)
		case "Status.JVM.Memory.Metaspace.Used":
			_ = rmb.RecordFlinkJvmMemoryMetaspaceUsedDataPoint(now, metric.Value)
		case "Status.JVM.Memory.Heap.Max":
			_ = rmb.RecordFlinkJvmMemoryHeapMaxDataPoint(now, metric.Value)
		case "Status.JVM.Memory.Heap.Used":
			_ = rmb.RecordFlinkJvmMemoryHeapUsedDataPoint(now, metric.Value)
		}
	}
}

func (s *flinkmetricsScraper) processTaskmanagerMetrics(now pcommon.Timestamp, taskmanagerMetricInstances []*models.TaskmanagerMetrics) {
	for _, taskmanagerMetrics := range taskmanagerMetricInstances {
		rb := s.mb.NewResourceBuilder()
		rb.SetHostName(taskmanagerMetrics.Host)
		rb.SetFlinkTaskmanagerID(taskmanagerMetrics.TaskmanagerID)
		rb.SetFlinkResourceTypeTaskmanager()
		rmb := s.mb.ResourceMetricsBuilder(rb.Emit())
		for _, metric := range taskmanagerMetrics.Metrics {
			switch metric.ID {
			case "Status.JVM.GarbageCollector.G1_Young_Generation.Count":
				_ = rmb.RecordFlinkJvmGcCollectionsCountDataPoint(now, metric.Value, metadata.AttributeGarbageCollectorNameG1YoungGeneration)
			case "Status.JVM.GarbageCollector.G1_Old_Generation.Count":
				_ = rmb.RecordFlinkJvmGcCollectionsCountDataPoint(now, metric.Value, metadata.AttributeGarbageCollectorNameG1OldGeneration)
			case "Status.JVM.GarbageCollector.G1_Old_Generation.Time":
				_ = rmb.RecordFlinkJvmGcCollectionsTimeDataPoint(now, metric.Value, metadata.AttributeGarbageCollectorNameG1OldGeneration)
			case "Status.JVM.GarbageCollector.G1_Young_Generation.Time":
				_ = rmb.RecordFlinkJvmGcCollectionsTimeDataPoint(now, metric.Value, metadata.AttributeGarbageCollectorNameG1YoungGeneration)
			case "Status.JVM.CPU.Load":
				_ = rmb.RecordFlinkJvmCPULoadDataPoint(now, metric.Value)
			case "Status.Flink.Memory.Managed.Used":
				_ = rmb.RecordFlinkMemoryManagedUsedDataPoint(now, metric.Value)
			case "Status.Flink.Memory.Managed.Total":
				_ = rmb.RecordFlinkMemoryManagedTotalDataPoint(now, metric.Value)
			case "Status.JVM.Memory.Mapped.TotalCapacity":
				_ = rmb.RecordFlinkJvmMemoryMappedTotalCapacityDataPoint(now, metric.Value)
			case "Status.JVM.Memory.Mapped.MemoryUsed":
				_ = rmb.RecordFlinkJvmMemoryMappedUsedDataPoint(now, metric.Value)
			case "Status.JVM.CPU.Time":
				_ = rmb.RecordFlinkJvmCPUTimeDataPoint(now, metric.Value)
			case "Status.JVM.Threads.Count":
				_ = rmb.RecordFlinkJvmThreadsCountDataPoint(now, metric.Value)
			case "Status.JVM.Memory.Heap.Committed":
				_ = rmb.RecordFlinkJvmMemoryHeapCommittedDataPoint(now, metric.Value)
			case "Status.JVM.Memory.Metaspace.Committed":
				_ = rmb.RecordFlinkJvmMemoryMetaspaceCommittedDataPoint(now, metric.Value)
			case "Status.JVM.Memory.NonHeap.Max":
				_ = rmb.RecordFlinkJvmMemoryNonheapMaxDataPoint(now, metric.Value)
			case "Status.JVM.Memory.NonHeap.Committed":
				_ = rmb.RecordFlinkJvmMemoryNonheapCommittedDataPoint(now, metric.Value)
			case "Status.JVM.Memory.NonHeap.Used":
				_ = rmb.RecordFlinkJvmMemoryNonheapUsedDataPoint(now, metric.Value)
			case "Status.JVM.Memory.Metaspace.Max":
				_ = rmb.RecordFlinkJvmMemoryMetaspaceMaxDataPoint(now, metric.Value)
			case "Status.JVM.Memory.Direct.MemoryUsed":
				_ = rmb.RecordFlinkJvmMemoryDirectUsedDataPoint(now, metric.Value)
			case "Status.JVM.Memory.Direct.TotalCapacity":
				_ = rmb.RecordFlinkJvmMemoryDirectTotalCapacityDataPoint(now, metric.Value)
			case "Status.JVM.ClassLoader.ClassesLoaded":
				_ = rmb.RecordFlinkJvmClassLoaderClassesLoadedDataPoint(now, metric.Value)
			case "Status.JVM.Memory.Metaspace.Used":
				_ = rmb.RecordFlinkJvmMemoryMetaspaceUsedDataPoint(now, metric.Value)
			case "Status.JVM.Memory.Heap.Max":
				_ = rmb.RecordFlinkJvmMemoryHeapMaxDataPoint(now, metric.Value)
			case "Status.JVM.Memory.Heap.Used":
				_ = rmb.RecordFlinkJvmMemoryHeapUsedDataPoint(now, metric.Value)
			}
		}
	}
}

func (s *flinkmetricsScraper) processJobsMetrics(now pcommon.Timestamp, jobsMetricsInstances []*models.JobMetrics) {
	for _, jobsMetrics := range jobsMetricsInstances {
		rb := s.mb.NewResourceBuilder()
		rb.SetHostName(jobsMetrics.Host)
		rb.SetFlinkJobName(jobsMetrics.JobName)
		rmb := s.mb.ResourceMetricsBuilder(rb.Emit())
		for _, metric := range jobsMetrics.Metrics {
			switch metric.ID {
			case "numRestarts":
				_ = rmb.RecordFlinkJobRestartCountDataPoint(now, metric.Value)
			case "lastCheckpointSize":
				_ = rmb.RecordFlinkJobLastCheckpointSizeDataPoint(now, metric.Value)
			case "lastCheckpointDuration":
				_ = rmb.RecordFlinkJobLastCheckpointTimeDataPoint(now, metric.Value)
			case "numberOfInProgressCheckpoints":
				_ = rmb.RecordFlinkJobCheckpointInProgressDataPoint(now, metric.Value)
			case "numberOfCompletedCheckpoints":
				_ = rmb.RecordFlinkJobCheckpointCountDataPoint(now, metric.Value, metadata.AttributeCheckpointCompleted)
			case "numberOfFailedCheckpoints":
				_ = rmb.RecordFlinkJobCheckpointCountDataPoint(now, metric.Value, metadata.AttributeCheckpointFailed)
			}
		}
	}
}

func (s *flinkmetricsScraper) processSubtaskMetrics(now pcommon.Timestamp, subtaskMetricsInstances []*models.SubtaskMetrics) {
	for _, subtaskMetrics := range subtaskMetricsInstances {
		rb := s.mb.NewResourceBuilder()
		rb.SetHostName(subtaskMetrics.Host)
		rb.SetFlinkTaskmanagerID(subtaskMetrics.TaskmanagerID)
		rb.SetFlinkJobName(subtaskMetrics.JobName)
		rb.SetFlinkTaskName(subtaskMetrics.TaskName)
		rb.SetFlinkSubtaskIndex(subtaskMetrics.SubtaskIndex)
		rmb := s.mb.ResourceMetricsBuilder(rb.Emit())
		for _, metric := range subtaskMetrics.Metrics {
			switch {
			// record task metrics
			case metric.ID == "numRecordsIn":
				_ = rmb.RecordFlinkTaskRecordCountDataPoint(now, metric.Value, metadata.AttributeRecordIn)
			case metric.ID == "numRecordsOut":
				_ = rmb.RecordFlinkTaskRecordCountDataPoint(now, metric.Value, metadata.AttributeRecordOut)
			case metric.ID == "numLateRecordsDropped":
				_ = rmb.RecordFlinkTaskRecordCountDataPoint(now, metric.Value, metadata.AttributeRecordDropped)
				// record operator metrics
			case strings.Contains(metric.ID, ".numRecordsIn"):
				operatorName := strings.Split(metric.ID, ".numRecordsIn")
				_ = rmb.RecordFlinkOperatorRecordCountDataPoint(now, metric.Value, operatorName[0], metadata.AttributeRecordIn)
			case strings.Contains(metric.ID, ".numRecordsOut"):
				operatorName := strings.Split(metric.ID, ".numRecordsOut")
				_ = rmb.RecordFlinkOperatorRecordCountDataPoint(now, metric.Value, operatorName[0], metadata.AttributeRecordOut)
			case strings.Contains(metric.ID, ".numLateRecordsDropped"):
				operatorName := strings.Split(metric.ID, ".numLateRecordsDropped")
				_ = rmb.RecordFlinkOperatorRecordCountDataPoint(now, metric.Value, operatorName[0], metadata.AttributeRecordDropped)
			case strings.Contains(metric.ID, ".currentOutputWatermark"):
				operatorName := strings.Split(metric.ID, ".currentOutputWatermark")
				_ = rmb.RecordFlinkOperatorWatermarkOutputDataPoint(now, metric.Value, operatorName[0])
			}
		}
	}
}
