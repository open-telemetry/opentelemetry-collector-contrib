// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate mdatagen metadata.yaml
//go:generate mdatagen internal/aws/ec2/metadata.yaml
//go:generate mdatagen internal/aws/ecs/metadata.yaml
//go:generate mdatagen internal/aws/eks/metadata.yaml
//go:generate mdatagen internal/aws/elasticbeanstalk/metadata.yaml
//go:generate mdatagen internal/aws/lambda/metadata.yaml
//go:generate mdatagen internal/azure/aks/metadata.yaml
//go:generate mdatagen internal/azure/metadata.yaml
//go:generate mdatagen internal/consul/metadata.yaml
//go:generate mdatagen internal/docker/metadata.yaml
//go:generate mdatagen internal/gcp/metadata.yaml
//go:generate mdatagen internal/heroku/metadata.yaml
//go:generate mdatagen internal/openshift/metadata.yaml
//go:generate mdatagen internal/system/metadata.yaml

// package resourcedetectionprocessor implements a processor
// which can be used to detect resource information from the host,
// in a format that conforms to the OpenTelemetry resource semantic conventions, and append or
// override the resource value in telemetry data with this information.
package resourcedetectionprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor"
