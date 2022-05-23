// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package flinkmetricsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/flinkmetricsreceiver"

import (
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/flinkmetricsreceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/flinkmetricsreceiver/internal/models"
)

func (s *flinkmetricsScraper) processJobmanagerMetrics(now pcommon.Timestamp, jobmanagerMetrics *models.JobmanagerMetrics) {
	for _, metric := range jobmanagerMetrics.Metrics {
		switch metric.ID {
		case "Status.JVM.CPU.Load":
			_ = s.mb.RecordFlinkJobmanagerJvmCPULoadDataPoint(now, metric.Value)
		case "Status.JVM.GarbageCollector.PS_MarkSweep.Time":
			_ = s.mb.RecordFlinkJobmanagerJvmGarbageCollectorCollectionTimeDataPoint(now, metric.Value, "PS_MarkSweep")
		case "Status.JVM.GarbageCollector.PS_Scavenge.Time":
			_ = s.mb.RecordFlinkJobmanagerJvmGarbageCollectorCollectionTimeDataPoint(now, metric.Value, "PS_Scavenge")
		case "Status.JVM.GarbageCollector.PS_MarkSweep.Count":
			_ = s.mb.RecordFlinkJobmanagerJvmGarbageCollectorCollectionCountDataPoint(now, metric.Value, "PS_MarkSweep")
		case "Status.JVM.GarbageCollector.PS_Scavenge.Count":
			_ = s.mb.RecordFlinkJobmanagerJvmGarbageCollectorCollectionCountDataPoint(now, metric.Value, "PS_Scavenge")
		case "Status.Flink.Memory.Managed.Used":
			_ = s.mb.RecordFlinkJobmanagerMemoryManagedUsedDataPoint(now, metric.Value)
		case "Status.Flink.Memory.Managed.Total":
			_ = s.mb.RecordFlinkJobmanagerMemoryManagedTotalDataPoint(now, metric.Value)
		case "Status.JVM.Memory.Mapped.TotalCapacity":
			_ = s.mb.RecordFlinkJobmanagerJvmMemoryMappedTotalCapacityDataPoint(now, metric.Value)
		case "Status.JVM.Memory.Mapped.MemoryUsed":
			_ = s.mb.RecordFlinkJobmanagerJvmMemoryMappedUsedDataPoint(now, metric.Value)
		case "Status.JVM.CPU.Time":
			_ = s.mb.RecordFlinkJobmanagerJvmCPUTimeDataPoint(now, metric.Value)
		case "Status.JVM.Threads.Count":
			_ = s.mb.RecordFlinkJobmanagerJvmThreadsCountDataPoint(now, metric.Value)
		case "Status.JVM.Memory.Heap.Committed":
			_ = s.mb.RecordFlinkJobmanagerJvmMemoryHeapCommittedDataPoint(now, metric.Value)
		case "Status.JVM.Memory.Metaspace.Committed":
			_ = s.mb.RecordFlinkJobmanagerJvmMemoryMetaspaceCommittedDataPoint(now, metric.Value)
		case "Status.JVM.Memory.NonHeap.Max":
			_ = s.mb.RecordFlinkJobmanagerJvmMemoryNonHeapMaxDataPoint(now, metric.Value)
		case "Status.JVM.Memory.NonHeap.Committed":
			_ = s.mb.RecordFlinkJobmanagerJvmMemoryNonHeapCommittedDataPoint(now, metric.Value)
		case "Status.JVM.Memory.NonHeap.Used":
			_ = s.mb.RecordFlinkJobmanagerJvmMemoryNonHeapUsedDataPoint(now, metric.Value)
		case "Status.JVM.Memory.Metaspace.Max":
			_ = s.mb.RecordFlinkJobmanagerJvmMemoryMetaspaceMaxDataPoint(now, metric.Value)
		case "Status.JVM.Memory.Direct.MemoryUsed":
			_ = s.mb.RecordFlinkJobmanagerJvmMemoryDirectUsedDataPoint(now, metric.Value)
		case "Status.JVM.Memory.Direct.TotalCapacity":
			_ = s.mb.RecordFlinkJobmanagerJvmMemoryDirectTotalCapacityDataPoint(now, metric.Value)
		case "Status.JVM.ClassLoader.ClassesLoaded":
			_ = s.mb.RecordFlinkJobmanagerJvmClassLoaderClassesLoadedDataPoint(now, metric.Value)
		case "Status.JVM.Memory.Metaspace.Used":
			_ = s.mb.RecordFlinkJobmanagerJvmMemoryMetaspaceUsedDataPoint(now, metric.Value)
		case "Status.JVM.Memory.Heap.Max":
			_ = s.mb.RecordFlinkJobmanagerJvmMemoryHeapMaxDataPoint(now, metric.Value)
		case "Status.JVM.Memory.Heap.Used":
			_ = s.mb.RecordFlinkJobmanagerJvmMemoryHeapUsedDataPoint(now, metric.Value)
		}
	}
	s.mb.EmitForResource(metadata.WithHostName(jobmanagerMetrics.Host))
}

func (s *flinkmetricsScraper) processTaskmanagerMetrics(now pcommon.Timestamp, taskmanagerMetricInstances []*models.TaskmanagerMetrics) {
	for _, taskmanagerMetrics := range taskmanagerMetricInstances {
		for _, metric := range taskmanagerMetrics.Metrics {

			switch metric.ID {
			case "Status.JVM.GarbageCollector.G1_Young_Generation.Count":
				_ = s.mb.RecordFlinkTaskmanagerJvmGarbageCollectorCollectionCountDataPoint(now, metric.Value, "G1_Young_Generation")
			case "Status.JVM.GarbageCollector.G1_Old_Generation.Count":
				_ = s.mb.RecordFlinkTaskmanagerJvmGarbageCollectorCollectionCountDataPoint(now, metric.Value, "G1_Old_Generation")
			case "Status.JVM.GarbageCollector.G1_Old_Generation.Time":
				_ = s.mb.RecordFlinkTaskmanagerJvmGarbageCollectorCollectionTimeDataPoint(now, metric.Value, "G1_Old_Generation")
			case "Status.JVM.GarbageCollector.G1_Young_Generation.Time":
				_ = s.mb.RecordFlinkTaskmanagerJvmGarbageCollectorCollectionTimeDataPoint(now, metric.Value, "G1_Young_Generation")
			case "Status.JVM.CPU.Load":
				_ = s.mb.RecordFlinkTaskmanagerJvmCPULoadDataPoint(now, metric.Value)
			case "Status.Flink.Memory.Managed.Used":
				_ = s.mb.RecordFlinkTaskmanagerMemoryManagedUsedDataPoint(now, metric.Value)
			case "Status.Flink.Memory.Managed.Total":
				_ = s.mb.RecordFlinkTaskmanagerMemoryManagedTotalDataPoint(now, metric.Value)
			case "Status.JVM.Memory.Mapped.TotalCapacity":
				_ = s.mb.RecordFlinkTaskmanagerJvmMemoryMappedTotalCapacityDataPoint(now, metric.Value)
			case "Status.JVM.Memory.Mapped.MemoryUsed":
				_ = s.mb.RecordFlinkTaskmanagerJvmMemoryMappedUsedDataPoint(now, metric.Value)
			case "Status.JVM.CPU.Time":
				_ = s.mb.RecordFlinkTaskmanagerJvmCPUTimeDataPoint(now, metric.Value)
			case "Status.JVM.Threads.Count":
				_ = s.mb.RecordFlinkTaskmanagerJvmThreadsCountDataPoint(now, metric.Value)
			case "Status.JVM.Memory.Heap.Committed":
				_ = s.mb.RecordFlinkTaskmanagerJvmMemoryHeapCommittedDataPoint(now, metric.Value)
			case "Status.JVM.Memory.Metaspace.Committed":
				_ = s.mb.RecordFlinkTaskmanagerJvmMemoryMetaspaceCommittedDataPoint(now, metric.Value)
			case "Status.JVM.Memory.NonHeap.Max":
				_ = s.mb.RecordFlinkTaskmanagerJvmMemoryNonHeapMaxDataPoint(now, metric.Value)
			case "Status.JVM.Memory.NonHeap.Committed":
				_ = s.mb.RecordFlinkTaskmanagerJvmMemoryNonHeapCommittedDataPoint(now, metric.Value)
			case "Status.JVM.Memory.NonHeap.Used":
				_ = s.mb.RecordFlinkTaskmanagerJvmMemoryNonHeapUsedDataPoint(now, metric.Value)
			case "Status.JVM.Memory.Metaspace.Max":
				_ = s.mb.RecordFlinkTaskmanagerJvmMemoryMetaspaceMaxDataPoint(now, metric.Value)
			case "Status.JVM.Memory.Direct.MemoryUsed":
				_ = s.mb.RecordFlinkTaskmanagerJvmMemoryDirectUsedDataPoint(now, metric.Value)
			case "Status.JVM.Memory.Direct.TotalCapacity":
				_ = s.mb.RecordFlinkTaskmanagerJvmMemoryDirectTotalCapacityDataPoint(now, metric.Value)
			case "Status.JVM.ClassLoader.ClassesLoaded":
				_ = s.mb.RecordFlinkTaskmanagerJvmClassLoaderClassesLoadedDataPoint(now, metric.Value)
			case "Status.JVM.Memory.Metaspace.Used":
				_ = s.mb.RecordFlinkTaskmanagerJvmMemoryMetaspaceUsedDataPoint(now, metric.Value)
			case "Status.JVM.Memory.Heap.Max":
				_ = s.mb.RecordFlinkTaskmanagerJvmMemoryHeapMaxDataPoint(now, metric.Value)
			case "Status.JVM.Memory.Heap.Used":
				_ = s.mb.RecordFlinkTaskmanagerJvmMemoryHeapUsedDataPoint(now, metric.Value)
			}
		}
	}
	if len(taskmanagerMetricInstances) > 0 {
		s.mb.EmitForResource(
			metadata.WithHostName(taskmanagerMetricInstances[0].Host),
			metadata.WithFlinkTaskmanagerID(taskmanagerMetricInstances[0].TaskmanagerID),
		)
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
				_ = s.mb.RecordFlinkJobCheckpointsCountDataPoint(now, metric.Value, metadata.AttributeCheckpointInProgress)
			case "numberOfCompletedCheckpoints":
				_ = s.mb.RecordFlinkJobCheckpointsCountDataPoint(now, metric.Value, metadata.AttributeCheckpointCompleted)
			case "numberOfFailedCheckpoints":
				_ = s.mb.RecordFlinkJobCheckpointsCountDataPoint(now, metric.Value, metadata.AttributeCheckpointFailed)
			}
		}
	}
	if len(jobsMetricsInstances) > 0 {
		s.mb.EmitForResource(
			metadata.WithHostName(jobsMetricsInstances[0].Host),
			metadata.WithFlinkJobName(jobsMetricsInstances[0].JobName),
		)
	}
}

func (s *flinkmetricsScraper) processSubtaskMetrics(now pcommon.Timestamp, subtaskMetricsInstances []*models.SubtaskMetrics) {
	for _, subtaskMetrics := range subtaskMetricsInstances {
		for i, metric := range subtaskMetrics.Metrics {
			switch {
			// record task metrics
			case metric.ID == "numRecordsIn":
				_ = s.mb.RecordFlinkTaskRecordCountDataPoint(now, metric.Value, metadata.AttributeRecordIn)
			case metric.ID == "numRecordsOut":
				_ = s.mb.RecordFlinkTaskRecordCountDataPoint(now, metric.Value, metadata.AttributeRecordOut)
			case metric.ID == "numLateRecordsDropped":
				_ = s.mb.RecordFlinkTaskRecordCountDataPoint(now, metric.Value, metadata.AttributeRecordLateRecordsDropped)
			}
			if i == len(subtaskMetrics.Metrics)-1 {
				s.mb.EmitForResource(
					metadata.WithHostName(subtaskMetrics.Host),
					metadata.WithFlinkTaskmanagerID(subtaskMetrics.TaskmanagerID),
					metadata.WithFlinkJobName(subtaskMetrics.JobName),
					metadata.WithFlinkTaskName(subtaskMetrics.TaskName),
					metadata.WithFlinkSubtaskIndex(subtaskMetrics.SubtaskIndex),
				)
			}
		}
	}

	for _, subtaskMetrics := range subtaskMetricsInstances {
		for i, metric := range subtaskMetrics.Metrics {
			switch {
			// record operator metrics
			case strings.Contains(metric.ID, ".numRecordsIn"):
				fields := strings.Split(metric.ID, ".numRecordsIn")
				_ = s.mb.RecordFlinkOperatorRecordCountDataPoint(now, metric.Value, fields[0], metadata.AttributeRecordIn)
			case strings.Contains(metric.ID, ".numRecordsOut"):
				fields := strings.Split(metric.ID, ".numRecordsOut")
				_ = s.mb.RecordFlinkOperatorRecordCountDataPoint(now, metric.Value, fields[0], metadata.AttributeRecordOut)
			case strings.Contains(metric.ID, ".numLateRecordsDropped"):
				fields := strings.Split(metric.ID, ".numLateRecordsDropped")
				_ = s.mb.RecordFlinkOperatorRecordCountDataPoint(now, metric.Value, fields[0], metadata.AttributeRecordLateRecordsDropped)
			case strings.Contains(metric.ID, ".currentOutputWatermark"):
				fields := strings.Split(metric.ID, ".currentOutputWatermark")
				_ = s.mb.RecordFlinkOperatorWatermarkOutputDataPoint(now, metric.Value, fields[0])
			}
			if i == len(subtaskMetrics.Metrics)-1 {
				s.mb.EmitForResource(
					metadata.WithHostName(subtaskMetrics.Host),
					metadata.WithFlinkTaskmanagerID(subtaskMetrics.TaskmanagerID),
					metadata.WithFlinkJobName(subtaskMetrics.JobName),
					metadata.WithFlinkTaskName(subtaskMetrics.TaskName),
					metadata.WithFlinkSubtaskIndex(subtaskMetrics.SubtaskIndex),
				)
			}
		}
	}
}
