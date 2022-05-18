package expvarreceiver

import (
	"context"
	"net/http"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

type expVarScraper struct {
	cfg        *Config
	set        *component.ReceiverCreateSettings
	httpClient *http.Client
}

func newExpVarScraper(cfg *Config, set component.ReceiverCreateSettings) *expVarScraper {
	return &expVarScraper{
		cfg: cfg,
		set: &set,
	}
}

func (e expVarScraper) start(_ context.Context, host component.Host) error {
	httpClient, err := e.cfg.HTTP.ToClient(host.GetExtensions(), e.set.TelemetrySettings)
	if err != nil {
		return err
	}
	e.httpClient = httpClient
	return nil
}

func (e expVarScraper) scrape(ctx context.Context) (pmetric.Metrics, error) {
	//TODO implement me
	panic("implement me")
}
