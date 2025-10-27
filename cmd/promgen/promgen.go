// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"flag"
	"fmt"
	"log"
	"maps"
	"math"
	"net/http"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"google.golang.org/protobuf/proto"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/aws/cloudwatch/histograms"
)

const updatePeriod = time.Second

func main() {
	readFile := flag.String("read", "", "Path to protobuf file to read and display")
	flag.Parse()

	if *readFile != "" {
		if err := readProtobufFile(*readFile); err != nil {
			log.Fatalf("Error reading protobuf file: %v", err)
		}
		return
	}

	start := time.Now()
	generator := NewGenerator()

	monotonicCounter := MetricDefinition{
		Name: "monotonic_counter",
		Type: TypeCounter,
		Help: "A counter that increases forever",
		Update: func(collector prometheus.Collector, _ time.Time) error {
			counter, err := collector.(*prometheus.CounterVec).GetMetricWith(prometheus.Labels{})
			if err != nil {
				return err
			}
			counter.Inc()
			return nil
		},
	}

	if err := generator.AddMetric(monotonicCounter); err != nil {
		log.Fatalf("unable to add metric: %v", err)
	}

	sinusoidalGauge := MetricDefinition{
		Name: "sinusoidal_gauge",
		Type: TypeGauge,
		Help: "A gauge that oscillates between -1 and 1",
		Update: func(collector prometheus.Collector, timestamp time.Time) error {
			gauge, err := collector.(*prometheus.GaugeVec).GetMetricWith(prometheus.Labels{})
			if err != nil {
				return err
			}
			newVal := math.Sin(2 * math.Pi * float64(timestamp.Unix()) / 20)
			gauge.Set(newVal)
			return nil
		},
	}

	if err := generator.AddMetric(sinusoidalGauge); err != nil {
		log.Fatalf("unable to add metric: %v", err)
	}

	gammaHistogram := MetricDefinition{
		Name: "gamma_histogram",
		Type: TypeHistogram,
		Help: "A histogram whose values follow a gamma distribution",
		Update: func(collector prometheus.Collector, _ time.Time) error {
			histogram, err := collector.(*prometheus.HistogramVec).GetMetricWith(prometheus.Labels{})
			if err != nil {
				return err
			}
			numObservations := generator.rand.Int() % 10
			for range numObservations {
				histogram.Observe(GammaRandom(generator.rand, 2.0, 2.0))
			}
			return nil
		},
	}

	if err := generator.AddMetric(gammaHistogram); err != nil {
		log.Fatalf("unable to add metric: %v", err)
	}

	exponentialSummary := MetricDefinition{
		Name: "exponential_summary",
		Type: TypeSummary,
		Help: "A summary whose values follow an exponential distribution",
		Update: func(collector prometheus.Collector, _ time.Time) error {
			summary, err := collector.(*prometheus.SummaryVec).GetMetricWith(prometheus.Labels{})
			if err != nil {
				return err
			}
			numObservations := generator.rand.Int() % 10
			for range numObservations {
				summary.Observe(generator.rand.ExpFloat64())
			}
			return nil
		},
	}

	if err := generator.AddMetric(exponentialSummary); err != nil {
		log.Fatalf("unable to add metric: %v", err)
	}

	gammaNativeHistogram := MetricDefinition{
		Name: "gamma_native_histogram",
		Type: TypeNativeHistogram,
		Help: "A native histogram whose values follow a gamma distribution",
		Update: func(collector prometheus.Collector, _ time.Time) error {
			histogram, err := collector.(*prometheus.HistogramVec).GetMetricWith(prometheus.Labels{})
			if err != nil {
				return err
			}
			numObservations := generator.rand.Int() % 10
			for range numObservations {
				histogram.Observe(GammaRandom(generator.rand, 2.0, 2.0))
			}
			return nil
		},
	}

	if err := generator.AddMetric(gammaNativeHistogram); err != nil {
		log.Fatalf("unable to add metric: %v", err)
	}

	testCases := histograms.TestCases()
	for _, tc := range testCases {
		tName := "tc_" + strings.ToLower(strings.ReplaceAll(tc.Name, " ", "_"))
		tMetricDefinition := MetricDefinition{
			Name: tName,
			Type: TypeHistogram,
			Help: tc.Name,
			CreateCollector: func() (prometheus.Collector, error) {
				// prometheus gives default buckets if boundaries is empty. we want one big bucket instead
				boundaries := tc.Input.Boundaries
				if len(boundaries) == 0 {
					boundaries = []float64{math.Inf(1)}
				}

				return prometheus.NewHistogramVec(
					prometheus.HistogramOpts{
						Name:    tName,
						Help:    "My first test case",
						Buckets: boundaries,
					},
					slices.Collect(maps.Keys(tc.Input.Attributes)),
				), nil
			},
			Update: func(collector prometheus.Collector, _ time.Time) error {
				// Only update once
				if time.Since(start) > 2*updatePeriod {
					return nil
				}
				histogram, err := collector.(*prometheus.HistogramVec).GetMetricWith(tc.Input.Attributes)
				if err != nil {
					return err
				}
				for _, v := range generateDatapoints(tc.Input) {
					histogram.Observe(v)
				}
				return nil
			},
		}

		if err := generator.AddMetric(tMetricDefinition); err != nil {
			log.Fatalf("unable to add metric: %v", err)
		}
	}

	// Start updating metrics periodically
	go func() {
		ticker := time.NewTicker(updatePeriod)
		defer ticker.Stop()
		for t := range ticker.C {
			if err := generator.UpdateMetrics(t); err != nil {
				log.Printf("Error updating metrics: %v", err)
			}
		}
	}()

	// Start HTTP server
	http.Handle("/metrics", generator)
	log.Printf("Starting server on :8080")
	server := &http.Server{
		Addr:         ":8080",
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	log.Fatal(server.ListenAndServe())
}

func generateDatapoints(in histograms.HistogramInput) []float64 {
	if in.Count == 0 {
		return []float64{}
	}

	dps := []float64{}
	totalGenerated := 0.0

	for i, count := range in.Counts {
		if count == 0 {
			continue
		}

		var bucketValue float64
		switch {
		case len(in.Boundaries) == 0:
			bucketValue = in.Sum / float64(in.Count)
		case i == 0:
			if in.Min != nil {
				bucketValue = (*in.Min + in.Boundaries[0]) / 2
			} else {
				bucketValue = in.Boundaries[0] - 1
			}
		case i < len(in.Boundaries):
			bucketValue = (in.Boundaries[i-1] + in.Boundaries[i]) / 2
		default:
			if in.Max != nil {
				bucketValue = (in.Boundaries[len(in.Boundaries)-1] + *in.Max) / 2
			} else {
				bucketValue = in.Boundaries[len(in.Boundaries)-1] + 1
			}
		}

		for j := uint64(0); j < count; j++ {
			dps = append(dps, bucketValue)
			totalGenerated += bucketValue
		}
	}

	if len(dps) > 0 && totalGenerated != 0 && len(in.Boundaries) > 0 {
		ratio := in.Sum / totalGenerated
		for i := range dps {
			dps[i] *= ratio
		}
	}
	return dps
}

// readProtobufFile reads and displays the contents of a protobuf file
func readProtobufFile(filename string) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read file %s: %w", filename, err)
	}

	fmt.Printf("File size: %d bytes\n", len(data))
	count := 0
	offset := 0

	for offset < len(data) {
		foundMessage := false

		// Try to find the next complete message by growing the slice
		for end := offset + 10; end <= len(data); end++ {
			var message dto.MetricFamily
			if err := proto.Unmarshal(data[offset:end], &message); err == nil && message.GetName() != "" {
				// Check if we have a valid message
				if end >= len(data) {
					// End of data
					if count == 0 {
						fmt.Printf("Successfully parsed MetricFamily messages:\n")
					}
					fmt.Printf("\n=== MetricFamily %d: %s ===\n", count+1, message.GetName())
					printMetricFamily(&message)
					count++
					offset = end
					foundMessage = true
					break
				}

				// Try parsing one more byte to see if message changes
				var nextMessage dto.MetricFamily
				if err := proto.Unmarshal(data[offset:end+1], &nextMessage); err != nil ||
					nextMessage.GetName() != message.GetName() ||
					len(nextMessage.GetMetric()) != len(message.GetMetric()) {
					// Message boundary found
					if count == 0 {
						fmt.Printf("Successfully parsed MetricFamily messages:\n")
					}
					fmt.Printf("\n=== MetricFamily %d: %s ===\n", count+1, message.GetName())
					printMetricFamily(&message)
					count++
					offset = end
					foundMessage = true
					break
				}
			}
		}

		if !foundMessage {
			break
		}
	}

	if count > 0 {
		fmt.Printf("\nTotal MetricFamilies: %d\n", count)
	} else {
		fmt.Printf("No MetricFamily messages found\n")
	}

	return nil
}

func printMetricFamily(mf *dto.MetricFamily) {
	fmt.Printf("MetricFamily: %s\n", mf.GetName())
	fmt.Printf("  Type: %s\n", mf.GetType())
	fmt.Printf("  Help: %s\n", mf.GetHelp())
	fmt.Printf("  Metrics: %d\n", len(mf.GetMetric()))

	for i, metric := range mf.GetMetric() {
		fmt.Printf("  Metric %d:\n", i)

		// Print labels
		if len(metric.GetLabel()) > 0 {
			fmt.Printf("    Labels: ")
			for _, label := range metric.GetLabel() {
				fmt.Printf("%s=%s ", label.GetName(), label.GetValue())
			}
			fmt.Printf("\n")
		}

		// Print metric value based on type
		switch mf.GetType() {
		case dto.MetricType_COUNTER:
			if counter := metric.GetCounter(); counter != nil {
				fmt.Printf("    Counter Value: %f\n", counter.GetValue())
			}
		case dto.MetricType_GAUGE:
			if gauge := metric.GetGauge(); gauge != nil {
				fmt.Printf("    Gauge Value: %f\n", gauge.GetValue())
			}
		case dto.MetricType_HISTOGRAM:
			if histogram := metric.GetHistogram(); histogram != nil {
				fmt.Printf("    Histogram Count: %d, Sum: %f\n", histogram.GetSampleCount(), histogram.GetSampleSum())
				fmt.Printf("    Buckets: %d\n", len(histogram.GetBucket()))
				for j, bucket := range histogram.GetBucket() {
					if j < 3 {
						fmt.Printf("      le=%f count=%d\n", bucket.GetUpperBound(), bucket.GetCumulativeCount())
					} else if j == 3 {
						fmt.Printf("      ... (%d more buckets)\n", len(histogram.GetBucket())-3)
						break
					}
				}
			}
		case dto.MetricType_SUMMARY:
			if summary := metric.GetSummary(); summary != nil {
				fmt.Printf("    Summary Count: %d, Sum: %f\n", summary.GetSampleCount(), summary.GetSampleSum())
				fmt.Printf("    Quantiles: %d\n", len(summary.GetQuantile()))
			}
		}

		if metric.GetTimestampMs() != 0 {
			fmt.Printf("    Timestamp: %d\n", metric.GetTimestampMs())
		}
	}
}
