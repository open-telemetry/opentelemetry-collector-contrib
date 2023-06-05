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
	"go.uber.org/multierr"
	"go.uber.org/zap"
)

type prweWAL struct {
	mu        sync.Mutex // mu protects the fields below.
	wal       *wal.Log
	walConfig *WALConfig
	walPath   string

	exportSink func(ctx context.Context, reqL []*prompb.WriteRequest) error

	stopOnce  sync.Once
	stopChan  chan struct{}
	rNotify   chan struct{}
	rWALIndex *atomic.Uint64
	wWALIndex *atomic.Uint64

	log *zap.Logger
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

func newWAL(walConfig *WALConfig, exportSink func(context.Context, []*prompb.WriteRequest) error) (*prweWAL, error) {
	if walConfig == nil {
		// There are cases for which the WAL can be disabled.
		// TODO: Perhaps log that the WAL wasn't enabled.
		return nil, errNilConfig
	}

	wal := prweWAL{
		exportSink: exportSink,
		walConfig:  walConfig,
		stopChan:   make(chan struct{}),
		rNotify:    make(chan struct{}),
		rWALIndex:  &atomic.Uint64{},
		wWALIndex:  &atomic.Uint64{},
		log:        zap.NewNop(),
	}

	return &wal, nil
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
	errNilConfig     = errors.New("expecting a non-nil configuration")
)

// retrieveWALIndices queries the WriteAheadLog for its current first and last indices.
func (prwe *prweWAL) retrieveWALIndices() (err error) {
	prwe.mu.Lock()
	defer prwe.mu.Unlock()

	err = prwe.closeWAL()
	if err != nil {
		return err
	}

	log, walPath, err := prwe.walConfig.createWAL()
	if err != nil {
		return err
	}

	prwe.wal = log
	prwe.walPath = walPath

	rIndex, err := prwe.wal.FirstIndex()
	if err != nil {
		return fmt.Errorf("prometheusremotewriteexporter: failed to retrieve the first WAL index: %w", err)
	}

	// NOTE: github.com/tidwall/wal reading starts at 1, but FirstIndex() may return 0 if the wal is empty
	prwe.rWALIndex.Store(max(rIndex, 1))

	wIndex, err := prwe.wal.LastIndex()
	if err != nil {
		return fmt.Errorf("prometheusremotewriteexporter: failed to retrieve the last WAL index: %w", err)
	}
	prwe.wWALIndex.Store(wIndex)
	return nil
}

func (prwe *prweWAL) stop() error {
	err := errAlreadyClosed
	prwe.stopOnce.Do(func() {
		prwe.mu.Lock()
		defer prwe.mu.Unlock()

		close(prwe.stopChan)
		err = prwe.closeWAL()
	})
	return err
}

// run begins reading from the WAL until prwe.stopChan is closed.
func (prwe *prweWAL) run(ctx context.Context) (err error) {
	var logger *zap.Logger
	logger, err = loggerFromContext(ctx)
	if err != nil {
		return
	}
	prwe.log = logger

	if err = prwe.retrieveWALIndices(); err != nil {
		logger.Error("unable to start write-ahead log", zap.Error(err))
		return
	}

	runCtx, cancel := context.WithCancel(ctx)

	// Start the process of exporting but wait until the exporting has started.
	waitUntilStartedCh := make(chan bool)
	go func() {
		signalStart := func() { close(waitUntilStartedCh) }
		defer cancel()
		for {
			select {
			case <-runCtx.Done():
				return
			case <-prwe.stopChan:
				return
			default:
				err := prwe.continuallyPopWALThenExport(runCtx, signalStart)
				signalStart = func() {}
				if err != nil {
					// log err
					logger.Error("error processing WAL entries", zap.Error(err))
					// Restart WAL
					if errS := prwe.retrieveWALIndices(); errS != nil {
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
func (prwe *prweWAL) continuallyPopWALThenExport(ctx context.Context, signalStart func()) (err error) {
	var reqL []*prompb.WriteRequest
	defer func() {
		// Keeping it within a closure to ensure that the later
		// updated value of reqL is always flushed to disk.
		if errL := prwe.exportSink(ctx, reqL); errL != nil {
			err = multierr.Append(err, errL)
		}
	}()

	freshTimer := func() *time.Timer {
		return time.NewTimer(prwe.walConfig.truncateFrequency())
	}

	timer := freshTimer()
	defer func() {
		// Added in a closure to ensure we capture the later
		// updated value of timer when changed in the loop below.
		timer.Stop()
	}()

	signalStart()

	maxCountPerUpload := prwe.walConfig.bufferSize()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-prwe.stopChan:
			return nil
		default:
		}

		var req *prompb.WriteRequest
		req, err = prwe.readPrompbFromWAL(ctx, prwe.rWALIndex.Load())
		if err != nil {
			return err
		}
		reqL = append(reqL, req)

		shouldExport := false
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

		if err = prwe.exportThenFrontTruncateWAL(ctx, reqL); err != nil {
			return err
		}
		// Reset but reuse the write requests slice.
		reqL = reqL[:0]
	}
}

func (prwe *prweWAL) closeWAL() error {
	if prwe.wal != nil {
		err := prwe.wal.Close()
		prwe.wal = nil
		return err
	}
	return nil
}

func (prwe *prweWAL) syncAndTruncateFront() error {
	prwe.mu.Lock()
	defer prwe.mu.Unlock()

	if prwe.wal == nil {
		return errNilWAL
	}

	// Save all the entries that aren't yet committed, to the tail of the WAL.
	if err := prwe.wal.Sync(); err != nil {
		return err
	}
	// Truncate the WAL from the front for the entries that we already
	// read from the WAL and had already exported.
	if err := prwe.wal.TruncateFront(prwe.rWALIndex.Load()); err != nil && !errors.Is(err, wal.ErrOutOfRange) {
		return err
	}
	return nil
}

func (prwe *prweWAL) exportThenFrontTruncateWAL(ctx context.Context, reqL []*prompb.WriteRequest) error {
	if len(reqL) == 0 {
		return nil
	}
	if cErr := ctx.Err(); cErr != nil {
		return nil
	}

	if errL := prwe.exportSink(ctx, reqL); errL != nil {
		return errL
	}
	if err := prwe.syncAndTruncateFront(); err != nil {
		return err
	}
	// Reset by retrieving the respective read and write WAL indices.
	return prwe.retrieveWALIndices()
}

// persistToWAL is the routine that'll be hooked into the exporter's receiving side and it'll
// write them to the Write-Ahead-Log so that shutdowns won't lose data, and that the routine that
// reads from the WAL can then process the previously serialized requests.
func (prwe *prweWAL) persistToWAL(requests []*prompb.WriteRequest) error {
	prwe.mu.Lock()
	defer prwe.mu.Unlock()

	// Write all the requests to the WAL in a batch.
	batch := new(wal.Batch)
	for _, req := range requests {
		protoBlob, err := proto.Marshal(req)
		if err != nil {
			return err
		}
		wIndex := prwe.wWALIndex.Add(1)
		prwe.log.Debug("write", zap.Uint64("index", wIndex))
		batch.Write(wIndex, protoBlob)
	}

	// notify possibly waiting tailing routine of write
	select {
	case prwe.rNotify <- struct{}{}:
		// tailing routine is actively waiting for write
	default:
		// no receiver, ignore
	}

	return prwe.wal.WriteBatch(batch)
}

// read repeatedly attempts to fetch a *prompb.WriteRequest from the WAL,
// aborting on error.
// prwe.mu is temporarily acquired for the individual read attempts only,
// allowing writes to happen while waiting
func (prwe *prweWAL) readPrompbFromWAL(ctx context.Context, index uint64) (*prompb.WriteRequest, error) {

	if prwe == nil {
		return nil, fmt.Errorf("attempt to read from closed WAL")
	}

	try := func() (*prompb.WriteRequest, error) {
		prwe.mu.Lock()
		defer prwe.mu.Unlock()

		prwe.log.Debug("read", zap.Uint64("index", index))
		protoBlob, err := prwe.wal.Read(index)
		if err != nil {
			return nil, err
		}

		var req prompb.WriteRequest
		if err := proto.Unmarshal(protoBlob, &req); err != nil {
			return nil, err
		}

		prwe.rWALIndex.Add(1)
		return &req, nil
	}

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-prwe.stopChan:
			return nil, fmt.Errorf("attempt to read from WAL after stopped")
		default:
		}

		req, err := try()
		if errors.Is(err, wal.ErrNotFound) {
			prwe.log.Debug("wal empty - waiting for write")

			select {
			case <-prwe.rNotify:
			case <-ctx.Done():
				return nil, ctx.Err()
			}

			continue
		} else if err != nil {
			return nil, err
		}

		return req, nil
	}
}

func max(a, b uint64) uint64 {
	if a > b {
		return a
	}
	return b
}
