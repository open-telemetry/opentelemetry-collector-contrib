// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sclient // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/k8s/k8sclient"

type endpointInfo struct {
	name       string // service name
	namespace  string // namespace name
	podKeyList []string
}
