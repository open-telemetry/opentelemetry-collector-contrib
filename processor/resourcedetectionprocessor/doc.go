// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate make mdatagen
//go:generate make mdatagen MDATAGEN_METADATA_YAML=internal/aws/ec2/metadata.yaml
//go:generate make mdatagen MDATAGEN_METADATA_YAML=internal/aws/ecs/metadata.yaml
//go:generate make mdatagen MDATAGEN_METADATA_YAML=internal/aws/eks/metadata.yaml
//go:generate make mdatagen MDATAGEN_METADATA_YAML=internal/aws/elasticbeanstalk/metadata.yaml
//go:generate make mdatagen MDATAGEN_METADATA_YAML=internal/aws/lambda/metadata.yaml
//go:generate make mdatagen MDATAGEN_METADATA_YAML=internal/azure/aks/metadata.yaml
//go:generate make mdatagen MDATAGEN_METADATA_YAML=internal/azure/metadata.yaml
//go:generate make mdatagen MDATAGEN_METADATA_YAML=internal/consul/metadata.yaml
//go:generate make mdatagen MDATAGEN_METADATA_YAML=internal/digitalocean/metadata.yaml
//go:generate make mdatagen MDATAGEN_METADATA_YAML=internal/docker/metadata.yaml
//go:generate make mdatagen MDATAGEN_METADATA_YAML=internal/gcp/metadata.yaml
//go:generate make mdatagen MDATAGEN_METADATA_YAML=internal/heroku/metadata.yaml
//go:generate make mdatagen MDATAGEN_METADATA_YAML=internal/hetzner/metadata.yaml
//go:generate make mdatagen MDATAGEN_METADATA_YAML=internal/openshift/metadata.yaml
//go:generate make mdatagen MDATAGEN_METADATA_YAML=internal/system/metadata.yaml
//go:generate make mdatagen MDATAGEN_METADATA_YAML=internal/k8snode/metadata.yaml
//go:generate make mdatagen MDATAGEN_METADATA_YAML=internal/kubeadm/metadata.yaml
//go:generate make mdatagen MDATAGEN_METADATA_YAML=internal/dynatrace/metadata.yaml
//go:generate make mdatagen MDATAGEN_METADATA_YAML=internal/akamai/metadata.yaml
//go:generate make mdatagen MDATAGEN_METADATA_YAML=internal/scaleway/metadata.yaml
//go:generate make mdatagen MDATAGEN_METADATA_YAML=internal/upcloud/metadata.yaml
//go:generate make mdatagen MDATAGEN_METADATA_YAML=internal/vultr/metadata.yaml

// package resourcedetectionprocessor implements a processor
// which can be used to detect resource information from the host,
// in a format that conforms to the OpenTelemetry resource semantic conventions, and append or
// override the resource value in telemetry data with this information.
package resourcedetectionprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor"
