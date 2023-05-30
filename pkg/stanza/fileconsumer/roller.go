// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fileconsumer // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer"

import "context"

type roller interface {
	readLostFiles(context.Context, []*Reader)
	roll(context.Context, []*Reader)
	cleanup()
}
