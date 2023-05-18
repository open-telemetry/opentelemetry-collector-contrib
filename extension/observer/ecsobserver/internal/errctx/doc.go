// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package errctx allow attaching values to an error in a structural way
// using WithValue and read the value out using ValueFrom.
// It is inspired by context package and works along with error wrapping
// introduced in go 1.13.
package errctx // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/ecsobserver/internal/errctx"
