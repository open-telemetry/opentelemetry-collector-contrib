// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"log"
	"math/rand/v2"
	"net"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	logspb "go.opentelemetry.io/proto/otlp/collector/logs/v1" //nolint:depguard // The harness intentionally implements the OTLP logs gRPC service.
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"       //nolint:depguard // The harness intentionally builds raw OTLP protobuf requests.
	logpb "go.opentelemetry.io/proto/otlp/logs/v1"            //nolint:depguard // The harness intentionally builds raw OTLP protobuf requests.
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"   //nolint:depguard // The harness intentionally builds raw OTLP protobuf requests.
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	_ "google.golang.org/grpc/encoding/gzip"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
)

type server struct {
	logspb.UnimplementedLogsServiceServer
	healthpb.UnimplementedHealthServer

	started     time.Time
	cfg         config
	tallyClient *http.Client

	requests    atomic.Int64
	received    atomic.Int64
	records     atomic.Int64
	slow        atomic.Int64
	failures    atomic.Int64
	inflight    atomic.Int64
	maxInflight atomic.Int64
	draining    atomic.Bool
}

type config struct {
	grpcAddr      string
	httpAddr      string
	maxRecvMiB    int
	slowAfter     time.Duration
	slowFor       time.Duration
	slowSleep     time.Duration
	slowModulo    int
	slowRemainder int
	failAfter     time.Duration
	failFor       time.Duration
	unreadyAfter  time.Duration
	unreadyFor    time.Duration
	exitAfter     time.Duration
	tallyURL      string
	mode          string
}

type tallyServer struct {
	received atomic.Int64
	accepted atomic.Int64
	failures atomic.Int64
}

type loadgenConfig struct {
	endpoint          string
	duration          time.Duration
	rate              int
	workers           int
	batchSize         int
	allowFailures     bool
	payloadProfile    string
	payloadSizeBytes  int
	body              string
	serviceName       string
	tenant            string
	compression       string
	randomPayloadSeed int64
}

func main() {
	cfg := config{
		grpcAddr:      envString("OTLP_GRPC_ADDR", ":4317"),
		httpAddr:      envString("HTTP_ADDR", ":13133"),
		maxRecvMiB:    envInt("MAX_RECV_MSG_SIZE_MIB", 64),
		slowAfter:     envDuration("SLOW_AFTER", 0),
		slowFor:       envDuration("SLOW_FOR", 0),
		slowSleep:     envDuration("SLOW_SLEEP", 6*time.Second),
		slowModulo:    envInt("SLOW_MODULO", 0),
		slowRemainder: envInt("SLOW_REMAINDER", 0),
		failAfter:     envDuration("FAIL_AFTER", 0),
		failFor:       envDuration("FAIL_FOR", 0),
		tallyURL:      envString("TALLY_URL", ""),
		mode:          envString("MODE", "backend"),
		// Readiness is separate from liveness so the LB resolver can be tested
		// against terminating/unready EndpointSlice conditions.
		unreadyAfter: envDuration("UNREADY_AFTER", 0),
		unreadyFor:   envDuration("UNREADY_FOR", 0),
		exitAfter:    envDuration("EXIT_AFTER", 0),
	}

	if cfg.mode == "loadgen" {
		if err := runLoadgen(context.Background(), loadgenConfigFromEnv()); err != nil {
			log.Fatalf("run loadgen: %v", err)
		}
		return
	}

	if cfg.mode == "tally" {
		t := &tallyServer{}
		mux := http.NewServeMux()
		mux.HandleFunc("/add", t.handleAdd)
		mux.HandleFunc("/metrics", t.handleMetrics)
		mux.HandleFunc("/healthcheck", t.handleHealth)
		log.Printf("serving tally http on %s", cfg.httpAddr)
		if err := serveHTTP(cfg.httpAddr, mux); err != nil {
			log.Fatalf("serve tally: %v", err)
		}
		return
	}

	s := &server{
		started:     time.Now(),
		cfg:         cfg,
		tallyClient: &http.Client{Timeout: 500 * time.Millisecond},
	}

	if cfg.exitAfter > 0 {
		go func() {
			time.Sleep(cfg.exitAfter)
			log.Printf("exit_after elapsed, terminating")
			os.Exit(0)
		}()
	}

	grpcOptions := []grpc.ServerOption{}
	if cfg.maxRecvMiB > 0 {
		grpcOptions = append(grpcOptions, grpc.MaxRecvMsgSize(cfg.maxRecvMiB*1024*1024))
	}
	grpcServer := grpc.NewServer(grpcOptions...)
	logspb.RegisterLogsServiceServer(grpcServer, s)
	healthpb.RegisterHealthServer(grpcServer, s)

	lis, err := net.Listen("tcp", cfg.grpcAddr)
	if err != nil {
		log.Fatalf("listen grpc: %v", err)
	}
	go func() {
		log.Printf("serving otlp grpc on %s", cfg.grpcAddr)
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("serve grpc: %v", err)
		}
	}()

	mux := http.NewServeMux()
	mux.HandleFunc("/healthcheck", s.handleHealth)
	mux.HandleFunc("/ready", s.handleReady)
	mux.HandleFunc("/drain", s.handleDrain)
	mux.HandleFunc("/undrain", s.handleUndrain)
	mux.HandleFunc("/metrics", s.handleMetrics)
	mux.HandleFunc("/debug/state", s.handleState)

	log.Printf("serving http on %s", cfg.httpAddr)
	if err := serveHTTP(cfg.httpAddr, mux); err != nil {
		log.Fatalf("serve http: %v", err)
	}
}

func (s *server) Export(ctx context.Context, req *logspb.ExportLogsServiceRequest) (*logspb.ExportLogsServiceResponse, error) {
	s.requests.Add(1)
	inflight := s.inflight.Add(1)
	updateMax(&s.maxInflight, inflight)
	defer s.inflight.Add(-1)

	count := countLogRecords(req)
	s.received.Add(int64(count))
	s.report("received", count)

	if s.shouldSlow() && s.inWindow(s.cfg.slowAfter, s.cfg.slowFor) {
		s.slow.Add(1)
		select {
		case <-time.After(s.cfg.slowSleep):
		case <-ctx.Done():
			s.failures.Add(1)
			s.report("failed", count)
			return nil, ctx.Err()
		}
	}

	if s.inWindow(s.cfg.failAfter, s.cfg.failFor) {
		s.failures.Add(1)
		s.report("failed", count)
		return nil, status.Error(codes.Unavailable, "saw7500 injected backend failure")
	}

	s.records.Add(int64(count))
	s.report("accepted", count)
	return &logspb.ExportLogsServiceResponse{}, nil
}

func loadgenConfigFromEnv() loadgenConfig {
	return loadgenConfig{
		endpoint:          envString("OTLP_ENDPOINT", "localhost:4317"),
		duration:          envDuration("LOAD_DURATION", 720*time.Second),
		rate:              envInt("LOAD_RATE", 250),
		workers:           envInt("LOAD_WORKERS", 8),
		batchSize:         envInt("BATCH_SIZE", 100),
		allowFailures:     envBool("ALLOW_EXPORT_FAILURES", true),
		payloadProfile:    envString("PAYLOAD_PROFILE", "repeated"),
		payloadSizeBytes:  envInt("PAYLOAD_SIZE_BYTES", 0),
		body:              envString("LOG_BODY", "bigid-local-saw-7500-log"),
		serviceName:       envString("SERVICE_NAME", "bigid-local-saw-7500"),
		tenant:            envString("TENANT", "bigid"),
		compression:       envString("OTLP_COMPRESSION", ""),
		randomPayloadSeed: int64(envInt("PAYLOAD_RANDOM_SEED", 1)),
	}
}

func runLoadgen(ctx context.Context, cfg loadgenConfig) error {
	if cfg.duration <= 0 {
		return errors.New("LOAD_DURATION must be positive")
	}
	if cfg.rate <= 0 {
		return errors.New("LOAD_RATE must be positive")
	}
	if cfg.workers <= 0 {
		return errors.New("LOAD_WORKERS must be positive")
	}
	if cfg.batchSize <= 0 {
		return errors.New("BATCH_SIZE must be positive")
	}

	payload, err := loadgenPayload(cfg)
	if err != nil {
		return err
	}

	opts := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}
	if cfg.compression != "" {
		opts = append(opts, grpc.WithDefaultCallOptions(grpc.UseCompressor(cfg.compression)))
	}
	conn, err := grpc.NewClient(cfg.endpoint, opts...)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()

	client := logspb.NewLogsServiceClient(conn)
	ctx, cancel := context.WithTimeout(ctx, cfg.duration+30*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	errCh := make(chan error, cfg.workers)
	for worker := 0; worker < cfg.workers; worker++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			generated, err := runLoadgenWorker(ctx, client, cfg, payload, workerID)
			log.Printf("logs generated worker=%d logs=%d", workerID, generated)
			if err != nil {
				errCh <- err
			}
		}(worker)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			return err
		}
	}
	return nil
}

func runLoadgenWorker(ctx context.Context, client logspb.LogsServiceClient, cfg loadgenConfig, payload string, workerID int) (int64, error) {
	perWorkerRate := float64(cfg.rate) / float64(cfg.workers)
	interval := time.Duration(float64(time.Second) * float64(cfg.batchSize) / perWorkerRate)
	interval = max(interval, 10*time.Millisecond)

	deadline := time.Now().Add(cfg.duration)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	req := buildLoadgenRequest(payload, cfg.batchSize, cfg.serviceName, cfg.tenant)
	var generated int64
	var exportFailures int64
	var lastFailureLog time.Time
	defer func() {
		if exportFailures > 0 {
			log.Printf("export failures worker=%d failures=%d", workerID, exportFailures)
		}
	}()
	for {
		if time.Now().After(deadline) {
			return generated, nil
		}
		select {
		case <-ctx.Done():
			return generated, ctx.Err()
		case <-ticker.C:
			if _, err := client.Export(ctx, req); err != nil {
				if !cfg.allowFailures {
					return generated, err
				}
				exportFailures++
				now := time.Now()
				if shouldLogLoadgenFailure(exportFailures, lastFailureLog, now) {
					log.Printf("export failed, continuing worker=%d failures=%d: %v", workerID, exportFailures, err)
					lastFailureLog = now
				}
				continue
			}
			generated += int64(cfg.batchSize)
		}
	}
}

func shouldLogLoadgenFailure(failures int64, lastLog, now time.Time) bool {
	return failures == 1 || failures%100 == 0 || now.Sub(lastLog) >= 30*time.Second
}

func loadgenPayload(cfg loadgenConfig) (string, error) {
	if cfg.payloadSizeBytes <= 0 {
		return cfg.body, nil
	}

	switch strings.ToLower(cfg.payloadProfile) {
	case "repeated":
		return strings.Repeat("A", cfg.payloadSizeBytes), nil
	case "random":
		const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
		seed := uint64(cfg.randomPayloadSeed)
		rng := rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15))
		var builder strings.Builder
		builder.Grow(cfg.payloadSizeBytes)
		for range cfg.payloadSizeBytes {
			builder.WriteByte(alphabet[rng.IntN(len(alphabet))])
		}
		return builder.String(), nil
	default:
		return "", fmt.Errorf("unknown PAYLOAD_PROFILE %q; expected repeated or random", cfg.payloadProfile)
	}
}

func buildLoadgenRequest(payload string, batchSize int, serviceName, tenant string) *logspb.ExportLogsServiceRequest {
	records := make([]*logpb.LogRecord, 0, batchSize)
	now := uint64(time.Now().UnixNano())
	for range batchSize {
		records = append(records, &logpb.LogRecord{
			TimeUnixNano: now,
			Body: &commonpb.AnyValue{
				Value: &commonpb.AnyValue_StringValue{StringValue: payload},
			},
			Attributes: []*commonpb.KeyValue{
				{
					Key: "tenant",
					Value: &commonpb.AnyValue{
						Value: &commonpb.AnyValue_StringValue{StringValue: tenant},
					},
				},
			},
		})
	}

	return &logspb.ExportLogsServiceRequest{
		ResourceLogs: []*logpb.ResourceLogs{
			{
				Resource: &resourcepb.Resource{
					Attributes: []*commonpb.KeyValue{
						{
							Key: "service.name",
							Value: &commonpb.AnyValue{
								Value: &commonpb.AnyValue_StringValue{StringValue: serviceName},
							},
						},
					},
				},
				ScopeLogs: []*logpb.ScopeLogs{
					{LogRecords: records},
				},
			},
		},
	}
}

func (s *server) report(kind string, records int) {
	if s.cfg.tallyURL == "" || records <= 0 {
		return
	}
	url := fmt.Sprintf("%s/add?kind=%s&records=%d", strings.TrimRight(s.cfg.tallyURL, "/"), kind, records)
	req, err := http.NewRequest(http.MethodPost, url, http.NoBody)
	if err != nil {
		log.Printf("build tally request: %v", err)
		return
	}
	resp, err := s.tallyClient.Do(req)
	if err != nil {
		log.Printf("post tally %s: %v", kind, err)
		return
	}
	_ = resp.Body.Close()
	if resp.StatusCode >= 300 {
		log.Printf("post tally %s: status=%d", kind, resp.StatusCode)
	}
}

func (s *server) Check(context.Context, *healthpb.HealthCheckRequest) (*healthpb.HealthCheckResponse, error) {
	statusValue := healthpb.HealthCheckResponse_SERVING
	if s.isUnready() {
		statusValue = healthpb.HealthCheckResponse_NOT_SERVING
	}
	return &healthpb.HealthCheckResponse{Status: statusValue}, nil
}

func (*server) Watch(*healthpb.HealthCheckRequest, healthpb.Health_WatchServer) error {
	return status.Error(codes.Unimplemented, "watch is not implemented")
}

func (*server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok\n"))
}

func (s *server) handleReady(w http.ResponseWriter, _ *http.Request) {
	if s.isUnready() {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte("not ready\n"))
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ready\n"))
}

func (s *server) handleDrain(w http.ResponseWriter, _ *http.Request) {
	s.draining.Store(true)
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) handleUndrain(w http.ResponseWriter, _ *http.Request) {
	s.draining.Store(false)
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) handleMetrics(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	fmt.Fprintf(w, "# TYPE saw7500_backend_requests_total counter\n")
	fmt.Fprintf(w, "saw7500_backend_requests_total %d\n", s.requests.Load())
	fmt.Fprintf(w, "# TYPE saw7500_backend_log_records_total counter\n")
	fmt.Fprintf(w, "saw7500_backend_log_records_total %d\n", s.records.Load())
	fmt.Fprintf(w, "# TYPE saw7500_backend_received_log_records_total counter\n")
	fmt.Fprintf(w, "saw7500_backend_received_log_records_total %d\n", s.received.Load())
	fmt.Fprintf(w, "# TYPE saw7500_backend_slow_requests_total counter\n")
	fmt.Fprintf(w, "saw7500_backend_slow_requests_total %d\n", s.slow.Load())
	fmt.Fprintf(w, "# TYPE saw7500_backend_failures_total counter\n")
	fmt.Fprintf(w, "saw7500_backend_failures_total %d\n", s.failures.Load())
	fmt.Fprintf(w, "# TYPE saw7500_backend_inflight gauge\n")
	fmt.Fprintf(w, "saw7500_backend_inflight %d\n", s.inflight.Load())
	fmt.Fprintf(w, "# TYPE saw7500_backend_max_inflight gauge\n")
	fmt.Fprintf(w, "saw7500_backend_max_inflight %d\n", s.maxInflight.Load())
	fmt.Fprintf(w, "# TYPE saw7500_backend_ready gauge\n")
	if s.isUnready() {
		fmt.Fprintf(w, "saw7500_backend_ready 0\n")
	} else {
		fmt.Fprintf(w, "saw7500_backend_ready 1\n")
	}
	fmt.Fprintf(w, "# TYPE saw7500_backend_draining gauge\n")
	if s.draining.Load() {
		fmt.Fprintf(w, "saw7500_backend_draining 1\n")
	} else {
		fmt.Fprintf(w, "saw7500_backend_draining 0\n")
	}
	fmt.Fprintf(w, "# TYPE saw7500_backend_slow_eligible gauge\n")
	if s.shouldSlow() {
		fmt.Fprintf(w, "saw7500_backend_slow_eligible 1\n")
	} else {
		fmt.Fprintf(w, "saw7500_backend_slow_eligible 0\n")
	}
}

func (s *server) handleState(w http.ResponseWriter, _ *http.Request) {
	fmt.Fprintf(w, "uptime=%s ready=%t draining=%t slow=%t requests=%d records=%d failures=%d\n",
		time.Since(s.started).Truncate(time.Second),
		!s.isUnready(),
		s.draining.Load(),
		s.inWindow(s.cfg.slowAfter, s.cfg.slowFor),
		s.requests.Load(),
		s.records.Load(),
		s.failures.Load(),
	)
}

func (*tallyServer) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok\n"))
}

func (t *tallyServer) handleAdd(w http.ResponseWriter, r *http.Request) {
	records, err := strconv.Atoi(r.URL.Query().Get("records"))
	if err != nil || records < 0 {
		http.Error(w, "invalid records", http.StatusBadRequest)
		return
	}
	switch r.URL.Query().Get("kind") {
	case "received":
		t.received.Add(int64(records))
	case "accepted":
		t.accepted.Add(int64(records))
	case "failed":
		t.failures.Add(int64(records))
	default:
		http.Error(w, "invalid kind", http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (t *tallyServer) handleMetrics(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	fmt.Fprintf(w, "# TYPE saw7500_tally_received_log_records_total counter\n")
	fmt.Fprintf(w, "saw7500_tally_received_log_records_total %d\n", t.received.Load())
	fmt.Fprintf(w, "# TYPE saw7500_tally_accepted_log_records_total counter\n")
	fmt.Fprintf(w, "saw7500_tally_accepted_log_records_total %d\n", t.accepted.Load())
	fmt.Fprintf(w, "# TYPE saw7500_tally_failed_log_records_total counter\n")
	fmt.Fprintf(w, "saw7500_tally_failed_log_records_total %d\n", t.failures.Load())
}

func (s *server) isUnready() bool {
	return s.draining.Load() || s.inWindow(s.cfg.unreadyAfter, s.cfg.unreadyFor)
}

func (s *server) shouldSlow() bool {
	if s.cfg.slowModulo <= 0 {
		return true
	}
	if s.cfg.slowRemainder < 0 || s.cfg.slowRemainder >= s.cfg.slowModulo {
		return false
	}
	hostname := envString("HOSTNAME", "fakebackend")
	if ordinal, ok := hostnameOrdinal(hostname); ok {
		return ordinal%s.cfg.slowModulo == s.cfg.slowRemainder
	}
	hash := fnv.New32a()
	_, _ = hash.Write([]byte(hostname))
	return int(hash.Sum32()%uint32(s.cfg.slowModulo)) == s.cfg.slowRemainder
}

func hostnameOrdinal(hostname string) (int, bool) {
	match := trailingOrdinalRe.FindStringSubmatch(hostname)
	if match == nil {
		return 0, false
	}
	ordinal, err := strconv.Atoi(match[1])
	if err != nil {
		return 0, false
	}
	return ordinal, true
}

func (s *server) inWindow(after, forDuration time.Duration) bool {
	if after <= 0 {
		return false
	}
	elapsed := time.Since(s.started)
	if elapsed < after {
		return false
	}
	if forDuration <= 0 {
		return true
	}
	return elapsed < after+forDuration
}

func countLogRecords(req *logspb.ExportLogsServiceRequest) int {
	total := 0
	for _, rl := range req.GetResourceLogs() {
		for _, sl := range rl.GetScopeLogs() {
			total += len(sl.GetLogRecords())
		}
	}
	return total
}

func updateMax(target *atomic.Int64, value int64) {
	for {
		current := target.Load()
		if value <= current {
			return
		}
		if target.CompareAndSwap(current, value) {
			return
		}
	}
}

func envString(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func envDuration(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	duration, err := time.ParseDuration(value)
	if err != nil {
		log.Fatalf("parse %s=%q as duration: %v", key, value, err)
	}
	return duration
}

func envInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		log.Fatalf("parse %s=%q as int: %v", key, value, err)
	}
	return parsed
}

func envBool(key string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		log.Fatalf("parse %s=%q as bool: %v", key, value, err)
	}
	return parsed
}

func serveHTTP(addr string, handler http.Handler) error {
	server := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}
	return server.ListenAndServe()
}

var _ = envInt

var trailingOrdinalRe = regexp.MustCompile(`-(\d+)$`)
