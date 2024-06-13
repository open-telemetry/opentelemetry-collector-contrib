package ibmstorageprotectreceiver

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"syscall"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configtelemetry"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// // ---------------------------------------------------------------//
// // Debug Exporter that exports data to console used for debugging //
// // ---------------------------------------------------------------//

// // Config defines configuration for debug exporter.

type ConfigDE struct {
	LogLevel zapcore.Level `mapstructure:"loglevel,omitempty"`

	// Verbosity defines the debug exporter verbosity.
	Verbosity configtelemetry.Level `mapstructure:"verbosity,omitempty"`

	//SamplingInitial defines how many samples are initially logged during each second.
	SamplingInitial int `mapstructure:"sampling_initial"`

	// SamplingThereafter defines the sampling rate after the initial samples are logged.
	SamplingThereafter int `mapstructure:"sampling_thereafter"`
}

type Common struct {
	Verbosity          configtelemetry.Level
	WarnLogLevel       bool
	LogLevel           zapcore.Level
	SamplingInitial    int
	SamplingThereafter int
}

type loggingExporter struct {
	verbosity        configtelemetry.Level
	logger           *zap.Logger
	metricsMarshaler pmetric.Marshaler
}

type dataBuffer struct {
	buf bytes.Buffer
}

// Exporter Component Name
var componentType = component.MustNewType("debug")

var onceWarnLogLevel sync.Once

var knownSyncErrors = []error{
	// sync /dev/stdout: invalid argument
	syscall.EINVAL,
	// sync /dev/stdout: not supported
	syscall.ENOTSUP,
	// sync /dev/stdout: inappropriate ioctl for device
	syscall.ENOTTY,
	// sync /dev/stdout: bad file descriptor
	syscall.EBADF,
}

var (
	supportedLevels map[configtelemetry.Level]struct{} = map[configtelemetry.Level]struct{}{
		configtelemetry.LevelDetailed: {}, //When set to detailed, pipeline data is verbosely logged
	}
)

const (
	defaultSamplingInitial    = 2
	defaultSamplingThereafter = 500
)

var _ component.Config = (*ConfigDE)(nil)
var _ confmap.Unmarshaler = (*ConfigDE)(nil)

// NewFactory creates a factory for Debug exporter
func NewFactoryDE() exporter.Factory {
	return exporter.NewFactory(
		componentType,
		createDefaultConfigDE,
		exporter.WithMetrics(createMetricsDExporter, MetricsStability))
}

func createDefaultConfigDE() component.Config {
	return &ConfigDE{
		LogLevel:           zapcore.InfoLevel,
		Verbosity:          configtelemetry.LevelDetailed,
		SamplingInitial:    defaultSamplingInitial,
		SamplingThereafter: defaultSamplingThereafter,
	}
}

// NewTextMetricsMarshaler returns a pmetric.Marshaler to encode to OTLP text bytes.
func NewTextMetricsMarshaler() pmetric.Marshaler {
	return textMetricsMarshaler{}
}

type textMetricsMarshaler struct{}

// Creating Debug Exporter Component
func createMetricsDExporter(ctx context.Context, set exporter.CreateSettings, config component.Config) (exporter.Metrics, error) {
	cfg := config.(*ConfigDE)
	return CreateMetricsDExporter(ctx, set, config, &Common{
		Verbosity:          cfg.Verbosity,
		LogLevel:           cfg.LogLevel,
		SamplingInitial:    cfg.SamplingInitial,
		SamplingThereafter: cfg.SamplingThereafter,
	})
}

func CreateMetricsDExporter(ctx context.Context, set exporter.CreateSettings, config component.Config, c *Common) (exporter.Metrics, error) {
	exporterLogger := c.createLogger(set.TelemetrySettings.Logger)
	s := newLoggingExporter(exporterLogger, c.Verbosity)
	return exporterhelper.NewMetricsExporter(ctx, set, config,
		s.pushMetrics,
		exporterhelper.WithCapabilities(consumer.Capabilities{MutatesData: false}),
		exporterhelper.WithTimeout(exporterhelper.TimeoutSettings{Timeout: 0}),
		exporterhelper.WithShutdown(LoggerSync(exporterLogger)),
	)
}

// Displaying received data to console in trace format
func (s *loggingExporter) pushMetrics(_ context.Context, md pmetric.Metrics) error {
	s.logger.Info("MetricsExporter",
		zap.Int("resource metrics", md.ResourceMetrics().Len()),
		zap.Int("metrics", md.MetricCount()),
		zap.Int("data points", md.DataPointCount()))
	if s.verbosity != configtelemetry.LevelDetailed {
		return nil
	}

	buf, err := s.metricsMarshaler.MarshalMetrics(md)
	if err != nil {
		return err
	}
	s.logger.Info("Output of debug exporter:")
	s.logger.Info(string(buf))
	return nil
}

// MarshalMetrics pmetric.Metrics to OTLP text.
func (textMetricsMarshaler) MarshalMetrics(md pmetric.Metrics) ([]byte, error) {
	buf := dataBuffer{}
	rms := md.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		buf.logEntry("ResourceMetrics #%d", i)
		rm := rms.At(i)
		buf.logAttributes("Resource attributes", rm.Resource().Attributes())
		ilms := rm.ScopeMetrics()
		for j := 0; j < ilms.Len(); j++ {
			buf.logEntry("ScopeMetrics #%d", j)
			ilm := ilms.At(j)
			buf.logEntry("ScopeMetrics SchemaURL: %s", ilm.SchemaUrl())
			buf.logInstrumentationScope(ilm.Scope())
			metrics := ilm.Metrics()
			for k := 0; k < metrics.Len(); k++ {
				buf.logEntry("Metric #%d", k)
				metric := metrics.At(k)
				buf.logMetricDescriptor(metric)
				buf.logMetricDataPoints(metric)
			}
		}
	}

	return buf.buf.Bytes(), nil
}

func (b *dataBuffer) logEntry(format string, a ...any) {
	b.buf.WriteString(fmt.Sprintf(format, a...))
	b.buf.WriteString("\n")
}

func (b *dataBuffer) logAttributes(header string, m pcommon.Map) {
	if m.Len() == 0 {
		return
	}

	b.logEntry("%s:", header)
	attrPrefix := "     ->"

	// Add offset to attributes if needed.
	headerParts := strings.Split(header, "->")
	if len(headerParts) > 1 {
		attrPrefix = headerParts[0] + attrPrefix
	}

	m.Range(func(k string, v pcommon.Value) bool {
		b.logEntry("%s %s: %s", attrPrefix, k, valueToString(v))
		return true
	})
}

func (b *dataBuffer) logInstrumentationScope(il pcommon.InstrumentationScope) {
	b.logEntry(
		"InstrumentationScope %s %s",
		il.Name(),
		il.Version())
	b.logAttributes("InstrumentationScope attributes", il.Attributes())
}

func (b *dataBuffer) logMetricDescriptor(md pmetric.Metric) {
	b.logEntry("Descriptor:")
	b.logEntry("     -> Name: %s", md.Name())
	b.logEntry("     -> Description: %s", md.Description())
	b.logEntry("     -> Unit: %s", md.Unit())
	b.logEntry("     -> DataType: %s", md.Type().String())
}

func (b *dataBuffer) logDataPointAttributes(attributes pcommon.Map) {
	b.logAttributes("Data point attributes", attributes)
}

func (b *dataBuffer) logExemplars(description string, se pmetric.ExemplarSlice) {
	if se.Len() == 0 {
		return
	}

	b.logEntry("%s:", description)

	for i := 0; i < se.Len(); i++ {
		e := se.At(i)
		b.logEntry("Exemplar #%d", i)
		b.logEntry("     -> Trace ID: %s", e.TraceID())
		b.logEntry("     -> Span ID: %s", e.SpanID())
		b.logEntry("     -> Timestamp: %s", e.Timestamp())
		switch e.ValueType() {
		case pmetric.ExemplarValueTypeInt:
			b.logEntry("     -> Value: %d", e.IntValue())
		case pmetric.ExemplarValueTypeDouble:
			b.logEntry("     -> Value: %f", e.DoubleValue())
		}
		b.logAttributes("     -> FilteredAttributes", e.FilteredAttributes())
	}
}

func valueToString(v pcommon.Value) string {
	return fmt.Sprintf("%s(%s)", v.Type().String(), v.AsString())
}

func (b *dataBuffer) logNumberDataPoints(ps pmetric.NumberDataPointSlice) {
	for i := 0; i < ps.Len(); i++ {
		p := ps.At(i)
		b.logEntry("NumberDataPoints #%d", i)
		b.logDataPointAttributes(p.Attributes())

		b.logEntry("StartTimestamp: %s", p.StartTimestamp())
		b.logEntry("Timestamp: %s", p.Timestamp())
		switch p.ValueType() {
		case pmetric.NumberDataPointValueTypeInt:
			b.logEntry("Value: %d", p.IntValue())
		case pmetric.NumberDataPointValueTypeDouble:
			b.logEntry("Value: %f", p.DoubleValue())
		}

		b.logExemplars("Exemplars", p.Exemplars())
	}
}

func (b *dataBuffer) logMetricDataPoints(m pmetric.Metric) {
	switch m.Type() {
	case pmetric.MetricTypeEmpty:
		return
	case pmetric.MetricTypeGauge:
		b.logNumberDataPoints(m.Gauge().DataPoints())
	}
}

func newLoggingExporter(logger *zap.Logger, verbosity configtelemetry.Level) *loggingExporter {
	return &loggingExporter{
		verbosity:        verbosity,
		logger:           logger,
		metricsMarshaler: NewTextMetricsMarshaler(),
	}
}

// knownSyncError returns true if the given error is one of the known
// non-actionable errors returned by Sync on Linux and macOS.
func knownSyncError(err error) bool {
	for _, syncError := range knownSyncErrors {
		if errors.Is(err, syncError) {
			return true
		}
	}

	return false
}

func LoggerSync(logger *zap.Logger) func(context.Context) error {
	return func(context.Context) error {
		// Currently Sync() return a different error depending on the OS.
		// Since these are not actionable ignore them.
		err := logger.Sync()
		osErr := &os.PathError{}
		if errors.As(err, &osErr) {
			wrappedErr := osErr.Unwrap()
			if knownSyncError(wrappedErr) {
				err = nil
			}
		}
		return err
	}
}

func (c *Common) createLogger(logger *zap.Logger) *zap.Logger {
	if c.WarnLogLevel {
		onceWarnLogLevel.Do(func() {
			logger.Warn(
				"'loglevel' option is deprecated in favor of 'verbosity'. Set 'verbosity' to equivalent value to preserve behavior.",
				zap.Stringer("loglevel", c.LogLevel),
				zap.Stringer("equivalent verbosity level", c.Verbosity),
			)
		})
	}
	core := zapcore.NewSamplerWithOptions(
		logger.Core(),
		1*time.Second,
		c.SamplingInitial,
		c.SamplingThereafter,
	)

	return zap.New(core)
}

func mapLevel(level zapcore.Level) (configtelemetry.Level, error) {
	switch level {
	case zapcore.DebugLevel:
		return configtelemetry.LevelDetailed, nil
	case zapcore.InfoLevel:
		return configtelemetry.LevelNormal, nil
	case zapcore.WarnLevel, zapcore.ErrorLevel,
		zapcore.DPanicLevel, zapcore.PanicLevel, zapcore.FatalLevel:
		// Anything above info is mapped to 'basic' level.
		return configtelemetry.LevelBasic, nil
	default:
		return configtelemetry.LevelNone, fmt.Errorf("log level %q is not supported", level)
	}
}

func (cfg *ConfigDE) Unmarshal(conf *confmap.Conf) error {
	if conf.IsSet("loglevel") && conf.IsSet("verbosity") {
		return fmt.Errorf("'loglevel' and 'verbosity' are incompatible. Use only 'verbosity' instead")
	}

	if err := conf.Unmarshal(cfg); err != nil {
		return err
	}

	if conf.IsSet("loglevel") {
		verbosity, err := mapLevel(cfg.LogLevel)
		if err != nil {
			return fmt.Errorf("failed to map 'loglevel': %w", err)
		}

		// 'verbosity' is unset but 'loglevel' is set.
		// Override default verbosity.
		cfg.Verbosity = verbosity
	}

	return nil
}

// Validate checks if the exporter configuration is valid
func (cfg *ConfigDE) Validate() error {
	if _, ok := supportedLevels[cfg.Verbosity]; !ok {
		return fmt.Errorf("verbosity level %q is not supported", cfg.Verbosity)
	}

	return nil
}
