// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package scraperinttest // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/scraperinttest"

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode"

	"github.com/testcontainers/testcontainers-go"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
)

func EqualsLatestMetrics(expected pmetric.Metrics, sink *consumertest.MetricsSink, compareOpts []pmetrictest.CompareMetricsOption) func() bool {
	return func() bool {
		allMetrics := sink.AllMetrics()
		return len(allMetrics) > 0 && nil == pmetrictest.CompareMetrics(expected, allMetrics[len(allMetrics)-1], compareOpts...)
	}
}

func RunScript(script []string) testcontainers.ContainerHook {
	return func(ctx context.Context, container testcontainers.Container) error {
		code, r, err := container.Exec(ctx, script)
		if err != nil {
			return err
		}
		if code == 0 {
			return nil
		}

		// Try to read the error message for the sake of debugging
		errBytes, readerErr := io.ReadAll(r)
		if readerErr != nil {
			return fmt.Errorf("setup script returned non-zero exit code: %d", code)
		}

		// Error message may have non-printable chars, so clean it up
		errStr := strings.Map(func(r rune) rune {
			if unicode.IsPrint(r) {
				return r
			}
			return -1
		}, string(errBytes))
		return errors.New(strings.TrimSpace(errStr))
	}
}
