// Copyright OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package hostmetadata

import (
	"sync"

	"go.opentelemetry.io/collector/consumer/pdata"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/dimensions"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/splunk"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/collection"
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

func (s *Syncer) Sync(md pdata.Metrics) {
	// skip if already synced or if metrics data is empty
	if md.ResourceMetrics().Len() == 0 {
		return
	}
	s.once.Do(func() {
		s.syncOnResource(md.ResourceMetrics().At(0).Resource())
	})
}

func (s *Syncer) syncOnResource(res pdata.Resource) {
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
	err := s.dimClient.PushMetadata([]*collection.MetadataUpdate{metadataUpdate})
	if err != nil {
		s.logger.Error("Failed to push host metadata update", zap.Error(err))
		return
	}

	s.logger.Info("Host metadata synchronized")
}

func (s *Syncer) prepareMetadataUpdate(props map[string]string, hostID splunk.HostID) *collection.MetadataUpdate {
	return &collection.MetadataUpdate{
		ResourceIDKey: string(hostID.Key),
		ResourceID:    collection.ResourceID(hostID.ID),
		MetadataDelta: collection.MetadataDelta{
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
