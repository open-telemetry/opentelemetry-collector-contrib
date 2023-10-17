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
	for _, metric := range jobmanagerMetrics.Metrics {
		switch metric.ID {
		case "Status.JVM.CPU.Load":
			_ = s.mb.RecordFlinkJvmCPULoadDataPoint(now, metric.Value)
		case "Status.JVM.GarbageCollector.PS_MarkSweep.Time":
			_ = s.mb.RecordFlinkJvmGcCollectionsTimeDataPoint(now, metric.Value, metadata.AttributeGarbageCollectorNamePSMarkSweep)
		case "Status.JVM.GarbageCollector.PS_Scavenge.Time":
			_ = s.mb.RecordFlinkJvmGcCollectionsTimeDataPoint(now, metric.Value, metadata.AttributeGarbageCollectorNamePSScavenge)
		case "Status.JVM.GarbageCollector.PS_MarkSweep.Count":
			_ = s.mb.RecordFlinkJvmGcCollectionsCountDataPoint(now, metric.Value, metadata.AttributeGarbageCollectorNamePSMarkSweep)
		case "Status.JVM.GarbageCollector.PS_Scavenge.Count":
			_ = s.mb.RecordFlinkJvmGcCollectionsCountDataPoint(now, metric.Value, metadata.AttributeGarbageCollectorNamePSScavenge)
		case "Status.Flink.Memory.Managed.Used":
			_ = s.mb.RecordFlinkMemoryManagedUsedDataPoint(now, metric.Value)
		case "Status.Flink.Memory.Managed.Total":
			_ = s.mb.RecordFlinkMemoryManagedTotalDataPoint(now, metric.Value)
		case "Status.JVM.Memory.Mapped.TotalCapacity":
			_ = s.mb.RecordFlinkJvmMemoryMappedTotalCapacityDataPoint(now, metric.Value)
		case "Status.JVM.Memory.Mapped.MemoryUsed":
			_ = s.mb.RecordFlinkJvmMemoryMappedUsedDataPoint(now, metric.Value)
		case "Status.JVM.CPU.Time":
			_ = s.mb.RecordFlinkJvmCPUTimeDataPoint(now, metric.Value)
		case "Status.JVM.Threads.Count":
			_ = s.mb.RecordFlinkJvmThreadsCountDataPoint(now, metric.Value)
		case "Status.JVM.Memory.Heap.Committed":
			_ = s.mb.RecordFlinkJvmMemoryHeapCommittedDataPoint(now, metric.Value)
		case "Status.JVM.Memory.Metaspace.Committed":
			_ = s.mb.RecordFlinkJvmMemoryMetaspaceCommittedDataPoint(now, metric.Value)
		case "Status.JVM.Memory.NonHeap.Max":
			_ = s.mb.RecordFlinkJvmMemoryNonheapMaxDataPoint(now, metric.Value)
		case "Status.JVM.Memory.NonHeap.Committed":
			_ = s.mb.RecordFlinkJvmMemoryNonheapCommittedDataPoint(now, metric.Value)
		case "Status.JVM.Memory.NonHeap.Used":
			_ = s.mb.RecordFlinkJvmMemoryNonheapUsedDataPoint(now, metric.Value)
		case "Status.JVM.Memory.Metaspace.Max":
			_ = s.mb.RecordFlinkJvmMemoryMetaspaceMaxDataPoint(now, metric.Value)
		case "Status.JVM.Memory.Direct.MemoryUsed":
			_ = s.mb.RecordFlinkJvmMemoryDirectUsedDataPoint(now, metric.Value)
		case "Status.JVM.Memory.Direct.TotalCapacity":
			_ = s.mb.RecordFlinkJvmMemoryDirectTotalCapacityDataPoint(now, metric.Value)
		case "Status.JVM.ClassLoader.ClassesLoaded":
			_ = s.mb.RecordFlinkJvmClassLoaderClassesLoadedDataPoint(now, metric.Value)
		case "Status.JVM.Memory.Metaspace.Used":
			_ = s.mb.RecordFlinkJvmMemoryMetaspaceUsedDataPoint(now, metric.Value)
		case "Status.JVM.Memory.Heap.Max":
			_ = s.mb.RecordFlinkJvmMemoryHeapMaxDataPoint(now, metric.Value)
		case "Status.JVM.Memory.Heap.Used":
			_ = s.mb.RecordFlinkJvmMemoryHeapUsedDataPoint(now, metric.Value)
		}
	}
	rb := s.mb.NewResourceBuilder()
	rb.SetHostName(jobmanagerMetrics.Host)
	rb.SetFlinkResourceTypeJobmanager()
	s.mb.EmitForResource(metadata.WithResource(rb.Emit()))
}

func (s *flinkmetricsScraper) processTaskmanagerMetrics(now pcommon.Timestamp, taskmanagerMetricInstances []*models.TaskmanagerMetrics) {
	for _, taskmanagerMetrics := range taskmanagerMetricInstances {
		for _, metric := range taskmanagerMetrics.Metrics {
			switch metric.ID {
			case "Status.JVM.GarbageCollector.G1_Young_Generation.Count":
				_ = s.mb.RecordFlinkJvmGcCollectionsCountDataPoint(now, metric.Value, metadata.AttributeGarbageCollectorNameG1YoungGeneration)
			case "Status.JVM.GarbageCollector.G1_Old_Generation.Count":
				_ = s.mb.RecordFlinkJvmGcCollectionsCountDataPoint(now, metric.Value, metadata.AttributeGarbageCollectorNameG1OldGeneration)
			case "Status.JVM.GarbageCollector.G1_Old_Generation.Time":
				_ = s.mb.RecordFlinkJvmGcCollectionsTimeDataPoint(now, metric.Value, metadata.AttributeGarbageCollectorNameG1OldGeneration)
			case "Status.JVM.GarbageCollector.G1_Young_Generation.Time":
				_ = s.mb.RecordFlinkJvmGcCollectionsTimeDataPoint(now, metric.Value, metadata.AttributeGarbageCollectorNameG1YoungGeneration)
			case "Status.JVM.CPU.Load":
				_ = s.mb.RecordFlinkJvmCPULoadDataPoint(now, metric.Value)
			case "Status.Flink.Memory.Managed.Used":
				_ = s.mb.RecordFlinkMemoryManagedUsedDataPoint(now, metric.Value)
			case "Status.Flink.Memory.Managed.Total":
				_ = s.mb.RecordFlinkMemoryManagedTotalDataPoint(now, metric.Value)
			case "Status.JVM.Memory.Mapped.TotalCapacity":
				_ = s.mb.RecordFlinkJvmMemoryMappedTotalCapacityDataPoint(now, metric.Value)
			case "Status.JVM.Memory.Mapped.MemoryUsed":
				_ = s.mb.RecordFlinkJvmMemoryMappedUsedDataPoint(now, metric.Value)
			case "Status.JVM.CPU.Time":
				_ = s.mb.RecordFlinkJvmCPUTimeDataPoint(now, metric.Value)
			case "Status.JVM.Threads.Count":
				_ = s.mb.RecordFlinkJvmThreadsCountDataPoint(now, metric.Value)
			case "Status.JVM.Memory.Heap.Committed":
				_ = s.mb.RecordFlinkJvmMemoryHeapCommittedDataPoint(now, metric.Value)
			case "Status.JVM.Memory.Metaspace.Committed":
				_ = s.mb.RecordFlinkJvmMemoryMetaspaceCommittedDataPoint(now, metric.Value)
			case "Status.JVM.Memory.NonHeap.Max":
				_ = s.mb.RecordFlinkJvmMemoryNonheapMaxDataPoint(now, metric.Value)
			case "Status.JVM.Memory.NonHeap.Committed":
				_ = s.mb.RecordFlinkJvmMemoryNonheapCommittedDataPoint(now, metric.Value)
			case "Status.JVM.Memory.NonHeap.Used":
				_ = s.mb.RecordFlinkJvmMemoryNonheapUsedDataPoint(now, metric.Value)
			case "Status.JVM.Memory.Metaspace.Max":
				_ = s.mb.RecordFlinkJvmMemoryMetaspaceMaxDataPoint(now, metric.Value)
			case "Status.JVM.Memory.Direct.MemoryUsed":
				_ = s.mb.RecordFlinkJvmMemoryDirectUsedDataPoint(now, metric.Value)
			case "Status.JVM.Memory.Direct.TotalCapacity":
				_ = s.mb.RecordFlinkJvmMemoryDirectTotalCapacityDataPoint(now, metric.Value)
			case "Status.JVM.ClassLoader.ClassesLoaded":
				_ = s.mb.RecordFlinkJvmClassLoaderClassesLoadedDataPoint(now, metric.Value)
			case "Status.JVM.Memory.Metaspace.Used":
				_ = s.mb.RecordFlinkJvmMemoryMetaspaceUsedDataPoint(now, metric.Value)
			case "Status.JVM.Memory.Heap.Max":
				_ = s.mb.RecordFlinkJvmMemoryHeapMaxDataPoint(now, metric.Value)
			case "Status.JVM.Memory.Heap.Used":
				_ = s.mb.RecordFlinkJvmMemoryHeapUsedDataPoint(now, metric.Value)
			}
		}
		rb := s.mb.NewResourceBuilder()
		rb.SetHostName(taskmanagerMetrics.Host)
		rb.SetFlinkTaskmanagerID(taskmanagerMetrics.TaskmanagerID)
		rb.SetFlinkResourceTypeTaskmanager()
		s.mb.EmitForResource(metadata.WithResource(rb.Emit()))
	}
}

func (s *flinkmetricsScraper) processJobsMetrics(now pcommon.Timestamp, jobsMetricsInstances []*models.JobMetrics) {
	for _, jobsMetrics := range jobsMetricsInstances {
		for _, metric := range jobsMetrics.Metrics {
			switch metric.ID {
			case "numRestarts":
				_ = s.mb.RecordFlinkJobRestartCountDataPoint(now, metric.Value)
			case "lastCheckpointSize":
				_ = s.mb.RecordFlinkJobLastCheckpointSizeDataPoint(now, metric.Value)
			case "lastCheckpointDuration":
				_ = s.mb.RecordFlinkJobLastCheckpointTimeDataPoint(now, metric.Value)
			case "numberOfInProgressCheckpoints":
				_ = s.mb.RecordFlinkJobCheckpointInProgressDataPoint(now, metric.Value)
			case "numberOfCompletedCheckpoints":
				_ = s.mb.RecordFlinkJobCheckpointCountDataPoint(now, metric.Value, metadata.AttributeCheckpointCompleted)
			case "numberOfFailedCheckpoints":
				_ = s.mb.RecordFlinkJobCheckpointCountDataPoint(now, metric.Value, metadata.AttributeCheckpointFailed)
			}
		}
		rb := s.mb.NewResourceBuilder()
		rb.SetHostName(jobsMetrics.Host)
		rb.SetFlinkJobName(jobsMetrics.JobName)
		s.mb.EmitForResource(metadata.WithResource(rb.Emit()))
	}
}

func (s *flinkmetricsScraper) processSubtaskMetrics(now pcommon.Timestamp, subtaskMetricsInstances []*models.SubtaskMetrics) {
	for _, subtaskMetrics := range subtaskMetricsInstances {
		for _, metric := range subtaskMetrics.Metrics {
			switch {
			// record task metrics
			case metric.ID == "numRecordsIn":
				_ = s.mb.RecordFlinkTaskRecordCountDataPoint(now, metric.Value, metadata.AttributeRecordIn)
			case metric.ID == "numRecordsOut":
				_ = s.mb.RecordFlinkTaskRecordCountDataPoint(now, metric.Value, metadata.AttributeRecordOut)
			case metric.ID == "numLateRecordsDropped":
				_ = s.mb.RecordFlinkTaskRecordCountDataPoint(now, metric.Value, metadata.AttributeRecordDropped)
				// record operator metrics
			case strings.Contains(metric.ID, ".numRecordsIn"):
				operatorName := strings.Split(metric.ID, ".numRecordsIn")
				_ = s.mb.RecordFlinkOperatorRecordCountDataPoint(now, metric.Value, operatorName[0], metadata.AttributeRecordIn)
			case strings.Contains(metric.ID, ".numRecordsOut"):
				operatorName := strings.Split(metric.ID, ".numRecordsOut")
				_ = s.mb.RecordFlinkOperatorRecordCountDataPoint(now, metric.Value, operatorName[0], metadata.AttributeRecordOut)
			case strings.Contains(metric.ID, ".numLateRecordsDropped"):
				operatorName := strings.Split(metric.ID, ".numLateRecordsDropped")
				_ = s.mb.RecordFlinkOperatorRecordCountDataPoint(now, metric.Value, operatorName[0], metadata.AttributeRecordDropped)
			case strings.Contains(metric.ID, ".currentOutputWatermark"):
				operatorName := strings.Split(metric.ID, ".currentOutputWatermark")
				_ = s.mb.RecordFlinkOperatorWatermarkOutputDataPoint(now, metric.Value, operatorName[0])
			}
		}
		rb := s.mb.NewResourceBuilder()
		rb.SetHostName(subtaskMetrics.Host)
		rb.SetFlinkTaskmanagerID(subtaskMetrics.TaskmanagerID)
		rb.SetFlinkJobName(subtaskMetrics.JobName)
		rb.SetFlinkTaskName(subtaskMetrics.TaskName)
		rb.SetFlinkSubtaskIndex(subtaskMetrics.SubtaskIndex)
		s.mb.EmitForResource(metadata.WithResource(rb.Emit()))
	}
}
