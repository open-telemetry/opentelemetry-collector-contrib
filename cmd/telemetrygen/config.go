// Copyright The OpenTelemetry Authors
// Copyright (c) 2018 The Jaeger Authors.
// SPDX-License-Identifier: Apache-2.0

//go:generate mdatagen metadata.yaml

package main // import "github.com/open-telemetry/opentelemetry-collector-contrib/telemetrygen/internal/telemetrygen"

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/telemetrygen/internal/metadata"
	logs2 "github.com/open-telemetry/opentelemetry-collector-contrib/cmd/telemetrygen/pkg/logs"
	metrics2 "github.com/open-telemetry/opentelemetry-collector-contrib/cmd/telemetrygen/pkg/metrics"
	traces2 "github.com/open-telemetry/opentelemetry-collector-contrib/cmd/telemetrygen/pkg/traces"
)

var (
	tracesCfg  *traces2.Config
	metricsCfg *metrics2.Config
	logsCfg    *logs2.Config
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
	Short:   fmt.Sprintf("Simulates a client generating traces. (Stability level: %s)", metadata.TracesStability),
	Example: "telemetrygen traces",
	RunE: func(_ *cobra.Command, _ []string) error {
		return traces2.Start(tracesCfg)
	},
}

// metricsCmd is the command responsible for sending metrics
var metricsCmd = &cobra.Command{
	Use:     "metrics",
	Short:   fmt.Sprintf("Simulates a client generating metrics. (Stability level: %s)", metadata.MetricsStability),
	Example: "telemetrygen metrics",
	RunE: func(_ *cobra.Command, _ []string) error {
		return metrics2.Start(metricsCfg)
	},
}

// logsCmd is the command responsible for sending logs
var logsCmd = &cobra.Command{
	Use:     "logs",
	Short:   fmt.Sprintf("Simulates a client generating logs. (Stability level: %s)", metadata.LogsStability),
	Example: "telemetrygen logs",
	RunE: func(_ *cobra.Command, _ []string) error {
		return logs2.Start(logsCfg)
	},
}

func init() {
	rootCmd.AddCommand(tracesCmd, metricsCmd, logsCmd)

	tracesCfg = new(traces2.Config)
	tracesCfg.Flags(tracesCmd.Flags())

	metricsCfg = new(metrics2.Config)
	metricsCfg.Flags(metricsCmd.Flags())

	logsCfg = new(logs2.Config)
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
