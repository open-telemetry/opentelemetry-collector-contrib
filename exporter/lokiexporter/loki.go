// Copyright The OpenTelemetry Authors
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

package lokiexporter

import (
	"context"
	"errors"
	"net/url"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/pdata"
)

type lokiExporter struct {
	pushLogData func(ctx context.Context, ld pdata.Logs) (numDroppedLogs int, err error)
	stop        func(ctx context.Context) (err error)
	start       func(ctx context.Context, host component.Host) (err error)
}

func newExporter(config *Config) (*lokiExporter, error) {
	if _, err := url.Parse(config.Endpoint); config.Endpoint == "" || err != nil {
		return nil, errors.New("endpoint must be a valid URL")
	}

	_, err := config.HTTPClientSettings.ToClient()
	if err != nil {
		return nil, err
	}

	if len(config.AttributesForLabels) == 0 {
		return nil, errors.New("attributes_for_labels must have a least one label")
	}

	return &lokiExporter{
		pushLogData: pushLogData,
		stop:        stop,
		start:       start,
	}, nil
}

// TODO: Implement in second PR
func pushLogData(ctx context.Context, ld pdata.Logs) (numDroppedLogs int, err error) {
	return 0, nil
}

// TODO: Implement in second PR
func start(ctx context.Context, host component.Host) (err error) {
	return nil
}

// TODO: Implement in second PR
func stop(ctx context.Context) (err error) {
	return nil
}
