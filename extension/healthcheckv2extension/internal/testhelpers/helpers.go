// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package testhelpers // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckv2extension/internal/testhelpers"

import (
	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckv2extension/internal/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckv2extension/internal/status"
)

// PipelineMetadata groups together component and instance IDs for a hypothetical pipeline used
// for testing purposes.
type PipelineMetadata struct {
	PipelineID  component.ID
	ReceiverID  *component.InstanceID
	ProcessorID *component.InstanceID
	ExporterID  *component.InstanceID
}

// InstanceIDs returns a slice of instanceIDs for components within the hypothetical pipeline.
func (p *PipelineMetadata) InstanceIDs() []*component.InstanceID {
	return []*component.InstanceID{p.ReceiverID, p.ProcessorID, p.ExporterID}
}

// NewPipelineMetadata returns a metadata for a hypothetical pipeline.
func NewPipelineMetadata(typestr string) *PipelineMetadata {
	pipelineID := component.MustNewID(typestr)
	return &PipelineMetadata{
		PipelineID: pipelineID,
		ReceiverID: &component.InstanceID{
			ID:   component.NewIDWithName(component.MustNewType(typestr), "in"),
			Kind: component.KindReceiver,
			PipelineIDs: map[component.ID]struct{}{
				pipelineID: {},
			},
		},
		ProcessorID: &component.InstanceID{
			ID:   component.MustNewID("batch"),
			Kind: component.KindProcessor,
			PipelineIDs: map[component.ID]struct{}{
				pipelineID: {},
			},
		},
		ExporterID: &component.InstanceID{
			ID:   component.NewIDWithName(component.MustNewType(typestr), "out"),
			Kind: component.KindExporter,
			PipelineIDs: map[component.ID]struct{}{
				pipelineID: {},
			},
		},
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
	instanceIDs []*component.InstanceID,
	statuses ...component.Status,
) {
	for _, st := range statuses {
		for _, id := range instanceIDs {
			agg.RecordStatus(id, component.NewStatusEvent(st))
		}
	}
}

func ErrPriority(config *common.ComponentHealthConfig) status.ErrorPriority {
	if config != nil && config.IncludeRecoverable && !config.IncludePermanent {
		return status.PriorityRecoverable
	}
	return status.PriorityPermanent
}
