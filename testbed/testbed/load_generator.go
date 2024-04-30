// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package testbed // import "github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/collector/consumer/consumererror"
	"golang.org/x/text/message"
)

var printer = message.NewPrinter(message.MatchLanguage("en"))

// LoadGenerator is intended to be exercised by a TestCase to generate and send telemetry to an OtelcolRunner instance.
// The simplest ready implementation is the ProviderSender that unites a DataProvider with a DataSender.
type LoadGenerator interface {
	Start(options LoadOptions)
	Stop()
	IsReady() bool
	DataItemsSent() uint64
	IncDataItemsSent()
	PermanentErrors() uint64
	GetStats() string
}

// LoadOptions defines the options to use for generating the load.
type LoadOptions struct {
	// DataItemsPerSecond specifies how many spans, metric data points, or log
	// records to generate each second.
	DataItemsPerSecond int

	// ItemsPerBatch specifies how many spans, metric data points, or log
	// records per batch to generate. Should be greater than zero. The number
	// of batches generated per second will be DataItemsPerSecond/ItemsPerBatch.
	ItemsPerBatch int

	// Attributes to add to each generated data item. Can be empty.
	Attributes map[string]string

	// Parallel specifies how many goroutines to send from.
	Parallel int
}

var _ LoadGenerator = (*ProviderSender)(nil)

// ProviderSender is a simple load generator.
type ProviderSender struct {
	Provider DataProvider
	Sender   DataSender

	// Number of data items (spans or metric data points) sent.
	dataItemsSent atomic.Uint64
	startedAt     time.Time
	startMutex    sync.Mutex

	// Number of permanent errors received
	permanentErrors    atomic.Uint64
	nonPermanentErrors atomic.Uint64

	stopOnce   sync.Once
	stopWait   sync.WaitGroup
	stopSignal chan struct{}

	options LoadOptions

	// Record information about previous errors to avoid flood of error messages.
	prevErr  error
	sendType string
}

// NewLoadGenerator creates a ProviderSender to send DataProvider-generated telemetry via a DataSender.
func NewLoadGenerator(dataProvider DataProvider, sender DataSender) (LoadGenerator, error) {
	if sender == nil {
		return nil, fmt.Errorf("cannot create load generator without DataSender")
	}

	ps := &ProviderSender{
		stopSignal: make(chan struct{}),
		Sender:     sender,
		Provider:   dataProvider,
	}

	switch t := ps.Sender.(type) {
	case TraceDataSender:
		ps.sendType = "traces"
	case MetricDataSender:
		ps.sendType = "metrics"
	case LogDataSender:
		ps.sendType = "logs"
	default:
		ps.sendType = fmt.Sprintf("invalid-%T", t)
	}

	return ps, nil
}

// Start the load.
func (ps *ProviderSender) Start(options LoadOptions) {
	ps.options = options

	if ps.options.ItemsPerBatch == 0 {
		// 10 items per batch by default.
		ps.options.ItemsPerBatch = 10
	}

	log.Printf("Starting load generator at %d items/sec.", ps.options.DataItemsPerSecond)

	// Indicate that generation is in progress.
	ps.stopWait.Add(1)

	// Begin generation
	go ps.generate()
	ps.startMutex.Lock()
	defer ps.startMutex.Unlock()
	ps.startedAt = time.Now()
}

// Stop the load.
func (ps *ProviderSender) Stop() {
	ps.stopOnce.Do(func() {
		// Signal generate() to stop.
		close(ps.stopSignal)

		// Wait for it to stop.
		ps.stopWait.Wait()

		// Print stats.
		log.Printf("Stopped generator. %s", ps.GetStats())
	})
}

func (ps *ProviderSender) IsReady() bool {
	endpoint := ps.Sender.GetEndpoint()
	if endpoint == nil {
		return true
	}
	conn, err := net.Dial(ps.Sender.GetEndpoint().Network(), ps.Sender.GetEndpoint().String())
	if err == nil && conn != nil {
		conn.Close()
		return true
	}
	return false
}

// GetStats returns the stats as a printable string.
func (ps *ProviderSender) GetStats() string {
	ps.startMutex.Lock()
	defer ps.startMutex.Unlock()
	sent := ps.DataItemsSent()
	return printer.Sprintf("Sent:%10d %s (%d/sec)", sent, ps.sendType, int(float64(sent)/time.Since(ps.startedAt).Seconds()))
}

func (ps *ProviderSender) DataItemsSent() uint64 {
	return ps.dataItemsSent.Load()
}

func (ps *ProviderSender) PermanentErrors() uint64 {
	return ps.permanentErrors.Load()
}

func (ps *ProviderSender) NonPermanentErrors() uint64 {
	return ps.nonPermanentErrors.Load()
}

// IncDataItemsSent is used when a test bypasses the ProviderSender and sends data
// directly via its Sender. This is necessary so that the total number of sent
// items in the end is correct, because the reports are printed from ProviderSender's
// fields. This is not the best way, a better approach would be to refactor the
// reports to use their own counter and load generator and other sending sources
// to contribute to this counter. This could be done as a future improvement.
func (ps *ProviderSender) IncDataItemsSent() {
	ps.dataItemsSent.Add(1)
}

func (ps *ProviderSender) generate() {
	// Indicate that generation is done at the end
	defer ps.stopWait.Done()

	if ps.options.DataItemsPerSecond == 0 {
		return
	}

	ps.Provider.SetLoadGeneratorCounters(&ps.dataItemsSent)

	err := ps.Sender.Start()
	if err != nil {
		log.Printf("Cannot start sender: %v", err)
		return
	}

	numWorkers := 1

	if ps.options.Parallel > 0 {
		numWorkers = ps.options.Parallel
	}

	var workers sync.WaitGroup

	for i := 0; i < numWorkers; i++ {
		workers.Add(1)

		go func() {
			defer workers.Done()
			t := time.NewTicker(time.Second / time.Duration(ps.options.DataItemsPerSecond/ps.options.ItemsPerBatch/numWorkers))
			defer t.Stop()
			for {
				select {
				case <-t.C:
					switch ps.Sender.(type) {
					case TraceDataSender:
						ps.generateTrace()
					case MetricDataSender:
						ps.generateMetrics()
					case LogDataSender:
						ps.generateLog()
					default:
						log.Printf("Invalid type of ProviderSender sender")
					}
				case <-ps.stopSignal:
					return
				}
			}
		}()
	}

	workers.Wait()

	// Send all pending generated data.
	ps.Sender.Flush()
}

func (ps *ProviderSender) generateTrace() {
	traceSender := ps.Sender.(TraceDataSender)

	traceData, done := ps.Provider.GenerateTraces()
	if done {
		return
	}

	for {
		err := traceSender.ConsumeTraces(context.Background(), traceData)
		if err == nil {
			ps.prevErr = nil
			break
		}

		if !consumererror.IsPermanent(err) {
			ps.nonPermanentErrors.Add(uint64(traceData.SpanCount()))
			continue
		}

		ps.permanentErrors.Add(uint64(traceData.SpanCount()))

		// update prevErr to err if it's different than last observed error
		if ps.prevErr == nil || ps.prevErr.Error() != err.Error() {
			ps.prevErr = err
			log.Printf("Cannot send traces: %v", err)
		}
		break
	}
}

func (ps *ProviderSender) generateMetrics() {
	metricSender := ps.Sender.(MetricDataSender)

	metricData, done := ps.Provider.GenerateMetrics()
	if done {
		return
	}

	for {
		err := metricSender.ConsumeMetrics(context.Background(), metricData)
		if err == nil {
			ps.prevErr = nil
			break
		}

		if !consumererror.IsPermanent(err) {
			ps.nonPermanentErrors.Add(uint64(metricData.DataPointCount()))
			continue
		}

		ps.permanentErrors.Add(uint64(metricData.DataPointCount()))

		// update prevErr to err if it's different than last observed error
		if ps.prevErr == nil || ps.prevErr.Error() != err.Error() {
			ps.prevErr = err
			log.Printf("Cannot send metrics: %v", err)
		}
		break
	}
}

func (ps *ProviderSender) generateLog() {
	logSender := ps.Sender.(LogDataSender)

	logData, done := ps.Provider.GenerateLogs()
	if done {
		return
	}
	for {
		err := logSender.ConsumeLogs(context.Background(), logData)
		if err == nil {
			ps.prevErr = nil
			break
		}

		if !consumererror.IsPermanent(err) {
			ps.nonPermanentErrors.Add(uint64(logData.LogRecordCount()))
			continue
		}

		ps.permanentErrors.Add(uint64(logData.LogRecordCount()))

		// update prevErr to err if it's different than last observed error
		if ps.prevErr == nil || ps.prevErr.Error() != err.Error() {
			ps.prevErr = err
			log.Printf("Cannot send logs: %v", err)
		}
		break
	}
}
