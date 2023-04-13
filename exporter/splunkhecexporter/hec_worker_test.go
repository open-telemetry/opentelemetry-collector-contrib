// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splunkhecexporter

import (
	"context"
	"errors"
)

var errHecSendFailed = errors.New("hec send failed")

type mockHecWorker struct {
	failSend bool
}

func (m *mockHecWorker) send(_ context.Context, _ buffer, _ map[string]string, _ bool) error {
	if m.failSend {
		return errHecSendFailed
	}
	return nil
}

var _ hecWorker = &mockHecWorker{}
