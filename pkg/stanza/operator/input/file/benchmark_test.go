// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package file

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
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
	line := stringWithLength(49) + "\n"
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
				cfg := NewConfig("test_id")
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
				cfg := NewConfig("test_id")
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
				cfg := NewConfig("test_id")
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
				cfg := NewConfig("test_id")
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
				cfg := NewConfig("test_id")
				cfg.Include = []string{
					"file*.log",
				}
				cfg.FingerprintSize = 10 * fileconsumer.DefaultFingerprintSize
				return cfg
			},
		},
		{
			name: "FngrPrntSmall",
			paths: []string{
				"file0.log",
			},
			config: func() *Config {
				cfg := NewConfig("test_id")
				cfg.Include = []string{
					"file*.log",
				}
				cfg.FingerprintSize = fileconsumer.DefaultFingerprintSize / 10
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
			cfg.OutputIDs = []string{"fake"}
			for i, inc := range cfg.Include {
				cfg.Include[i] = filepath.Join(rootDir, inc)
			}
			cfg.StartAt = "beginning"

			op, err := cfg.Build(testutil.Logger(b))
			require.NoError(b, err)

			fakeOutput := testutil.NewFakeOutput(b)
			err = op.SetOutputs([]operator.Operator{fakeOutput})
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
				<-fakeOutput.Received
			}
		})
	}
}
