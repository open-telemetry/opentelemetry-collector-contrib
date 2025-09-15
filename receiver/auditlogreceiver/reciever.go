package auditlogreceiver

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"
)

// AuditLogEntry represents a stored audit log entry
type AuditLogEntry struct {
	ID        string    `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	Body      []byte    `json:"body"`
	Processed bool      `json:"processed"`
}

type auditLogReceiver struct {
	logger       *zap.Logger
	consumer     consumer.Logs
	server       *http.Server
	storage      map[string][]byte
	cfg          *Config
	ctx          context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
	entryCounter int64
	mutex        sync.RWMutex
}

func newReceiver(cfg *Config, set receiver.Settings, consumer consumer.Logs) (*auditLogReceiver, error) {
	ctx, cancel := context.WithCancel(context.Background())
	
	r := &auditLogReceiver{
		logger:   set.Logger,
		consumer: consumer,
		storage:  make(map[string][]byte),
		cfg:      cfg,
		ctx:      ctx,
		cancel:   cancel,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/logs", r.handleAuditLogs)

	r.server = &http.Server{
		Addr:    cfg.Endpoint, // comes from confighttp.ServerConfig
		Handler: mux,
	}

	return r, nil
}

func (r *auditLogReceiver) Start(ctx context.Context, host component.Host) error {
	// Start HTTP server
	go func() {
		if err := r.server.ListenAndServe(); err != http.ErrServerClosed {
			r.logger.Error("HTTP server error", zap.Error(err))
		}
	}()

	// Start processing goroutine
	r.wg.Add(1)
	go r.processStoredLogs()

	return nil
}

func (r *auditLogReceiver) Shutdown(ctx context.Context) error {
	// Cancel context to stop processing goroutine
	r.cancel()
	
	// Wait for processing goroutine to finish
	r.wg.Wait()
	
	// Shutdown HTTP server
	return r.server.Shutdown(ctx)
}

// processStoredLogs processes audit logs stored in persistence memory
func (r *auditLogReceiver) processStoredLogs() {
	defer r.wg.Done()

	ticker := time.NewTicker(1 * time.Second) // Process every second
	defer ticker.Stop()

	for {
		select {
		case <-r.ctx.Done():
			r.logger.Info("Stopping audit log processing goroutine")
			return
		case <-ticker.C:
			r.processPendingLogs()
		}
	}
}

// processPendingLogs processes all unprocessed audit log entries
func (r *auditLogReceiver) processPendingLogs() {
	r.mutex.RLock()
	keys := make([]string, 0, len(r.storage))
	for key := range r.storage {
		keys = append(keys, key)
	}
	r.mutex.RUnlock()
	
	for _, key := range keys {
		r.mutex.RLock()
		data, exists := r.storage[key]
		r.mutex.RUnlock()
		
		if !exists {
			continue
		}
		
		var entry AuditLogEntry
		if err := json.Unmarshal(data, &entry); err != nil {
			r.logger.Error("Failed to unmarshal audit log entry", zap.String("key", key), zap.Error(err))
			continue
		}
		
		// Skip if already processed
		if entry.Processed {
			continue
		}
		
		// Process the audit log
		if err := r.processAuditLog(&entry); err != nil {
			r.logger.Error("Failed to process audit log", zap.String("key", key), zap.Error(err))
			continue
		}
		
		// Mark as processed
		entry.Processed = true
		processedData, err := json.Marshal(entry)
		if err != nil {
			r.logger.Error("Failed to marshal processed entry", zap.String("key", key), zap.Error(err))
			continue
		}
		
		r.mutex.Lock()
		r.storage[key] = processedData
		r.mutex.Unlock()
	}
}


// processAuditLog processes a single audit log entry
func (r *auditLogReceiver) processAuditLog(entry *AuditLogEntry) error {
	// Log to Collector logger
	r.logger.Info("Processing audit log", 
		zap.String("id", entry.ID),
		zap.Time("timestamp", entry.Timestamp),
		zap.ByteString("body", entry.Body))
	
	// Create OpenTelemetry log record
	logs := plog.NewLogs()
	resourceLogs := logs.ResourceLogs().AppendEmpty()
	scopeLogs := resourceLogs.ScopeLogs().AppendEmpty()
	logRecord := scopeLogs.LogRecords().AppendEmpty()
	
	// Set timestamp - will be set automatically by the collector
	
	// Set severity
	logRecord.SetSeverityNumber(plog.SeverityNumberInfo)
	logRecord.SetSeverityText("INFO")
	
	// Set body
	logRecord.Body().SetStr(string(entry.Body))
	
	// Add attributes
	attrs := logRecord.Attributes()
	attrs.PutStr("audit_log_id", entry.ID)
	attrs.PutStr("receiver", "auditlogreceiver")
	
	// Send to consumer
	ctx := context.Background()
	return r.consumer.ConsumeLogs(ctx, logs)
}

func (r *auditLogReceiver) handleAuditLogs(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "only POST allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check content type for OTLP
	contentType := req.Header.Get("Content-Type")
	if contentType != "application/x-protobuf" && contentType != "application/json" {
		http.Error(w, "unsupported content type, expected application/x-protobuf or application/json", http.StatusBadRequest)
		return
	}

	body, err := io.ReadAll(req.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}
	defer req.Body.Close()

	// Generate unique ID for this entry
	r.mutex.Lock()
	r.entryCounter++
	entryID := fmt.Sprintf("audit_log_%d", r.entryCounter)
	key := entryID
	r.mutex.Unlock()

	// Create audit log entry
	entry := AuditLogEntry{
		ID:        entryID,
		Timestamp: time.Now(),
		Body:      body,
		Processed: false,
	}

	// Marshal entry to JSON
	entryData, err := json.Marshal(entry)
	if err != nil {
		r.logger.Error("Failed to marshal audit log entry", zap.Error(err))
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Store in persistence memory
	r.mutex.Lock()
	r.storage[key] = entryData
	r.mutex.Unlock()

	r.logger.Info("Stored audit log entry", zap.String("id", entryID), zap.String("content_type", contentType))

	// Return accepted immediately
	w.WriteHeader(http.StatusAccepted)
}
