// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sclient // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/k8s/k8sclient"

type replicaSetInfo struct {
	name   string
	owners []*replicaSetOwner
}

type replicaSetOwner struct {
	kind string
	name string
}
