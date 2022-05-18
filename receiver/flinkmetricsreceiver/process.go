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
			_ = s.mb.RecordFlinkmetricsJobmanagerStatusJvmCPULoadDataPoint(now, metric.Value)
		case "Status.JVM.GarbageCollector.PS_MarkSweep.Time":
			_ = s.mb.RecordFlinkmetricsJobmanagerStatusJvmGarbageCollectorCollectionTimeDataPoint(now, metric.Value, "PS_MarkSweep")
		case "Status.JVM.GarbageCollector.PS_Scavenge.Time":
			_ = s.mb.RecordFlinkmetricsJobmanagerStatusJvmGarbageCollectorCollectionTimeDataPoint(now, metric.Value, "PS_Scavenge")
		case "Status.JVM.GarbageCollector.PS_MarkSweep.Count":
			_ = s.mb.RecordFlinkmetricsJobmanagerStatusJvmGarbageCollectorCollectionCountDataPoint(now, metric.Value, "PS_MarkSweep")
		case "Status.JVM.GarbageCollector.PS_Scavenge.Count":
			_ = s.mb.RecordFlinkmetricsJobmanagerStatusJvmGarbageCollectorCollectionCountDataPoint(now, metric.Value, "PS_Scavenge")
		case "Status.Flink.Memory.Managed.Used":
			_ = s.mb.RecordFlinkmetricsJobmanagerStatusFlinkMemoryManagedUsedDataPoint(now, metric.Value)
		case "Status.Flink.Memory.Managed.Total":
			_ = s.mb.RecordFlinkmetricsJobmanagerStatusFlinkMemoryManagedTotalDataPoint(now, metric.Value)
		case "Status.JVM.Memory.Mapped.TotalCapacity":
			_ = s.mb.RecordFlinkmetricsJobmanagerStatusJvmMemoryMappedTotalCapacityDataPoint(now, metric.Value)
		case "Status.JVM.Memory.Mapped.MemoryUsed":
			_ = s.mb.RecordFlinkmetricsJobmanagerStatusJvmMemoryMappedUsedDataPoint(now, metric.Value)
		case "Status.JVM.CPU.Time":
			_ = s.mb.RecordFlinkmetricsJobmanagerStatusJvmCPUTimeDataPoint(now, metric.Value)
		case "Status.JVM.Threads.Count":
			_ = s.mb.RecordFlinkmetricsJobmanagerStatusJvmThreadsCountDataPoint(now, metric.Value)
		case "Status.JVM.Memory.Heap.Committed":
			_ = s.mb.RecordFlinkmetricsJobmanagerStatusJvmMemoryHeapCommittedDataPoint(now, metric.Value)
		case "Status.JVM.Memory.Metaspace.Committed":
			_ = s.mb.RecordFlinkmetricsJobmanagerStatusJvmMemoryMetaspaceCommittedDataPoint(now, metric.Value)
		case "Status.JVM.Memory.NonHeap.Max":
			_ = s.mb.RecordFlinkmetricsJobmanagerStatusJvmMemoryNonHeapMaxDataPoint(now, metric.Value)
		case "Status.JVM.Memory.NonHeap.Committed":
			_ = s.mb.RecordFlinkmetricsJobmanagerStatusJvmMemoryNonHeapCommittedDataPoint(now, metric.Value)
		case "Status.JVM.Memory.NonHeap.Used":
			_ = s.mb.RecordFlinkmetricsJobmanagerStatusJvmMemoryNonHeapUsedDataPoint(now, metric.Value)
		case "Status.JVM.Memory.Metaspace.Max":
			_ = s.mb.RecordFlinkmetricsJobmanagerStatusJvmMemoryMetaspaceMaxDataPoint(now, metric.Value)
		case "Status.JVM.Memory.Direct.MemoryUsed":
			_ = s.mb.RecordFlinkmetricsJobmanagerStatusJvmMemoryDirectUsedDataPoint(now, metric.Value)
		case "Status.JVM.Memory.Direct.TotalCapacity":
			_ = s.mb.RecordFlinkmetricsJobmanagerStatusJvmMemoryDirectTotalCapacityDataPoint(now, metric.Value)
		case "Status.JVM.ClassLoader.ClassesLoaded":
			_ = s.mb.RecordFlinkmetricsJobmanagerStatusJvmClassLoaderClassesLoadedDataPoint(now, metric.Value)
		case "Status.JVM.Memory.Metaspace.Used":
			_ = s.mb.RecordFlinkmetricsJobmanagerStatusJvmMemoryMetaspaceUsedDataPoint(now, metric.Value)
		case "Status.JVM.Memory.Heap.Max":
			_ = s.mb.RecordFlinkmetricsJobmanagerStatusJvmMemoryHeapMaxDataPoint(now, metric.Value)
		case "Status.JVM.Memory.Heap.Used":
			_ = s.mb.RecordFlinkmetricsJobmanagerStatusJvmMemoryHeapUsedDataPoint(now, metric.Value)
		}
	}
	s.mb.EmitForResource(metadata.WithHost(jobmanagerMetrics.Host))
}

func (s *flinkmetricsScraper) processTaskmanagerMetrics(now pcommon.Timestamp, taskmanagerMetricInstances []*models.TaskmanagerMetrics) {
	for _, taskmanagerMetrics := range taskmanagerMetricInstances {
		for _, metric := range taskmanagerMetrics.Metrics {

			switch metric.ID {
			case "Status.JVM.GarbageCollector.G1_Young_Generation.Count":
				_ = s.mb.RecordFlinkmetricsTaskmanagerStatusJvmGarbageCollectorCollectionCountDataPoint(now, metric.Value, "G1_Young_Generation")
			case "Status.JVM.GarbageCollector.G1_Old_Generation.Count":
				_ = s.mb.RecordFlinkmetricsTaskmanagerStatusJvmGarbageCollectorCollectionCountDataPoint(now, metric.Value, "G1_Old_Generation")
			case "Status.JVM.GarbageCollector.G1_Old_Generation.Time":
				_ = s.mb.RecordFlinkmetricsTaskmanagerStatusJvmGarbageCollectorCollectionTimeDataPoint(now, metric.Value, "G1_Old_Generation")
			case "Status.JVM.GarbageCollector.G1_Young_Generation.Time":
				_ = s.mb.RecordFlinkmetricsTaskmanagerStatusJvmGarbageCollectorCollectionTimeDataPoint(now, metric.Value, "G1_Young_Generation")
			case "Status.JVM.CPU.Load":
				_ = s.mb.RecordFlinkmetricsTaskmanagerStatusJvmCPULoadDataPoint(now, metric.Value)
			case "Status.Flink.Memory.Managed.Used":
				_ = s.mb.RecordFlinkmetricsTaskmanagerStatusFlinkMemoryManagedUsedDataPoint(now, metric.Value)
			case "Status.Flink.Memory.Managed.Total":
				_ = s.mb.RecordFlinkmetricsTaskmanagerStatusFlinkMemoryManagedTotalDataPoint(now, metric.Value)
			case "Status.JVM.Memory.Mapped.TotalCapacity":
				_ = s.mb.RecordFlinkmetricsTaskmanagerStatusJvmMemoryMappedTotalCapacityDataPoint(now, metric.Value)
			case "Status.JVM.Memory.Mapped.MemoryUsed":
				_ = s.mb.RecordFlinkmetricsTaskmanagerStatusJvmMemoryMappedUsedDataPoint(now, metric.Value)
			case "Status.JVM.CPU.Time":
				_ = s.mb.RecordFlinkmetricsTaskmanagerStatusJvmCPUTimeDataPoint(now, metric.Value)
			case "Status.JVM.Threads.Count":
				_ = s.mb.RecordFlinkmetricsTaskmanagerStatusJvmThreadsCountDataPoint(now, metric.Value)
			case "Status.JVM.Memory.Heap.Committed":
				_ = s.mb.RecordFlinkmetricsTaskmanagerStatusJvmMemoryHeapCommittedDataPoint(now, metric.Value)
			case "Status.JVM.Memory.Metaspace.Committed":
				_ = s.mb.RecordFlinkmetricsTaskmanagerStatusJvmMemoryMetaspaceCommittedDataPoint(now, metric.Value)
			case "Status.JVM.Memory.NonHeap.Max":
				_ = s.mb.RecordFlinkmetricsTaskmanagerStatusJvmMemoryNonHeapMaxDataPoint(now, metric.Value)
			case "Status.JVM.Memory.NonHeap.Committed":
				_ = s.mb.RecordFlinkmetricsTaskmanagerStatusJvmMemoryNonHeapCommittedDataPoint(now, metric.Value)
			case "Status.JVM.Memory.NonHeap.Used":
				_ = s.mb.RecordFlinkmetricsTaskmanagerStatusJvmMemoryNonHeapUsedDataPoint(now, metric.Value)
			case "Status.JVM.Memory.Metaspace.Max":
				_ = s.mb.RecordFlinkmetricsTaskmanagerStatusJvmMemoryMetaspaceMaxDataPoint(now, metric.Value)
			case "Status.JVM.Memory.Direct.MemoryUsed":
				_ = s.mb.RecordFlinkmetricsTaskmanagerStatusJvmMemoryDirectUsedDataPoint(now, metric.Value)
			case "Status.JVM.Memory.Direct.TotalCapacity":
				_ = s.mb.RecordFlinkmetricsTaskmanagerStatusJvmMemoryDirectTotalCapacityDataPoint(now, metric.Value)
			case "Status.JVM.ClassLoader.ClassesLoaded":
				_ = s.mb.RecordFlinkmetricsTaskmanagerStatusJvmClassLoaderClassesLoadedDataPoint(now, metric.Value)
			case "Status.JVM.Memory.Metaspace.Used":
				_ = s.mb.RecordFlinkmetricsTaskmanagerStatusJvmMemoryMetaspaceUsedDataPoint(now, metric.Value)
			case "Status.JVM.Memory.Heap.Max":
				_ = s.mb.RecordFlinkmetricsTaskmanagerStatusJvmMemoryHeapMaxDataPoint(now, metric.Value)
			case "Status.JVM.Memory.Heap.Used":
				_ = s.mb.RecordFlinkmetricsTaskmanagerStatusJvmMemoryHeapUsedDataPoint(now, metric.Value)
			}
		}
	}
	if len(taskmanagerMetricInstances) > 0 {
		s.mb.EmitForResource(metadata.WithHost(taskmanagerMetricInstances[0].Host), metadata.WithTaskmanagerID(taskmanagerMetricInstances[0].TaskmanagerID))
	}
}

func (s *flinkmetricsScraper) processJobsMetrics(now pcommon.Timestamp, jobsMetricsInstances []*models.JobMetrics) {
	for _, jobsMetrics := range jobsMetricsInstances {
		for _, metric := range jobsMetrics.Metrics {
			switch metric.ID {
			case "numRestarts":
				_ = s.mb.RecordFlinkmetricsJobRestartCountDataPoint(now, metric.Value)
			case "lastCheckpointSize":
				_ = s.mb.RecordFlinkmetricsJobLastCheckpointSizeDataPoint(now, metric.Value)
			case "lastCheckpointDuration":
				_ = s.mb.RecordFlinkmetricsJobLastCheckpointTimeDataPoint(now, metric.Value)
			case "numberOfInProgressCheckpoints":
				_ = s.mb.RecordFlinkmetricsJobCheckpointsCountDataPoint(now, metric.Value, metadata.AttributeCheckpointInProgress)
			case "numberOfCompletedCheckpoints":
				_ = s.mb.RecordFlinkmetricsJobCheckpointsCountDataPoint(now, metric.Value, metadata.AttributeCheckpointCompleted)
			case "numberOfFailedCheckpoints":
				_ = s.mb.RecordFlinkmetricsJobCheckpointsCountDataPoint(now, metric.Value, metadata.AttributeCheckpointFailed)
			}
		}
	}
	if len(jobsMetricsInstances) > 0 {
		s.mb.EmitForResource(metadata.WithHost(jobsMetricsInstances[0].Host), metadata.WithJobName(jobsMetricsInstances[0].JobName))
	}
}

func (s *flinkmetricsScraper) processSubtaskMetrics(now pcommon.Timestamp, subtaskMetricsInstances []*models.SubtaskMetrics) {
	for _, subtaskMetrics := range subtaskMetricsInstances {
		for i, metric := range subtaskMetrics.Metrics {
			switch {
			// record task metrics
			case metric.ID == "numRecordsIn":
				_ = s.mb.RecordFlinkmetricsTaskRecordCountDataPoint(now, metric.Value, metadata.AttributeRecordIn)
			case metric.ID == "numRecordsOut":
				_ = s.mb.RecordFlinkmetricsTaskRecordCountDataPoint(now, metric.Value, metadata.AttributeRecordOut)
			case metric.ID == "numLateRecordsDropped":
				_ = s.mb.RecordFlinkmetricsTaskRecordCountDataPoint(now, metric.Value, metadata.AttributeRecordLateRecordsDropped)
			}
			if i == len(subtaskMetrics.Metrics)-1 {
				s.mb.EmitForResource(metadata.WithHost(subtaskMetrics.Host), metadata.WithTaskmanagerID(subtaskMetrics.TaskmanagerID), metadata.WithJobName(subtaskMetrics.JobName), metadata.WithTaskName(subtaskMetrics.TaskName), metadata.WithSubtaskIndex(subtaskMetrics.SubtaskIndex))
			}
		}
	}

	for _, subtaskMetrics := range subtaskMetricsInstances {
		for i, metric := range subtaskMetrics.Metrics {
			switch {
			// record operator metrics
			case strings.Contains(metric.ID, ".numRecordsIn"):
				fields := strings.Split(metric.ID, ".numRecordsIn")
				_ = s.mb.RecordFlinkmetricsOperatorRecordCountDataPoint(now, metric.Value, fields[0], metadata.AttributeRecordIn)
			case strings.Contains(metric.ID, ".numRecordsOut"):
				fields := strings.Split(metric.ID, ".numRecordsOut")
				_ = s.mb.RecordFlinkmetricsOperatorRecordCountDataPoint(now, metric.Value, fields[0], metadata.AttributeRecordOut)
			case strings.Contains(metric.ID, ".numLateRecordsDropped"):
				fields := strings.Split(metric.ID, ".numLateRecordsDropped")
				_ = s.mb.RecordFlinkmetricsOperatorRecordCountDataPoint(now, metric.Value, fields[0], metadata.AttributeRecordLateRecordsDropped)
			case strings.Contains(metric.ID, ".currentOutputWatermark"):
				fields := strings.Split(metric.ID, ".currentOutputWatermark")
				_ = s.mb.RecordFlinkmetricsOperatorWatermarkOutputDataPoint(now, metric.Value, fields[0])
			}
			if i == len(subtaskMetrics.Metrics)-1 {
				s.mb.EmitForResource(metadata.WithHost(subtaskMetrics.Host), metadata.WithTaskmanagerID(subtaskMetrics.TaskmanagerID), metadata.WithJobName(subtaskMetrics.JobName), metadata.WithTaskName(subtaskMetrics.TaskName), metadata.WithSubtaskIndex(subtaskMetrics.SubtaskIndex))
			}
		}
	}
}
