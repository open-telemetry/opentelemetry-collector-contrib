package sqlquery

const (
	DriverHDB       = "hdb"
	DriverMySQL     = "mysql"
	DriverOracle    = "oracle"
	DriverPostgres  = "postgres"
	DriverSnowflake = "snowflake"
	DriverSQLServer = "sqlserver"
	DriverTDS       = "tds"
)

// IsValidDriver checks if the given driver name is supported
func IsValidDriver(driver string) bool {
	switch driver {
	case DriverHDB, DriverMySQL, DriverOracle, DriverPostgres, DriverSnowflake, DriverSQLServer, DriverTDS:
		return true
	default:
		return false
	}
}
