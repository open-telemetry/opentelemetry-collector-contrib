// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package migrate // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/migrate"

// Migrator is an interface that all migration types must implement.  It is basically a marker interface.  All Transformers are also Migrators
type Migrator interface {
	IsMigrator()
}

var (
	_ Migrator = (*AttributeChangeSet)(nil)
	_ Migrator = (*MultiConditionalAttributeSet)(nil)
	_ Migrator = (*SignalNameChange)(nil)
	_ Migrator = (*ConditionalAttributeSet)(nil)
	_ Migrator = (*SignalNameChange)(nil)
)
