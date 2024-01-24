package elasticprocessor

type Config struct {
	AddSystemMetrics bool `mapstructure:"add_system_metrics"`
}

func (c *Config) Validate() error {
	return nil
}
