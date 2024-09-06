package operator

import (
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/migrate"
)

type LogAttributeOperator struct {
	AttributeChange migrate.AttributeChangeSet
}

func (o LogAttributeOperator) Apply(logs plog.ScopeLogs) error {
	for l := 0; l < logs.LogRecords().Len(); l++ {
		log := logs.LogRecords().At(l)
		if err := o.AttributeChange.Apply(log.Attributes()); err != nil {
			return err
		}
	}
	return nil
}


func (o LogAttributeOperator) Rollback(logs plog.ScopeLogs) error {
	for l := 0; l < logs.LogRecords().Len(); l++ {
		log := logs.LogRecords().At(l)
		if err := o.AttributeChange.Rollback(log.Attributes()); err != nil {
			return err
		}
	}
	return nil
}

