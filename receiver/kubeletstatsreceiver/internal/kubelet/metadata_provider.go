// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kubelet // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/internal/kubelet"

import (
	"encoding/json"

	v1 "k8s.io/api/core/v1"
)

// MetadataProvider wraps a RestClient, returning an unmarshaled metadata.
type MetadataProvider struct {
	rc RestClient
}

func NewMetadataProvider(rc RestClient) *MetadataProvider {
	return &MetadataProvider{rc: rc}
}

// Pods calls the /pods endpoint and unmarshals the
// results into a v1.PodList struct.
func (p *MetadataProvider) Pods() (*v1.PodList, error) {
	pods, err := p.rc.Pods()
	if err != nil {
		return nil, err
	}
	var out v1.PodList
	err = json.Unmarshal(pods, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}
