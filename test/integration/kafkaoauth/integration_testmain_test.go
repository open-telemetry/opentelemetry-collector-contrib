//go:build integration

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkaoauth

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	kafkacontainers "github.com/open-telemetry/opentelemetry-collector-contrib/test/integration/kafkaoauth/testcontainers"
)

// kafkaOAuthEnv is a shared Kafka+Keycloak environment for the integration OAuth tests.
// It is created once per `go test` process, which keeps `-count=10` runs fast.
var kafkaOAuthEnv *kafkacontainers.Environment

func TestMain(m *testing.M) {
	// We intentionally do not pull images in tests. If they are missing, skip the suite.
	checkCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	missing, err := kafkacontainers.CheckImagesPresent(checkCtx)
	cancel()
	if err != nil {
		fmt.Fprintf(os.Stderr, "SKIP: Kafka OAuth integration tests require docker + pre-pulled images: %v\n", err)
		os.Exit(0)
	}
	if len(missing) > 0 {
		images := kafkacontainers.RequiredImages()
		fmt.Fprintf(os.Stderr, "SKIP: Kafka OAuth integration tests require pre-pulled images. Please run:\n  docker pull %s\n  docker pull %s\nMissing: %v\n", images[0], images[1], missing)
		os.Exit(0)
	}

	startCtx, startCancel := context.WithTimeout(context.Background(), 60*time.Second)
	env, err := kafkacontainers.NewEnvironmentWithTTL(startCtx, accessTokenTTL)
	startCancel()
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: failed to start Kafka OAuth integration environment: %v\n", err)
		os.Exit(1)
	}
	kafkaOAuthEnv = env

	code := m.Run()

	// Best-effort tear down. Note: `-timeout` in `go test` does not cover work after m.Run returns.
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 10*time.Second)
	_ = kafkaOAuthEnv.Close(stopCtx)
	stopCancel()

	os.Exit(code)
}
