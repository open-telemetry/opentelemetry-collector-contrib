// Copyright The OpenTelemetry Authors
// Copyright (c) 2018 The Jaeger Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package telemetrygen // import "github.com/open-telemetry/opentelemetry-collector-contrib/telemetrygen/internal/telemetrygen"

import (
	"errors"
	"os"

	"github.com/spf13/cobra"
)

// rootCmd is the root command on which will be run children commands
var rootCmd = &cobra.Command{
	Use:     "telemetrygen",
	Short:   "Telemetrygen simulates a client generating traces and metrics",
	Example: "telemetrygen metrics\ntelemetrygen traces",
}

// tracesCmd is the command responsible for sending traces
var tracesCmd = &cobra.Command{
	Use:     "traces",
	Short:   "Simulates a client generating traces",
	Example: "telemetrygen traces",
	RunE: func(cmd *cobra.Command, args []string) error {
		err := errors.New("[WIP] The 'traces' command is a work in progress, come back later")
		return err
	},
}

// metricsCmd is the command responsible for sending metrics
var metricsCmd = &cobra.Command{
	Use:     "metrics",
	Short:   "Simulates a client generating metrics",
	Example: "telemetrygen metrics",
	RunE: func(cmd *cobra.Command, args []string) error {
		err := errors.New("[WIP] The 'metrics' command is a work in progress, come back later")
		return err
	},
}

func init() {
	rootCmd.AddCommand(tracesCmd, metricsCmd)

	// Disabling completion command for end user
	// https://github.com/spf13/cobra/blob/master/shell_completions.md
	rootCmd.CompletionOptions.DisableDefaultCmd = true

}

// Execute tries to run the input command
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		// TODO: Uncomment the line below when using Run instead of RunE in the xxxCmd functions
		// fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
