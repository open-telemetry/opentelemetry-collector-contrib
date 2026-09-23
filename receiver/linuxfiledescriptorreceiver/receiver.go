package linuxfiledescriptorreceiver

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"
)

type receiverImpl struct {
	consumer consumer.Metrics
	config   *Config
	cancel   context.CancelFunc
	settings receiver.Settings
}

func newReceiver(
	settings receiver.Settings,
	config *Config,
	consumer consumer.Metrics,
) (receiver.Metrics, error) {
	if config.Interval <= 0 {
		return nil, fmt.Errorf("collection_interval must be greater than zero")
	}

	return &receiverImpl{
		settings: settings,
		config:   config,
		consumer: consumer,
	}, nil
}

func (r *receiverImpl) Start(ctx context.Context, _ component.Host) error {
	ctx, r.cancel = context.WithCancel(ctx)

	go func() {
		ticker := time.NewTicker(r.config.Interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				metrics, err := r.scrape()
				if err != nil {
					r.settings.Logger.Error(
						"Failed to collect Linux file descriptor metrics",
						zap.Error(err),
					)
					continue
				}

				if err := r.consumer.ConsumeMetrics(ctx, metrics); err != nil {
					r.settings.Logger.Error(
						"Failed to consume Linux file descriptor metrics",
						zap.Error(err),
					)
				}

			case <-ctx.Done():
				return
			}
		}
	}()

	return nil
}

func (r *receiverImpl) Shutdown(_ context.Context) error {
	if r.cancel != nil {
		r.cancel()
	}

	return nil
}

func (r *receiverImpl) scrape() (pmetric.Metrics, error) {
	path := r.config.Path
	if path == "" {
		path = "/proc/sys/fs/file-nr"
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return pmetric.Metrics{}, fmt.Errorf("reading %s: %w", path, err)
	}

	allocated, unused, maximum, err := parseFileNr(string(data))
	if err != nil {
		return pmetric.Metrics{}, err
	}

	usedPercent := float64(allocated) / float64(maximum) * 100

	metrics := pmetric.NewMetrics()
	resourceMetrics := metrics.ResourceMetrics().AppendEmpty()
	scopeMetrics := resourceMetrics.ScopeMetrics().AppendEmpty()

	addGauge(
		scopeMetrics.Metrics().AppendEmpty(),
		"linux.file_descriptors.allocated",
		float64(allocated),
	)

	addGauge(
		scopeMetrics.Metrics().AppendEmpty(),
		"linux.file_descriptors.unused",
		float64(unused),
	)

	addGauge(
		scopeMetrics.Metrics().AppendEmpty(),
		"linux.file_descriptors.maximum",
		float64(maximum),
	)

	addGauge(
		scopeMetrics.Metrics().AppendEmpty(),
		"linux.file_descriptors.used_percent",
		usedPercent,
	)

	return metrics, nil
}

func addGauge(metric pmetric.Metric, name string, value float64) {
	metric.SetName(name)
	metric.SetUnit("1")
	metric.SetDescription("Linux system-wide file descriptor metric")

	point := metric.SetEmptyGauge().DataPoints().AppendEmpty()
	point.SetDoubleValue(value)
}

func parseFileNr(data string) (uint64, uint64, uint64, error) {
	fields := strings.Fields(data)

	if len(fields) != 3 {
		return 0, 0, 0, fmt.Errorf(
			"expected 3 values in file-nr, got %d",
			len(fields),
		)
	}

	allocated, err := strconv.ParseUint(fields[0], 10, 64)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid allocated value: %w", err)
	}

	unused, err := strconv.ParseUint(fields[1], 10, 64)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid unused value: %w", err)
	}

	maximum, err := strconv.ParseUint(fields[2], 10, 64)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid maximum value: %w", err)
	}

	if maximum == 0 {
		return 0, 0, 0, fmt.Errorf("maximum file descriptors is zero")
	}

	if allocated > maximum {
		return 0, 0, 0, fmt.Errorf(
			"allocated file descriptors (%d) exceed maximum (%d)",
			allocated,
			maximum,
		)
	}

	return allocated, unused, maximum, nil
}
