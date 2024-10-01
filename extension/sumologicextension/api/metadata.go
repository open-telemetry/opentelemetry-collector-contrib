// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package api // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/sumologicextension/api"

type OpenMetadataHostDetails struct {
	Name        string `json:"name"`
	OsName      string `json:"osName"`
	OsVersion   string `json:"osVersion"`
	Environment string `json:"environment"`
}

type OpenMetadataCollectorDetails struct {
	RunningVersion string `json:"runningVersion"`
}

type OpenMetadataNetworkDetails struct {
	HostIPAddress string `json:"hostIpAddress"`
}

type OpenMetadataRequestPayload struct {
	HostDetails      OpenMetadataHostDetails      `json:"hostDetails"`
	CollectorDetails OpenMetadataCollectorDetails `json:"collectorDetails"`
	NetworkDetails   OpenMetadataNetworkDetails   `json:"networkDetails"`
	TagDetails       map[string]any               `json:"tagDetails"`
}
