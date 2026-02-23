// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pdatautil // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/routingconnector/internal/pdatautil"

// OnceValue to set a given value only once.
type OnceValue[K any] struct {
	val    K
	isInit bool
}

func (ov *OnceValue[K]) IsInit() bool {
	return ov.isInit
}

func (ov *OnceValue[K]) Init(val K) {
	ov.isInit = true
	ov.val = val
}

func (ov *OnceValue[K]) Value() K {
	return ov.val
}
