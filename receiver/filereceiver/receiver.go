// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package filereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/filereceiver"

import (
	"context"
	"fmt"
	"os"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.uber.org/zap"
)

type fileReceiver struct {
	consumer consumer.Metrics
	path     string
	logger   *zap.Logger
	cancel   context.CancelFunc
}

func (r *fileReceiver) Start(_ context.Context, _ component.Host) error {
	var ctx context.Context
	ctx, r.cancel = context.WithCancel(context.Background())

	file, err := os.Open(r.path)
	if err != nil {
		return fmt.Errorf("failed to open file %q: %w", r.path, err)
	}

	fr := newFileReader(r.logger, r.consumer, file)
	go fr.readAll(ctx)
	return nil
}

func (r *fileReceiver) Shutdown(ctx context.Context) error {
	if r.cancel != nil {
		r.cancel()
	}
	return nil
}
