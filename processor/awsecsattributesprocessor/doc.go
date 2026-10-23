// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate make mdatagen

// Package awsecsattributesprocessor enriches logs, metrics, traces and profiles
// with AWS ECS metadata for daemonset-style (one collector per host) ECS on EC2
// deployments.
package awsecsattributesprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/awsecsattributesprocessor"
