// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package hostmetadata // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/internal/hostmetadata"

import (
	"sync"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/internal/dimensions"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk"
	metadata "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/experimentalmetricmetadata"
)

// Syncer is a config structure for host metadata syncer.
type Syncer struct {
	logger    *zap.Logger
	dimClient dimensions.MetadataUpdateClient
	once      sync.Once
}

// NewSyncer creates new instance of host metadata syncer.
func NewSyncer(logger *zap.Logger, dimClient dimensions.MetadataUpdateClient) *Syncer {
	return &Syncer{
		logger:    logger,
		dimClient: dimClient,
	}
}

func (s *Syncer) Sync(md pmetric.Metrics) {
	// skip if already synced or if metrics data is empty
	if md.ResourceMetrics().Len() == 0 {
		return
	}
	s.once.Do(func() {
		s.syncOnResource(md.ResourceMetrics().At(0).Resource())
	})
}

func (s *Syncer) syncOnResource(res pcommon.Resource) {
	// If resourcedetection processor is enabled, all the metrics should have resource attributes
	// that can be used to update host metadata.
	// Based of this assumption we check just one ResourceMetrics object,
	hostID, ok := splunk.ResourceToHostID(res)
	if !ok {
		// if no attributes found, we assume that resourcedetection is not enabled or
		// it doesn't set right attributes, and we do not retry.
		s.logger.Error("Not found any host attributes. Host metadata synchronization skipped. " +
			"Make sure that \"resourcedetection\" processor is enabled in the pipeline with one of " +
			"the cloud provider detectors or environment variable detector setting \"host.name\" attribute")
		return
	}

	props := s.scrapeHostProperties()
	if len(props) == 0 {
		// do not retry if scraping failed.
		s.logger.Error("Failed to fetch system properties. Host metadata synchronization skipped")
		return
	}

	metadataUpdate := s.prepareMetadataUpdate(props, hostID)
	s.logger.Info("Preparing to sync host properties to host dimension", zap.String("dimension_key", string(hostID.Key)),
		zap.String("dimension_value", hostID.ID), zap.Any("properties", props))
	err := s.dimClient.PushMetadata([]*metadata.MetadataUpdate{metadataUpdate})
	if err != nil {
		s.logger.Error("Failed to push host metadata update", zap.Error(err))
		return
	}

	s.logger.Info("Host metadata synchronized")
}

func (s *Syncer) prepareMetadataUpdate(props map[string]string, hostID splunk.HostID) *metadata.MetadataUpdate {
	return &metadata.MetadataUpdate{
		ResourceIDKey: string(hostID.Key),
		ResourceID:    metadata.ResourceID(hostID.ID),
		MetadataDelta: metadata.MetadataDelta{
			MetadataToUpdate: props,
		},
	}
}

func (s *Syncer) scrapeHostProperties() map[string]string {
	props := make(map[string]string)

	cpu, err := getCPU()
	if err == nil {
		for k, v := range cpu.toStringMap() {
			props[k] = v
		}
	} else {
		s.logger.Warn("Failed to scrape host hostCPU metadata", zap.Error(err))
	}

	mem, err := getMemory()
	if err == nil {
		for k, v := range mem.toStringMap() {
			props[k] = v
		}
	} else {
		s.logger.Warn("Failed to scrape host memory metadata", zap.Error(err))
	}

	os, err := getOS()
	if err == nil {
		for k, v := range os.toStringMap() {
			props[k] = v
		}
	} else {
		s.logger.Warn("Failed to scrape host hostOS metadata", zap.Error(err))
	}

	return props
}
