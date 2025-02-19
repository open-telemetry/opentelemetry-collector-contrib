// Copyright The OpenTelemetry Authors
// Copyright (c) 2018 The Jaeger Authors.
// SPDX-License-Identifier: Apache-2.0

//go:generate mdatagen metadata.yaml

package main // import "github.com/open-telemetry/opentelemetry-collector-contrib/telemetrygen/internal/telemetrygen"

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/telemetrygen/pkg/logs"
	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/telemetrygen/pkg/metrics"
	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/telemetrygen/pkg/traces"
)

var (
	tracesCfg  *traces.Config
	metricsCfg *metrics.Config
	logsCfg    *logs.Config
)

// rootCmd is the root command on which will be run children commands
var rootCmd = &cobra.Command{
	Use:     "telemetrygen",
	Short:   "Telemetrygen simulates a client generating traces, metrics, and logs",
	Example: "telemetrygen traces\ntelemetrygen metrics\ntelemetrygen logs",
}

// tracesCmd is the command responsible for sending traces
var tracesCmd = &cobra.Command{
	Use:     "traces",
	Short:   "Simulates a client generating traces. (Stability level: alpha)",
	Example: "telemetrygen traces",
	RunE: func(_ *cobra.Command, _ []string) error {
		return traces.Start(tracesCfg)
	},
}

// metricsCmd is the command responsible for sending metrics
var metricsCmd = &cobra.Command{
	Use:     "metrics",
	Short:   "Simulates a client generating metrics. (Stability level: development)",
	Example: "telemetrygen metrics",
	RunE: func(_ *cobra.Command, _ []string) error {
		return metrics.Start(metricsCfg)
	},
}

// logsCmd is the command responsible for sending logs
var logsCmd = &cobra.Command{
	Use:     "logs",
	Short:   "Simulates a client generating metrics. (Stability level: development)",
	Example: "telemetrygen logs",
	RunE: func(_ *cobra.Command, _ []string) error {
		return logs.Start(logsCfg)
	},
}

func init() {
	rootCmd.AddCommand(tracesCmd, metricsCmd, logsCmd)

	tracesCfg = traces.NewConfig()
	tracesCfg.Flags(tracesCmd.Flags())

	metricsCfg = metrics.NewConfig()
	metricsCfg.Flags(metricsCmd.Flags())

	logsCfg = logs.NewConfig()
	logsCfg.Flags(logsCmd.Flags())

	// Disabling completion command for end user
	// https://github.com/spf13/cobra/blob/master/shell_completions.md
	rootCmd.CompletionOptions.DisableDefaultCmd = true
}

// Execute tries to run the input command
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		// TODO: Uncomment the line below when using run instead of RunE in the xxxCmd functions
		// fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
