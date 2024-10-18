// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azuremonitorexporter

import "github.com/microsoft/ApplicationInsights-Go/appinsights"

// telemetryClient is an interface for telemetry clients.
type telemetryClient interface {
	Track(telemetry appinsights.Telemetry)
	Channel() telemetryChannel
}

// telemetryChannel is an interface for telemetry channels.
type telemetryChannel interface {
	Flush()
	Close()
}

// appInsightsTelemetryClient is a wrapper around appinsights.TelemetryClient.
type appInsightsTelemetryClient struct {
	client appinsights.TelemetryClient
}

// Track sends telemetry data using the underlying client.
func (t *appInsightsTelemetryClient) Track(telemetry appinsights.Telemetry) {
	t.client.Track(telemetry)
}

// Channel returns the telemetry channel of the underlying client.
func (t *appInsightsTelemetryClient) Channel() telemetryChannel {
	return &appInsightsTelemetryChannel{channel: t.client.Channel()}
}

// appInsightsTelemetryChannel is a wrapper around appinsights.TelemetryChannel.
type appInsightsTelemetryChannel struct {
	channel appinsights.TelemetryChannel
}

// Flush flushes the telemetry data.
func (t *appInsightsTelemetryChannel) Flush() {
	t.channel.Flush()
}

// Close closes the telemetry channel.
func (t *appInsightsTelemetryChannel) Close() {
	t.channel.Close()
}