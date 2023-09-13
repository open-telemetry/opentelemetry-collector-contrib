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
	logs          []int
	maxConcurrent int
	name          string
	desiredFiles  int
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

func max(nums ...int) int {
	_max := 0
	for _, n := range nums {
		if _max < n {
			_max = n
		}
	}
	return _max
}

func (fileSize fileSizeBenchmark) createFiles(b *testing.B, rootDir string) int {

	// the number of logs written to a file is selected in round-robin fashion from fileSize.logs
	// eg. fileSize.logs = [10,100]
	// It will create one half of files with 10*b.N lines and other with 100*b.N lines
	// ileSize.logs = [10,100,500]
	// It will create one third of files with 10*b.N lines and other with 100*b.N and remaining third with 500*b.N

	getMessage := func(m int) string { return fmt.Sprintf("message %d", m) }
	logs := make([]string, 0, b.N) // collect all the logs at beginning itself to and reuse same to write to files
	for i := 0; i < max(fileSize.logs...)*b.N; i++ {
		logs = append(logs, getMessage(i))
	}
	totalLogs := 0
	for i := 0; i < fileSize.desiredFiles; i++ {
		file := openFile(b, filepath.Join(rootDir, fmt.Sprintf("file_%s.log", uuid.NewString())))
		// Use uuid.NewString() to introduce some randomness in file logs
		// or else file consumer will detect a duplicate based on fingerprint
		linesToWrite := b.N * fileSize.logs[i%len(fileSize.logs)]
		file.WriteString(uuid.NewString() + strings.Join(logs[:linesToWrite], "\n") + "\n")
		totalLogs += linesToWrite
	}
	return totalLogs
}

func BenchmarkFileSizeVarying(b *testing.B) {
	testCases := []fileSizeBenchmark{
		{
			name:          "varying_sizes",
			logs:          []int{10, 1000},
			maxConcurrent: 50,
			desiredFiles:  100,
		},
		{
			name:          "varying_sizes_more_files",
			logs:          []int{10, 1000},
			maxConcurrent: 50,
			desiredFiles:  200,
		},
		{
			name:          "varying_sizes_more_files_throttled",
			logs:          []int{10, 1000},
			maxConcurrent: 30,
			desiredFiles:  200,
		},
		{
			name:          "same_size_small",
			logs:          []int{10},
			maxConcurrent: 50,
			desiredFiles:  50,
		},
		{
			name:          "same_size_small_throttled",
			logs:          []int{10},
			maxConcurrent: 10,
			desiredFiles:  100,
		},
	}
	for _, fileSize := range testCases {
		b.Run(fileSize.name, func(b *testing.B) {
			rootDir := b.TempDir()
			cfg := NewConfig().includeDir(rootDir)
			cfg.StartAt = "beginning"
			cfg.MaxConcurrentFiles = fileSize.maxConcurrent
			emitCalls := make(chan *emitParams, max(fileSize.logs...)*b.N) // large enough to hold all the logs

			operator, _ := buildTestManager(b, cfg, withEmitChan(emitCalls), withReaderChan())
			operator.persister = testutil.NewMockPersister("test")
			defer func() {
				require.NoError(b, operator.Stop())
			}()

			// create first set of files
			totalLogs := fileSize.createFiles(b, rootDir)
			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer wg.Done()
				var once sync.Once
				// wait for logs from two poll() cycles
				for i := 0; i < totalLogs*2; i++ {
					once.Do(func() {
						// Reset once we get the first log
						b.ResetTimer()
					})
					<-emitCalls
				}
				// Stop the timer, as we're measuring log throughput
				b.StopTimer()
			}()
			operator.poll(context.Background())

			// create new set of files, call poll() again
			fileSize.createFiles(b, rootDir)
			operator.poll(context.Background())

			wg.Wait()
		})

	}
}
