// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package solarwindsapmsettingsextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/solarwindsapmsettingsextension"

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/extension"
	"go.uber.org/zap"
)

const (
	jsonOutputFile       = "solarwinds-apm-settings.json"
	httpsContextDeadline = 10 * time.Second
)

type arguments struct {
	BucketCapacity               float64 `json:"BucketCapacity"`
	BucketRate                   float64 `json:"BucketRate"`
	TriggerRelaxedBucketCapacity float64 `json:"TriggerRelaxedBucketCapacity"`
	TriggerRelaxedBucketRate     float64 `json:"TriggerRelaxedBucketRate"`
	TriggerStrictBucketCapacity  float64 `json:"TriggerStrictBucketCapacity"`
	TriggerStrictBucketRate      float64 `json:"TriggerStrictBucketRate"`
}
type setting struct {
	Value     int64     `json:"value"`
	Flags     string    `json:"flags"`
	Timestamp int64     `json:"timestamp"`
	TTL       int64     `json:"ttl"`
	Arguments arguments `json:"arguments"`
}

type settingWithWarning struct {
	setting
	Warning string `json:"warning"`
}

type solarwindsapmSettingsExtension struct {
	config            *Config
	cancel            context.CancelFunc
	telemetrySettings component.TelemetrySettings
	client            *http.Client
	request           *http.Request
}

func newSolarwindsApmSettingsExtension(extensionCfg *Config, settings extension.Settings) (extension.Extension, error) {
	settingsExtension := &solarwindsapmSettingsExtension{
		config:            extensionCfg,
		telemetrySettings: settings.TelemetrySettings,
	}
	return settingsExtension, nil
}

func (extension *solarwindsapmSettingsExtension) Start(_ context.Context, host component.Host) error {
	extension.telemetrySettings.Logger.Info("starting up solarwinds apm settings extension")
	ctx := context.Background()
	ctx, extension.cancel = context.WithCancel(ctx)
	hostname, err := os.Hostname()
	if err != nil {
		return err
	}
	serviceName := resolveServiceNameBestEffort()
	keyArr := strings.Split(extension.config.Key, ":")
	if len(keyArr) == 2 {
		serviceName = keyArr[1]
	}
	httpClientConfig := confighttp.NewDefaultClientConfig()
	httpClientConfig.DisableKeepAlives = true
	extension.client, err = httpClientConfig.ToClient(ctx, host, extension.telemetrySettings)
	if err != nil {
		return err
	}
	extension.request, err = http.NewRequest(http.MethodGet, "https://"+extension.config.Endpoint+"/v1/settings/"+serviceName+"/"+hostname, http.NoBody)
	if err != nil {
		return err
	}
	extension.request.Header.Add("Authorization", "Bearer "+keyArr[0])

	outputFile := path.Join(os.TempDir(), jsonOutputFile)
	// initial refresh
	refresh(extension, outputFile)

	go func() {
		ticker := time.NewTicker(extension.config.Interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				refresh(extension, outputFile)
			case <-ctx.Done():
				extension.telemetrySettings.Logger.Info("received ctx.Done() from ticker")
				return
			}
		}
	}()

	return nil
}

func (extension *solarwindsapmSettingsExtension) Shutdown(_ context.Context) error {
	extension.telemetrySettings.Logger.Info("shutting down solarwinds apm settings extension")
	if extension.cancel != nil {
		extension.cancel()
	}
	return nil
}

func refresh(extension *solarwindsapmSettingsExtension, filename string) {
	extension.telemetrySettings.Logger.Info("time to refresh", zap.String("url", extension.request.URL.String()))
	ctx, cancel := context.WithTimeout(context.Background(), httpsContextDeadline)
	defer cancel()
	req := extension.request.WithContext(ctx)
	resp, err := extension.client.Do(req)
	if err != nil {
		extension.telemetrySettings.Logger.Error("unable to send request", zap.Error(err))
		return
	}
	defer func() {
		closeErr := resp.Body.Close()
		if closeErr != nil {
			extension.telemetrySettings.Logger.Error("unable to close response body", zap.Error(closeErr))
		}
	}()
	if resp.StatusCode != http.StatusOK {
		extension.telemetrySettings.Logger.Error("received non-OK HTTP status", zap.Int("StatusCode", resp.StatusCode))
		return
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		extension.telemetrySettings.Logger.Error("unable to read response body", zap.Error(err))
		return
	}
	var settingObj settingWithWarning
	err = json.Unmarshal(body, &settingObj)
	if err != nil {
		extension.telemetrySettings.Logger.Error("unable to unmarshal setting", zap.Error(err))
		return
	}
	var settings []setting
	settings = append(settings, settingObj.setting)
	if content, err := json.Marshal(settings); err != nil {
		extension.telemetrySettings.Logger.Error("unable to marshal setting JSON[] byte from settings", zap.Error(err))
	} else {
		if err := os.WriteFile(filename, content, 0o600); err != nil {
			extension.telemetrySettings.Logger.Error("unable to write "+filename, zap.Error(err))
		} else {
			if settingObj.Warning != "" {
				extension.telemetrySettings.Logger.Warn(filename + " is refreshed (soft disabled)")
			} else {
				extension.telemetrySettings.Logger.Info(filename + " is refreshed")
			}
			extension.telemetrySettings.Logger.Info(string(content))
		}
	}
}
