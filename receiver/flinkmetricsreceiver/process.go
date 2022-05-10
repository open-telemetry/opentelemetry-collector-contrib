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

func (s *flinkmetricsScraper) ProcessJobmanagerMetrics(now pcommon.Timestamp, jobmanagerMetrics *models.JobmanagerMetrics) {
	for _, metric := range jobmanagerMetrics.Metrics {
		switch metric.ID {
		case "Status.JVM.CPU.Load":
			_ = s.mb.RecordFlinkJobmanagerStatusJvmCPULoadDataPoint(now, metric.Value, jobmanagerMetrics.Host)
		case "Status.JVM.GarbageCollector.PS_MarkSweep.Time":
			_ = s.mb.RecordFlinkJobmanagerStatusJvmGarbageCollectorCollectionTimeDataPoint(now, metric.Value, jobmanagerMetrics.Host, "PS_MarkSweep")
		case "Status.JVM.GarbageCollector.PS_Scavenge.Time":
			_ = s.mb.RecordFlinkJobmanagerStatusJvmGarbageCollectorCollectionTimeDataPoint(now, metric.Value, jobmanagerMetrics.Host, "PS_Scavenge")
		case "Status.JVM.GarbageCollector.PS_MarkSweep.Count":
			_ = s.mb.RecordFlinkJobmanagerStatusJvmGarbageCollectorCollectionCountDataPoint(now, metric.Value, jobmanagerMetrics.Host, "PS_MarkSweep")
		case "Status.JVM.GarbageCollector.PS_Scavenge.Count":
			_ = s.mb.RecordFlinkJobmanagerStatusJvmGarbageCollectorCollectionCountDataPoint(now, metric.Value, jobmanagerMetrics.Host, "PS_Scavenge")
		case "Status.Flink.Memory.Managed.Used":
			_ = s.mb.RecordFlinkJobmanagerStatusFlinkMemoryManagedUsedDataPoint(now, metric.Value, jobmanagerMetrics.Host)
		case "Status.Flink.Memory.Managed.Total":
			_ = s.mb.RecordFlinkJobmanagerStatusFlinkMemoryManagedTotalDataPoint(now, metric.Value, jobmanagerMetrics.Host)
		case "Status.JVM.Memory.Mapped.TotalCapacity":
			_ = s.mb.RecordFlinkJobmanagerStatusJvmMemoryMappedTotalCapacityDataPoint(now, metric.Value, jobmanagerMetrics.Host)
		case "Status.JVM.Memory.Mapped.MemoryUsed":
			_ = s.mb.RecordFlinkJobmanagerStatusJvmMemoryMappedUsedDataPoint(now, metric.Value, jobmanagerMetrics.Host)
		case "Status.JVM.CPU.Time":
			_ = s.mb.RecordFlinkJobmanagerStatusJvmCPUTimeDataPoint(now, metric.Value, jobmanagerMetrics.Host)
		case "Status.JVM.Threads.Count":
			_ = s.mb.RecordFlinkJobmanagerStatusJvmThreadsCountDataPoint(now, metric.Value, jobmanagerMetrics.Host)
		case "Status.JVM.Memory.Heap.Committed":
			_ = s.mb.RecordFlinkJobmanagerStatusJvmMemoryHeapCommittedDataPoint(now, metric.Value, jobmanagerMetrics.Host)
		case "Status.JVM.Memory.Metaspace.Committed":
			_ = s.mb.RecordFlinkJobmanagerStatusJvmMemoryMetaspaceCommittedDataPoint(now, metric.Value, jobmanagerMetrics.Host)
		case "Status.JVM.Memory.NonHeap.Max":
			_ = s.mb.RecordFlinkJobmanagerStatusJvmMemoryNonHeapMaxDataPoint(now, metric.Value, jobmanagerMetrics.Host)
		case "Status.JVM.Memory.NonHeap.Committed":
			_ = s.mb.RecordFlinkJobmanagerStatusJvmMemoryNonHeapCommittedDataPoint(now, metric.Value, jobmanagerMetrics.Host)
		case "Status.JVM.Memory.NonHeap.Used":
			_ = s.mb.RecordFlinkJobmanagerStatusJvmMemoryNonHeapUsedDataPoint(now, metric.Value, jobmanagerMetrics.Host)
		case "Status.JVM.Memory.Metaspace.Max":
			_ = s.mb.RecordFlinkJobmanagerStatusJvmMemoryMetaspaceMaxDataPoint(now, metric.Value, jobmanagerMetrics.Host)
		case "Status.JVM.Memory.Direct.MemoryUsed":
			_ = s.mb.RecordFlinkJobmanagerStatusJvmMemoryDirectUsedDataPoint(now, metric.Value, jobmanagerMetrics.Host)
		case "Status.JVM.Memory.Direct.TotalCapacity":
			_ = s.mb.RecordFlinkJobmanagerStatusJvmMemoryDirectTotalCapacityDataPoint(now, metric.Value, jobmanagerMetrics.Host)
		case "Status.JVM.ClassLoader.ClassesLoaded":
			_ = s.mb.RecordFlinkJobmanagerStatusJvmClassLoaderClassesLoadedDataPoint(now, metric.Value, jobmanagerMetrics.Host)
		case "Status.JVM.Memory.Metaspace.Used":
			_ = s.mb.RecordFlinkJobmanagerStatusJvmMemoryMetaspaceUsedDataPoint(now, metric.Value, jobmanagerMetrics.Host)
		case "Status.JVM.Memory.Heap.Max":
			_ = s.mb.RecordFlinkJobmanagerStatusJvmMemoryHeapMaxDataPoint(now, metric.Value, jobmanagerMetrics.Host)
		case "Status.JVM.Memory.Heap.Used":
			_ = s.mb.RecordFlinkJobmanagerStatusJvmMemoryHeapUsedDataPoint(now, metric.Value, jobmanagerMetrics.Host)
		}
	}
}

func (s *flinkmetricsScraper) ProcessTaskmanagerMetrics(now pcommon.Timestamp, taskmanagerMetricInstances []*models.TaskmanagerMetrics) {
	for _, taskmanagerMetrics := range taskmanagerMetricInstances {
		for _, metric := range taskmanagerMetrics.Metrics {

			switch metric.ID {
			case "Status.JVM.GarbageCollector.G1_Young_Generation.Count":
				_ = s.mb.RecordFlinkTaskmanagerStatusJvmGarbageCollectorCollectionCountDataPoint(now, metric.Value, taskmanagerMetrics.Host, taskmanagerMetrics.TaskmanagerID, "G1_Young_Generation")
			case "Status.JVM.GarbageCollector.G1_Old_Generation.Count":
				_ = s.mb.RecordFlinkTaskmanagerStatusJvmGarbageCollectorCollectionCountDataPoint(now, metric.Value, taskmanagerMetrics.Host, taskmanagerMetrics.TaskmanagerID, "G1_Old_Generation")
			case "Status.JVM.GarbageCollector.G1_Old_Generation.Time":
				_ = s.mb.RecordFlinkTaskmanagerStatusJvmGarbageCollectorCollectionTimeDataPoint(now, metric.Value, taskmanagerMetrics.Host, taskmanagerMetrics.TaskmanagerID, "G1_Old_Generation")
			case "Status.JVM.GarbageCollector.G1_Young_Generation.Time":
				_ = s.mb.RecordFlinkTaskmanagerStatusJvmGarbageCollectorCollectionTimeDataPoint(now, metric.Value, taskmanagerMetrics.Host, taskmanagerMetrics.TaskmanagerID, "G1_Young_Generation")
			case "Status.JVM.CPU.Load":
				_ = s.mb.RecordFlinkTaskmanagerStatusJvmCPULoadDataPoint(now, metric.Value, taskmanagerMetrics.Host, taskmanagerMetrics.TaskmanagerID)
			case "Status.Flink.Memory.Managed.Used":
				_ = s.mb.RecordFlinkTaskmanagerStatusFlinkMemoryManagedUsedDataPoint(now, metric.Value, taskmanagerMetrics.Host, taskmanagerMetrics.TaskmanagerID)
			case "Status.Flink.Memory.Managed.Total":
				_ = s.mb.RecordFlinkTaskmanagerStatusFlinkMemoryManagedTotalDataPoint(now, metric.Value, taskmanagerMetrics.Host, taskmanagerMetrics.TaskmanagerID)
			case "Status.JVM.Memory.Mapped.TotalCapacity":
				_ = s.mb.RecordFlinkTaskmanagerStatusJvmMemoryMappedTotalCapacityDataPoint(now, metric.Value, taskmanagerMetrics.Host, taskmanagerMetrics.TaskmanagerID)
			case "Status.JVM.Memory.Mapped.MemoryUsed":
				_ = s.mb.RecordFlinkTaskmanagerStatusJvmMemoryMappedUsedDataPoint(now, metric.Value, taskmanagerMetrics.Host, taskmanagerMetrics.TaskmanagerID)
			case "Status.JVM.CPU.Time":
				_ = s.mb.RecordFlinkTaskmanagerStatusJvmCPUTimeDataPoint(now, metric.Value, taskmanagerMetrics.Host, taskmanagerMetrics.TaskmanagerID)
			case "Status.JVM.Threads.Count":
				_ = s.mb.RecordFlinkTaskmanagerStatusJvmThreadsCountDataPoint(now, metric.Value, taskmanagerMetrics.Host, taskmanagerMetrics.TaskmanagerID)
			case "Status.JVM.Memory.Heap.Committed":
				_ = s.mb.RecordFlinkTaskmanagerStatusJvmMemoryHeapCommittedDataPoint(now, metric.Value, taskmanagerMetrics.Host, taskmanagerMetrics.TaskmanagerID)
			case "Status.JVM.Memory.Metaspace.Committed":
				_ = s.mb.RecordFlinkTaskmanagerStatusJvmMemoryMetaspaceCommittedDataPoint(now, metric.Value, taskmanagerMetrics.Host, taskmanagerMetrics.TaskmanagerID)
			case "Status.JVM.Memory.NonHeap.Max":
				_ = s.mb.RecordFlinkTaskmanagerStatusJvmMemoryNonHeapMaxDataPoint(now, metric.Value, taskmanagerMetrics.Host, taskmanagerMetrics.TaskmanagerID)
			case "Status.JVM.Memory.NonHeap.Committed":
				_ = s.mb.RecordFlinkTaskmanagerStatusJvmMemoryNonHeapCommittedDataPoint(now, metric.Value, taskmanagerMetrics.Host, taskmanagerMetrics.TaskmanagerID)
			case "Status.JVM.Memory.NonHeap.Used":
				_ = s.mb.RecordFlinkTaskmanagerStatusJvmMemoryNonHeapUsedDataPoint(now, metric.Value, taskmanagerMetrics.Host, taskmanagerMetrics.TaskmanagerID)
			case "Status.JVM.Memory.Metaspace.Max":
				_ = s.mb.RecordFlinkTaskmanagerStatusJvmMemoryMetaspaceMaxDataPoint(now, metric.Value, taskmanagerMetrics.Host, taskmanagerMetrics.TaskmanagerID)
			case "Status.JVM.Memory.Direct.MemoryUsed":
				_ = s.mb.RecordFlinkTaskmanagerStatusJvmMemoryDirectUsedDataPoint(now, metric.Value, taskmanagerMetrics.Host, taskmanagerMetrics.TaskmanagerID)
			case "Status.JVM.Memory.Direct.TotalCapacity":
				_ = s.mb.RecordFlinkTaskmanagerStatusJvmMemoryDirectTotalCapacityDataPoint(now, metric.Value, taskmanagerMetrics.Host, taskmanagerMetrics.TaskmanagerID)
			case "Status.JVM.ClassLoader.ClassesLoaded":
				_ = s.mb.RecordFlinkTaskmanagerStatusJvmClassLoaderClassesLoadedDataPoint(now, metric.Value, taskmanagerMetrics.Host, taskmanagerMetrics.TaskmanagerID)
			case "Status.JVM.Memory.Metaspace.Used":
				_ = s.mb.RecordFlinkTaskmanagerStatusJvmMemoryMetaspaceUsedDataPoint(now, metric.Value, taskmanagerMetrics.Host, taskmanagerMetrics.TaskmanagerID)
			case "Status.JVM.Memory.Heap.Max":
				_ = s.mb.RecordFlinkTaskmanagerStatusJvmMemoryHeapMaxDataPoint(now, metric.Value, taskmanagerMetrics.Host, taskmanagerMetrics.TaskmanagerID)
			case "Status.JVM.Memory.Heap.Used":
				_ = s.mb.RecordFlinkTaskmanagerStatusJvmMemoryHeapUsedDataPoint(now, metric.Value, taskmanagerMetrics.Host, taskmanagerMetrics.TaskmanagerID)
			}
		}
	}
}

func (s *flinkmetricsScraper) ProcessJobsMetrics(now pcommon.Timestamp, jobsMetricsInstances []*models.JobMetrics) {
	for _, jobsMetrics := range jobsMetricsInstances {
		for _, metric := range jobsMetrics.Metrics {
			switch metric.ID {
			case "numRestarts":
				_ = s.mb.RecordFlinkJobRestartCountDataPoint(now, metric.Value, jobsMetrics.Host, jobsMetrics.JobName)
			case "lastCheckpointSize":
				_ = s.mb.RecordFlinkJobLastCheckpointSizeDataPoint(now, metric.Value, jobsMetrics.Host, jobsMetrics.JobName)
			case "lastCheckpointDuration":
				_ = s.mb.RecordFlinkJobLastCheckpointTimeDataPoint(now, metric.Value, jobsMetrics.Host, jobsMetrics.JobName)
			case "numberOfInProgressCheckpoints":
				_ = s.mb.RecordFlinkJobCheckpointsCountDataPoint(now, metric.Value, jobsMetrics.Host, jobsMetrics.JobName, metadata.AttributeCheckpointInProgress)
			case "numberOfCompletedCheckpoints":
				_ = s.mb.RecordFlinkJobCheckpointsCountDataPoint(now, metric.Value, jobsMetrics.Host, jobsMetrics.JobName, metadata.AttributeCheckpointCompleted)
			case "numberOfFailedCheckpoints":
				_ = s.mb.RecordFlinkJobCheckpointsCountDataPoint(now, metric.Value, jobsMetrics.Host, jobsMetrics.JobName, metadata.AttributeCheckpointFailed)
			}
		}
	}
}

func (s *flinkmetricsScraper) ProcessSubtaskMetrics(now pcommon.Timestamp, subtaskMetricsInstances []*models.SubtaskMetrics) {
	for _, subtaskMetrics := range subtaskMetricsInstances {
		for _, metric := range subtaskMetrics.Metrics {
			switch {
			// record task metrics
			case metric.ID == "numRecordsIn":
				_ = s.mb.RecordFlinkTaskRecordCountDataPoint(now, metric.Value, subtaskMetrics.Host, subtaskMetrics.TaskmanagerID, subtaskMetrics.JobName, subtaskMetrics.TaskName, subtaskMetrics.SubtaskIndex, metadata.AttributeRecordIn)
			case metric.ID == "numRecordsOut":
				_ = s.mb.RecordFlinkTaskRecordCountDataPoint(now, metric.Value, subtaskMetrics.Host, subtaskMetrics.TaskmanagerID, subtaskMetrics.JobName, subtaskMetrics.TaskName, subtaskMetrics.SubtaskIndex, metadata.AttributeRecordOut)
			case metric.ID == "numLateRecordsDropped":
				_ = s.mb.RecordFlinkTaskRecordCountDataPoint(now, metric.Value, subtaskMetrics.Host, subtaskMetrics.TaskmanagerID, subtaskMetrics.JobName, subtaskMetrics.TaskName, subtaskMetrics.SubtaskIndex, metadata.AttributeRecordLateRecordsDropped)
			// record operator metrics
			case strings.Contains(metric.ID, ".numRecordsIn"):
				fields := strings.Split(metric.ID, ".numRecordsIn")
				_ = s.mb.RecordFlinkOperatorRecordCountDataPoint(now, metric.Value, subtaskMetrics.Host, subtaskMetrics.TaskmanagerID, subtaskMetrics.JobName, fields[0], subtaskMetrics.SubtaskIndex, metadata.AttributeRecordIn)
			case strings.Contains(metric.ID, ".numRecordsOut"):
				fields := strings.Split(metric.ID, ".numRecordsOut")
				_ = s.mb.RecordFlinkOperatorRecordCountDataPoint(now, metric.Value, subtaskMetrics.Host, subtaskMetrics.TaskmanagerID, subtaskMetrics.JobName, fields[0], subtaskMetrics.SubtaskIndex, metadata.AttributeRecordOut)
			case strings.Contains(metric.ID, ".numLateRecordsDropped"):
				fields := strings.Split(metric.ID, ".numLateRecordsDropped")
				_ = s.mb.RecordFlinkOperatorRecordCountDataPoint(now, metric.Value, subtaskMetrics.Host, subtaskMetrics.TaskmanagerID, subtaskMetrics.JobName, fields[0], subtaskMetrics.SubtaskIndex, metadata.AttributeRecordLateRecordsDropped)
			case strings.Contains(metric.ID, ".currentOutputWatermark"):
				fields := strings.Split(metric.ID, ".currentOutputWatermark")
				_ = s.mb.RecordFlinkOperatorWatermarkOutputDataPoint(now, metric.Value, subtaskMetrics.Host, subtaskMetrics.TaskmanagerID, subtaskMetrics.JobName, fields[0], subtaskMetrics.SubtaskIndex)
			}
		}
	}
}
