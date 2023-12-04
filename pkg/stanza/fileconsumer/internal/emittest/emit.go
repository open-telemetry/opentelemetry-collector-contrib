// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package emittest

import (
	"context"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/emit"
)

type Call struct {
	Token      []byte
	Attributes map[string]any
}

func CallChanFunc(emitChan chan *Call) emit.Callback {
	return func(_ context.Context, token []byte, attrs map[string]any) error {
		copied := make([]byte, len(token))
		copy(copied, token)
		emitChan <- &Call{
			Token:      copied,
			Attributes: attrs,
		}
		return nil
	}
}

func TokenChanFunc(received chan []byte) emit.Callback {
	return func(_ context.Context, token []byte, _ map[string]any) error {
		received <- token
		return nil
	}
}

func NopFunc(_ context.Context, _ []byte, _ map[string]any) error {
	return nil
}
