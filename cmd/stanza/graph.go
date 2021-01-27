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
	"os"

	"github.com/opentelemetry/opentelemetry-log-collection/agent"
	"github.com/opentelemetry/opentelemetry-log-collection/database"
	"github.com/opentelemetry/opentelemetry-log-collection/operator"
	"github.com/opentelemetry/opentelemetry-log-collection/plugin"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// GraphFlags are the flags that can be supplied when running the graph command
type GraphFlags struct {
	*RootFlags
}

// NewGraphCommand creates a command for printing the pipeline as a graph
func NewGraphCommand(rootFlags *RootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "graph",
		Args:  cobra.NoArgs,
		Short: "Export a dot-formatted representation of the operator graph",
		Run:   func(command *cobra.Command, args []string) { runGraph(command, args, rootFlags) },
	}
}

func runGraph(_ *cobra.Command, _ []string, flags *RootFlags) {
	var sugaredLogger *zap.SugaredLogger
	if flags.Debug {
		sugaredLogger = newDefaultLoggerAt(zapcore.DebugLevel, "")
	} else {
		sugaredLogger = newDefaultLoggerAt(zapcore.InfoLevel, "")
	}
	defer func() {
		_ = sugaredLogger.Sync()
	}()

	cfg, err := agent.NewConfigFromGlobs(flags.ConfigFiles)
	if err != nil {
		sugaredLogger.Errorw("Failed to read configs from glob", zap.Any("error", err))
		os.Exit(1)
	}

	if errs := plugin.RegisterPlugins(flags.PluginDir, operator.DefaultRegistry); len(errs) != 0 {
		sugaredLogger.Errorw("Got errors parsing parsing", "errors", err)
	}

	buildContext := operator.NewBuildContext(database.NewStubDatabase(), sugaredLogger)
	pipeline, err := cfg.Pipeline.BuildPipeline(buildContext, nil)
	if err != nil {
		sugaredLogger.Errorw("Failed to build operator pipeline", zap.Any("error", err))
		os.Exit(1)
	}

	dotGraph, err := pipeline.Render()
	if err != nil {
		sugaredLogger.Errorw("Failed to marshal dot graph", zap.Any("error", err))
		os.Exit(1)
	}

	dotGraph = append(dotGraph, '\n')
	_, err = stdout.Write(dotGraph)
	if err != nil {
		sugaredLogger.Errorw("Failed to write dot graph to stdout", zap.Any("error", err))
		os.Exit(1)
	}
}
