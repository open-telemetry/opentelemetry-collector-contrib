package statusreporterextension

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.uber.org/zap"
)

// statusReporterExtension implements an extension that exposes health information
// about the collector at the "/api/otel-status" endpoint, accessible from other pods.
// It provides live status of traces and metrics exporters by scraping the metrics endpoint
// when the API endpoint is called.
type statusReporterExtension struct {
	config *Config
	component.TelemetrySettings
	cancel context.CancelFunc
	server *http.Server
	mu     sync.RWMutex           // Protects concurrent access to status
	status map[string]interface{} // Used only as fallback if metrics endpoint is unreachable

	// Track previous values of metrics to detect changes
	prevMetricsSent map[string]int64 // Previous values of otelcol_exporter_sent_metric_points_total
	lastSuccessTime map[string]int64 // Last time a successful export was detected
}

var _ extension.Extension = (*statusReporterExtension)(nil)

// newStatusReporterExtension creates a new statusReporterExtension with the given config and telemetry settings.
func newStatusReporterExtension(cfg *Config, tel component.TelemetrySettings) extension.Extension {
	return &statusReporterExtension{
		config:            cfg,
		TelemetrySettings: tel,
		prevMetricsSent:   make(map[string]int64),
		lastSuccessTime:   make(map[string]int64),
	}
}

// Start initializes and starts the status reporter HTTP server.
// It implements the extension.Extension interface.
func (h *statusReporterExtension) Start(ctx context.Context, host component.Host) error {
	ctx, cancel := context.WithCancel(ctx)
	h.cancel = cancel

	// Initialize status map (only used as fallback if metrics endpoint is unreachable)
	h.status = make(map[string]interface{})
	h.status = map[string]interface{}{
		"engine_id": h.config.EngineID,
		"pod_name":  h.config.PodName,
		"traces": map[string]interface{}{
			"connection_status": "Unknown",
			"last_sync_ts":      0,
			"error_message":     nil,
			"timestamp":         time.Now().Unix(),
		},
		"metrics": map[string]interface{}{
			"connection_status": "Unknown",
			"last_sync_ts":      0,
			"error_message":     nil,
			"timestamp":         time.Now().Unix(),
		},
	}

	// Start HTTP server to expose the status endpoint
	mux := http.NewServeMux()
	mux.HandleFunc("/api/otel-status", h.statusHandler)

	h.server = &http.Server{
		Addr:    ":" + strconv.Itoa(h.config.Port), // Listen on all interfaces using configured port
		Handler: mux,
	}

	go func() {
		if err := h.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			h.Logger.Error("Error starting status reporter HTTP server", zap.Error(err))
		}
	}()

	h.Logger.Info("Status reporter extension started", zap.String("endpoint", ":"+strconv.Itoa(h.config.Port)+"/api/otel-status"))
	return nil
}

// Shutdown stops the status reporter HTTP server and background monitoring.
// It implements the extension.Extension interface.
func (h *statusReporterExtension) Shutdown(ctx context.Context) error {
	if h.cancel != nil {
		h.cancel()
	}

	// Shutdown the HTTP server
	if h.server != nil {
		if err := h.server.Shutdown(ctx); err != nil {
			h.Logger.Error("Error shutting down status reporter HTTP server", zap.Error(err))
		}
	}

	h.Logger.Info("Status reporter extension stopped")
	return nil
}

// statusHandler handles GET requests to the /api/otel-status endpoint.
// It returns the current status information as JSON by collecting live status
// directly from the metrics endpoint when called.
// If authentication is enabled, it validates the secret header against the LH_INSTANCE_SECRET
// environment variable or the configured secret value.
func (h *statusReporterExtension) statusHandler(w http.ResponseWriter, r *http.Request) {
	// Only allow GET requests
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check authentication if enabled
	if h.config.AuthEnabled {
		// Get the secret from the header
		secretHeader := r.Header.Get("Secret")

		// Get the expected secret from environment variable or config
		lhInstanceSecret := os.Getenv("LH_INSTANCE_SECRET")
		if lhInstanceSecret == "" {
			lhInstanceSecret = h.config.SecretValue
		}

		// Validate the secret
		if secretHeader != lhInstanceSecret {
			h.Logger.Warn("Authentication failed for status endpoint",
				zap.String("remote_addr", r.RemoteAddr),
				zap.String("user_agent", r.UserAgent()))
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		h.Logger.Debug("Authentication successful for status endpoint",
			zap.String("remote_addr", r.RemoteAddr))
	}

	// Collect live status at the time of API call
	status := h.collectStatus()

	// Log status information
	tracesStatus, _ := status["traces"].(map[string]interface{})["connection_status"].(string)
	metricsStatus, _ := status["metrics"].(map[string]interface{})["connection_status"].(string)
	h.Logger.Debug("Status endpoint called",
		zap.String("engine_id", h.config.EngineID),
		zap.String("traces_status", tracesStatus),
		zap.String("metrics_status", metricsStatus))

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(status); err != nil {
		h.Logger.Error("Failed to encode status response", zap.Error(err))
	}
}

// collectStatus collects the current live status by scraping the metrics endpoint.
// This is the primary function for getting status information and is called
// directly when the API endpoint is accessed.
func (h *statusReporterExtension) collectStatus() map[string]interface{} {
	resp, err := http.Get(h.config.MetricsEndpoint)
	if err != nil {
		h.Logger.Error("failed to scrape /metrics", zap.Error(err))
		// Return last known status if available, or a default status
		h.mu.RLock()
		defer h.mu.RUnlock()
		if h.status != nil {
			return h.status
		}
		return map[string]interface{}{
			"engine_id": h.config.EngineID,
			"pod_name":  h.config.PodName,
			"traces": map[string]interface{}{
				"connection_status": "Unknown",
				"last_sync_ts":      0,
				"error_message":     "Failed to scrape metrics endpoint",
				"timestamp":         time.Now().Unix(),
			},
			"metrics": map[string]interface{}{
				"connection_status": "Unknown",
				"last_sync_ts":      0,
				"error_message":     "Failed to scrape metrics endpoint",
				"timestamp":         time.Now().Unix(),
			},
		}
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	metrics := string(body)

	// Initialize metrics for traces and metrics
	tracesErrors := int64(0)
	tracesErrorMsg := ""
	metricsErrors := int64(0)
	metricsErrorMsg := ""

	// Current values of sent metrics
	tracesSentPoints := int64(0)
	metricsSentPoints := int64(0)

	// Get current timestamps from our stored map
	h.mu.RLock()
	tracesLastSuccess := h.lastSuccessTime["traces"]
	metricsLastSuccess := h.lastSuccessTime["metrics"]
	prevTracesSent := h.prevMetricsSent["traces"]
	prevMetricsSent := h.prevMetricsSent["metrics"]
	h.mu.RUnlock()

	// Parse metrics data
	for _, line := range strings.Split(metrics, "\n") {
		if strings.HasPrefix(line, "#") || len(line) == 0 {
			continue // skip comments and empty lines
		}

		parts := strings.Fields(line)
		if len(parts) != 2 {
			continue
		}

		val, err := strconv.ParseInt(strings.Split(parts[1], ".")[0], 10, 64)
		if err != nil {
			continue
		}

		switch {
		// --- Traces success ---
		case strings.HasPrefix(line, "otelcol_exporter_sent_spans_total"):
			tracesSentPoints = val
			if val > prevTracesSent {
				tracesLastSuccess = time.Now().Unix()
			}

		// --- Traces errors ---
		case strings.HasPrefix(line, "otelcol_exporter_send_failed_spans_total"):
			if val > 0 {
				tracesErrors = val
			}

		// --- Metrics success ---
		case strings.HasPrefix(line, "otelcol_exporter_sent_metric_points_total"):
			metricsSentPoints = val
			if val > prevMetricsSent {
				metricsLastSuccess = time.Now().Unix()
			}

		// --- Metrics errors ---
		case strings.HasPrefix(line, "otelcol_exporter_send_failed_metric_points_total"):
			if err == nil && val > 0 {
				metricsErrors = val
			}

		// Also check for successful RPC calls as an additional indicator
		case strings.HasPrefix(line, "rpc_client_duration_milliseconds_count") &&
			strings.Contains(line, "rpc_service=\"opentelemetry.proto.collector.metrics.v1.MetricsService\"") &&
			strings.Contains(line, "rpc_grpc_status_code=\"0\""):
			parts := strings.Fields(line)
			if len(parts) == 2 {
				if val > 0 {
					// If we have successful RPC calls and no previous timestamp, set one
					if metricsLastSuccess == 0 {
						metricsLastSuccess = time.Now().Unix()
					}
				}
			}
		}
	}

	// Update our stored values
	h.mu.Lock()
	h.prevMetricsSent["traces"] = tracesSentPoints
	h.prevMetricsSent["metrics"] = metricsSentPoints
	h.lastSuccessTime["traces"] = tracesLastSuccess
	h.lastSuccessTime["metrics"] = metricsLastSuccess
	h.mu.Unlock()

	now := time.Now().Unix()

	// Determine traces status
	tracesStatus := "Active"
	if tracesErrors > 0 {
		tracesStatus = "Failed"
		tracesErrorMsg = fmt.Sprintf("Failed to send %d traces", tracesErrors)
	} else if tracesLastSuccess == 0 {
		tracesStatus = "Unknown"
	} else if now-tracesLastSuccess > int64(h.config.StaleThreshold) {
		tracesStatus = "Warning"
	}

	// Determine metrics status
	metricsStatus := "Active"
	if metricsErrors > 0 {
		metricsStatus = "Failed"
		metricsErrorMsg = fmt.Sprintf("Failed to send %d metrics", metricsErrors)
	} else if metricsLastSuccess == 0 {
		metricsStatus = "Unknown"
	} else if now-metricsLastSuccess > int64(h.config.StaleThreshold) {
		metricsStatus = "Warning"
	}

	// Prepare and return payload
	status := map[string]interface{}{
		"engine_id": h.config.EngineID,
		"pod_name":  h.config.PodName,
		"traces": map[string]interface{}{
			"connection_status": tracesStatus,
			"last_sync_ts":      tracesLastSuccess,
			"error_message":     tracesErrorMsg,
			"timestamp":         now,
		},
		"metrics": map[string]interface{}{
			"connection_status": metricsStatus,
			"last_sync_ts":      metricsLastSuccess,
			"error_message":     metricsErrorMsg,
			"timestamp":         now,
		},
	}

	// Update the stored status
	h.mu.Lock()
	h.status = status
	h.mu.Unlock()

	return status
}
