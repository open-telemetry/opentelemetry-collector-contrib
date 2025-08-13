// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package zstd // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/otelarrow/compression/zstd"

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"runtime"
	"sync"

	zstdlib "github.com/klauspost/compress/zstd"
	"google.golang.org/grpc"
	"google.golang.org/grpc/encoding"
)

// NamePrefix is prefix, with N for compression level.
const NamePrefix = "zstdarrow"

// Level is an integer value mapping to compression level.
// [0] implies disablement; not registered in grpc
// [1,2] fastest i.e., "zstdarrow1", "zstdarrow2"
// [3-5] default
// [6-9] better
// [10] best.
type Level uint

const (
	// DefaultLevel is a reasonable balance of compression and cpu usage.
	DefaultLevel Level = 5
	// MinLevel is fast and cheap.
	MinLevel Level = 1
	// MaxLevel is slow and expensive.
	MaxLevel Level = 10
)

type EncoderConfig struct {
	// Level is meaningful in the range [0, 10].  No invalid
	// values, they all map into 4 default configurations.  (default: 5)
	// See `zstdlib.WithEncoderLevel()`.
	Level Level `mapstructure:"level"`
	// WindowSizeMiB is a Zstd-library parameter that controls how
	// much window of text is visible to the compressor at a time.
	// It is the dominant factor that determines memory usage.
	// If zero, the window size is determined by level.  (default: 0)
	// See `zstdlib.WithWindowSize()`.
	WindowSizeMiB uint32 `mapstructure:"window_size_mib"`
	// Concurrency is a Zstd-library parameter that configures the
	// use of background goroutines to improve compression speed.
	// 0 means to let the library decide (it will use up to GOMAXPROCS),
	// and 1 means to avoid background workers.  (default: 1)
	// See `zstdlib.WithEncoderConcurrency()`.
	Concurrency uint `mapstructure:"concurrency"`
}

type DecoderConfig struct {
	// MemoryLimitMiB is a memory limit control for the decoder,
	// as a way to limit overall memory use by Zstd.
	// See `zstdlib.WithDecoderMaxMemory()`.
	MemoryLimitMiB uint32 `mapstructure:"memory_limit_mib"`
	// MaxWindowSizeMiB limits window sizes that can be configured
	// in the corresponding encoder's `EncoderConfig.WindowSizeMiB`
	// setting, as a way to control memory usage.
	// See `zstdlib.WithDecoderMaxWindow()`.
	MaxWindowSizeMiB uint32 `mapstructure:"max_window_size_mib"`
	// Concurrency is a Zstd-library parameter that configures the
	// use of background goroutines to improve decompression speed.
	// 0 means to let the library decide (it will use up to GOMAXPROCS),
	// and 1 means to avoid background workers.  (default: 1)
	// See `zstdlib.WithDecoderConcurrency()`.
	Concurrency uint `mapstructure:"concurrency"`
}

type encoder struct {
	lock sync.Mutex // protects cfg
	cfg  EncoderConfig
	pool mru[*writer]
}

type decoder struct {
	lock sync.Mutex // protects cfg
	cfg  DecoderConfig
	pool mru[*reader]
}

type reader struct {
	*zstdlib.Decoder
	Gen
	pool *mru[*reader]
}

type writer struct {
	*zstdlib.Encoder
	Gen
	pool *mru[*writer]
}

type combined struct {
	enc encoder
	dec decoder
}

type instance struct {
	lock    sync.Mutex
	byLevel map[Level]*combined
}

var _ encoding.Compressor = &combined{}

var staticInstances = &instance{
	byLevel: map[Level]*combined{},
}

func (g *Gen) generation() Gen {
	return *g
}

func DefaultEncoderConfig() EncoderConfig {
	return EncoderConfig{
		Level:       DefaultLevel, // Determines other defaults
		Concurrency: 1,            // Avoids extra CPU/memory
	}
}

func DefaultDecoderConfig() DecoderConfig {
	return DecoderConfig{
		Concurrency:      1,   // Avoids extra CPU/memory
		MemoryLimitMiB:   128, // More conservative than library default
		MaxWindowSizeMiB: 32,  // Corresponds w/ "best" level default
	}
}

func validate(level Level, f func() error) error {
	if level > MaxLevel {
		return fmt.Errorf("level out of range [0,10]: %d", level)
	}
	if level < MinLevel {
		return fmt.Errorf("level out of range [0,10]: %d", level)
	}
	return f()
}

func (cfg EncoderConfig) Validate() error {
	return validate(cfg.Level, func() error {
		var buf bytes.Buffer
		test, err := zstdlib.NewWriter(&buf, cfg.options()...)
		if test != nil {
			test.Close()
		}
		return err
	})
}

func (cfg DecoderConfig) Validate() error {
	return validate(MinLevel, func() error {
		var buf bytes.Buffer
		test, err := zstdlib.NewReader(&buf, cfg.options()...)
		if test != nil {
			test.Close()
		}
		return err
	})
}

func init() {
	staticInstances.lock.Lock()
	defer staticInstances.lock.Unlock()
	resetLibrary()
}

func resetLibrary() {
	for level := MinLevel; level <= MaxLevel; level++ {
		var combi combined
		combi.enc.cfg = DefaultEncoderConfig()
		combi.dec.cfg = DefaultDecoderConfig()
		combi.enc.cfg.Level = level
		encoding.RegisterCompressor(&combi)
		staticInstances.byLevel[level] = &combi
	}
}

func SetEncoderConfig(cfg EncoderConfig) error {
	if err := cfg.Validate(); err != nil {
		return err
	}

	updateOne := func(enc *encoder) {
		enc.lock.Lock()
		defer enc.lock.Unlock()
		enc.cfg = cfg
		enc.pool.Reset()
	}

	staticInstances.lock.Lock()
	defer staticInstances.lock.Unlock()

	updateOne(&staticInstances.byLevel[cfg.Level].enc)
	return nil
}

func SetDecoderConfig(cfg DecoderConfig) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	updateOne := func(dec *decoder) {
		dec.lock.Lock()
		defer dec.lock.Unlock()
		dec.cfg = cfg
		dec.pool.Reset()
	}

	staticInstances.lock.Lock()
	defer staticInstances.lock.Unlock()

	for level := MinLevel; level <= MaxLevel; level++ {
		updateOne(&staticInstances.byLevel[level].dec)
	}
	return nil
}

func (cfg EncoderConfig) options() (opts []zstdlib.EOption) {
	opts = append(opts, zstdlib.WithEncoderLevel(zstdlib.EncoderLevelFromZstd(int(cfg.Level))))

	if cfg.Concurrency != 0 {
		opts = append(opts, zstdlib.WithEncoderConcurrency(int(cfg.Concurrency)))
	}
	if cfg.WindowSizeMiB != 0 {
		opts = append(opts, zstdlib.WithWindowSize(int(cfg.WindowSizeMiB<<20)))
	}

	return opts
}

func (e *encoder) getConfig() EncoderConfig {
	e.lock.Lock()
	defer e.lock.Unlock()
	return e.cfg
}

func (cfg EncoderConfig) Name() string {
	return fmt.Sprint(NamePrefix, cfg.Level)
}

func (cfg EncoderConfig) CallOption() grpc.CallOption {
	if cfg.Level < MinLevel || cfg.Level > MaxLevel {
		return grpc.UseCompressor(EncoderConfig{Level: DefaultLevel}.Name())
	}
	return grpc.UseCompressor(cfg.Name())
}

func (cfg DecoderConfig) options() (opts []zstdlib.DOption) {
	if cfg.Concurrency != 0 {
		opts = append(opts, zstdlib.WithDecoderConcurrency(int(cfg.Concurrency)))
	}
	if cfg.MaxWindowSizeMiB != 0 {
		opts = append(opts, zstdlib.WithDecoderMaxWindow(uint64(cfg.MaxWindowSizeMiB)<<20))
	}
	if cfg.MemoryLimitMiB != 0 {
		opts = append(opts, zstdlib.WithDecoderMaxMemory(uint64(cfg.MemoryLimitMiB)<<20))
	}

	return opts
}

func (d *decoder) getConfig() DecoderConfig {
	d.lock.Lock()
	defer d.lock.Unlock()
	return d.cfg
}

func (c *combined) Compress(w io.Writer) (io.WriteCloser, error) {
	z, gen := c.enc.pool.Get()
	if z == nil {
		encoder, err := zstdlib.NewWriter(w, c.enc.getConfig().options()...)
		if err != nil {
			return nil, err
		}
		z = &writer{Encoder: encoder, pool: &c.enc.pool, Gen: gen}
	} else {
		z.Reset(w)
	}
	return z, nil
}

func (w *writer) Close() error {
	defer w.pool.Put(w)
	return w.Encoder.Close()
}

func (c *combined) Decompress(r io.Reader) (io.Reader, error) {
	z, gen := c.dec.pool.Get()
	if z == nil {
		decoder, err := zstdlib.NewReader(r, c.dec.getConfig().options()...)
		if err != nil {
			return nil, err
		}
		z = &reader{Decoder: decoder, pool: &c.dec.pool, Gen: gen}

		// zstd decoders need to be closed when they are evicted from
		// the freelist. Note that the finalizer is attached to the
		// reader object, not to the decoder, because zstd maintains
		// background references to the decoder that prevent it from
		// being GC'ed.
		runtime.SetFinalizer(z, (*reader).Close)
	} else if err := z.Reset(r); err != nil {
		return nil, err
	}
	return z, nil
}

func (r *reader) Read(p []byte) (n int, err error) {
	n, err = r.Decoder.Read(p)
	if errors.Is(err, io.EOF) {
		r.pool.Put(r)
	}
	return n, err
}

func (c *combined) Name() string {
	return c.enc.cfg.Name()
}
