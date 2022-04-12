package vmwarevcenterreceiver

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.uber.org/multierr"
	"go.uber.org/zap"
)

var _ component.Receiver = (*vcenterReceiver)(nil)

type vcenterReceiver struct {
	config  *Config
	logger  *zap.Logger
	scraper component.Receiver
}

func (v *vcenterReceiver) Start(ctx context.Context, host component.Host) error {
	var err error
	if v.scraper != nil {
		scraperErr := v.scraper.Start(ctx, host)
		if scraperErr != nil {
			err = multierr.Append(err, scraperErr)
		}
	}
	return err
}

func (v *vcenterReceiver) Shutdown(ctx context.Context) error {
	var err error
	if v.scraper != nil {
		scraperErr := v.scraper.Shutdown(ctx)
		if scraperErr != nil {
			err = multierr.Append(err, scraperErr)
		}
	}
	return err
}
