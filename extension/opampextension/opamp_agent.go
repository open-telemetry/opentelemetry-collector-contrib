// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opampextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/opampextension"

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"sort"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/oklog/ulid/v2"
	"github.com/open-telemetry/opamp-go/client"
	"github.com/open-telemetry/opamp-go/client/types"
	"github.com/open-telemetry/opamp-go/protobufs"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/extension"
	semconv "go.opentelemetry.io/collector/semconv/v1.18.0"
	"go.uber.org/zap"
	"golang.org/x/exp/maps"
	"gopkg.in/yaml.v3"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/opampcustommessages"
)

const redactedVal = "[REDACTED]"

// Paths that will not have values redacted when reporting the effective config.
var unredactedPaths = []string{
	"service::pipelines",
}

type opampAgent struct {
	cfg    *Config
	logger *zap.Logger

	agentType    string
	agentVersion string

	instanceID uuid.UUID

	eclk            sync.RWMutex
	effectiveConfig *confmap.Conf

	// lifetimeCtx is canceled on Stop of the component
	lifetimeCtx       context.Context
	lifetimeCtxCancel context.CancelFunc

	reportFunc func(*component.StatusEvent)

	capabilities Capabilities

	agentDescription *protobufs.AgentDescription

	opampClient client.OpAMPClient

	customCapabilityRegistry *customCapabilityRegistry
}

var _ opampcustommessages.CustomCapabilityRegistry = (*opampAgent)(nil)

func (o *opampAgent) Start(ctx context.Context, _ component.Host) error {
	header := http.Header{}
	for k, v := range o.cfg.Server.GetHeaders() {
		header.Set(k, string(v))
	}

	tls, err := o.cfg.Server.GetTLSSetting().LoadTLSConfig(ctx)
	if err != nil {
		return err
	}

	o.lifetimeCtx, o.lifetimeCtxCancel = context.WithCancel(context.Background())

	if o.cfg.PPID != 0 {
		go monitorPPID(o.lifetimeCtx, o.cfg.PPIDPollInterval, o.cfg.PPID, o.reportFunc)
	}

	settings := types.StartSettings{
		Header:         header,
		TLSConfig:      tls,
		OpAMPServerURL: o.cfg.Server.GetEndpoint(),
		InstanceUid:    types.InstanceUid(o.instanceID),
		Callbacks: types.CallbacksStruct{
			OnConnectFunc: func(_ context.Context) {
				o.logger.Debug("Connected to the OpAMP server")
			},
			OnConnectFailedFunc: func(_ context.Context, err error) {
				o.logger.Error("Failed to connect to the OpAMP server", zap.Error(err))
			},
			OnErrorFunc: func(_ context.Context, err *protobufs.ServerErrorResponse) {
				o.logger.Error("OpAMP server returned an error response", zap.String("message", err.ErrorMessage))
			},
			GetEffectiveConfigFunc: func(_ context.Context) (*protobufs.EffectiveConfig, error) {
				return o.composeEffectiveConfig(), nil
			},
			OnMessageFunc: o.onMessage,
		},
		Capabilities: o.capabilities.toAgentCapabilities(),
	}

	if err := o.createAgentDescription(); err != nil {
		return err
	}

	if err := o.opampClient.SetAgentDescription(o.agentDescription); err != nil {
		return err
	}

	o.logger.Debug("Starting OpAMP client...")

	if err := o.opampClient.Start(context.Background(), settings); err != nil {
		return err
	}

	o.logger.Debug("OpAMP client started")

	return nil
}

func (o *opampAgent) Shutdown(ctx context.Context) error {
	if o.lifetimeCtxCancel != nil {
		o.lifetimeCtxCancel()
	}

	o.logger.Debug("OpAMP agent shutting down...")
	if o.opampClient == nil {
		return nil
	}
	o.logger.Debug("Stopping OpAMP client...")
	err := o.opampClient.Stop(ctx)
	// Opamp-go considers this an error, but the collector does not.
	// https://github.com/open-telemetry/opamp-go/issues/255
	if err != nil && strings.EqualFold(err.Error(), "cannot stop because not started") {
		return nil
	}
	return err
}

func (o *opampAgent) NotifyConfig(ctx context.Context, conf *confmap.Conf) error {
	if o.capabilities.ReportsEffectiveConfig {
		o.updateEffectiveConfig(conf)
		return o.opampClient.UpdateEffectiveConfig(ctx)
	}
	return nil
}

func (o *opampAgent) Register(capability string, opts ...opampcustommessages.CustomCapabilityRegisterOption) (opampcustommessages.CustomCapabilityHandler, error) {
	return o.customCapabilityRegistry.Register(capability, opts...)
}

func (o *opampAgent) updateEffectiveConfig(conf *confmap.Conf) {
	o.eclk.Lock()
	defer o.eclk.Unlock()

	o.effectiveConfig = conf
}

func newOpampAgent(cfg *Config, set extension.Settings) (*opampAgent, error) {
	agentType := set.BuildInfo.Command

	sn, ok := set.Resource.Attributes().Get(semconv.AttributeServiceName)
	if ok {
		agentType = sn.AsString()
	}

	agentVersion := set.BuildInfo.Version

	sv, ok := set.Resource.Attributes().Get(semconv.AttributeServiceVersion)
	if ok {
		agentVersion = sv.AsString()
	}

	uid, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("could not generate uuidv7: %w", err)
	}

	if cfg.InstanceUID != "" {
		uid, err = parseInstanceIDString(cfg.InstanceUID)
		if err != nil {
			return nil, fmt.Errorf("could not parse configured instance id: %w", err)
		}
	} else {
		sid, ok := set.Resource.Attributes().Get(semconv.AttributeServiceInstanceID)
		if ok {
			uid, err = uuid.Parse(sid.AsString())
			if err != nil {
				return nil, err
			}
		}
	}

	opampClient := cfg.Server.GetClient(set.Logger)
	agent := &opampAgent{
		cfg:                      cfg,
		logger:                   set.Logger,
		agentType:                agentType,
		agentVersion:             agentVersion,
		instanceID:               uid,
		capabilities:             cfg.Capabilities,
		opampClient:              opampClient,
		customCapabilityRegistry: newCustomCapabilityRegistry(set.Logger, opampClient),
		reportFunc:               set.ReportStatus,
	}

	return agent, nil
}

func parseInstanceIDString(instanceUID string) (uuid.UUID, error) {
	parsedUUID, uuidParseErr := uuid.Parse(instanceUID)
	if uuidParseErr == nil {
		return parsedUUID, nil
	}

	parsedULID, ulidParseErr := ulid.Parse(instanceUID)
	if ulidParseErr == nil {
		return uuid.UUID(parsedULID), nil
	}

	return uuid.Nil, errors.Join(uuidParseErr, ulidParseErr)
}

func stringKeyValue(key, value string) *protobufs.KeyValue {
	return &protobufs.KeyValue{
		Key: key,
		Value: &protobufs.AnyValue{
			Value: &protobufs.AnyValue_StringValue{StringValue: value},
		},
	}
}

func (o *opampAgent) createAgentDescription() error {
	hostname, err := os.Hostname()
	if err != nil {
		return err
	}

	ident := []*protobufs.KeyValue{
		stringKeyValue(semconv.AttributeServiceInstanceID, o.instanceID.String()),
		stringKeyValue(semconv.AttributeServiceName, o.agentType),
		stringKeyValue(semconv.AttributeServiceVersion, o.agentVersion),
	}

	// Initially construct using a map to properly deduplicate any keys that
	// are both automatically determined and defined in the config
	nonIdentifyingAttributeMap := map[string]string{}
	nonIdentifyingAttributeMap[semconv.AttributeOSType] = runtime.GOOS
	nonIdentifyingAttributeMap[semconv.AttributeHostArch] = runtime.GOARCH
	nonIdentifyingAttributeMap[semconv.AttributeHostName] = hostname

	for k, v := range o.cfg.AgentDescription.NonIdentifyingAttributes {
		nonIdentifyingAttributeMap[k] = v
	}

	// Sort the non identifying attributes to give them a stable order for tests
	keys := maps.Keys(nonIdentifyingAttributeMap)
	sort.Strings(keys)

	nonIdent := make([]*protobufs.KeyValue, 0, len(nonIdentifyingAttributeMap))
	for _, k := range keys {
		v := nonIdentifyingAttributeMap[k]
		nonIdent = append(nonIdent, stringKeyValue(k, v))
	}

	o.agentDescription = &protobufs.AgentDescription{
		IdentifyingAttributes:    ident,
		NonIdentifyingAttributes: nonIdent,
	}

	return nil
}

func (o *opampAgent) updateAgentIdentity(instanceID uuid.UUID) {
	o.logger.Debug("OpAMP agent identity is being changed",
		zap.String("old_id", o.instanceID.String()),
		zap.String("new_id", instanceID.String()))
	o.instanceID = instanceID
}

func redactConfig(cfg any, parentPath string) {
	switch val := cfg.(type) {
	case map[string]any:
		for k, v := range val {
			path := parentPath
			if path == "" {
				path = k
			} else {
				path += "::" + k
			}
			// We don't want to redact certain parts of the config
			// that are known not to contain secrets, e.g. pipelines.
			for _, p := range unredactedPaths {
				if p == path {
					return
				}
			}
			switch x := v.(type) {
			case map[string]any:
				redactConfig(x, path)
			case []any:
				redactConfig(x, path)
			default:
				val[k] = redactedVal
			}
		}
	case []any:
		for i, v := range val {
			switch x := v.(type) {
			case map[string]any:
				redactConfig(x, parentPath)
			case []any:
				redactConfig(x, parentPath)
			default:
				val[i] = redactedVal
			}
		}
	}
}

func (o *opampAgent) composeEffectiveConfig() *protobufs.EffectiveConfig {
	o.eclk.RLock()
	defer o.eclk.RUnlock()

	if !o.capabilities.ReportsEffectiveConfig || o.effectiveConfig == nil {
		return nil
	}

	m := o.effectiveConfig.ToStringMap()
	redactConfig(m, "")
	conf, err := yaml.Marshal(m)
	if err != nil {
		o.logger.Error("cannot unmarshal effectiveConfig", zap.Any("conf", o.effectiveConfig), zap.Error(err))
		return nil
	}

	return &protobufs.EffectiveConfig{
		ConfigMap: &protobufs.AgentConfigMap{
			ConfigMap: map[string]*protobufs.AgentConfigFile{
				"": {Body: conf},
			},
		},
	}
}

func (o *opampAgent) onMessage(_ context.Context, msg *types.MessageData) {
	if msg.AgentIdentification != nil {
		instanceID, err := uuid.FromBytes(msg.AgentIdentification.NewInstanceUid)
		if err != nil {
			o.logger.Error("Invalid agent ID provided as new instance UID", zap.Error(err))
		} else {
			o.updateAgentIdentity(instanceID)
		}
	}

	if msg.CustomMessage != nil {
		o.customCapabilityRegistry.ProcessMessage(msg.CustomMessage)
	}
}
