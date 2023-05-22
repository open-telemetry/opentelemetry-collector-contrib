// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kubelet // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/internal/kubelet"

import (
	"encoding/json"

	stats "k8s.io/kubelet/pkg/apis/stats/v1alpha1"
)

// StatsProvider wraps a RestClient, returning an unmarshaled
// stats.Summary struct from the kubelet API.
type StatsProvider struct {
	rc RestClient
}

func NewStatsProvider(rc RestClient) *StatsProvider {
	return &StatsProvider{rc: rc}
}

// StatsSummary calls the /stats/summary kubelet endpoint and unmarshals the
// results into a stats.Summary struct.
func (p *StatsProvider) StatsSummary() (*stats.Summary, error) {
	summary, err := p.rc.StatsSummary()
	if err != nil {
		return nil, err
	}
	var out stats.Summary
	err = json.Unmarshal(summary, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}
