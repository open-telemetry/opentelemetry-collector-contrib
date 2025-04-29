// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fileconsumer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/fingerprint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/internal/filetest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"
)

type fileInputBenchmark struct {
	name   string
	paths  []string
	config func() *Config
}

func BenchmarkFileInput(b *testing.B) {
	cases := []fileInputBenchmark{
		{
			name: "Single",
			paths: []string{
				"file0.log",
			},
			config: func() *Config {
				cfg := NewConfig()
				cfg.Include = []string{
					"file0.log",
				}
				return cfg
			},
		},
		{
			name: "Glob",
			paths: []string{
				"file0.log",
				"file1.log",
				"file2.log",
				"file3.log",
			},
			config: func() *Config {
				cfg := NewConfig()
				cfg.Include = []string{"file*.log"}
				return cfg
			},
		},
		{
			name: "MultiGlob",
			paths: []string{
				"file0.log",
				"file1.log",
				"log0.log",
				"log1.log",
			},
			config: func() *Config {
				cfg := NewConfig()
				cfg.Include = []string{
					"file*.log",
					"log*.log",
				}
				return cfg
			},
		},
		{
			name: "MaxConcurrent",
			paths: []string{
				"file0.log",
				"file1.log",
				"file2.log",
				"file3.log",
			},
			config: func() *Config {
				cfg := NewConfig()
				cfg.Include = []string{
					"file*.log",
				}
				cfg.MaxConcurrentFiles = 2
				return cfg
			},
		},
		{
			name: "FngrPrntLarge",
			paths: []string{
				"file0.log",
			},
			config: func() *Config {
				cfg := NewConfig()
				cfg.Include = []string{
					"file*.log",
				}
				cfg.FingerprintSize = 10 * fingerprint.DefaultSize
				return cfg
			},
		},
		{
			name: "FngrPrntSmall",
			paths: []string{
				"file0.log",
			},
			config: func() *Config {
				cfg := NewConfig()
				cfg.Include = []string{
					"file*.log",
				}
				cfg.FingerprintSize = fingerprint.DefaultSize / 10
				return cfg
			},
		},
		{
			name: "NoFlush",
			paths: []string{
				"file0.log",
			},
			config: func() *Config {
				cfg := NewConfig()
				cfg.Include = []string{
					"file*.log",
				}
				cfg.FlushPeriod = 0
				return cfg
			},
		},
		{
			name: "Many",
			paths: func() []string {
				paths := make([]string, 100)
				for i := range paths {
					paths[i] = fmt.Sprintf("file%d.log", i)
				}
				return paths
			}(),
			config: func() *Config {
				cfg := NewConfig()
				cfg.Include = []string{"file*.log"}
				cfg.MaxConcurrentFiles = 100
				return cfg
			},
		},
	}

	// Pregenerate some lines which we can write to the files
	// to avoid measuring the time it takes to generate them
	// and to reduce the amount of syscalls in the benchmark.
	uniqueLines := 10
	severalLines := ""
	for i := 0; i < uniqueLines; i++ {
		severalLines += string(filetest.TokenWithLength(999)) + "\n"
	}

	for _, bench := range cases {
		b.Run(bench.name, func(b *testing.B) {
			b.ReportAllocs()
			rootDir := b.TempDir()

			var files []*os.File
			for _, path := range bench.paths {
				f := filetest.OpenFile(b, filepath.Join(rootDir, path))
				// Initialize the file to ensure a unique fingerprint
				_, err := f.WriteString(f.Name() + "\n")
				require.NoError(b, err)
				// Write half the content before starting the benchmark
				for i := 0; i < b.N/2; i++ {
					_, err := f.WriteString(severalLines)
					require.NoError(b, err)
				}
				require.NoError(b, f.Sync())
				files = append(files, f)
			}

			cfg := bench.config()
			for i, inc := range cfg.Include {
				cfg.Include[i] = filepath.Join(rootDir, inc)
			}
			cfg.StartAt = "beginning"
			// Use aggressive poll interval so we're not measuring excess sleep time
			cfg.PollInterval = time.Microsecond

			doneChan := make(chan bool, len(files))
			callback := func(_ context.Context, tokens [][]byte, _ map[string]any, _ int64) error {
				if len(tokens) > 0 && len(tokens[len(tokens)-1]) == 0 {
					doneChan <- true
				}
				return nil
			}
			set := componenttest.NewNopTelemetrySettings()
			op, err := cfg.Build(set, callback)
			require.NoError(b, err)

			b.ResetTimer()
			require.NoError(b, op.Start(testutil.NewUnscopedMockPersister()))
			defer func() {
				require.NoError(b, op.Stop())
			}()

			var wg sync.WaitGroup
			for _, file := range files {
				wg.Add(1)
				go func(f *os.File) {
					defer wg.Done()
					// Write the other half of the content while running
					for i := 0; i < b.N/2; i++ {
						_, err := f.WriteString(severalLines)
						assert.NoError(b, err)
					}
					// Signal end of file
					_, err := f.WriteString("\n")
					assert.NoError(b, err)
					assert.NoError(b, f.Sync())
				}(file)
			}

			// Timer continues to run until all files have been read
			for dones := 0; dones < len(files); dones++ {
				<-doneChan
			}
			wg.Wait()
		})
	}
}

func BenchmarkConsumeFiles(b *testing.B) {
	cases := []fileInputBenchmark{
		{
			name: "Single",
			paths: []string{
				"file0.log",
			},
			config: func() *Config {
				cfg := NewConfig()
				cfg.Include = []string{
					"file0.log",
				}
				cfg.MaxLogSize = 1 * 1024 * 1024
				cfg.InitialBufferSize = 1 * 1024 * 1024
				cfg.FingerprintSize = fingerprint.DefaultSize / 10
				return cfg
			},
		},
		{
			name: "Multiple",
			paths: func() []string {
				paths := make([]string, 100)
				for i := range paths {
					paths[i] = fmt.Sprintf("file%d.log", i)
				}
				return paths
			}(),
			config: func() *Config {
				cfg := NewConfig()
				cfg.Include = []string{"file*.log"}
				cfg.Encoding = ""
				cfg.FingerprintSize = fingerprint.DefaultSize / 10
				cfg.MaxLogSize = 1 * 1024 * 1024
				cfg.InitialBufferSize = 1 * 1024 * 1024
				cfg.MaxConcurrentFiles = 10
				return cfg
			},
		},
	}

	// Pregenerate some lines which we can write to the files
	// to avoid measuring the time it takes to generate them
	// and to reduce the amount of syscalls in the benchmark.
	uniqueLines := 10
	severalLines := ""
	for i := 0; i < uniqueLines; i++ {
		severalLines += string(filetest.TokenWithLength(999)) + "\n"
	}

	for _, bench := range cases {
		b.Run(bench.name, func(b *testing.B) {
			rootDir := b.TempDir()

			var consumePaths []string
			var files []*os.File
			for _, path := range bench.paths {
				consumePath := filepath.Join(rootDir, path)
				consumePaths = append(consumePaths, consumePath)
				f := filetest.OpenFile(b, consumePath)
				files = append(files, f)
				// Initialize the file to ensure a unique fingerprint
				_, err := f.WriteString(f.Name() + "\n")
				require.NoError(b, err)
				for i := 0; i < b.N; i++ {
					_, err := f.WriteString(severalLines)
					require.NoError(b, err)
				}
				require.NoError(b, f.Sync())
			}

			cfg := bench.config()
			for i, inc := range cfg.Include {
				cfg.Include[i] = filepath.Join(rootDir, inc)
			}
			cfg.StartAt = "beginning"
			// Use a long poll so that we don't trigger it.
			cfg.PollInterval = 1 * time.Hour

			doneChan := make(chan bool, len(files))
			numTokens := &atomic.Int64{}
			callback := func(_ context.Context, tokens [][]byte, _ map[string]any, _ int64) error {
				if numTokens.Add(int64(len(tokens))) == int64(len(files)*(b.N*uniqueLines+1)) {
					close(doneChan)
				}
				return nil
			}
			set := componenttest.NewNopTelemetrySettings()
			op, err := cfg.Build(set, callback)
			require.NoError(b, err)

			require.NoError(b, op.Start(testutil.NewUnscopedMockPersister()))
			defer func() {
				require.NoError(b, op.Stop())
			}()

			b.ReportAllocs()
			b.ResetTimer()
			for len(consumePaths) > op.maxBatchFiles {
				op.consume(context.Background(), consumePaths[:op.maxBatchFiles])
				consumePaths = consumePaths[op.maxBatchFiles:]
			}
			op.consume(context.Background(), consumePaths)
			<-doneChan
		})
	}
}
