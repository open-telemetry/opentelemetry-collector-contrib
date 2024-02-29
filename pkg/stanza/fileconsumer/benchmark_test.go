// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fileconsumer

import (
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/filetest"
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
	content []byte
}

func simpleTextFile(file *os.File, numLines int, lineLen int) *benchFile {
	distinctFirstLine := fmt.Sprintf("%s\n", file.Name())
	numBytes := len(distinctFirstLine) + numLines*lineLen + 1 // +1 for the newline signaling EOF
	content := make([]byte, numBytes)
	rand.Read(content) // fill with random bytes

	// In order to efficiently detect EOF, we want it to end with an empty newline.
	// We'll just remove all newlines and then add new ones in a controlled manner.
	nonNewline := content[0]
	for i := 1; nonNewline == '\n'; i++ {
		nonNewline = content[i]
	}

	// Copy in the distinct first line
	copy(content[:len(distinctFirstLine)], []byte(distinctFirstLine))

	// Remove all newlines after the first line
	for i := len(distinctFirstLine); i < len(content); i++ {
		if content[i] == '\n' {
			content[i] = nonNewline
		}
	}

	// Add newlines to the end of each line
	for i := len(distinctFirstLine) + lineLen - 1; i < len(content)-1; i += lineLen {
		content[i] = '\n'
	}

	// Overwrite the last rune with a newline to signal EOF
	content[len(content)-1] = '\n'

	return &benchFile{File: file, content: content}
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
	}

	for _, bench := range cases {
		b.Run(bench.name, func(b *testing.B) {
			rootDir := b.TempDir()

			var files []*benchFile
			for _, path := range bench.paths {
				file := filetest.OpenFile(b, filepath.Join(rootDir, path))
				files = append(files, simpleTextFile(file, b.N, 100))
			}

			cfg := bench.config()
			for i, inc := range cfg.Include {
				cfg.Include[i] = filepath.Join(rootDir, inc)
			}
			cfg.StartAt = "beginning"
			// Use aggresive poll interval so we're not measuring sleep time
			cfg.PollInterval = time.Nanosecond

			doneChan := make(chan bool, len(files))
			callback := func(_ context.Context, token []byte, _ map[string]any) error {
				if len(token) == 0 {
					doneChan <- true
				}
				return nil
			}
			op, err := cfg.Build(testutil.Logger(b), callback)
			require.NoError(b, err)

			// Write some of the content before starting
			for _, file := range files {
				_, err := file.File.Write(file.content[:len(file.content)/2])
				require.NoError(b, err)
			}

			b.ResetTimer()
			require.NoError(b, op.Start(testutil.NewUnscopedMockPersister()))
			defer func() {
				require.NoError(b, op.Stop())
			}()

			// Write the remainder of content while running
			var wg sync.WaitGroup
			for _, file := range files {
				wg.Add(1)
				go func(f *benchFile) {
					defer wg.Done()
					_, err := f.File.Write(f.content[len(f.content)/2:])
					require.NoError(b, err)
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
