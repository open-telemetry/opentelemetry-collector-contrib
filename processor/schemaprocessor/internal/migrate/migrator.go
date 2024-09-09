package migrate

type Migrator interface {
	IsMigrator()
}


var (
	_ Migrator = (*AttributeChangeSet)(nil)
	_ Migrator = (*MultiConditionalAttributeSet)(nil)
)