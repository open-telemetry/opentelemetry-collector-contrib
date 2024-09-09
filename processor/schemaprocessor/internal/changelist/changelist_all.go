package changelist
//
//import (
//	"go.opentelemetry.io/collector/pdata/plog"
//
//	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/migrate"
//	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/operator"
//)
//
//type ChangeListAll[T any] struct {
//	Migrators []migrate.Migrator[T]
//}
//
//func (c ChangeListAll[T]) Apply(signal T) error {
//	for i := 0; i < len(c.Migrators); i++ {
//		migrator := c.Migrators[i]
//		switch typ := signal.(type) {
//		case plog.Logs:
//
//		}
//		if err := migrator.Apply(signal); err != nil {
//			return err
//		}
//	}
//	return nil
//}
//
//func (c ChangeListAll[T]) Rollback(signal T) error {
//	for i := 0; i < len(c.Migrators); i++ {
//		migrator := c.Migrators[len(c.Migrators)-1-i]
//		if err := migrator.Rollback(signal); err != nil {
//			return err
//		}
//	}
//	return nil
//}
