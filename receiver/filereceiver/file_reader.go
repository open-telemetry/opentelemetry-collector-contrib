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
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

// stringReader is the only function we use from *bufio.Reader. We define it
// so that it can be swapped out for testing.
type stringReader interface {
	ReadString(delim byte) (string, error)
}

// fileReader
type fileReader struct {
	logger       *zap.Logger
	stringReader stringReader
	unm          pmetric.Unmarshaler
	consumer     consumer.Metrics
}

func newFileReader(logger *zap.Logger, consumer consumer.Metrics, file *os.File) fileReader {
	return fileReader{
		logger:       logger,
		consumer:     consumer,
		stringReader: bufio.NewReader(file),
		unm:          &pmetric.JSONUnmarshaler{},
	}
}

// readAll calls readline for each line in the file until all lines have been
// read or the context is cancelled.
func (fr fileReader) readAll(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			err := fr.readLine(ctx)
			if err != nil {
				if errors.Is(err, io.EOF) {
					fr.logger.Debug("EOF reached")
					return
				}
				fr.logger.Error("error reading line", zap.Error(err))
				return
			}
		}
	}
}

// readLine reads the next line in the file, converting it into metrics and
// passing it to the the consumer member.
func (fr fileReader) readLine(ctx context.Context) error {
	line, err := fr.stringReader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read line from input file: %w", err)
	}
	metrics, err := fr.unm.UnmarshalMetrics([]byte(line))
	if err != nil {
		return fmt.Errorf("failed to unmarshal metrics: %w", err)
	}
	return fr.consumer.ConsumeMetrics(ctx, metrics)
}
