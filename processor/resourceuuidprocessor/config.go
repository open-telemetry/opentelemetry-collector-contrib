package resourceuuidprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourceuuidprocessor"

import (
	"errors"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/component"
)

const (
	defaultEndpoint        = "http://localhost:2301/api/v1/provider/graph?kind=Pod&edges=false"
	defaultRefreshInterval = 2 * time.Minute
	defaultRetryInterval   = 2 * time.Minute
	defaultCacheTTL        = 10 * time.Minute
	defaultCacheSize       = 10000
	defaultTimeout         = 5 * time.Second
	defaultPodUIDAttribute = "k8s.pod.uid"
	defaultTargetAttribute = "k8s.pod.uuid"
)

// Config defines the configuration of the resource uuid processor.
type Config struct {
	// Endpoint is the eworker provider graph endpoint, ideally with kind=Pod&edges=false.
	Endpoint string `mapstructure:"endpoint"`
	// RefreshInterval is how often the whole pod list is pulled.
	RefreshInterval time.Duration `mapstructure:"refresh_interval"`
	// RetryInterval is how often the list is pulled again when a record referred to a pod that has no uuid yet.
	RetryInterval time.Duration `mapstructure:"retry_interval"`
	// CacheTTL is how long a pod uuid stays cached after it was last seen in the endpoint response.
	CacheTTL time.Duration `mapstructure:"cache_ttl"`
	// CacheSize is the maximum number of pods kept in the cache.
	CacheSize int `mapstructure:"cache_size"`
	// Timeout is the HTTP timeout of one poll.
	Timeout time.Duration `mapstructure:"timeout"`
	// PodUIDAttribute is the resource attribute holding the pod uid.
	PodUIDAttribute string `mapstructure:"pod_uid_attribute"`
	// TargetAttribute is the resource attribute the uuid is written to.
	TargetAttribute string `mapstructure:"target_attribute"`
}

var _ component.Config = (*Config)(nil)

// Validate checks if the processor configuration is valid.
func (cfg *Config) Validate() error {
	if cfg.Endpoint == "" {
		return errors.New("endpoint must be set")
	}
	if cfg.RefreshInterval <= 0 || cfg.RetryInterval <= 0 || cfg.CacheTTL <= 0 || cfg.Timeout <= 0 {
		return errors.New("refresh_interval, retry_interval, cache_ttl and timeout must be greater than 0")
	}
	if cfg.CacheSize <= 0 {
		return errors.New("cache_size must be greater than 0")
	}
	if cfg.RetryInterval > cfg.CacheTTL {
		return fmt.Errorf("retry_interval (%s) must not exceed cache_ttl (%s)", cfg.RetryInterval, cfg.CacheTTL)
	}
	if cfg.PodUIDAttribute == "" || cfg.TargetAttribute == "" {
		return errors.New("pod_uid_attribute and target_attribute must be set")
	}
	return nil
}
