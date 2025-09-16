package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/storage/dbstorage"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/auditlogreceiver"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/extension/xextension/storage"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"
)

type testConsumer struct {
	logger *zap.Logger
}

func (tc *testConsumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (tc *testConsumer) ConsumeLogs(ctx context.Context, logs plog.Logs) error {
	tc.logger.Info("Received logs", zap.Int("count", logs.LogRecordCount()))
	for i := 0; i < logs.ResourceLogs().Len(); i++ {
		for j := 0; j < logs.ResourceLogs().At(i).ScopeLogs().Len(); j++ {
			for k := 0; k < logs.ResourceLogs().At(i).ScopeLogs().At(j).LogRecords().Len(); k++ {
				logRecord := logs.ResourceLogs().At(i).ScopeLogs().At(j).LogRecords().At(k)
				tc.logger.Info("Log received",
					zap.String("body", logRecord.Body().AsString()),
					zap.Any("timestamp", logRecord.Timestamp()))
			}
		}
	}
	return nil
}

func main() {
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	// Clean up any existing database file to avoid corruption issues
	dbFile := "./auditlog.db"
	if _, err := os.Stat(dbFile); err == nil {
		logger.Info("Removing existing database file to avoid corruption")
		if err := os.Remove(dbFile); err != nil {
			logger.Warn("Failed to remove existing database file", zap.Error(err))
		}
	}

	// Create SQL storage extension
	storageFactory := dbstorage.NewFactory()
	storageCfg := storageFactory.CreateDefaultConfig().(*dbstorage.Config)
	storageCfg.DriverName = "sqlite3"
	storageCfg.DataSource = "./auditlog.db"

	storageExt, err := storageFactory.Create(context.Background(), extension.Settings{
		ID:                component.NewID(component.MustNewType("db_storage")),
		TelemetrySettings: component.TelemetrySettings{Logger: logger},
	}, storageCfg)
	if err != nil {
		log.Fatalf("Failed to create storage extension: %v", err)
	}

	// Start storage extension
	if err := storageExt.Start(context.Background(), nil); err != nil {
		log.Fatalf("Failed to start storage extension: %v", err)
	}

	// Test SQL database connection with timeout
	logger.Info("Testing SQL database connection...")
	storageExtTyped, ok := storageExt.(storage.Extension)
	if !ok {
		log.Fatalf("Storage extension does not implement storage.Extension")
	}

	// Create a timeout context for SQL operations
	sqlCtx, sqlCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer sqlCancel()

	testClient, err := storageExtTyped.GetClient(sqlCtx, component.KindReceiver, component.NewID(component.MustNewType("db_storage")), "test")
	if err != nil {
		log.Fatalf("Failed to get SQL test client: %v", err)
	}

	// Test basic SQL operations
	testKey := "connection_test"
	testValue := []byte("test_value")
	if err := testClient.Set(sqlCtx, testKey, testValue); err != nil {
		log.Fatalf("Failed to set test value in SQL database: %v", err)
	}

	retrievedValue, err := testClient.Get(sqlCtx, testKey)
	if err != nil {
		log.Fatalf("Failed to get test value from SQL database: %v", err)
	}

	if string(retrievedValue) != string(testValue) {
		log.Fatalf("SQL test failed: expected %s, got %s", string(testValue), string(retrievedValue))
	}

	// Clean up test data
	if err := testClient.Delete(sqlCtx, testKey); err != nil {
		logger.Warn("Failed to clean up test data", zap.Error(err))
	}

	logger.Info("SQL database connection test successful!")

	// Create the receiver factory
	factory := auditlogreceiver.NewFactory()

	// Create config with specific endpoint and storage
	cfg := factory.CreateDefaultConfig().(*auditlogreceiver.Config)
	cfg.Endpoint = "0.0.0.0:4310"
	cfg.StorageID = component.NewID(component.MustNewType("db_storage"))

	// Create consumer
	consumer := &testConsumer{logger: logger}

	// Create receiver settings
	settings := receiver.Settings{
		ID:                component.NewID(factory.Type()),
		TelemetrySettings: component.TelemetrySettings{Logger: logger},
	}

	// Create the receiver directly using the internal function
	recv, err := auditlogreceiver.NewReceiver(cfg, settings, consumer)
	if err != nil {
		log.Fatalf("Failed to create receiver: %v", err)
	}

	// Create a simple host that provides the storage extension
	host := &simpleHost{
		Host:       componenttest.NewNopHost(),
		storageExt: storageExt,
	}

	// Start the receiver
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = recv.Start(ctx, host)
	if err != nil {
		log.Fatalf("Failed to start receiver: %v", err)
	}

	logger.Info("Audit log receiver started successfully with SQL storage")
	logger.Info("You can now send POST requests to http://localhost:4310/v1/logs with JSON data")

	// Wait for interrupt signal
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c

	logger.Info("Shutting down...")

	// Shutdown the receiver
	err = recv.Shutdown(ctx)
	if err != nil {
		log.Fatalf("Failed to shutdown receiver: %v", err)
	}

	// Shutdown storage extension
	if err := storageExt.Shutdown(ctx); err != nil {
		log.Fatalf("Failed to shutdown storage extension: %v", err)
	}
}

// simpleHost extends componenttest.NewNopHost() to provide storage extension
type simpleHost struct {
	component.Host
	storageExt extension.Extension
}

func (h *simpleHost) GetExtensions() map[component.ID]component.Component {
	return map[component.ID]component.Component{
		component.NewID(component.MustNewType("db_storage")): h.storageExt,
	}
}
