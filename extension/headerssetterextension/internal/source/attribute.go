// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package source // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/headerssetterextension/internal/source"

import (
	"context"
	"encoding/json"

	"go.opentelemetry.io/collector/client"
)

var _ Source = (*ContextSource)(nil)

type AttributeSource struct {
	Key          string
	DefaultValue string
}

func (ts *AttributeSource) Get(ctx context.Context) (string, error) {
	cl := client.FromContext(ctx)
	attr := cl.Auth.GetAttribute(ts.Key)

	switch a := attr.(type) {
	case string:
		return a, nil
	case nil:
		return "", nil
	default:
		b, err := json.Marshal(attr)
		if err != nil {
			return "", err
		}
		return string(b), nil
	}
}
