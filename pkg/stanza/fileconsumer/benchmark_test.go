// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fileconsumer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/fingerprint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"
)

type fileInputBenchmark struct {
	name   string
	paths  []string
	config func() *Config
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

func BenchmarkLogsThroughput(b *testing.B) {
	getMessage := func(f, m int) string { return fmt.Sprintf("file %d, message %d\n", f, m) }
	rootDir := b.TempDir()
	file := openFile(b, filepath.Join(rootDir, "file0.log"))
	file1 := openFile(b, filepath.Join(rootDir, "file1.log"))
	cfg := NewConfig().includeDir(rootDir)
	cfg.StartAt = "beginning"
	cfg.MaxConcurrentFiles = 8
	emitCalls := make(chan *emitParams, b.N*5)
	operator, _ := buildTestManager(b, cfg, withEmitChan(emitCalls), withReaderChan())
	operator.persister = testutil.NewMockPersister("test")

	total := b.N * 100
	factor := 2000

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		totalLogs := 2*total + 2*(total/factor)
		for i := 0; i < totalLogs; i++ {
			<-emitCalls
			// c := <-emitCalls
		}
		// fmt.Println(totalLogs)
	}()
	b.ResetTimer()

	// write more logs in one file
	for i := 0; i < total; i++ {
		writeString(b, file, getMessage(0, i))
	}
	// write less logs in one file
	for i := 0; i < total/factor; i++ {
		writeString(b, file1, getMessage(1, i))
	}

	start := time.Now()

	if useThreadPool.IsEnabled() {
		operator.pollConcurrent(context.Background())
	} else {
		operator.poll(context.Background())
	}
	// // create different files for second poll
	file = openFile(b, filepath.Join(rootDir, "file2.log"))
	file1 = openFile(b, filepath.Join(rootDir, "file3.log"))
	// // write more logs in one file
	for i := 0; i < total; i++ {
		writeString(b, file, getMessage(2, i))
	}
	// write less logs in one file
	for i := 0; i < total/factor; i++ {
		writeString(b, file1, getMessage(3, i))
	}
	// start2 := time.Now()
	if useThreadPool.IsEnabled() {
		operator.pollConcurrent(context.Background())
	} else {
		operator.poll(context.Background())
	}
	wg.Wait()
	fmt.Println(time.Now().Sub(start), b.N)
}
