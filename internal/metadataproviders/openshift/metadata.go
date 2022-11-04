// Copyright The OpenTelemetry Authors
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

package openshift // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders/openshift"

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	conventions "go.opentelemetry.io/collector/semconv/v1.16.0"
)

// Provider gets cluster metadata from Openshift.
type Provider interface {
	K8SClusterVersion(context.Context) (string, error)
	OpenShiftClusterVersion(context.Context) (string, error)
	Infrastructure(context.Context) (*InfrastructureMetadata, error)
}

// NewProvider creates a new metadata provider.
func NewProvider(address, token string) Provider {
	return &openshiftProvider{
		address: address,
		token:   token,
		client:  &http.Client{},
	}
}

type openshiftProvider struct {
	client  *http.Client
	address string
	token   string
}

func (o *openshiftProvider) makeOCPRequest(ctx context.Context, endpoint, target string) (*http.Request, error) {
	addr := fmt.Sprintf("%s/apis/config.openshift.io/v1/%s/%s/status", o.address, endpoint, target)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, addr, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", o.token))
	return req, nil
}

// OpenShiftClusterVersion requests the ClusterVersion from the openshift api.
func (o *openshiftProvider) OpenShiftClusterVersion(ctx context.Context) (string, error) {
	req, err := o.makeOCPRequest(ctx, "clusterversions", "version")
	if err != nil {
		return "", err
	}
	resp, err := o.client.Do(req)
	if err != nil {
		return "", err
	}
	res := ocpClusterVersionAPIResponse{}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "", err
	}
	return res.Status.Desired.Version, nil
}

// ClusterVersion requests Infrastructure details from the openshift api.
func (o *openshiftProvider) Infrastructure(ctx context.Context) (*InfrastructureMetadata, error) {
	req, err := o.makeOCPRequest(ctx, "infrastructures", "cluster")
	if err != nil {
		return nil, err
	}
	resp, err := o.client.Do(req)
	if err != nil {
		return nil, err
	}
	res := infrastructureAPIResponse{}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}

	var (
		region   string
		platform string
		provider string
	)

	switch strings.ToLower(res.Status.PlatformStatus.Type) {
	case "aws":
		provider = conventions.AttributeCloudProviderAWS
		platform = conventions.AttributeCloudPlatformAWSOpenshift
		region = strings.ToLower(res.Status.PlatformStatus.Aws.Region)
	case "azure":
		provider = conventions.AttributeCloudProviderAzure
		platform = conventions.AttributeCloudPlatformAzureOpenshift
		region = strings.ToLower(res.Status.PlatformStatus.Azure.CloudName)
	case "gcp":
		provider = conventions.AttributeCloudProviderGCP
		platform = conventions.AttributeCloudPlatformGoogleCloudOpenshift
		region = strings.ToLower(res.Status.PlatformStatus.GCP.Region)
	case "ibmcloud":
		provider = conventions.AttributeCloudProviderIbmCloud
		platform = conventions.AttributeCloudPlatformIbmCloudOpenshift
		region = strings.ToLower(res.Status.PlatformStatus.IBMCloud.Location)
	case "openstack":
		region = strings.ToLower(res.Status.PlatformStatus.OpenStack.CloudName)
	}

	return &InfrastructureMetadata{
		Name:     res.Status.InfrastructureName,
		Topology: res.Status.ControlPlaneTopology,
		Provider: provider,
		Platform: platform,
		Region:   region,
	}, nil
}

// K8SClusterVersion requests the ClusterVersion from the kubernetes api.
func (o *openshiftProvider) K8SClusterVersion(ctx context.Context) (string, error) {
	addr := fmt.Sprintf("%s/version", o.address)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, addr, nil)
	if err != nil {
		return "", err
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", o.token))

	resp, err := o.client.Do(req)
	if err != nil {
		return "", err
	}
	res := k8sClusterVersionAPIResponse{}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "", err
	}
	version := res.GitVersion
	if strings.Contains(version, "+") {
		version = strings.Split(version, "+")[0]
	}
	return version, nil
}

// InfrastructureMetadata bundles the most important Infrastructure details.
type InfrastructureMetadata struct {
	Name     string `json:"name"`
	Region   string `json:"region"`
	Platform string `json:"platform"`
	Provider string `json:"provider"`
	Topology string `json:"topology"`
}

type ocpClusterVersionAPIResponse struct {
	Status struct {
		Desired struct {
			Version string `json:"version"`
		} `json:"desired"`
	} `json:"status"`
}

type infrastructureAPIResponse struct {
	Status struct {
		ControlPlaneTopology   string `json:"controlPlaneTopology"`
		InfrastructureName     string `json:"infrastructureName"`
		InfrastructureTopology string `json:"infrastructureTopology"`
		Platform               string `json:"platform"`
		PlatformStatus         struct {
			Aws struct {
				Region string `json:"region"`
			} `json:"aws"`
			Azure struct {
				CloudName string `json:"cloudName"`
			} `json:"azure"`
			Baremetal struct{} `json:"baremetal"`
			GCP       struct {
				Region string `json:"region"`
			} `json:"gcp"`
			IBMCloud struct {
				Location string `json:"location"`
			} `json:"ibmcloud"`
			OpenStack struct {
				CloudName string `json:"cloudName"`
			} `json:"openstack"`
			OVirt   struct{} `json:"ovirt"`
			VSphere struct{} `json:"vsphere"`
			Type    string   `json:"type"`
		} `json:"platformStatus"`
	} `json:"status"`
}

type k8sClusterVersionAPIResponse struct {
	GitVersion string `json:"gitVersion"`
}
