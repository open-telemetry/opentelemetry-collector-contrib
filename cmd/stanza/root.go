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

package main

import (
	"context"
	"fmt"
	"log"
	"net/http"

	// This package registers its HTTP endpoints for profiling using an init hook
	_ "net/http/pprof"
	"os"
	"runtime"
	"runtime/pprof"
	"sync"
	"time"

	agent "github.com/opentelemetry/opentelemetry-log-collection/agent"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// RootFlags are the root level flags that be provided when invoking stanza from the command line
type RootFlags struct {
	DatabaseFile       string
	ConfigFiles        []string
	PluginDir          string
	PprofPort          int
	CPUProfile         string
	CPUProfileDuration time.Duration
	MemProfile         string
	MemProfileDelay    time.Duration

	LogFile string
	Debug   bool
}

// NewRootCmd will return a root level command
func NewRootCmd() *cobra.Command {
	rootFlags := &RootFlags{}

	root := &cobra.Command{
		Use:   "stanza [-c ./config.yaml]",
		Short: "A log parser and router",
		Long:  "A log parser and router",
		Args:  cobra.NoArgs,
		Run:   func(command *cobra.Command, args []string) { runRoot(command, args, rootFlags) },
	}

	rootFlagSet := root.PersistentFlags()
	rootFlagSet.StringVar(&rootFlags.LogFile, "log_file", "", "write logs to configured path rather than stderr")
	rootFlagSet.StringSliceVarP(&rootFlags.ConfigFiles, "config", "c", []string{defaultConfig()}, "path to a config file")
	rootFlagSet.StringVar(&rootFlags.PluginDir, "plugin_dir", defaultPluginDir(), "path to the plugin directory")
	rootFlagSet.StringVar(&rootFlags.DatabaseFile, "database", "", "path to the stanza offset database")
	rootFlagSet.BoolVar(&rootFlags.Debug, "debug", false, "debug logging")

	// Profiling flags
	rootFlagSet.IntVar(&rootFlags.PprofPort, "pprof_port", 0, "listen port for pprof profiling")
	rootFlagSet.StringVar(&rootFlags.CPUProfile, "cpu_profile", "", "path to cpu profile output")
	rootFlagSet.DurationVar(&rootFlags.CPUProfileDuration, "cpu_profile_duration", 60*time.Second, "duration to run the cpu profile")
	rootFlagSet.StringVar(&rootFlags.MemProfile, "mem_profile", "", "path to memory profile output")
	rootFlagSet.DurationVar(&rootFlags.MemProfileDelay, "mem_profile_delay", 10*time.Second, "time to wait before writing a memory profile")

	// Set profiling flags to hidden
	hiddenFlags := []string{"pprof_port", "cpu_profile", "cpu_profile_duration", "mem_profile", "mem_profile_delay"}
	for _, flag := range hiddenFlags {
		err := rootFlagSet.MarkHidden(flag)
		if err != nil {
			// MarkHidden only fails if the flag does not exist
			panic(err)
		}
	}

	root.AddCommand(NewGraphCommand(rootFlags))
	root.AddCommand(NewVersionCommand())
	root.AddCommand(NewOffsetsCmd(rootFlags))

	return root
}

func runRoot(command *cobra.Command, _ []string, flags *RootFlags) {
	var logger *zap.SugaredLogger
	if flags.Debug {
		logger = newDefaultLoggerAt(zapcore.DebugLevel, flags.LogFile)
	} else {
		logger = newDefaultLoggerAt(zapcore.InfoLevel, flags.LogFile)
	}
	defer func() {
		_ = logger.Sync()
	}()

	agent, err := agent.NewBuilder(logger).
		WithConfigFiles(flags.ConfigFiles).
		WithPluginDir(flags.PluginDir).
		WithDatabaseFile(flags.DatabaseFile).
		Build()
	if err != nil {
		logger.Errorw("Failed to build agent", zap.Any("error", err))
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(command.Context())
	service, err := newAgentService(ctx, agent, cancel)
	if err != nil {
		logger.Errorf("Failed to create agent service", zap.Any("error", err))
		os.Exit(1)
	}

	profilingWg := startProfiling(ctx, flags, logger)

	err = service.Run()
	if err != nil {
		logger.Errorw("Failed to run agent service", zap.Any("error", err))
		os.Exit(1)
	}

	profilingWg.Wait()
}

func startProfiling(ctx context.Context, flags *RootFlags, logger *zap.SugaredLogger) *sync.WaitGroup {
	wg := &sync.WaitGroup{}

	// Start pprof listening on port
	if flags.PprofPort != 0 {
		// pprof endpoints registered by importing net/pprof
		var srv http.Server
		srv.Addr = fmt.Sprintf(":%d", flags.PprofPort)

		wg.Add(1)
		go func() {
			defer wg.Done()
			logger.Info(srv.ListenAndServe())
		}()

		wg.Add(1)

		go func() {
			defer wg.Done()
			<-ctx.Done()
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			err := srv.Shutdown(ctx)
			if err != nil {
				logger.Warnw("Errored shutting down pprof server", zap.Error(err))
			}
		}()
	}

	// Start CPU profile for configured duration
	if flags.CPUProfile != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()

			f, err := os.Create(flags.CPUProfile)
			if err != nil {
				logger.Errorw("Failed to create CPU profile", zap.Error(err))
			}
			defer f.Close()

			if err := pprof.StartCPUProfile(f); err != nil {
				log.Fatal("could not start CPU profile: ", err)
			}

			select {
			case <-ctx.Done():
			case <-time.After(flags.CPUProfileDuration):
			}
			pprof.StopCPUProfile()
		}()
	}

	// Start memory profile after configured delay
	if flags.MemProfile != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()

			select {
			case <-ctx.Done():
			case <-time.After(flags.MemProfileDelay):
			}

			f, err := os.Create(flags.MemProfile)
			if err != nil {
				logger.Errorw("Failed to create memory profile", zap.Error(err))
			}
			defer f.Close() // error handling omitted for example

			runtime.GC() // get up-to-date statistics
			if err := pprof.WriteHeapProfile(f); err != nil {
				log.Fatal("could not write memory profile: ", err)
			}
		}()
	}

	return wg
}
