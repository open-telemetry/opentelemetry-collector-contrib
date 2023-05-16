package tlscheckreceiver

import (
	"fmt"
	"github.com/juju/errors"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/tlscheckreceiver/internal/configtls"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/tlscheckreceiver/internal/metadata"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
	"go.uber.org/multierr"
	"net/url"
)

var (
	errInvalidEndpoint = errors.New(`"endpoint" must be in the form of <scheme>://<hostname>:<port>`)
	errInvalidCertPath = errors.New(`"local cert path invalid"`)
)

const defaultEndpoint = "http://localhost:433"

type Config struct {
	scraperhelper.ScraperControllerSettings `mapstructure:",squash"`
	configtls.TLSCertsClientSettings        `mapstructure:",squash"`

	metadata.MetricsBuilderConfig `mapstructure:",squash"`
}

// Validate validates the configuration by checking for missing or invalid fields
func (cfg *Config) Validate() error {
	var err error

	_, parseErr := url.Parse(cfg.Endpoint)
	if parseErr != nil {
		wrappedErr := fmt.Errorf("%s: %w", errInvalidEndpoint.Error(), parseErr)
		err = multierr.Append(err, wrappedErr)
	}

	return err
}
