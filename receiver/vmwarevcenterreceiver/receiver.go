package vmwarevcenterreceiver

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.uber.org/multierr"
	"go.uber.org/zap"
)

var _ component.Receiver = (*vcenterReceiver)(nil)

type vcenterReceiver struct {
	config       *Config
	logger       *zap.Logger
	scraper      component.Receiver
	logsReceiver component.Receiver
}

func (v *vcenterReceiver) Start(ctx context.Context, host component.Host) error {
	var err error
	if v.scraper != nil {
		scraperErr := v.scraper.Start(ctx, host)
		if scraperErr != nil {
			err = multierr.Append(err, scraperErr)
		}
	}
	if v.logsReceiver != nil {
		logErr := v.logsReceiver.Start(ctx, host)
		if logErr != nil {
			err = multierr.Append(err, logErr)
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
	if v.logsReceiver != nil {
		logErr := v.logsReceiver.Shutdown(ctx)
		if logErr != nil {
			err = multierr.Append(err, logErr)
		}
	}
	return err
}
