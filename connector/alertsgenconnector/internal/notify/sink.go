// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package notify

type Sink interface { Send(event map[string]any) error }

type Nop struct{}

func (Nop) Send(_ map[string]any) error { return nil }
