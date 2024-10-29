// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package testhelpers // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/status/testhelpers"

import (
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/pipeline"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/status"
)

// PipelineMetadata groups together component and instance IDs for a hypothetical pipeline used
// for testing purposes.
type PipelineMetadata struct {
	PipelineID  pipeline.ID
	ReceiverID  *componentstatus.InstanceID
	ProcessorID *componentstatus.InstanceID
	ExporterID  *componentstatus.InstanceID
}

// InstanceIDs returns a slice of instanceIDs for components within the hypothetical pipeline.
func (p *PipelineMetadata) InstanceIDs() []*componentstatus.InstanceID {
	return []*componentstatus.InstanceID{p.ReceiverID, p.ProcessorID, p.ExporterID}
}

// NewPipelineMetadata returns a metadata for a hypothetical pipeline.
func NewPipelineMetadata(typestr string) *PipelineMetadata {
	pipelineID := pipeline.MustNewID(typestr)
	return &PipelineMetadata{
		PipelineID:  pipelineID,
		ReceiverID:  componentstatus.NewInstanceID(component.NewIDWithName(component.MustNewType(typestr), "in"), component.KindReceiver).WithPipelines(pipelineID),
		ProcessorID: componentstatus.NewInstanceID(component.MustNewID("batch"), component.KindProcessor).WithPipelines(pipelineID),
		ExporterID:  componentstatus.NewInstanceID(component.NewIDWithName(component.MustNewType(typestr), "out"), component.KindExporter).WithPipelines(pipelineID),
	}
}

// NewPipelines returns a map of hypothetical pipelines identified by their stringified typeVal.
func NewPipelines(typestrs ...string) map[string]*PipelineMetadata {
	result := make(map[string]*PipelineMetadata, len(typestrs))
	for _, typestr := range typestrs {
		result[typestr] = NewPipelineMetadata(typestr)
	}
	return result
}

// SeedAggregator records a status event for each instanceID.
func SeedAggregator(
	agg *status.Aggregator,
	instanceIDs []*componentstatus.InstanceID,
	statuses ...componentstatus.Status,
) {
	for _, st := range statuses {
		for _, id := range instanceIDs {
			agg.RecordStatus(id, componentstatus.NewEvent(st))
		}
	}
}
