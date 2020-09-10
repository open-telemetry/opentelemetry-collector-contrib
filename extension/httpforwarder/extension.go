// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package httpforwarder

import (
	"context"

	"go.opentelemetry.io/collector/component"
)

type httpForwarder struct {
}

var _ component.ServiceExtension = (*httpForwarder)(nil)

func (h httpForwarder) Start(_ context.Context, _ component.Host) error {
	// TODO: Start HTTP server
	return nil
}

func (h httpForwarder) Shutdown(_ context.Context) error {
	// TODO: Shutdown HTTP server
	return nil
}
