// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusremotewriteexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusremotewriteexporter"

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/prometheus/prometheus/prompb"
	"github.com/tidwall/wal"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusremotewriteexporter/internal/metadata"
)

type prwWalTelemetry interface {
	recordWALWrites(ctx context.Context)
	recordWALWritesFailures(ctx context.Context)
}

type prwWalTelemetryOTel struct {
	telemetryBuilder *metadata.TelemetryBuilder
	otelAttrs        []attribute.KeyValue
}

func (p *prwWalTelemetryOTel) recordWALWrites(ctx context.Context) {
	p.telemetryBuilder.ExporterPrometheusremotewriteWalWrites.Add(ctx, 1, metric.WithAttributes(p.otelAttrs...))
}

func (p *prwWalTelemetryOTel) recordWALWritesFailures(ctx context.Context) {
	p.telemetryBuilder.ExporterPrometheusremotewriteWalWritesFailures.Add(ctx, 1, metric.WithAttributes(p.otelAttrs...))
}

func newPRWWalTelemetry(set exporter.Settings) (prwWalTelemetry, error) {
	telemetryBuilder, err := metadata.NewTelemetryBuilder(set.TelemetrySettings)
	if err != nil {
		return nil, err
	}
	return &prwWalTelemetryOTel{
		telemetryBuilder: telemetryBuilder,
		otelAttrs:        []attribute.KeyValue{},
	}, nil
}

type prweWAL struct {
	wg        sync.WaitGroup // wg waits for the go routines to finish.
	mu        sync.Mutex     // mu protects the fields below.
	wal       *wal.Log
	walConfig *WALConfig
	walPath   string

	exportSink func(ctx context.Context, reqL []*prompb.WriteRequest) error

	stopOnce  sync.Once
	stopChan  chan struct{}
	rNotify   chan struct{}
	rWALIndex *atomic.Uint64
	wWALIndex *atomic.Uint64

	telemetry prwWalTelemetry
}

const (
	defaultWALBufferSize        = 300
	defaultWALTruncateFrequency = 1 * time.Minute
)

type WALConfig struct {
	Directory         string        `mapstructure:"directory"`
	BufferSize        int           `mapstructure:"buffer_size"`
	TruncateFrequency time.Duration `mapstructure:"truncate_frequency"`
}

func (wc *WALConfig) bufferSize() int {
	if wc.BufferSize > 0 {
		return wc.BufferSize
	}
	return defaultWALBufferSize
}

func (wc *WALConfig) truncateFrequency() time.Duration {
	if wc.TruncateFrequency > 0 {
		return wc.TruncateFrequency
	}
	return defaultWALTruncateFrequency
}

func newWAL(walConfig *WALConfig, set exporter.Settings, exportSink func(context.Context, []*prompb.WriteRequest) error) (*prweWAL, error) {
	if walConfig == nil {
		// There are cases for which the WAL can be disabled.
		// TODO: Perhaps log that the WAL wasn't enabled.
		return nil, nil
	}

	telemetryPRWWal, err := newPRWWalTelemetry(set)
	if err != nil {
		return nil, err
	}

	return &prweWAL{
		exportSink: exportSink,
		walConfig:  walConfig,
		stopChan:   make(chan struct{}),
		rNotify:    make(chan struct{}),
		rWALIndex:  &atomic.Uint64{},
		wWALIndex:  &atomic.Uint64{},
		telemetry:  telemetryPRWWal,
	}, nil
}

func (wc *WALConfig) createWAL() (*wal.Log, string, error) {
	walPath := filepath.Join(wc.Directory, "prom_remotewrite")
	log, err := wal.Open(walPath, &wal.Options{
		SegmentCacheSize: wc.bufferSize(),
		NoCopy:           true,
	})
	if err != nil {
		return nil, "", fmt.Errorf("prometheusremotewriteexporter: failed to open WAL: %w", err)
	}
	return log, walPath, nil
}

var (
	errAlreadyClosed = errors.New("already closed")
	errNilWAL        = errors.New("wal is nil")
)

// retrieveWALIndices queries the WriteAheadLog for its current first and last indices.
func (prweWAL *prweWAL) retrieveWALIndices() (err error) {
	prweWAL.mu.Lock()
	defer prweWAL.mu.Unlock()

	err = prweWAL.closeWAL()
	if err != nil {
		return err
	}

	log, walPath, err := prweWAL.walConfig.createWAL()
	if err != nil {
		return err
	}

	prweWAL.wal = log
	prweWAL.walPath = walPath

	rIndex, err := prweWAL.wal.FirstIndex()
	if err != nil {
		return fmt.Errorf("prometheusremotewriteexporter: failed to retrieve the first WAL index: %w", err)
	}
	prweWAL.rWALIndex.Store(rIndex)

	wIndex, err := prweWAL.wal.LastIndex()
	if err != nil {
		return fmt.Errorf("prometheusremotewriteexporter: failed to retrieve the last WAL index: %w", err)
	}
	prweWAL.wWALIndex.Store(wIndex)
	return nil
}

func (prweWAL *prweWAL) stop() error {
	err := errAlreadyClosed
	prweWAL.stopOnce.Do(func() {
		close(prweWAL.stopChan)
		prweWAL.wg.Wait()
		err = prweWAL.closeWAL()
	})
	return err
}

// run begins reading from the WAL until prwe.stopChan is closed.
func (prweWAL *prweWAL) run(ctx context.Context) (err error) {
	var logger *zap.Logger
	logger, err = loggerFromContext(ctx)
	if err != nil {
		return
	}

	if err = prweWAL.retrieveWALIndices(); err != nil {
		logger.Error("unable to start write-ahead log", zap.Error(err))
		return
	}

	runCtx, cancel := context.WithCancel(ctx)

	// Start the process of exporting but wait until the exporting has started.
	waitUntilStartedCh := make(chan bool)
	prweWAL.wg.Add(1)
	go func() {
		defer prweWAL.wg.Done()
		defer cancel()

		signalStart := func() { close(waitUntilStartedCh) }
		for {
			select {
			case <-runCtx.Done():
				return
			case <-prweWAL.stopChan:
				return
			default:
				err := prweWAL.continuallyPopWALThenExport(runCtx, signalStart)
				signalStart = func() {}
				if err != nil {
					// log err
					logger.Error("error processing WAL entries", zap.Error(err))
					// Restart WAL
					if errS := prweWAL.retrieveWALIndices(); errS != nil {
						logger.Error("unable to re-start write-ahead log after error", zap.Error(errS))
						return
					}
				}
			}
		}
	}()
	<-waitUntilStartedCh
	return nil
}

// continuallyPopWALThenExport reads a prompb.WriteRequest proto encoded blob from the WAL, and moves
// the WAL's front index forward until either the read buffer period expires or the maximum
// buffer size is exceeded. When either of the two conditions are matched, it then exports
// the requests to the Remote-Write endpoint, and then truncates the head of the WAL to where
// it last read from.
func (prweWAL *prweWAL) continuallyPopWALThenExport(ctx context.Context, signalStart func()) (err error) {
	var reqL []*prompb.WriteRequest
	defer func() {
		// Keeping it within a closure to ensure that the later
		// updated value of reqL is always flushed to disk.
		if errL := prweWAL.exportSink(ctx, reqL); errL != nil {
			err = multierr.Append(err, errL)
		}
	}()

	freshTimer := func() *time.Timer {
		return time.NewTimer(prweWAL.walConfig.truncateFrequency())
	}

	timer := freshTimer()
	defer func() {
		// Added in a closure to ensure we capture the later
		// updated value of timer when changed in the loop below.
		timer.Stop()
	}()

	signalStart()

	maxCountPerUpload := prweWAL.walConfig.bufferSize()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-prweWAL.stopChan:
			return nil
		default:
		}

		var req *prompb.WriteRequest
		req, err = prweWAL.readPrompbFromWAL(ctx, prweWAL.rWALIndex.Load())
		if err != nil {
			return err
		}
		reqL = append(reqL, req)

		var shouldExport bool
		select {
		case <-timer.C:
			shouldExport = true
		default:
			shouldExport = len(reqL) >= maxCountPerUpload
		}

		if !shouldExport {
			continue
		}

		// Otherwise, it is time to export, flush and then truncate the WAL, but also to kill the timer!
		timer.Stop()
		timer = freshTimer()

		if err = prweWAL.exportThenFrontTruncateWAL(ctx, reqL); err != nil {
			return err
		}
		// Reset but reuse the write requests slice.
		reqL = reqL[:0]
	}
}

func (prweWAL *prweWAL) closeWAL() error {
	if prweWAL.wal != nil {
		err := prweWAL.wal.Close()
		prweWAL.wal = nil
		return err
	}
	return nil
}

func (prweWAL *prweWAL) syncAndTruncateFront() error {
	prweWAL.mu.Lock()
	defer prweWAL.mu.Unlock()

	if prweWAL.wal == nil {
		return errNilWAL
	}

	// Save all the entries that aren't yet committed, to the tail of the WAL.
	if err := prweWAL.wal.Sync(); err != nil {
		return err
	}
	// Truncate the WAL from the front for the entries that we already
	// read from the WAL and had already exported.
	if err := prweWAL.wal.TruncateFront(prweWAL.rWALIndex.Load()); err != nil && !errors.Is(err, wal.ErrOutOfRange) {
		return err
	}
	return nil
}

func (prweWAL *prweWAL) exportThenFrontTruncateWAL(ctx context.Context, reqL []*prompb.WriteRequest) error {
	if len(reqL) == 0 {
		return nil
	}
	if cErr := ctx.Err(); cErr != nil {
		return nil
	}

	if errL := prweWAL.exportSink(ctx, reqL); errL != nil {
		return errL
	}
	if err := prweWAL.syncAndTruncateFront(); err != nil {
		return err
	}
	// Reset by retrieving the respective read and write WAL indices.
	return prweWAL.retrieveWALIndices()
}

// persistToWAL is the routine that'll be hooked into the exporter's receiving side and it'll
// write them to the Write-Ahead-Log so that shutdowns won't lose data, and that the routine that
// reads from the WAL can then process the previously serialized requests.
func (prweWAL *prweWAL) persistToWAL(requests []*prompb.WriteRequest) error {
	prweWAL.mu.Lock()
	defer prweWAL.mu.Unlock()

	// Write all the requests to the WAL in a batch.
	batch := new(wal.Batch)
	for _, req := range requests {
		protoBlob, err := proto.Marshal(req)
		if err != nil {
			return err
		}
		wIndex := prweWAL.wWALIndex.Add(1)
		batch.Write(wIndex, protoBlob)
	}

	// Notify reader go routine that is possibly waiting for writes.
	select {
	case prweWAL.rNotify <- struct{}{}:
	default:
	}

	return prweWAL.wal.WriteBatch(batch)
}

func (prweWAL *prweWAL) readPrompbFromWAL(ctx context.Context, index uint64) (wreq *prompb.WriteRequest, err error) {
	var protoBlob []byte
	for i := 0; i < 12; i++ {
		// Firstly check if we've been terminated, then exit if so.
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-prweWAL.stopChan:
			return nil, errors.New("attempt to read from WAL after stopped")
		default:
		}

		if index <= 0 {
			index = 1
		}

		prweWAL.mu.Lock()
		if prweWAL.wal == nil {
			return nil, errors.New("attempt to read from closed WAL")
		}
		protoBlob, err = prweWAL.wal.Read(index)
		if err == nil { // The read succeeded.
			req := new(prompb.WriteRequest)
			if err = proto.Unmarshal(protoBlob, req); err != nil {
				return nil, err
			}

			// Now increment the WAL's read index.
			prweWAL.rWALIndex.Add(1)

			prweWAL.mu.Unlock()
			return req, nil
		}
		prweWAL.mu.Unlock()

		// If WAL was empty, let's wait for a notification from
		// the writer go routine.
		if errors.Is(err, wal.ErrNotFound) {
			select {
			case <-prweWAL.rNotify:
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-prweWAL.stopChan:
				return nil, errors.New("attempt to read from WAL after stopped")
			}
		}

		if !errors.Is(err, wal.ErrNotFound) {
			return nil, err
		}
	}
	return nil, err
}
