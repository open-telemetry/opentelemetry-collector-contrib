package statusreporterextension

// Config holds the configuration for the statusreporter extension.
// This extension exposes health information about the collector at the "/api/otel-status" endpoint.
type Config struct {
	MetricsEndpoint string `mapstructure:"metrics_endpoint"` // endpoint to scrape metrics from
	StaleThreshold  int    `mapstructure:"stale_threshold"`  // threshold in seconds to consider data stale
	EngineID        string `mapstructure:"engine_id"`        // identifier for the engine (e.g., presto-01)
	PodName         string `mapstructure:"pod_name"`         // name of the pod where OTeL sidecar is attached
	Port            int    `mapstructure:"port"`             // port on which the status reporter server listens
	AuthEnabled     bool   `mapstructure:"auth_enabled"`     // whether authentication is enabled
	SecretValue     string `mapstructure:"secret_value"`     // fallback secret value if LH_INSTANCE_SECRET is not set
}
