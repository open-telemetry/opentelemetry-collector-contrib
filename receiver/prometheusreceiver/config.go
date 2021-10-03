// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package prometheusreceiver

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	commonconfig "github.com/prometheus/common/config"
	promconfig "github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/discovery/file"
	"github.com/prometheus/prometheus/discovery/kubernetes"
	"github.com/prometheus/prometheus/discovery/targetgroup"
	"go.opentelemetry.io/collector/config"
	"gopkg.in/yaml.v2"
)

const (
	// The key for Prometheus scraping configs.
	prometheusConfigKey = "config"
)

// Config defines configuration for Prometheus receiver.
type Config struct {
	config.ReceiverSettings `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct
	PrometheusConfig        *promconfig.Config       `mapstructure:"-"`
	BufferPeriod            time.Duration            `mapstructure:"buffer_period"`
	BufferCount             int                      `mapstructure:"buffer_count"`
	UseStartTimeMetric      bool                     `mapstructure:"use_start_time_metric"`
	StartTimeMetricRegex    string                   `mapstructure:"start_time_metric_regex"`

	// ConfigPlaceholder is just an entry to make the configuration pass a check
	// that requires that all keys present in the config actually exist on the
	// structure, ie.: it will error if an unknown key is present.
	ConfigPlaceholder interface{} `mapstructure:"config"`
}

var _ config.Receiver = (*Config)(nil)
var _ config.Unmarshallable = (*Config)(nil)

func checkFile(fn string) error {
	// Nothing set, nothing to error on.
	if fn == "" {
		return nil
	}
	_, err := os.Stat(fn)
	return err
}

func checkTLSConfig(tlsConfig commonconfig.TLSConfig) error {
	if err := checkFile(tlsConfig.CertFile); err != nil {
		return fmt.Errorf("error checking client cert file %q - %v", tlsConfig.CertFile, err)
	}
	if err := checkFile(tlsConfig.KeyFile); err != nil {
		return fmt.Errorf("error checking client key file %q - %v", tlsConfig.KeyFile, err)
	}
	if len(tlsConfig.CertFile) > 0 && len(tlsConfig.KeyFile) == 0 {
		return fmt.Errorf("client cert file %q specified without client key file", tlsConfig.CertFile)
	}
	if len(tlsConfig.KeyFile) > 0 && len(tlsConfig.CertFile) == 0 {
		return fmt.Errorf("client key file %q specified without client cert file", tlsConfig.KeyFile)
	}
	return nil
}

// Method to exercise the prometheus file discovery behavior to ensure there are no errors
// - reference https://github.com/prometheus/prometheus/blob/c0c22ed04200a8d24d1d5719f605c85710f0d008/discovery/file/file.go#L372
func checkSDFile(filename string) error {
	fd, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer fd.Close()

	content, err := ioutil.ReadAll(fd)
	if err != nil {
		return err
	}

	var targetGroups []*targetgroup.Group

	switch ext := filepath.Ext(filename); strings.ToLower(ext) {
	case ".json":
		if err := json.Unmarshal(content, &targetGroups); err != nil {
			return fmt.Errorf("Error in unmarshaling json file extension - %v", err)
		}
	case ".yml", ".yaml":
		if err := yaml.UnmarshalStrict(content, &targetGroups); err != nil {
			return fmt.Errorf("Error in unmarshaling yaml file extension - %v", err)
		}
	default:
		return fmt.Errorf("invalid file extension: %q", ext)
	}

	for i, tg := range targetGroups {
		if tg == nil {
			return fmt.Errorf("nil target group item found (index %d)", i)
		}
	}
	return nil
}

// Validate checks the receiver configuration is valid.
func (cfg *Config) Validate() error {
	promConfig := cfg.PrometheusConfig
	if promConfig == nil {
		return nil // noop receiver
	}
	if len(promConfig.ScrapeConfigs) == 0 {
		return errors.New("no Prometheus scrape_configs")
	}

	// Reject features that Prometheus supports but that the receiver doesn't support:
	// See:
	// * https://github.com/open-telemetry/opentelemetry-collector/issues/3863
	// * https://github.com/open-telemetry/wg-prometheus/issues/3
	unsupportedFeatures := make([]string, 0, 4)
	if len(promConfig.RemoteWriteConfigs) != 0 {
		unsupportedFeatures = append(unsupportedFeatures, "remote_write")
	}
	if len(promConfig.RemoteReadConfigs) != 0 {
		unsupportedFeatures = append(unsupportedFeatures, "remote_read")
	}
	if len(promConfig.RuleFiles) != 0 {
		unsupportedFeatures = append(unsupportedFeatures, "rule_files")
	}
	if len(promConfig.AlertingConfig.AlertRelabelConfigs) != 0 {
		unsupportedFeatures = append(unsupportedFeatures, "alert_config.relabel_configs")
	}
	if len(promConfig.AlertingConfig.AlertmanagerConfigs) != 0 {
		unsupportedFeatures = append(unsupportedFeatures, "alert_config.alertmanagers")
	}
	if len(unsupportedFeatures) != 0 {
		// Sort the values for deterministic error messages.
		sort.Strings(unsupportedFeatures)
		return fmt.Errorf("unsupported features:\n\t%s", strings.Join(unsupportedFeatures, "\n\t"))
	}

	for _, sc := range cfg.PrometheusConfig.ScrapeConfigs {
		for _, rc := range sc.MetricRelabelConfigs {
			if rc.TargetLabel == "__name__" {
				// TODO(#2297): Remove validation after renaming is fixed
				return fmt.Errorf("error validating scrapeconfig for job %v: %w", sc.JobName, errRenamingDisallowed)
			}
		}

		if sc.HTTPClientConfig.Authorization != nil {
			if err := checkFile(sc.HTTPClientConfig.Authorization.CredentialsFile); err != nil {
				return fmt.Errorf("error checking authorization credentials file %q - %s", sc.HTTPClientConfig.Authorization.CredentialsFile, err)
			}
		}

		if err := checkTLSConfig(sc.HTTPClientConfig.TLSConfig); err != nil {
			return err
		}

		for _, c := range sc.ServiceDiscoveryConfigs {
			switch c := c.(type) {
			case *kubernetes.SDConfig:
				if err := checkTLSConfig(c.HTTPClientConfig.TLSConfig); err != nil {
					return err
				}
			case *file.SDConfig:
				for _, file := range c.Files {
					files, err := filepath.Glob(file)
					if err != nil {
						return err
					}
					if len(files) != 0 {
						for _, f := range files {
							err = checkSDFile(f)
							if err != nil {
								return fmt.Errorf("checking SD file %q: %v", file, err)
							}
						}
						continue
					}
					return fmt.Errorf("file %q for file_sd in scrape job %q does not exist", file, sc.JobName)
				}
			}
		}
	}
	return nil
}

// Unmarshal a config.Parser into the config struct.
func (cfg *Config) Unmarshal(componentParser *config.Map) error {
	if componentParser == nil {
		return nil
	}
	// We need custom unmarshaling because prometheus "config" subkey defines its own
	// YAML unmarshaling routines so we need to do it explicitly.

	err := componentParser.UnmarshalExact(cfg)
	if err != nil {
		return fmt.Errorf("prometheus receiver failed to parse config: %s", err)
	}

	// Unmarshal prometheus's config values. Since prometheus uses `yaml` tags, so use `yaml`.
	promCfg, err := componentParser.Sub(prometheusConfigKey)
	if err != nil || len(promCfg.ToStringMap()) == 0 {
		return err
	}
	out, err := yaml.Marshal(promCfg.ToStringMap())
	if err != nil {
		return fmt.Errorf("prometheus receiver failed to marshal config to yaml: %s", err)
	}

	err = yaml.UnmarshalStrict(out, &cfg.PrometheusConfig)
	if err != nil {
		return fmt.Errorf("prometheus receiver failed to unmarshal yaml to prometheus config: %s", err)
	}

	return nil
}
