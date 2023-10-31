// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datasetexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datasetexporter"

import (
	"context"
	"math/rand"
	"time"

	"github.com/scalyr/dataset-go/pkg/client"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"
)

const statsDelim = "_"

const metricsPrefix = "datasetexporter" + statsDelim

// reportStatistics provides metrics to the meter based on client.Statistics()
func reportStatistics(meter metric.Meter, client *client.DataSetClient) {
	// https://pkg.go.dev/go.opentelemetry.io/otel/metric#example-Meter-Asynchronous_multiple

	client.Logger.Info("AAAAA - reportStatistics - BEGIN")
	// transfer metrics - BEGIN
	transferBytesSent, err := meter.Int64ObservableUpDownCounter(
		metricsPrefix+"transfer"+statsDelim+"bytesSent",
		metric.WithDescription("Amount of sent data"),
		metric.WithUnit("By"),
	)
	if err != nil {
		client.Logger.Error(
			"failed to register up/down counter for transferBytesSent",
			zap.Error(err),
		)
		return
	}

	transferBytesAccepted, err := meter.Int64ObservableUpDownCounter(
		metricsPrefix+"transfer"+statsDelim+"bytesAccepted",
		metric.WithDescription("Amount of accepted data"),
		metric.WithUnit("By"),
	)
	if err != nil {
		client.Logger.Error(
			"failed to register up/down counter for transferBytesAccepted",
			zap.Error(err),
		)
		return
	}

	transferBuffersProcessed, err := meter.Int64ObservableUpDownCounter(
		metricsPrefix+"transfer"+statsDelim+"buffersProcessed",
		metric.WithDescription("Amount of processed buffers"),
		metric.WithUnit("{tot}"),
	)
	if err != nil {
		client.Logger.Error(
			"failed to register up/down counter for transferBuffersProcessed",
			zap.Error(err),
		)
		return
	}

	transferAvgBufferBytes, err := meter.Float64ObservableUpDownCounter(
		metricsPrefix+"transfer"+statsDelim+"avgBufferBytes",
		metric.WithDescription("Average size of the buffer"),
		metric.WithUnit("By"),
	)
	if err != nil {
		client.Logger.Error(
			"failed to register up/down counter for transferAvgBufferBytes",
			zap.Error(err),
		)
		return
	}

	transferThroughputBpS, err := meter.Float64ObservableUpDownCounter(
		metricsPrefix+"transfer"+statsDelim+"throughputBpS",
		metric.WithDescription("Throughput"),
		metric.WithUnit("By/s"),
	)
	if err != nil {
		client.Logger.Error(
			"failed to register up/down counter for transferThroughputBpS",
			zap.Error(err),
		)
		return
	}

	transferSuccessRate, err := meter.Float64ObservableUpDownCounter(
		metricsPrefix+"transfer"+statsDelim+"successRate",
		metric.WithDescription("Success rate of data transfers"),
	)
	if err != nil {
		client.Logger.Error(
			"failed to register up/down counter for transferSuccessRate",
			zap.Error(err),
		)
		return
	}

	transferProcessingTime, err := meter.Float64ObservableUpDownCounter(
		metricsPrefix+"transfer"+statsDelim+"processingTime",
		metric.WithDescription("Processing time"),
		metric.WithUnit("s"),
	)
	if err != nil {
		client.Logger.Error(
			"failed to register up/down counter for transferProcessingTime",
			zap.Error(err),
		)
		return
	}
	// transfer metrics - END

	// events metrics - BEGIN
	eventsSuccessRate, err := meter.Float64ObservableUpDownCounter(
		metricsPrefix+"events"+statsDelim+"successRate",
		metric.WithDescription("Success rate of events processing"),
	)
	if err != nil {
		client.Logger.Error(
			"failed to register up/down counter for transferSuccessRate",
			zap.Error(err),
		)
		return
	}

	eventsProcessingTime, err := meter.Float64ObservableUpDownCounter(
		metricsPrefix+"events"+statsDelim+"processingTime",
		metric.WithDescription("Processing time"),
		metric.WithUnit("s"),
	)
	if err != nil {
		client.Logger.Error(
			"failed to register up/down counter for transferProcessingTime",
			zap.Error(err),
		)
		return
	}

	eventsBroken, err := meter.Int64ObservableUpDownCounter(
		metricsPrefix+"events"+statsDelim+"broken",
		metric.WithDescription("Amount of broken events"),
		metric.WithUnit("{tot}"),
	)
	if err != nil {
		client.Logger.Error(
			"failed to register up/down counter for eventsBroken",
			zap.Error(err),
		)
		return
	}

	eventsDropped, err := meter.Int64ObservableUpDownCounter(
		metricsPrefix+"events"+statsDelim+"dropped",
		metric.WithDescription("Amount of dropped events"),
		metric.WithUnit("{tot}"),
	)
	if err != nil {
		client.Logger.Error(
			"failed to register up/down counter for eventsDropped",
			zap.Error(err),
		)
		return
	}

	eventsEnqueued, err := meter.Int64ObservableUpDownCounter(
		metricsPrefix+"events"+statsDelim+"enqueued",
		metric.WithDescription("Amount of broken events"),
		metric.WithUnit("{tot}"),
	)
	if err != nil {
		client.Logger.Error(
			"failed to register up/down counter for eventsEnqueued",
			zap.Error(err),
		)
		return
	}

	eventsProcessed, err := meter.Int64ObservableUpDownCounter(
		metricsPrefix+"events"+statsDelim+"dropped",
		metric.WithDescription("Amount of processed events"),
		metric.WithUnit("{tot}"),
	)
	if err != nil {
		client.Logger.Error(
			"failed to register up/down counter for eventsProcessed",
			zap.Error(err),
		)
		return
	}

	eventsWaiting, err := meter.Int64ObservableUpDownCounter(
		metricsPrefix+"events"+statsDelim+"waiting",
		metric.WithDescription("Amount of processed events"),
		metric.WithUnit("{tot}"),
	)
	if err != nil {
		client.Logger.Error(
			"failed to register up/down counter for eventsWaiting",
			zap.Error(err),
		)
		return
	}
	// events metrics - END

	// buffers metrics - BEGIN
	buffersSuccessRate, err := meter.Float64ObservableUpDownCounter(
		metricsPrefix+"buffers"+statsDelim+"successRate",
		metric.WithDescription("Success rate of buffers processing"),
	)
	if err != nil {
		client.Logger.Error(
			"failed to register up/down counter for transferSuccessRate",
			zap.Error(err),
		)
		return
	}

	buffersProcessingTime, err := meter.Float64ObservableUpDownCounter(
		metricsPrefix+"buffers"+statsDelim+"processingTime",
		metric.WithDescription("Processing time"),
		metric.WithUnit("s"),
	)
	if err != nil {
		client.Logger.Error(
			"failed to register up/down counter for transferProcessingTime",
			zap.Error(err),
		)
		return
	}

	buffersBroken, err := meter.Int64ObservableUpDownCounter(
		metricsPrefix+"buffers"+statsDelim+"broken",
		metric.WithDescription("Amount of broken buffers"),
		metric.WithUnit("{tot}"),
	)
	if err != nil {
		client.Logger.Error(
			"failed to register up/down counter for buffersBroken",
			zap.Error(err),
		)
		return
	}

	buffersDropped, err := meter.Int64ObservableUpDownCounter(
		metricsPrefix+"buffers"+statsDelim+"dropped",
		metric.WithDescription("Amount of dropped buffers"),
		metric.WithUnit("{tot}"),
	)
	if err != nil {
		client.Logger.Error(
			"failed to register up/down counter for buffersDropped",
			zap.Error(err),
		)
		return
	}

	buffersEnqueued, err := meter.Int64ObservableUpDownCounter(
		metricsPrefix+"buffers"+statsDelim+"enqueued",
		metric.WithDescription("Amount of broken buffers"),
		metric.WithUnit("{tot}"),
	)
	if err != nil {
		client.Logger.Error(
			"failed to register up/down counter for buffersEnqueued",
			zap.Error(err),
		)
		return
	}

	buffersProcessed, err := meter.Int64ObservableUpDownCounter(
		metricsPrefix+"buffers"+statsDelim+"dropped",
		metric.WithDescription("Amount of processed buffers"),
		metric.WithUnit("{tot}"),
	)
	if err != nil {
		client.Logger.Error(
			"failed to register up/down counter for buffersProcessed",
			zap.Error(err),
		)
		return
	}

	buffersWaiting, err := meter.Int64ObservableUpDownCounter(
		metricsPrefix+"buffers"+statsDelim+"waiting",
		metric.WithDescription("Amount of processed buffers"),
		metric.WithUnit("{tot}"),
	)
	if err != nil {
		client.Logger.Error(
			"failed to register up/down counter for buffersWaiting",
			zap.Error(err),
		)
		return
	}
	// buffers metrics - END

	client.Logger.Info("AAAAA - reportStatistics - REGISTRATION")

	simpleCounterLoop, err := meter.Int64UpDownCounter(metricsPrefix + "SimpleCounterLoop")

	i := int64(0)
	go func() {
		r := rand.New(rand.NewSource(99))
		for {
			if r.Float64() < 0.4 {
				client.Logger.Info("AAAAA - reportStatistics - SimpleCounter - down by", zap.Int64("foo", i))
				simpleCounterLoop.Add(context.Background(), -1*i)
			} else {
				client.Logger.Info("AAAAA - reportStatistics - SimpleCounter - up by", zap.Int64("foo", i))
				simpleCounterLoop.Add(context.Background(), i)
			}

			time.Sleep(time.Duration(10 * r.Float64() * float64(time.Second.Nanoseconds())))
			i += 1
		}
	}()

	simpleCounterCallback, err := meter.Int64UpDownCounter(metricsPrefix + "SimpleCounterCallback")

	gaugeEventsProcessed, err := meter.Int64ObservableGauge(metricsPrefix + "gauge" + statsDelim + "eventsProcessed")

	client.Logger.Info("AAAAA - reportStatistics - Add Simple")

	_, err = meter.RegisterCallback(
		func(_ context.Context, o metric.Observer) error {
			client.Logger.Info("AAAAA - reportStatistics - CALLBACK BEGIN")
			stats := client.Statistics()

			client.Logger.Info("AAAAA - reportStatistics - STATS", zap.Bool("stats_is_nil", stats == nil))

			if stats == nil {
				return nil
			}

			simpleCounterCallback.Add(context.Background(), i)
			o.ObserveInt64(gaugeEventsProcessed, int64(stats.Events.Processed()))

			// transfer metrics
			o.ObserveInt64(transferBytesSent, int64(stats.Transfer.BytesSent()))
			o.ObserveInt64(transferBytesAccepted, int64(stats.Transfer.BytesAccepted()))
			o.ObserveInt64(transferBuffersProcessed, int64(stats.Transfer.BuffersProcessed()))
			o.ObserveFloat64(transferAvgBufferBytes, stats.Transfer.AvgBufferBytes())
			o.ObserveFloat64(transferThroughputBpS, stats.Transfer.ThroughputBpS())
			o.ObserveFloat64(transferSuccessRate, stats.Transfer.SuccessRate())
			o.ObserveFloat64(transferProcessingTime, stats.Transfer.ProcessingTime().Seconds())

			// events metrics
			o.ObserveFloat64(eventsSuccessRate, stats.Events.SuccessRate())
			o.ObserveFloat64(eventsProcessingTime, stats.Events.ProcessingTime().Seconds())
			o.ObserveInt64(eventsBroken, int64(stats.Events.Broken()))
			o.ObserveInt64(eventsDropped, int64(stats.Events.Dropped()))
			o.ObserveInt64(eventsEnqueued, int64(stats.Events.Enqueued()))
			o.ObserveInt64(eventsProcessed, int64(stats.Events.Processed()))
			o.ObserveInt64(eventsWaiting, int64(stats.Events.Waiting()))

			// buffers metrics
			o.ObserveFloat64(buffersSuccessRate, stats.Buffers.SuccessRate())
			o.ObserveFloat64(buffersProcessingTime, stats.Buffers.ProcessingTime().Seconds())
			o.ObserveInt64(buffersBroken, int64(stats.Buffers.Broken()))
			o.ObserveInt64(buffersDropped, int64(stats.Buffers.Dropped()))
			o.ObserveInt64(buffersEnqueued, int64(stats.Buffers.Enqueued()))
			o.ObserveInt64(buffersProcessed, int64(stats.Buffers.Processed()))
			o.ObserveInt64(buffersWaiting, int64(stats.Buffers.Waiting()))

			client.Logger.Info("AAAAA - reportStatistics - CALLBACK - END")
			return nil
		},
		// test
		gaugeEventsProcessed,
		// transfer metrics
		transferBytesSent,
		transferBytesAccepted,
		transferBuffersProcessed,
		transferAvgBufferBytes,
		transferThroughputBpS,
		transferSuccessRate,
		transferProcessingTime,
		// events metrics
		eventsSuccessRate,
		eventsProcessingTime,
		eventsBroken,
		eventsDropped,
		eventsEnqueued,
		eventsProcessed,
		eventsWaiting,
		// buffers metrics
		buffersSuccessRate,
		buffersProcessingTime,
		buffersBroken,
		buffersDropped,
		buffersEnqueued,
		buffersProcessed,
		buffersWaiting,
	)

	client.Logger.Info("AAAAA - reportStatistics - END", zap.Bool("err_is_nil", err == nil))

	if err != nil {
		client.Logger.Error(
			"failed to register callback",
			zap.Error(err),
		)
		return
	}
}
