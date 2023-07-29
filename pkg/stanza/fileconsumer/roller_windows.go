// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows
// +build windows

package fileconsumer // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer"

import "context"

type closeImmediately struct{}

func newRoller() roller {
	return &closeImmediately{}
}

func (r *closeImmediately) readLostFiles(ctx context.Context, readers []*Reader) {
	return
}

func (r *closeImmediately) roll(_ context.Context, readers []*Reader) {
	for _, reader := range readers {
		reader.Close()
	}
}

func (r *closeImmediately) cleanup() {
	return
}
