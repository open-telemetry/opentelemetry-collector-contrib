// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package supervisor

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"os"
	"runtime"
	"sort"
	"sync/atomic"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/knadh/koanf"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/providers/rawbytes"
	"github.com/oklog/ulid/v2"
	"github.com/open-telemetry/opamp-go/client"
	"github.com/open-telemetry/opamp-go/client/types"
	"github.com/open-telemetry/opamp-go/protobufs"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor/commander"
	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor/config"
	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor/healthchecker"
)

// This Supervisor is developed specifically for the OpenTelemetry Collector.
const agentType = "io.opentelemetry.collector"

// Supervisor implements supervising of OpenTelemetry Collector and uses OpAMPClient
// to work with an OpAMP Server.
type Supervisor struct {
	logger *zap.Logger

	// Commander that starts/stops the Agent process.
	commander *commander.Commander

	startedAt time.Time

	healthCheckTicker  *backoff.Ticker
	healthChecker      *healthchecker.HTTPHealthChecker
	lastHealthCheckErr error

	// Supervisor's own config.
	config config.Supervisor

	// Agent's instance id.
	instanceID ulid.ULID

	// The version of the agent.
	agentVersion string

	// A config section to be added to the Collector's config to fetch its own metrics.
	// TODO: store this persistently so that when starting we can compose the effective
	// config correctly.
	// https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/21078
	agentConfigOwnMetricsSection *atomic.Value

	// agentHealthCheckEndpoint is the endpoint the Collector's health check extension
	// will listen on for health check requests from the Supervisor.
	agentHealthCheckEndpoint string

	// Final effective config of the Collector.
	effectiveConfig *atomic.Value

	// Location of the effective config file.
	effectiveConfigFilePath string

	// Last received remote config.
	remoteConfig *protobufs.AgentRemoteConfig

	// A channel to indicate there is a new config to apply.
	hasNewConfig chan struct{}

	// The OpAMP client to connect to the OpAMP Server.
	opampClient client.OpAMPClient

	shuttingDown bool

	agentHasStarted               bool
	agentStartHealthCheckAttempts int
}

func NewSupervisor(logger *zap.Logger, configFile string) (*Supervisor, error) {
	s := &Supervisor{
		logger:                       logger,
		hasNewConfig:                 make(chan struct{}, 1),
		effectiveConfigFilePath:      "effective.yaml",
		agentConfigOwnMetricsSection: &atomic.Value{},
		effectiveConfig:              &atomic.Value{},
	}

	if err := s.loadConfig(configFile); err != nil {
		return nil, fmt.Errorf("error loading config: %w", err)
	}

	if err := s.getBootstrapInfo(); err != nil {
		s.logger.Error("Couldn't get agent version", zap.Error(err))
	}

	port, err := s.findRandomPort()

	if err != nil {
		return nil, fmt.Errorf("could not find port for health check: %w", err)
	}

	s.agentHealthCheckEndpoint = fmt.Sprintf("localhost:%d", port)

	s.createInstanceID()
	logger.Debug("Supervisor starting",
		zap.String("id", s.instanceID.String()), zap.String("type", agentType), zap.String("version", s.agentVersion))

	s.loadAgentEffectiveConfig()

	if err = s.startOpAMP(); err != nil {
		return nil, fmt.Errorf("cannot start OpAMP client: %w", err)
	}

	s.commander, err = commander.NewCommander(
		s.logger,
		s.config.Agent,
		"--config", s.effectiveConfigFilePath,
	)
	if err != nil {
		return nil, err
	}

	s.startHealthCheckTicker()
	go s.runAgentProcess()

	return s, nil
}

func (s *Supervisor) loadConfig(configFile string) error {
	if configFile == "" {
		return errors.New("path to config file cannot be empty")
	}

	k := koanf.New("::")
	if err := k.Load(file.Provider(configFile), yaml.Parser()); err != nil {
		return err
	}

	if err := k.Unmarshal("", &s.config); err != nil {
		return fmt.Errorf("cannot parse %v: %w", configFile, err)
	}

	return nil
}

// TODO: Implement bootstrapping https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/21071
// nolint: unparam
func (s *Supervisor) getBootstrapInfo() (err error) {
	s.agentVersion = "1.0.0"

	return nil
}

func (s *Supervisor) startOpAMP() error {
	s.opampClient = client.NewWebSocket(opAMPLogger{logger: s.logger})

	settings := types.StartSettings{
		OpAMPServerURL: s.config.Server.Endpoint,
		InstanceUid:    s.instanceID.String(),
		Callbacks: types.CallbacksStruct{
			OnConnectFunc: func() {
				s.logger.Debug("Connected to the server.")
			},
			OnConnectFailedFunc: func(err error) {
				s.logger.Error("Failed to connect to the server", zap.Error(err))
			},
			OnErrorFunc: func(err *protobufs.ServerErrorResponse) {
				s.logger.Error("Server returned an error response", zap.String("message", err.ErrorMessage))
			},
			OnMessageFunc: s.onMessage,
			OnOpampConnectionSettingsFunc: func(ctx context.Context, settings *protobufs.OpAMPConnectionSettings) error {
				// TODO: https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/21043
				s.logger.Debug("Received ConnectionSettings request")
				return nil
			},
			OnOpampConnectionSettingsAcceptedFunc: func(settings *protobufs.OpAMPConnectionSettings) {
				// TODO: https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/21043
				s.logger.Debug("ConnectionSettings accepted")
			},
			OnCommandFunc: func(command *protobufs.ServerToAgentCommand) error {
				cmdType := command.GetType()
				if *cmdType.Enum() == protobufs.CommandType_CommandType_Restart {
					// TODO: https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/21077
					s.logger.Debug("Received restart command")
				}
				return nil
			},
			SaveRemoteConfigStatusFunc: func(ctx context.Context, status *protobufs.RemoteConfigStatus) {
				// TODO: https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/21079
			},
			GetEffectiveConfigFunc: func(ctx context.Context) (*protobufs.EffectiveConfig, error) {
				return s.createEffectiveConfigMsg(), nil
			},
		},
		// TODO: Make capabilities configurable
		Capabilities: protobufs.AgentCapabilities_AgentCapabilities_AcceptsRemoteConfig |
			protobufs.AgentCapabilities_AgentCapabilities_ReportsRemoteConfig |
			protobufs.AgentCapabilities_AgentCapabilities_ReportsEffectiveConfig |
			protobufs.AgentCapabilities_AgentCapabilities_ReportsOwnMetrics |
			protobufs.AgentCapabilities_AgentCapabilities_ReportsHealth,
	}
	err := s.opampClient.SetAgentDescription(s.createAgentDescription())
	if err != nil {
		return err
	}

	err = s.opampClient.SetHealth(&protobufs.AgentHealth{Healthy: false})
	if err != nil {
		return err
	}

	s.logger.Debug("Starting OpAMP client...")

	err = s.opampClient.Start(context.Background(), settings)
	if err != nil {
		return err
	}

	s.logger.Debug("OpAMP Client started.")

	return nil
}

func (s *Supervisor) createInstanceID() {
	entropy := ulid.Monotonic(rand.New(rand.NewSource(0)), 0)
	s.instanceID = ulid.MustNew(ulid.Timestamp(time.Now()), entropy)

	// TODO: Persist instance ID. https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/21073
}

func keyVal(key, val string) *protobufs.KeyValue {
	return &protobufs.KeyValue{
		Key: key,
		Value: &protobufs.AnyValue{
			Value: &protobufs.AnyValue_StringValue{StringValue: val},
		},
	}
}

func (s *Supervisor) createAgentDescription() *protobufs.AgentDescription {
	hostname, _ := os.Hostname()

	return &protobufs.AgentDescription{
		IdentifyingAttributes: []*protobufs.KeyValue{
			keyVal("service.name", agentType),
			keyVal("service.version", s.agentVersion),
			keyVal("service.instance.id", s.instanceID.String()),
		},
		NonIdentifyingAttributes: []*protobufs.KeyValue{
			keyVal("os.family", runtime.GOOS),
			keyVal("host.name", hostname),
		},
	}
}

func (s *Supervisor) composeExtraLocalConfig() string {
	return fmt.Sprintf(`
service:
  telemetry:
    logs:
      # Enables JSON log output for the Agent.
      encoding: json
    resource:
      # Set resource attributes required by OpAMP spec.
      # See https://github.com/open-telemetry/opamp-spec/blob/main/specification.md#agentdescriptionidentifying_attributes
      service.name: %s
      service.version: %s
      service.instance.id: %s

  # Enable extension to allow the Supervisor to check health.
  extensions: [health_check]

extensions:
  health_check:
    endpoint: %s
`,
		agentType,
		s.agentVersion,
		s.instanceID.String(),
		s.agentHealthCheckEndpoint,
	)
}

func (s *Supervisor) loadAgentEffectiveConfig() {
	var effectiveConfigBytes []byte

	effFromFile, err := os.ReadFile(s.effectiveConfigFilePath)
	if err == nil {
		// We have an effective config file.
		effectiveConfigBytes = effFromFile
	} else {
		// No effective config file, just use the initial config.
		effectiveConfigBytes = []byte(s.composeExtraLocalConfig())
	}

	s.effectiveConfig.Store(string(effectiveConfigBytes))
}

// createEffectiveConfigMsg create an EffectiveConfig with the content of the
// current effective config.
func (s *Supervisor) createEffectiveConfigMsg() *protobufs.EffectiveConfig {
	cfgStr, ok := s.effectiveConfig.Load().(string)
	if !ok {
		cfgStr = ""
	}

	cfg := &protobufs.EffectiveConfig{
		ConfigMap: &protobufs.AgentConfigMap{
			ConfigMap: map[string]*protobufs.AgentConfigFile{
				"": {Body: []byte(cfgStr)},
			},
		},
	}

	return cfg
}

func (s *Supervisor) setupOwnMetrics(_ context.Context, settings *protobufs.TelemetryConnectionSettings) (configChanged bool) {
	var cfg string
	if settings.DestinationEndpoint == "" {
		// No destination. Disable metric collection.
		s.logger.Debug("Disabling own metrics pipeline in the config")
		cfg = ""
	} else {
		s.logger.Debug("Enabling own metrics pipeline in the config")

		port, err := s.findRandomPort()

		if err != nil {
			s.logger.Error("Could not setup own metrics", zap.Error(err))
			return
		}

		cfg = fmt.Sprintf(
			`
receivers:
  # Collect own metrics
  prometheus/own_metrics:
    config:
      scrape_configs:
        - job_name: 'otel-collector'
          scrape_interval: 10s
          static_configs:
            - targets: ['0.0.0.0:%d']  
exporters:
  otlphttp/own_metrics:
    metrics_endpoint: %s

service:
  telemetry:
    metrics:
      address: :%d
  pipelines:
    metrics/own_metrics:
      receivers: [prometheus/own_metrics]
      exporters: [otlphttp/own_metrics]
`,
			port,
			settings.DestinationEndpoint,
			port,
		)
	}

	s.agentConfigOwnMetricsSection.Store(cfg)

	// Need to recalculate the Agent config so that the metric config is included in it.
	configChanged, err := s.recalcEffectiveConfig()
	if err != nil {
		return
	}

	return configChanged
}

// composeEffectiveConfig composes the effective config from multiple sources:
// 1) the remote config from OpAMP Server
// 2) the own metrics config section
// 3) the local override config that is hard-coded in the Supervisor.
func (s *Supervisor) composeEffectiveConfig(config *protobufs.AgentRemoteConfig) (configChanged bool, err error) {
	var k = koanf.New(".")

	// Begin with empty config. We will merge received configs on top of it.
	if err = k.Load(rawbytes.Provider([]byte{}), yaml.Parser()); err != nil {
		return false, err
	}

	// Sort to make sure the order of merging is stable.
	var names []string
	for name := range config.Config.ConfigMap {
		if name == "" {
			// skip instance config
			continue
		}
		names = append(names, name)
	}

	sort.Strings(names)

	// Append instance config as the last item.
	names = append(names, "")

	// Merge received configs.
	for _, name := range names {
		item := config.Config.ConfigMap[name]
		var k2 = koanf.New(".")
		err = k2.Load(rawbytes.Provider(item.Body), yaml.Parser())
		if err != nil {
			return false, fmt.Errorf("cannot parse config named %s: %w", name, err)
		}
		err = k.Merge(k2)
		if err != nil {
			return false, fmt.Errorf("cannot merge config named %s: %w", name, err)
		}
	}

	// Merge own metrics config.
	ownMetricsCfg, ok := s.agentConfigOwnMetricsSection.Load().(string)
	if ok {
		if err = k.Load(rawbytes.Provider([]byte(ownMetricsCfg)), yaml.Parser()); err != nil {
			return false, err
		}
	}

	// Merge local config last since it has the highest precedence.
	if err = k.Load(rawbytes.Provider([]byte(s.composeExtraLocalConfig())), yaml.Parser()); err != nil {
		return false, err
	}

	// The merged final result is our effective config.
	effectiveConfigBytes, err := k.Marshal(yaml.Parser())
	if err != nil {
		return false, err
	}

	// Check if effective config is changed.
	newEffectiveConfig := string(effectiveConfigBytes)
	configChanged = false
	if s.effectiveConfig.Load().(string) != newEffectiveConfig {
		s.logger.Debug("Effective config changed.")
		s.effectiveConfig.Store(newEffectiveConfig)
		configChanged = true
	}

	return configChanged, nil
}

// Recalculate the Agent's effective config and if the config changes, signal to the
// background goroutine that the config needs to be applied to the Agent.
func (s *Supervisor) recalcEffectiveConfig() (configChanged bool, err error) {
	configChanged, err = s.composeEffectiveConfig(s.remoteConfig)
	if err != nil {
		s.logger.Error("Error composing effective config. Ignoring received config", zap.Error(err))
		return configChanged, err
	}

	return configChanged, nil
}

func (s *Supervisor) startAgent() {
	err := s.commander.Start(context.Background())
	if err != nil {
		s.logger.Error("Cannot start the agent", zap.Error(err))
		err = s.opampClient.SetHealth(&protobufs.AgentHealth{Healthy: false, LastError: fmt.Sprintf("Cannot start the agent: %v", err)})

		if err != nil {
			s.logger.Error("Failed to report OpAMP client health", zap.Error(err))
		}

		return
	}

	s.agentHasStarted = false
	s.agentStartHealthCheckAttempts = 0
	s.startedAt = time.Now()
	s.startHealthCheckTicker()

	s.healthChecker = healthchecker.NewHTTPHealthChecker(fmt.Sprintf("http://%s", s.agentHealthCheckEndpoint))
}

func (s *Supervisor) startHealthCheckTicker() {
	// Prepare health checker
	healthCheckBackoff := backoff.NewExponentialBackOff()
	healthCheckBackoff.MaxInterval = 60 * time.Second
	healthCheckBackoff.MaxElapsedTime = 0 // Never stop
	if s.healthCheckTicker != nil {
		s.healthCheckTicker.Stop()
	}
	s.healthCheckTicker = backoff.NewTicker(healthCheckBackoff)
}

func (s *Supervisor) healthCheck() {
	if !s.commander.IsRunning() {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)

	err := s.healthChecker.Check(ctx)
	cancel()

	if errors.Is(err, s.lastHealthCheckErr) {
		// No difference from last check. Nothing new to report.
		return
	}

	// Prepare OpAMP health report.
	health := &protobufs.AgentHealth{
		StartTimeUnixNano: uint64(s.startedAt.UnixNano()),
	}

	if err != nil {
		health.Healthy = false
		if !s.agentHasStarted && s.agentStartHealthCheckAttempts < 10 {
			health.LastError = "Agent is starting"
			s.agentStartHealthCheckAttempts += 1
		} else {
			health.LastError = err.Error()
			s.logger.Error("Agent is not healthy", zap.Error(err))
		}
	} else {
		s.agentHasStarted = true
		health.Healthy = true
		s.logger.Debug("Agent is healthy.")
	}

	// Report via OpAMP.
	if err2 := s.opampClient.SetHealth(health); err2 != nil {
		s.logger.Error("Could not report health to OpAMP server", zap.Error(err2))
		return
	}

	s.lastHealthCheckErr = err
}

func (s *Supervisor) runAgentProcess() {
	if _, err := os.Stat(s.effectiveConfigFilePath); err == nil {
		// We have an effective config file saved previously. Use it to start the agent.
		s.startAgent()
	}

	restartTimer := time.NewTimer(0)
	restartTimer.Stop()

	for {
		select {
		case <-s.hasNewConfig:
			restartTimer.Stop()
			s.stopAgentApplyConfig()
			s.startAgent()

		case <-s.commander.Done():
			if s.shuttingDown {
				break
			}

			s.logger.Debug("Agent process exited unexpectedly. Will restart in a bit...", zap.Int("pid", s.commander.Pid()), zap.Int("exit_code", s.commander.ExitCode()))
			errMsg := fmt.Sprintf(
				"Agent process PID=%d exited unexpectedly, exit code=%d. Will restart in a bit...",
				s.commander.Pid(), s.commander.ExitCode(),
			)
			err := s.opampClient.SetHealth(&protobufs.AgentHealth{Healthy: false, LastError: errMsg})

			if err != nil {
				s.logger.Error("Could not report health to OpAMP server", zap.Error(err))
			}

			// TODO: decide why the agent stopped. If it was due to bad config, report it to server.
			// https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/21079

			// Wait 5 seconds before starting again.
			restartTimer.Stop()
			restartTimer.Reset(5 * time.Second)

		case <-restartTimer.C:
			s.startAgent()

		case <-s.healthCheckTicker.C:
			s.healthCheck()
		}
	}
}

func (s *Supervisor) stopAgentApplyConfig() {
	s.logger.Debug("Stopping the agent to apply new config")
	cfg := s.effectiveConfig.Load().(string)
	err := s.commander.Stop(context.Background())

	if err != nil {
		s.logger.Error("Could not stop agent process", zap.Error(err))
	}

	s.writeEffectiveConfigToFile(cfg, s.effectiveConfigFilePath)
}

func (s *Supervisor) writeEffectiveConfigToFile(cfg string, filePath string) {
	f, err := os.Create(filePath)
	if err != nil {
		s.logger.Error("Cannot create effective config file", zap.Error(err))
	}
	defer f.Close()

	_, err = f.WriteString(cfg)

	if err != nil {
		s.logger.Error("Cannot write effective config file", zap.Error(err))
	}
}

func (s *Supervisor) Shutdown() {
	s.logger.Debug("Supervisor shutting down...")
	s.shuttingDown = true
	if s.commander != nil {
		err := s.commander.Stop(context.Background())

		if err != nil {
			s.logger.Error("Could not stop agent process", zap.Error(err))
		}
	}

	if s.opampClient != nil {
		err := s.opampClient.SetHealth(
			&protobufs.AgentHealth{
				Healthy: false, LastError: "Supervisor is shutdown",
			},
		)

		if err != nil {
			s.logger.Error("Could not report health to OpAMP server", zap.Error(err))
		}

		err = s.opampClient.Stop(context.Background())

		if err != nil {
			s.logger.Error("Could not stop the OpAMP client", zap.Error(err))
		}
	}
}

func (s *Supervisor) onMessage(ctx context.Context, msg *types.MessageData) {
	configChanged := false
	if msg.RemoteConfig != nil {
		s.remoteConfig = msg.RemoteConfig
		s.logger.Debug("Received remote config from server", zap.String("hash", fmt.Sprintf("%x", s.remoteConfig.ConfigHash)))

		var err error
		configChanged, err = s.recalcEffectiveConfig()
		if err != nil {
			err = s.opampClient.SetRemoteConfigStatus(&protobufs.RemoteConfigStatus{
				LastRemoteConfigHash: msg.RemoteConfig.ConfigHash,
				Status:               protobufs.RemoteConfigStatuses_RemoteConfigStatuses_FAILED,
				ErrorMessage:         err.Error(),
			})
			if err != nil {
				s.logger.Error("Could not report failed OpAMP remote config status", zap.Error(err))
			}
		} else {
			err = s.opampClient.SetRemoteConfigStatus(&protobufs.RemoteConfigStatus{
				LastRemoteConfigHash: msg.RemoteConfig.ConfigHash,
				Status:               protobufs.RemoteConfigStatuses_RemoteConfigStatuses_APPLIED,
			})
			if err != nil {
				s.logger.Error("Could not report applied OpAMP remote config status", zap.Error(err))
			}
		}
	}

	if msg.OwnMetricsConnSettings != nil {
		configChanged = s.setupOwnMetrics(ctx, msg.OwnMetricsConnSettings) || configChanged
	}

	if msg.AgentIdentification != nil {
		newInstanceID, err := ulid.Parse(msg.AgentIdentification.NewInstanceUid)
		if err != nil {
			s.logger.Error("Failed to parse instance ULID", zap.Error(err))
		}

		s.logger.Debug("Agent identity is changing",
			zap.String("old_id", s.instanceID.String()),
			zap.String("new_id", newInstanceID.String()))
		s.instanceID = newInstanceID
		err = s.opampClient.SetAgentDescription(s.createAgentDescription())
		if err != nil {
			s.logger.Error("Failed to send agent description to OpAMP server")
		}

		configChanged = true
	}

	if configChanged {
		err := s.opampClient.UpdateEffectiveConfig(ctx)
		if err != nil {
			s.logger.Error("The OpAMP client failed to update the effective config", zap.Error(err))
		}

		s.logger.Debug("Config is changed. Signal to restart the agent")
		// Signal that there is a new config.
		select {
		case s.hasNewConfig <- struct{}{}:
		default:
		}
	}
}

func (s *Supervisor) findRandomPort() (int, error) {
	l, err := net.Listen("tcp", "localhost:0")

	if err != nil {
		return 0, err
	}

	port := l.Addr().(*net.TCPAddr).Port

	err = l.Close()

	if err != nil {
		return 0, err
	}

	return port, nil
}
