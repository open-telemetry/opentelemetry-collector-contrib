// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"

import (
	"context"
	"fmt"
	"net/http"
)

type contextKey int

const clientContextKey contextKey = iota

// ContextWithClient returns a new context.Context with the provided *http.Client stored as a value.
func ContextWithClient(ctx context.Context, client *http.Client) context.Context {
	return context.WithValue(ctx, clientContextKey, client)
}

// ClientFromContext attempts to extract an *http.Client from the provided context.Context.
func ClientFromContext(ctx context.Context) (*http.Client, error) {
	v := ctx.Value(clientContextKey)
	if v == nil {
		return nil, fmt.Errorf("no http.Client in context")
	}
	var c *http.Client
	var ok bool
	if c, ok = v.(*http.Client); !ok {
		return nil, fmt.Errorf("invalid value found in context")
	}

	return c, nil
}
