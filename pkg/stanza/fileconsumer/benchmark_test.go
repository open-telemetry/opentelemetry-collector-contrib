// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fileconsumer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/fingerprint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"
)

type fileInputBenchmark struct {
	name   string
	paths  []string
	config func() *Config
}

type fileSizeBenchmark struct {
	sizes [2]int
}

type benchFile struct {
	*os.File
	log func(int)
}

func simpleTextFile(b *testing.B, file *os.File) *benchFile {
	line := string(tokenWithLength(49)) + "\n"
	return &benchFile{
		File: file,
		log: func(_ int) {
			_, err := file.WriteString(line)
			require.NoError(b, err)
		},
	}
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
	}

	for _, bench := range cases {
		b.Run(bench.name, func(b *testing.B) {
			rootDir := b.TempDir()

			var files []*benchFile
			for _, path := range bench.paths {
				file := openFile(b, filepath.Join(rootDir, path))
				files = append(files, simpleTextFile(b, file))
			}

			cfg := bench.config()
			for i, inc := range cfg.Include {
				cfg.Include[i] = filepath.Join(rootDir, inc)
			}
			cfg.StartAt = "beginning"

			received := make(chan []byte)

			op, err := cfg.Build(testutil.Logger(b), emitOnChan(received))
			require.NoError(b, err)

			// write half the lines before starting
			mid := b.N / 2
			for i := 0; i < mid; i++ {
				for _, file := range files {
					file.log(i)
				}
			}

			b.ResetTimer()
			err = op.Start(testutil.NewMockPersister("test"))
			defer func() {
				require.NoError(b, op.Stop())
			}()
			require.NoError(b, err)

			// write the remainder of lines while running
			go func() {
				for i := mid; i < b.N; i++ {
					for _, file := range files {
						file.log(i)
					}
				}
			}()

			for i := 0; i < b.N*len(files); i++ {
				<-received
			}
		})
	}
}

func max(x, y int) int {
	if x < y {
		return y
	}
	return x
}

func (fileSize fileSizeBenchmark) createFiles(b *testing.B, rootDir string) {
	// create 50 files, some with large file sizes, other's with rather small
	getMessage := func(m int) string { return fmt.Sprintf("message %d", m) }
	logs := make([]string, 0)
	for i := 0; i < max(fileSize.sizes[0], fileSize.sizes[1]); i++ {
		logs = append(logs, getMessage(i))
	}

	for i := 0; i < 50; i++ {
		file := openFile(b, filepath.Join(rootDir, fmt.Sprintf("file_%s.log", uuid.NewString())))
		file.WriteString(uuid.NewString() + strings.Join(logs[:fileSize.sizes[i%2]], "\n") + "\n")
	}
}

func BenchmarkFileSizeVarying(b *testing.B) {
	fileSize := fileSizeBenchmark{
		sizes: [2]int{b.N * 5000, b.N * 10}, // Half the files will be huge, other half will be smaller
	}
	rootDir := b.TempDir()
	cfg := NewConfig().includeDir(rootDir)
	cfg.StartAt = "beginning"
	cfg.MaxConcurrentFiles = 50
	totalLogs := fileSize.sizes[0]*50 + fileSize.sizes[1]*50
	emitCalls := make(chan *emitParams, totalLogs+10)

	operator, _ := buildTestManager(b, cfg, withEmitChan(emitCalls), withReaderChan())
	operator.persister = testutil.NewMockPersister("test")
	defer func() {
		require.NoError(b, operator.Stop())
	}()
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		var once sync.Once
		for i := 0; i < totalLogs; i++ {
			once.Do(func() {
				// Reset once we get the first log
				b.ResetTimer()
			})
			<-emitCalls
		}
		// Stop the timer, as we're measuring log throughput
		b.StopTimer()
	}()
	// create first set of files
	fileSize.createFiles(b, rootDir)
	operator.poll(context.Background())

	// create new set of files, call poll() again
	fileSize.createFiles(b, rootDir)
	operator.poll(context.Background())

	wg.Wait()
}
