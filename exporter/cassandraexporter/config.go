package cassandraexporter

type Config struct {
	DSN         string      `mapstructure:"dsn"`
	Keyspace    string      `mapstructure:"keyspace"`
	TraceTable  string      `mapstructure:"trace_table"`
	LogsTable   string      `mapstructure:"logs_table"`
	Replication Replication `mapstructure:"replication"`
	Compression Compression `mapstructure:"compression"`
}

type Replication struct {
	Class             string `mapstructure:"class"`
	ReplicationFactor int    `mapstructure:"replication_factor"`
}

type Compression struct {
	Algorithm string `mapstructure:"algorithm"'`
}
