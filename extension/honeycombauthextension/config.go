package honeycombauthextension

import (
	"errors"
	"go.opentelemetry.io/collector/config"
)

const (
	teamEnvKey         = "HONEYCOMB_TEAM"
	datasetEnvKey      = "HONEYCOMB_DATASET"
	teamMetadataKey    = "x-honeycomb-team"
	datasetMetadataKey = "x-honeycomb-dataset"
)

var (
	errNoTeamProvided    = errors.New("no team provided")
	errNoDatasetProvided = errors.New("no dataset provided")
)

type Config struct {
	config.ExtensionSettings `mapstructure:",squash"`
	Team                     string `mapstructure:"team"`
	Dataset                  string `mapstructure:"dataset"`
}

func (cfg *Config) Validate() error {
	if len(cfg.Team) == 0 {
		return errNoTeamProvided
	}
	if len(cfg.Dataset) == 0 {
		return errNoDatasetProvided
	}
	return nil
}
