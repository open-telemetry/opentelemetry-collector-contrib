// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate mdatagen metadata.yaml

// Package k8snodemetadataprocessor implements an OTel metrics processor that
// enriches metrics with Kubernetes node metadata. It watches Node objects via
// a SharedInformer and adds node taints as resource attributes with the format
// k8s.node.taint.<key> = <value>.
package k8snodemetadataprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8snodemetadataprocessor"
