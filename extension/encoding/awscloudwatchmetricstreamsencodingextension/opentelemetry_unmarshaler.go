// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awscloudwatchmetricstreamsencodingextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awscloudwatchmetricstreamsencodingextension"

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awscloudwatchmetricstreamsencodingextension/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/xstreamencoding"
)

var errInvalidUvarint = errors.New("invalid OTLP message length: failed to decode varint")

type formatOpenTelemetry10Unmarshaler struct {
	buildInfo component.BuildInfo
}

var _ pmetric.Unmarshaler = (*formatOpenTelemetry10Unmarshaler)(nil)

func (f *formatOpenTelemetry10Unmarshaler) UnmarshalMetrics(record []byte) (pmetric.Metrics, error) {
	// Decode as a stream but flush all at once using flush options
	decoder, err := f.NewMetricsDecoder(bytes.NewReader(record), encoding.WithFlushBytes(0), encoding.WithFlushItems(0))
	if err != nil {
		return pmetric.Metrics{}, err
	}

	metrics, err := decoder.DecodeMetrics()
	if err != nil {
		//nolint:errorlint
		if err == io.EOF {
			// EOF indicates no metrics were found, return any metrics that's available
			return metrics, nil
		}

		return pmetric.Metrics{}, err
	}

	return metrics, nil
}

func (f *formatOpenTelemetry10Unmarshaler) NewMetricsDecoder(reader io.Reader, options ...encoding.DecoderOption) (encoding.MetricsDecoder, error) {
	var bufReader *bufio.Reader
	if r, ok := reader.(*bufio.Reader); ok {
		bufReader = r
	} else {
		bufReader = bufio.NewReader(reader)
	}

	batchHelper := xstreamencoding.NewBatchHelper(options...)
	offSetTracker := batchHelper.Options().Offset

	if offSetTracker > 0 {
		_, err := bufReader.Discard(int(offSetTracker))
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil, fmt.Errorf("EOF reached before offset %d bytes were discarded", offSetTracker)
			}
			return nil, err
		}
	}

	offsetF := func() int64 {
		return offSetTracker
	}

	decodeF := func() (pmetric.Metrics, error) {
		md := pmetric.NewMetrics()

		var buf []byte
		for {
			peek, err := bufReader.Peek(binary.MaxVarintLen64)
			if err != nil && !errors.Is(err, io.EOF) {
				return pmetric.Metrics{}, err
			}

			if len(peek) == 0 {
				// reached EOF
				break
			}

			// Read next toRead bytes to get the length of the next OTLP metric message
			toRead, bytesRead := binary.Uvarint(peek)
			if bytesRead <= 0 {
				return pmetric.Metrics{}, errInvalidUvarint
			}

			// Skip bytesRead
			_, err = bufReader.Discard(bytesRead)
			if err != nil {
				return pmetric.Metrics{}, fmt.Errorf("unable to discard varint: %w", err)
			}

			// Reuse buffer, grow only if needed
			if cap(buf) < int(toRead) {
				buf = make([]byte, toRead)
			} else {
				buf = buf[:toRead]
			}

			// Read the OTLP metric message
			_, err = io.ReadFull(bufReader, buf)
			if err != nil {
				return pmetric.Metrics{}, fmt.Errorf("unable to read OTLP metric message: %w", err)
			}

			// unmarshal metric
			req := pmetricotlp.NewExportRequest()
			if err := req.UnmarshalProto(buf); err != nil {
				return pmetric.Metrics{}, fmt.Errorf("unable to unmarshal input: %w", err)
			}

			// add scope name and build info version to the resource metrics
			for i := 0; i < req.Metrics().ResourceMetrics().Len(); i++ {
				rm := req.Metrics().ResourceMetrics().At(i)
				for j := 0; j < rm.ScopeMetrics().Len(); j++ {
					sm := rm.ScopeMetrics().At(j)
					sm.Scope().SetName(metadata.ScopeName)
					sm.Scope().SetVersion(f.buildInfo.Version)
				}
			}
			req.Metrics().ResourceMetrics().MoveAndAppendTo(md.ResourceMetrics())

			advanced := int64(toRead) + int64(bytesRead)
			offSetTracker += advanced
			batchHelper.IncrementItems(1)
			batchHelper.IncrementBytes(advanced)

			if batchHelper.ShouldFlush() {
				batchHelper.Reset()

				return md, nil
			}
		}

		if md.ResourceMetrics().Len() == 0 {
			return pmetric.NewMetrics(), io.EOF
		}

		return md, nil
	}

	return xstreamencoding.NewMetricsDecoderAdapter(decodeF, offsetF), nil
}
