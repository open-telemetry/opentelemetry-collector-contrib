package changelist

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/migrate"
)

type ChangeList[T any] struct {
	Migrators []migrate.Migrator[T]
}

func (c ChangeList[T]) Apply(signal T) error {
	for i := 0; i < len(c.Migrators); i++ {
		migrator := c.Migrators[i]
		if err := migrator.Apply(signal); err != nil {
			return err
		}
	}
	return nil
}

func (c ChangeList[T]) Rollback(signal T) error {
	for i := 0; i < len(c.Migrators); i++ {
		migrator := c.Migrators[len(c.Migrators)-1-i]
		if err := migrator.Rollback(signal); err != nil {
			return err
		}
	}
	return nil
}