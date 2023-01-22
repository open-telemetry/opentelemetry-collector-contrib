package cassandraexporter

type Config struct {
	DSN        string `mapstructure:"dsn"`
	Keyspace   string `mapstructure:"keyspace"`
	TraceTable string `mapstructure:"trace_table"`
}
