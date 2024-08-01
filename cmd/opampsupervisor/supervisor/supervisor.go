// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package supervisor

import (
	"bytes"
	"context"
	_ "embed"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"sync/atomic"
	"text/template"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/google/uuid"
	"github.com/knadh/koanf/maps"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/providers/rawbytes"
	"github.com/knadh/koanf/v2"
	"github.com/open-telemetry/opamp-go/client"
	"github.com/open-telemetry/opamp-go/client/types"
	"github.com/open-telemetry/opamp-go/protobufs"
	"github.com/open-telemetry/opamp-go/server"
	serverTypes "github.com/open-telemetry/opamp-go/server/types"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configtls"
	semconv "go.opentelemetry.io/collector/semconv/v1.21.0"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor/commander"
	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor/config"
	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor/healthchecker"
)

var (
	//go:embed templates/nooppipeline.yaml
	noopPipelineTpl string

	//go:embed templates/extraconfig.yaml
	extraConfigTpl string

	//go:embed templates/opampextension.yaml
	opampextensionTpl string

	//go:embed templates/owntelemetry.yaml
	ownTelemetryTpl string

	lastRecvRemoteConfigFile     = "last_recv_remote_config.dat"
	lastRecvOwnMetricsConfigFile = "last_recv_own_metrics_config.dat"
)

const (
	persistentStateFilePath = "persistent_state.yaml"
	agentConfigFilePath     = "effective.yaml"
)

const maxBufferedCustomMessages = 10

// Supervisor implements supervising of OpenTelemetry Collector and uses OpAMPClient
// to work with an OpAMP Server.
type Supervisor struct {
	logger      *zap.Logger
	pidProvider pidProvider

	// Commander that starts/stops the Agent process.
	commander *commander.Commander

	startedAt time.Time

	healthCheckTicker  *backoff.Ticker
	healthChecker      *healthchecker.HTTPHealthChecker
	lastHealthCheckErr error

	// Supervisor's own config.
	config config.Supervisor

	agentDescription *atomic.Value

	// Supervisor's persistent state
	persistentState *persistentState

	noopPipelineTemplate   *template.Template
	opampextensionTemplate *template.Template
	extraConfigTemplate    *template.Template
	ownTelemetryTemplate   *template.Template

	agentConn *atomic.Value

	// A config section to be added to the Collector's config to fetch its own metrics.
	// TODO: store this persistently so that when starting we can compose the effective
	// config correctly.
	// https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/21078
	agentConfigOwnMetricsSection *atomic.Value

	// agentHealthCheckEndpoint is the endpoint the Collector's health check extension
	// will listen on for health check requests from the Supervisor.
	agentHealthCheckEndpoint string

	// Supervisor-assembled config to be given to the Collector.
	mergedConfig *atomic.Value

	// Final effective config of the Collector.
	effectiveConfig *atomic.Value

	// Last received remote config.
	remoteConfig *protobufs.AgentRemoteConfig

	// A channel to indicate there is a new config to apply.
	hasNewConfig chan struct{}

	// The OpAMP client to connect to the OpAMP Server.
	opampClient client.OpAMPClient

	doneChan chan struct{}
	agentWG  sync.WaitGroup

	customMessageToServer chan *protobufs.CustomMessage
	customMessageWG       sync.WaitGroup

	agentHasStarted               bool
	agentStartHealthCheckAttempts int
	agentRestarting               atomic.Bool

	// The OpAMP server to communicate with the Collector's OpAMP extension
	opampServer     server.OpAMPServer
	opampServerPort int
}

func NewSupervisor(logger *zap.Logger, configFile string) (*Supervisor, error) {
	s := &Supervisor{
		logger:                       logger,
		pidProvider:                  defaultPIDProvider{},
		hasNewConfig:                 make(chan struct{}, 1),
		agentConfigOwnMetricsSection: &atomic.Value{},
		mergedConfig:                 &atomic.Value{},
		effectiveConfig:              &atomic.Value{},
		agentDescription:             &atomic.Value{},
		doneChan:                     make(chan struct{}),
		customMessageToServer:        make(chan *protobufs.CustomMessage, maxBufferedCustomMessages),
		agentConn:                    &atomic.Value{},
	}
	if err := s.createTemplates(); err != nil {
		return nil, err
	}

	if err := s.loadConfig(configFile); err != nil {
		return nil, fmt.Errorf("error loading config: %w", err)
	}

	if err := s.config.Validate(); err != nil {
		return nil, fmt.Errorf("error validating config: %w", err)
	}

	if err := os.MkdirAll(s.config.Storage.Directory, 0700); err != nil {
		return nil, fmt.Errorf("error creating storage dir: %w", err)
	}

	var err error
	s.persistentState, err = loadOrCreatePersistentState(s.persistentStateFile())
	if err != nil {
		return nil, err
	}

	if err = s.getBootstrapInfo(); err != nil {
		return nil, fmt.Errorf("could not get bootstrap info from the Collector: %w", err)
	}

	healthCheckPort, err := s.findRandomPort()

	if err != nil {
		return nil, fmt.Errorf("could not find port for health check: %w", err)
	}

	s.agentHealthCheckEndpoint = fmt.Sprintf("localhost:%d", healthCheckPort)

	logger.Debug("Supervisor starting",
		zap.String("id", s.persistentState.InstanceID.String()))

	err = s.loadAndWriteInitialMergedConfig()
	if err != nil {
		return nil, fmt.Errorf("failed loading initial config: %w", err)
	}

	if err = s.startOpAMP(); err != nil {
		return nil, fmt.Errorf("cannot start OpAMP client: %w", err)
	}

	s.commander, err = commander.NewCommander(
		s.logger,
		s.config.Agent,
		"--config", agentConfigFilePath,
	)
	if err != nil {
		return nil, err
	}

	s.startHealthCheckTicker()

	s.agentWG.Add(1)
	go func() {
		defer s.agentWG.Done()
		s.runAgentProcess()
	}()

	s.customMessageWG.Add(1)
	go func() {
		defer s.customMessageWG.Done()
		s.forwardCustomMessagesToServerLoop()
	}()

	return s, nil
}

func (s *Supervisor) createTemplates() error {
	var err error

	if s.noopPipelineTemplate, err = template.New("nooppipeline").Parse(noopPipelineTpl); err != nil {
		return err
	}
	if s.extraConfigTemplate, err = template.New("extraconfig").Parse(extraConfigTpl); err != nil {
		return err
	}
	if s.opampextensionTemplate, err = template.New("opampextension").Parse(opampextensionTpl); err != nil {
		return err
	}
	if s.ownTelemetryTemplate, err = template.New("owntelemetry").Parse(ownTelemetryTpl); err != nil {
		return err
	}

	return nil
}

func (s *Supervisor) loadConfig(configFile string) error {
	if configFile == "" {
		return errors.New("path to config file cannot be empty")
	}

	k := koanf.New("::")
	if err := k.Load(file.Provider(configFile), yaml.Parser()); err != nil {
		return err
	}

	decodeConf := koanf.UnmarshalConf{
		Tag: "mapstructure",
	}

	s.config = config.DefaultSupervisor()
	if err := k.UnmarshalWithConf("", &s.config, decodeConf); err != nil {
		return fmt.Errorf("cannot parse %v: %w", configFile, err)
	}

	return nil
}

// getBootstrapInfo obtains the Collector's agent description by
// starting a Collector with a specific config that only starts
// an OpAMP extension, obtains the agent description, then
// shuts down the Collector. This only needs to happen
// once per Collector binary.
func (s *Supervisor) getBootstrapInfo() (err error) {
	s.opampServerPort, err = s.findRandomPort()
	if err != nil {
		return err
	}

	bootstrapConfig, err := s.composeNoopConfig()
	if err != nil {
		return err
	}

	err = os.WriteFile(agentConfigFilePath, bootstrapConfig, 0600)
	if err != nil {
		return fmt.Errorf("failed to write agent config: %w", err)
	}

	srv := server.New(newLoggerFromZap(s.logger))

	done := make(chan error, 1)
	var connected atomic.Bool

	// Start a one-shot server to get the Collector's agent description
	// using the Collector's OpAMP extension.
	err = srv.Start(flattenedSettings{
		endpoint: fmt.Sprintf("localhost:%d", s.opampServerPort),
		onConnectingFunc: func(_ *http.Request) (bool, int) {
			connected.Store(true)
			return true, http.StatusOK
		},
		onMessageFunc: func(_ serverTypes.Connection, message *protobufs.AgentToServer) {
			if message.AgentDescription != nil {
				instanceIDSeen := false
				s.setAgentDescription(message.AgentDescription)
				identAttr := message.AgentDescription.IdentifyingAttributes

				for _, attr := range identAttr {
					if attr.Key == semconv.AttributeServiceInstanceID {
						// TODO: Consider whether to attempt restarting the Collector.
						// https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/29864
						if attr.Value.GetStringValue() != s.persistentState.InstanceID.String() {
							done <- fmt.Errorf(
								"the Collector's instance ID (%s) does not match with the instance ID set by the Supervisor (%s)",
								attr.Value.GetStringValue(),
								s.persistentState.InstanceID.String())
							return
						}
						instanceIDSeen = true
					}
				}

				if !instanceIDSeen {
					done <- errors.New("the Collector did not specify an instance ID in its AgentDescription message")
					return
				}

				done <- nil
			}
		},
	}.toServerSettings())
	if err != nil {
		return err
	}

	defer func() {
		if stopErr := srv.Stop(context.Background()); stopErr != nil {
			err = errors.Join(err, fmt.Errorf("error when stopping the opamp server: %w", stopErr))
		}
	}()

	cmd, err := commander.NewCommander(
		s.logger,
		s.config.Agent,
		"--config", agentConfigFilePath,
	)
	if err != nil {
		return err
	}

	if err = cmd.Start(context.Background()); err != nil {
		return err
	}

	defer func() {
		if stopErr := cmd.Stop(context.Background()); stopErr != nil {
			err = errors.Join(err, fmt.Errorf("error when stopping the collector: %w", stopErr))
		}
	}()

	select {
	// TODO make timeout configurable
	case <-time.After(3 * time.Second):
		if connected.Load() {
			return errors.New("collector connected but never responded with an AgentDescription message")
		} else {
			return errors.New("collector's OpAMP client never connected to the Supervisor")
		}
	case err = <-done:
		return err
	}
}

func (s *Supervisor) startOpAMP() error {
	if err := s.startOpAMPClient(); err != nil {
		return err
	}

	if err := s.startOpAMPServer(); err != nil {
		return err
	}

	return nil
}

func (s *Supervisor) startOpAMPClient() error {
	s.opampClient = client.NewWebSocket(newLoggerFromZap(s.logger))

	tlsConfig, err := s.config.Server.TLSSetting.LoadTLSConfig(context.Background())
	if err != nil {
		return err
	}

	s.logger.Debug("Connecting to OpAMP server...", zap.String("endpoint", s.config.Server.Endpoint), zap.Any("headers", s.config.Server.Headers))
	settings := types.StartSettings{
		OpAMPServerURL: s.config.Server.Endpoint,
		Header:         s.config.Server.Headers,
		TLSConfig:      tlsConfig,
		InstanceUid:    types.InstanceUid(s.persistentState.InstanceID),
		Callbacks: types.CallbacksStruct{
			OnConnectFunc: func(_ context.Context) {
				s.logger.Debug("Connected to the server.")
			},
			OnConnectFailedFunc: func(_ context.Context, err error) {
				s.logger.Error("Failed to connect to the server", zap.Error(err))
			},
			OnErrorFunc: func(_ context.Context, err *protobufs.ServerErrorResponse) {
				s.logger.Error("Server returned an error response", zap.String("message", err.ErrorMessage))
			},
			OnMessageFunc: s.onMessage,
			OnOpampConnectionSettingsFunc: func(ctx context.Context, settings *protobufs.OpAMPConnectionSettings) error {
				//nolint:errcheck
				go s.onOpampConnectionSettings(ctx, settings)
				return nil
			},
			OnCommandFunc: func(_ context.Context, command *protobufs.ServerToAgentCommand) error {
				cmdType := command.GetType()
				if *cmdType.Enum() == protobufs.CommandType_CommandType_Restart {
					return s.handleRestartCommand()
				}
				return nil
			},
			SaveRemoteConfigStatusFunc: func(_ context.Context, _ *protobufs.RemoteConfigStatus) {
				// TODO: https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/21079
			},
			GetEffectiveConfigFunc: func(_ context.Context) (*protobufs.EffectiveConfig, error) {
				return s.createEffectiveConfigMsg(), nil
			},
		},
		Capabilities: s.config.Capabilities.SupportedCapabilities(),
	}
	ad := s.agentDescription.Load().(*protobufs.AgentDescription)
	if err = s.opampClient.SetAgentDescription(ad); err != nil {
		return err
	}

	if err = s.opampClient.SetHealth(&protobufs.ComponentHealth{Healthy: false}); err != nil {
		return err
	}

	s.logger.Debug("Starting OpAMP client...")
	if err = s.opampClient.Start(context.Background(), settings); err != nil {
		return err
	}
	s.logger.Debug("OpAMP client started.")

	return nil
}

// startOpAMPServer starts an OpAMP server that will communicate
// with an OpAMP extension running inside a Collector to receive
// data from inside the Collector. The internal server's lifetime is not
// matched to the Collector's process, but may be restarted
// depending on information received by the Supervisor from the remote
// OpAMP server.
func (s *Supervisor) startOpAMPServer() error {
	s.opampServer = server.New(newLoggerFromZap(s.logger))

	var err error
	s.opampServerPort, err = s.findRandomPort()
	if err != nil {
		return err
	}

	s.logger.Debug("Starting OpAMP server...")

	connected := &atomic.Bool{}

	err = s.opampServer.Start(flattenedSettings{
		endpoint: fmt.Sprintf("localhost:%d", s.opampServerPort),
		onConnectingFunc: func(_ *http.Request) (bool, int) {
			// Only allow one agent to be connected the this server at a time.
			alreadyConnected := connected.Swap(true)
			return !alreadyConnected, http.StatusConflict
		},
		onMessageFunc: s.handleAgentOpAMPMessage,
		onConnectionCloseFunc: func(_ serverTypes.Connection) {
			connected.Store(false)
		},
	}.toServerSettings())
	if err != nil {
		return err
	}

	s.logger.Debug("OpAMP server started.")

	return nil
}

func (s *Supervisor) handleAgentOpAMPMessage(conn serverTypes.Connection, message *protobufs.AgentToServer) {
	s.agentConn.Store(conn)

	s.logger.Debug("Received OpAMP message from the agent")
	if message.AgentDescription != nil {
		s.setAgentDescription(message.AgentDescription)
	}

	if message.EffectiveConfig != nil {
		if cfg, ok := message.EffectiveConfig.GetConfigMap().GetConfigMap()[""]; ok {
			s.logger.Debug("Received effective config from agent")
			s.effectiveConfig.Store(string(cfg.Body))
			err := s.opampClient.UpdateEffectiveConfig(context.Background())
			if err != nil {
				s.logger.Error("The OpAMP client failed to update the effective config", zap.Error(err))
			}
		} else {
			s.logger.Error("Got effective config message, but the instance config was not present. Ignoring effective config.")
		}
	}

	// Proxy client capabilities to server
	if message.CustomCapabilities != nil {
		err := s.opampClient.SetCustomCapabilities(message.CustomCapabilities)
		if err != nil {
			s.logger.Error("Failed to send custom capabilities to OpAMP server")
		}
	}

	// Proxy agent custom messages to server
	if message.CustomMessage != nil {
		select {
		case s.customMessageToServer <- message.CustomMessage:
		default:
			s.logger.Warn(
				"Buffer full, skipping forwarding custom message to server",
				zap.String("capability", message.CustomMessage.Capability),
				zap.String("type", message.CustomMessage.Type),
			)
		}
	}
}

func (s *Supervisor) forwardCustomMessagesToServerLoop() {
	for {
		select {
		case cm := <-s.customMessageToServer:
			for {
				sendingChan, err := s.opampClient.SendCustomMessage(cm)
				switch {
				case errors.Is(err, types.ErrCustomMessagePending):
					s.logger.Debug("Custom message pending, waiting to send...")
					<-sendingChan
					continue
				case err == nil: // OK
					s.logger.Debug("Custom message forwarded to server.")
				default:
					s.logger.Error("Failed to send custom message to OpAMP server")
				}
				break
			}
		case <-s.doneChan:
			return
		}
	}
}

// setAgentDescription sets the agent description, merging in any user-specified attributes from the supervisor configuration.
func (s *Supervisor) setAgentDescription(ad *protobufs.AgentDescription) {
	ad.IdentifyingAttributes = applyKeyValueOverrides(s.config.Agent.Description.IdentifyingAttributes, ad.IdentifyingAttributes)
	ad.NonIdentifyingAttributes = applyKeyValueOverrides(s.config.Agent.Description.NonIdentifyingAttributes, ad.NonIdentifyingAttributes)
	s.agentDescription.Store(ad)
}

// applyKeyValueOverrides merges the overrides map into the array of key value pairs.
// If a key from overrides already exists in the array of key value pairs, it is overwritten by the value from the overrides map.
// An array of KeyValue pair is returned, with each key value pair having a distinct key.
func applyKeyValueOverrides(overrides map[string]string, orig []*protobufs.KeyValue) []*protobufs.KeyValue {
	kvMap := make(map[string]*protobufs.KeyValue, len(orig)+len(overrides))

	for _, kv := range orig {
		kvMap[kv.Key] = kv
	}

	for k, v := range overrides {
		kvMap[k] = &protobufs.KeyValue{
			Key: k,
			Value: &protobufs.AnyValue{
				Value: &protobufs.AnyValue_StringValue{
					StringValue: v,
				},
			},
		}
	}

	// Sort keys for stable output, makes it easier to test.
	keys := make([]string, 0, len(kvMap))
	for k := range kvMap {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	kvOut := make([]*protobufs.KeyValue, 0, len(kvMap))
	for _, k := range keys {
		v := kvMap[k]
		kvOut = append(kvOut, v)
	}

	return kvOut
}

func (s *Supervisor) stopOpAMPClient() error {
	s.logger.Debug("Stopping OpAMP client...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := s.opampClient.Stop(ctx)
	// TODO(srikanthccv): remove context.DeadlineExceeded after https://github.com/open-telemetry/opamp-go/pull/213
	if err != nil && !errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	s.logger.Debug("OpAMP client stopped.")

	return nil
}

func (s *Supervisor) getHeadersFromSettings(protoHeaders *protobufs.Headers) http.Header {
	headers := make(http.Header)
	for _, header := range protoHeaders.Headers {
		headers.Add(header.Key, header.Value)
	}
	return headers
}

func (s *Supervisor) onOpampConnectionSettings(_ context.Context, settings *protobufs.OpAMPConnectionSettings) error {
	if settings == nil {
		s.logger.Debug("Received ConnectionSettings request with nil settings")
		return nil
	}

	newServerConfig := config.OpAMPServer{}

	if settings.DestinationEndpoint != "" {
		newServerConfig.Endpoint = settings.DestinationEndpoint
	}
	if settings.Headers != nil {
		newServerConfig.Headers = s.getHeadersFromSettings(settings.Headers)
	}
	if settings.Certificate != nil {
		if len(settings.Certificate.CaPublicKey) != 0 {
			newServerConfig.TLSSetting.CAPem = configopaque.String(settings.Certificate.CaPublicKey)
		}
		if len(settings.Certificate.PublicKey) != 0 {
			newServerConfig.TLSSetting.CertPem = configopaque.String(settings.Certificate.PublicKey)
		}
		if len(settings.Certificate.PrivateKey) != 0 {
			newServerConfig.TLSSetting.KeyPem = configopaque.String(settings.Certificate.PrivateKey)
		}
	} else {
		newServerConfig.TLSSetting = configtls.ClientConfig{Insecure: true}
	}

	if err := newServerConfig.Validate(); err != nil {
		s.logger.Error("New OpAMP settings resulted in invalid configuration", zap.Error(err))
		return err
	}

	if err := s.stopOpAMPClient(); err != nil {
		s.logger.Error("Cannot stop the OpAMP client", zap.Error(err))
		return err
	}

	// take a copy of the current OpAMP server config
	oldServerConfig := s.config.Server
	// update the OpAMP server config
	s.config.Server = newServerConfig

	if err := s.startOpAMPClient(); err != nil {
		s.logger.Error("Cannot connect to the OpAMP server using the new settings", zap.Error(err))
		// revert the OpAMP server config
		s.config.Server = oldServerConfig
		// start the OpAMP client with the old settings
		if err := s.startOpAMPClient(); err != nil {
			s.logger.Error("Cannot reconnect to the OpAMP server after restoring old settings", zap.Error(err))
			return err
		}
	}
	return nil
}

func (s *Supervisor) composeNoopPipeline() ([]byte, error) {
	var cfg bytes.Buffer
	err := s.noopPipelineTemplate.Execute(&cfg, map[string]any{
		"InstanceUid":    s.persistentState.InstanceID.String(),
		"SupervisorPort": s.opampServerPort,
	})
	if err != nil {
		return nil, err
	}

	return cfg.Bytes(), nil
}

func (s *Supervisor) composeNoopConfig() ([]byte, error) {
	var k = koanf.New("::")

	cfg, err := s.composeNoopPipeline()
	if err != nil {
		return nil, err
	}
	if err = k.Load(rawbytes.Provider(cfg), yaml.Parser(), koanf.WithMergeFunc(configMergeFunc)); err != nil {
		return nil, err
	}
	if err = k.Load(rawbytes.Provider(s.composeOpAMPExtensionConfig()), yaml.Parser(), koanf.WithMergeFunc(configMergeFunc)); err != nil {
		return nil, err
	}

	return k.Marshal(yaml.Parser())
}

func (s *Supervisor) composeExtraLocalConfig() []byte {
	var cfg bytes.Buffer
	resourceAttrs := map[string]string{}
	ad := s.agentDescription.Load().(*protobufs.AgentDescription)
	for _, attr := range ad.IdentifyingAttributes {
		resourceAttrs[attr.Key] = attr.Value.GetStringValue()
	}
	for _, attr := range ad.NonIdentifyingAttributes {
		resourceAttrs[attr.Key] = attr.Value.GetStringValue()
	}
	tplVars := map[string]any{
		"Healthcheck":        s.agentHealthCheckEndpoint,
		"ResourceAttributes": resourceAttrs,
		"SupervisorPort":     s.opampServerPort,
	}
	err := s.extraConfigTemplate.Execute(
		&cfg,
		tplVars,
	)
	if err != nil {
		s.logger.Error("Could not compose local config", zap.Error(err))
		return nil
	}

	return cfg.Bytes()
}

func (s *Supervisor) composeOpAMPExtensionConfig() []byte {
	orphanPollInterval := 5 * time.Second
	if s.config.Agent.OrphanDetectionInterval > 0 {
		orphanPollInterval = s.config.Agent.OrphanDetectionInterval
	}

	var cfg bytes.Buffer
	tplVars := map[string]any{
		"InstanceUid":      s.persistentState.InstanceID.String(),
		"SupervisorPort":   s.opampServerPort,
		"PID":              s.pidProvider.PID(),
		"PPIDPollInterval": orphanPollInterval,
	}
	err := s.opampextensionTemplate.Execute(
		&cfg,
		tplVars,
	)
	if err != nil {
		s.logger.Error("Could not compose local config", zap.Error(err))
		return nil
	}

	return cfg.Bytes()
}

func (s *Supervisor) loadAndWriteInitialMergedConfig() error {
	var lastRecvRemoteConfig, lastRecvOwnMetricsConfig []byte
	var err error

	if s.config.Capabilities.AcceptsRemoteConfig {
		// Try to load the last received remote config if it exists.
		lastRecvRemoteConfig, err = os.ReadFile(filepath.Join(s.config.Storage.Directory, lastRecvRemoteConfigFile))
		if err == nil {
			config := &protobufs.AgentRemoteConfig{}
			err = proto.Unmarshal(lastRecvRemoteConfig, config)
			if err != nil {
				s.logger.Error("Cannot parse last received remote config", zap.Error(err))
			} else {
				s.remoteConfig = config
			}
		} else {
			s.logger.Error("error while reading last received config", zap.Error(err))
		}
	} else {
		s.logger.Debug("Remote config is not supported, will not attempt to load config from fil")
	}

	if s.config.Capabilities.ReportsOwnMetrics {
		// Try to load the last received own metrics config if it exists.
		lastRecvOwnMetricsConfig, err = os.ReadFile(filepath.Join(s.config.Storage.Directory, lastRecvOwnMetricsConfigFile))
		if err == nil {
			set := &protobufs.TelemetryConnectionSettings{}
			err = proto.Unmarshal(lastRecvOwnMetricsConfig, set)
			if err != nil {
				s.logger.Error("Cannot parse last received own metrics config", zap.Error(err))
			} else {
				s.setupOwnMetrics(context.Background(), set)
			}
		}
	} else {
		s.logger.Debug("Own metrics is not supported, will not attempt to load config from file")
	}

	_, err = s.composeMergedConfig(s.remoteConfig)
	if err != nil {
		return fmt.Errorf("could not compose initial merged config: %w", err)
	}

	// write the initial merged config to disk
	cfg := s.mergedConfig.Load().(string)
	if err := os.WriteFile(agentConfigFilePath, []byte(cfg), 0600); err != nil {
		s.logger.Error("Failed to write agent config.", zap.Error(err))
	}

	return nil
}

// createEffectiveConfigMsg create an EffectiveConfig with the content of the
// current effective config.
func (s *Supervisor) createEffectiveConfigMsg() *protobufs.EffectiveConfig {
	cfgStr, ok := s.effectiveConfig.Load().(string)
	if !ok {
		cfgStr, ok = s.mergedConfig.Load().(string)
		if !ok {
			cfgStr = ""
		}
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
	var cfg bytes.Buffer
	if settings.DestinationEndpoint == "" {
		// No destination. Disable metric collection.
		s.logger.Debug("Disabling own metrics pipeline in the config")
	} else {
		s.logger.Debug("Enabling own metrics pipeline in the config")

		port, err := s.findRandomPort()

		if err != nil {
			s.logger.Error("Could not setup own metrics", zap.Error(err))
			return
		}

		err = s.ownTelemetryTemplate.Execute(
			&cfg,
			map[string]any{
				"PrometheusPort":  port,
				"MetricsEndpoint": settings.DestinationEndpoint,
			},
		)
		if err != nil {
			s.logger.Error("Could not setup own metrics", zap.Error(err))
			return
		}

	}
	s.agentConfigOwnMetricsSection.Store(cfg.String())

	// Need to recalculate the Agent config so that the metric config is included in it.
	configChanged, err := s.composeMergedConfig(s.remoteConfig)
	if err != nil {
		s.logger.Error("Error composing merged config for own metrics. Ignoring agent self metrics config", zap.Error(err))
		return
	}

	return configChanged
}

// composeMergedConfig composes the merged config from multiple sources:
// 1) the remote config from OpAMP Server
// 2) the own metrics config section
// 3) the local override config that is hard-coded in the Supervisor.
func (s *Supervisor) composeMergedConfig(config *protobufs.AgentRemoteConfig) (configChanged bool, err error) {
	var k = koanf.New("::")

	if c := config.GetConfig(); c != nil {
		// Sort to make sure the order of merging is stable.
		var names []string
		for name := range c.ConfigMap {
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
			item := c.ConfigMap[name]
			if item == nil {
				continue
			}
			var k2 = koanf.New("::")
			err = k2.Load(rawbytes.Provider(item.Body), yaml.Parser())
			if err != nil {
				return false, fmt.Errorf("cannot parse config named %s: %w", name, err)
			}
			err = k.Merge(k2)
			if err != nil {
				return false, fmt.Errorf("cannot merge config named %s: %w", name, err)
			}
		}
	} else {
		// Add noop pipeline
		var noopConfig []byte
		noopConfig, err = s.composeNoopPipeline()
		if err != nil {
			return false, fmt.Errorf("could not compose noop pipeline: %w", err)
		}

		if err = k.Load(rawbytes.Provider(noopConfig), yaml.Parser(), koanf.WithMergeFunc(configMergeFunc)); err != nil {
			return false, fmt.Errorf("could not merge noop pipeline: %w", err)
		}
	}

	// Merge own metrics config.
	ownMetricsCfg, ok := s.agentConfigOwnMetricsSection.Load().(string)
	if ok {
		if err = k.Load(rawbytes.Provider([]byte(ownMetricsCfg)), yaml.Parser(), koanf.WithMergeFunc(configMergeFunc)); err != nil {
			return false, err
		}
	}

	// Merge local config last since it has the highest precedence.
	if err = k.Load(rawbytes.Provider(s.composeExtraLocalConfig()), yaml.Parser(), koanf.WithMergeFunc(configMergeFunc)); err != nil {
		return false, err
	}

	if err = k.Load(rawbytes.Provider(s.composeOpAMPExtensionConfig()), yaml.Parser(), koanf.WithMergeFunc(configMergeFunc)); err != nil {
		return false, err
	}

	// The merged final result is our new merged config.
	newMergedConfigBytes, err := k.Marshal(yaml.Parser())
	if err != nil {
		return false, err
	}

	// Check if supervisor's merged config is changed.
	newMergedConfig := string(newMergedConfigBytes)
	configChanged = false

	oldConfig := s.mergedConfig.Swap(newMergedConfig)
	if oldConfig == nil || oldConfig.(string) != newMergedConfig {
		s.logger.Debug("Merged config changed.")
		configChanged = true
	}

	return configChanged, nil
}

func (s *Supervisor) handleRestartCommand() error {
	s.agentRestarting.Store(true)
	defer s.agentRestarting.Store(false)
	s.logger.Debug("Received restart command")
	err := s.commander.Restart(context.Background())
	if err != nil {
		s.logger.Error("Could not restart agent process", zap.Error(err))
	}
	return err
}

func (s *Supervisor) startAgent() {
	err := s.commander.Start(context.Background())
	if err != nil {
		s.logger.Error("Cannot start the agent", zap.Error(err))
		err = s.opampClient.SetHealth(&protobufs.ComponentHealth{Healthy: false, LastError: fmt.Sprintf("Cannot start the agent: %v", err)})

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
	health := &protobufs.ComponentHealth{
		StartTimeUnixNano: uint64(s.startedAt.UnixNano()),
	}

	if err != nil {
		health.Healthy = false
		if !s.agentHasStarted && s.agentStartHealthCheckAttempts < 10 {
			health.LastError = "Agent is starting"
			s.agentStartHealthCheckAttempts++
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
	if _, err := os.Stat(agentConfigFilePath); err == nil {
		// We have an effective config file saved previously. Use it to start the agent.
		s.logger.Debug("Effective config found, starting agent initial time")
		s.startAgent()
	}

	restartTimer := time.NewTimer(0)
	restartTimer.Stop()

	for {
		select {
		case <-s.hasNewConfig:
			s.logger.Debug("Restarting agent due to new config")
			restartTimer.Stop()
			s.stopAgentApplyConfig()
			s.startAgent()

		case <-s.commander.Exited():
			// the agent process exit is expected for restart command and will not attempt to restart
			if s.agentRestarting.Load() {
				continue
			}

			s.logger.Debug("Agent process exited unexpectedly. Will restart in a bit...", zap.Int("pid", s.commander.Pid()), zap.Int("exit_code", s.commander.ExitCode()))
			errMsg := fmt.Sprintf(
				"Agent process PID=%d exited unexpectedly, exit code=%d. Will restart in a bit...",
				s.commander.Pid(), s.commander.ExitCode(),
			)
			err := s.opampClient.SetHealth(&protobufs.ComponentHealth{Healthy: false, LastError: errMsg})

			if err != nil {
				s.logger.Error("Could not report health to OpAMP server", zap.Error(err))
			}

			// TODO: decide why the agent stopped. If it was due to bad config, report it to server.
			// https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/21079

			// Wait 5 seconds before starting again.
			if !restartTimer.Stop() {
				select {
				case <-restartTimer.C: // Try to drain the channel
				default:
				}
			}
			restartTimer.Reset(5 * time.Second)

		case <-restartTimer.C:
			s.logger.Debug("Agent starting after start backoff")
			s.startAgent()

		case <-s.healthCheckTicker.C:
			s.healthCheck()

		case <-s.doneChan:
			err := s.commander.Stop(context.Background())
			if err != nil {
				s.logger.Error("Could not stop agent process", zap.Error(err))
			}
			return
		}
	}
}

func (s *Supervisor) stopAgentApplyConfig() {
	s.logger.Debug("Stopping the agent to apply new config")
	cfg := s.mergedConfig.Load().(string)
	err := s.commander.Stop(context.Background())

	if err != nil {
		s.logger.Error("Could not stop agent process", zap.Error(err))
	}

	if err := os.WriteFile(agentConfigFilePath, []byte(cfg), 0600); err != nil {
		s.logger.Error("Failed to write agent config.", zap.Error(err))
	}
}

func (s *Supervisor) Shutdown() {
	s.logger.Debug("Supervisor shutting down...")
	close(s.doneChan)

	// Shutdown in order from producer to consumer (agent -> customMessageForwarder -> local OpAMP server -> client to remote OpAMP server).
	s.agentWG.Wait()
	s.customMessageWG.Wait()

	if s.opampServer != nil {
		s.logger.Debug("Stopping OpAMP server...")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err := s.opampServer.Stop(ctx)
		if err != nil {
			s.logger.Error("Could not stop the OpAMP Server")
		} else {
			s.logger.Debug("OpAMP server stopped.")
		}
	}

	if s.opampClient != nil {
		err := s.opampClient.SetHealth(
			&protobufs.ComponentHealth{
				Healthy: false, LastError: "Supervisor is shutdown",
			},
		)

		if err != nil {
			s.logger.Error("Could not report health to OpAMP server", zap.Error(err))
		}

		err = s.stopOpAMPClient()

		if err != nil {
			s.logger.Error("Could not stop the OpAMP client", zap.Error(err))
		}
	}

	if s.healthCheckTicker != nil {
		s.healthCheckTicker.Stop()
	}
}

func (s *Supervisor) saveLastReceivedConfig(config *protobufs.AgentRemoteConfig) error {
	cfg, err := proto.Marshal(config)
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(s.config.Storage.Directory, lastRecvRemoteConfigFile), cfg, 0600)
}

func (s *Supervisor) saveLastReceivedOwnTelemetrySettings(set *protobufs.TelemetryConnectionSettings, filePath string) error {
	cfg, err := proto.Marshal(set)
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(s.config.Storage.Directory, filePath), cfg, 0600)
}

func (s *Supervisor) onMessage(ctx context.Context, msg *types.MessageData) {
	configChanged := false
	if msg.RemoteConfig != nil {
		configChanged = configChanged || s.processRemoteConfigMessage(msg.RemoteConfig)
	}

	if msg.OwnMetricsConnSettings != nil {
		configChanged = configChanged || s.processOwnMetricsConnSettingsMessage(ctx, msg.OwnMetricsConnSettings)
	}

	if msg.AgentIdentification != nil {
		configChanged = configChanged || s.processAgentIdentificationMessage(msg.AgentIdentification)
	}

	// Update the agent config if any messages have touched the config
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

	messageToAgent := &protobufs.ServerToAgent{
		InstanceUid: s.persistentState.InstanceID[:],
	}
	haveMessageForAgent := false
	// Proxy server capabilities to opamp extension
	if msg.CustomCapabilities != nil {
		messageToAgent.CustomCapabilities = msg.CustomCapabilities
		haveMessageForAgent = true
	}

	// Proxy server messages to opamp extension
	if msg.CustomMessage != nil {
		messageToAgent.CustomMessage = msg.CustomMessage
		haveMessageForAgent = true
	}

	// Send any messages that need proxying to the agent.
	if haveMessageForAgent {
		conn, ok := s.agentConn.Load().(serverTypes.Connection)
		if ok {
			err := conn.Send(ctx, messageToAgent)
			if err != nil {
				s.logger.Error("Error forwarding message to agent from server", zap.Error(err))
			}
		}
	}
}

// processRemoteConfigMessage processes an AgentRemoteConfig message, returning true if the agent config has changed.
func (s *Supervisor) processRemoteConfigMessage(msg *protobufs.AgentRemoteConfig) bool {
	if err := s.saveLastReceivedConfig(msg); err != nil {
		s.logger.Error("Could not save last received remote config", zap.Error(err))
	}

	s.remoteConfig = msg
	s.logger.Debug("Received remote config from server", zap.String("hash", fmt.Sprintf("%x", s.remoteConfig.ConfigHash)))

	var err error
	configChanged, err := s.composeMergedConfig(s.remoteConfig)
	if err != nil {
		s.logger.Error("Error composing merged config. Reporting failed remote config status.", zap.Error(err))
		err = s.opampClient.SetRemoteConfigStatus(&protobufs.RemoteConfigStatus{
			LastRemoteConfigHash: msg.ConfigHash,
			Status:               protobufs.RemoteConfigStatuses_RemoteConfigStatuses_FAILED,
			ErrorMessage:         err.Error(),
		})
		if err != nil {
			s.logger.Error("Could not report failed OpAMP remote config status", zap.Error(err))
		}
	} else {
		err = s.opampClient.SetRemoteConfigStatus(&protobufs.RemoteConfigStatus{
			LastRemoteConfigHash: msg.ConfigHash,
			Status:               protobufs.RemoteConfigStatuses_RemoteConfigStatuses_APPLIED,
		})
		if err != nil {
			s.logger.Error("Could not report applied OpAMP remote config status", zap.Error(err))
		}
	}

	return configChanged
}

// processOwnMetricsConnSettingsMessage processes a TelemetryConnectionSettings message, returning true if the agent config has changed.
func (s *Supervisor) processOwnMetricsConnSettingsMessage(ctx context.Context, msg *protobufs.TelemetryConnectionSettings) bool {
	if err := s.saveLastReceivedOwnTelemetrySettings(msg, lastRecvOwnMetricsConfigFile); err != nil {
		s.logger.Error("Could not save last received own telemetry settings", zap.Error(err))
	}
	return s.setupOwnMetrics(ctx, msg)
}

// processAgentIdentificationMessage processes an AgentIdentification message, returning true if the agent config has changed.
func (s *Supervisor) processAgentIdentificationMessage(msg *protobufs.AgentIdentification) bool {
	newInstanceID, err := uuid.FromBytes(msg.NewInstanceUid)
	if err != nil {
		s.logger.Error("Failed to parse instance UUID", zap.Error(err))
		return false
	}

	s.logger.Debug("Agent identity is changing",
		zap.String("old_id", s.persistentState.InstanceID.String()),
		zap.String("new_id", newInstanceID.String()))

	err = s.persistentState.SetInstanceID(newInstanceID)
	if err != nil {
		s.logger.Error("Failed to persist new instance ID, instance ID will revert on restart.", zap.String("new_id", newInstanceID.String()), zap.Error(err))
	}

	err = s.opampClient.SetAgentDescription(s.agentDescription.Load().(*protobufs.AgentDescription))
	if err != nil {
		s.logger.Error("Failed to send agent description to OpAMP server")
	}

	return true
}

func (s *Supervisor) persistentStateFile() string {
	return filepath.Join(s.config.Storage.Directory, persistentStateFilePath)
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

// The default koanf behavior is to override lists in the config.
// Instead, we provide this function, which merges the source and destination config's
// extension lists by concatenating the two.
// Will be resolved by https://github.com/open-telemetry/opentelemetry-collector/issues/8754
func configMergeFunc(src, dest map[string]any) error {
	srcExtensions := maps.Search(src, []string{"service", "extensions"})
	destExtensions := maps.Search(dest, []string{"service", "extensions"})

	maps.Merge(src, dest)

	if destExt, ok := destExtensions.([]any); ok {
		if srcExt, ok := srcExtensions.([]any); ok {
			if service, ok := dest["service"].(map[string]any); ok {
				service["extensions"] = append(destExt, srcExt...)
			}
		}
	}

	return nil
}
