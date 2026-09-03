// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sqlqueryreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlqueryreceiver"

import (
	"database/sql"

	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/xreceiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/sqlquery"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlqueryreceiver/internal/metadata"
)

func NewFactory() receiver.Factory {
	return xreceiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		xreceiver.WithLogs(createLogsReceiverFunc(sql.Open, sqlquery.NewDbClient), metadata.LogsStability),
		xreceiver.WithMetrics(createMetricsReceiverFunc(sql.Open, sqlquery.NewDbClient), metadata.MetricsStability),
		xreceiver.WithDeprecatedTypeAlias(metadata.DeprecatedType),
	)
}
