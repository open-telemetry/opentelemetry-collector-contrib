// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate make mdatagen

// Package sdnotifyextension implements the `extensioncapabilities.PipelineWatcher`
// interface, integrating the collector with systemd via the sd_notify(3) protocol.
//
// When enabled, the extension notifies systemd of the collector's lifecycle:
//   - READY=1 is sent once all pipelines have started, unblocking
//     `systemctl start` for units of Type=notify or Type=notify-reload.
//   - STOPPING=1 is sent when pipelines shut down after SIGINT or SIGTERM.
//   - RELOADING=1 (with MONOTONIC_USEC) is sent on SIGHUP so systemd knows a
//     configuration reload is in progress; a second READY=1 follows once the
//     pipelines are back up. This enables zero-downtime reloads via
//     Type=notify-reload units with ReloadSignal=SIGHUP.
//   - WATCHDOG=1 is sent periodically (every WatchdogSec/2) when the unit sets
//     WatchdogSec=, acting as a keep-alive so systemd can detect a hung
//     collector and restart it. If WATCHDOG_USEC is unset, no pings are sent.
//
// If the NOTIFY_SOCKET environment variable is not set (i.e. the collector
// is not running under systemd), the extension operates as a no-op and doesn't
// fail at a startup.
package sdnotifyextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/sdnotifyextension"
