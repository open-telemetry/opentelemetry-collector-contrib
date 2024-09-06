package migrate

type Migrator[T any] interface {
	Apply(data T) error
	Rollback(data T) error
}


var (
	//_ Migrator[plog.ScopeLogs] = (*operator.LogAttributeOperator)(nil)
)