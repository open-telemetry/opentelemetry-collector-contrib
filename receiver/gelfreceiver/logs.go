package gelfreceiver

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	rcvr "go.opentelemetry.io/collector/receiver"

	// "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/gelf"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"github.com/Graylog2/go-gelf/gelf"
	"go.uber.org/zap"
)

type logsReceiver struct {
	gelfReader *gelf.Reader
	wg         *sync.WaitGroup
	cancel     context.CancelFunc
	logger     *zap.Logger
	consumer   consumer.Logs
	config     *Config
}

func (gelfRcv *logsReceiver) Start(ctx context.Context, host component.Host) error {
	fmt.Println("Starting")
	gelfRcv.logger.Log(zap.InfoLevel,"Starting GELF")
	ctx, cancel := context.WithCancel(context.Background())
	gelfRcv.cancel = cancel
	gelfReader, err := gelf.NewReader(gelfRcv.config.ListenAddress)
	if err != nil {
		return fmt.Errorf("failed to open connection: %w", err)
	}
	gelfRcv.gelfReader = gelfReader

	// FIXME: Set the size of the buffer using config. 
	buf := make([]byte, gelfRcv.config.LogBuffer)
	gelfRcv.wg.Add(1)
	for {
		msg, err := gelfReader.Read(buf)
		// fmt.Println("Reading ended!")
		str := string(buf[:msg])
		// fmt.Println(msg, err, str)
		if err == io.EOF {
			break
		}
		// FIXME: handle timestamp from config
		observedTime := pcommon.NewTimestampFromTime(time.Now())
		if err := gelfRcv.consumer.ConsumeLogs(ctx, gelfRcv.processLogs(observedTime, str)); err != nil {
			fmt.Print("Error consuming logs")
		}
	}

	return nil
}

func (gelfRcv *logsReceiver) Shutdown(ctx context.Context) error {
	// close(gelfRcv.gelfReader) //or write the GELF code in pkg and add a close connection function.
	// gelfRcv.wg.Wait()
	return nil
}

func newLogsReceiver(params rcvr.Settings, cfg *Config, logger *zap.Logger, consumer consumer.Logs) (*logsReceiver, error) {
	recv := &logsReceiver{
		config:   cfg,
		consumer: consumer,
		logger:   params.Logger,
		wg:       &sync.WaitGroup{},
	}

	return recv, nil
}

// Add a function for processing GELF messages
func (gelfRcv *logsReceiver) processLogs(now pcommon.Timestamp, message string) plog.Logs{
	logs := plog.NewLogs()
	resourceLogs := logs.ResourceLogs().AppendEmpty()
	resource := resourceLogs.Resource()
	resource.Attributes().PutStr("service.name", "testing")
	scopeLogs := resourceLogs.ScopeLogs().AppendEmpty()
	scopeLogs.Scope().SetName("gelf-receiver")
	logRecord := scopeLogs.LogRecords().AppendEmpty()
	logRecord.Body().SetStr(message)
	logRecord.SetTimestamp(now)
	
	return logs
}