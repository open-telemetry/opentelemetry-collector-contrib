// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opampextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/opampextension"

import (
	"context"
	"net/http"
	"os"
	"runtime"
	"sync"

	"github.com/google/uuid"
	"github.com/oklog/ulid/v2"
	"github.com/open-telemetry/opamp-go/client"
	"github.com/open-telemetry/opamp-go/client/types"
	"github.com/open-telemetry/opamp-go/protobufs"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/pdata/pcommon"
	semconv "go.opentelemetry.io/collector/semconv/v1.18.0"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

type opampAgent struct {
	cfg    *Config
	logger *zap.Logger

	agentType    string
	agentVersion string

	instanceID ulid.ULID

	eclk            sync.RWMutex
	effectiveConfig *confmap.Conf

	capabilities Capabilities

	agentDescription *protobufs.AgentDescription

	opampClient client.OpAMPClient
}

func (o *opampAgent) Start(_ context.Context, _ component.Host) error {
	// TODO: Add OpAMP HTTP transport support.
	o.opampClient = client.NewWebSocket(o.logger.Sugar())

	header := http.Header{}
	for k, v := range o.cfg.Server.WS.Headers {
		header.Set(k, string(v))
	}

	tls, err := o.cfg.Server.WS.TLSSetting.LoadTLSConfig()
	if err != nil {
		return err
	}

	settings := types.StartSettings{
		Header:         header,
		TLSConfig:      tls,
		OpAMPServerURL: o.cfg.Server.WS.Endpoint,
		InstanceUid:    o.instanceID.String(),
		Callbacks: types.CallbacksStruct{
			OnConnectFunc: func() {
				o.logger.Debug("Connected to the OpAMP server")
			},
			OnConnectFailedFunc: func(err error) {
				o.logger.Error("Failed to connect to the OpAMP server", zap.Error(err))
			},
			OnErrorFunc: func(err *protobufs.ServerErrorResponse) {
				o.logger.Error("OpAMP server returned an error response", zap.String("message", err.ErrorMessage))
			},
			GetEffectiveConfigFunc: func(ctx context.Context) (*protobufs.EffectiveConfig, error) {
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
	o.logger.Debug("OpAMP agent shutting down...")
	if o.opampClient == nil {
		return nil
	}
	o.logger.Debug("Stopping OpAMP client...")
	return o.opampClient.Stop(ctx)
}

func (o *opampAgent) NotifyConfig(ctx context.Context, conf *confmap.Conf) error {
	if o.capabilities.ReportsEffectiveConfig {
		o.updateEffectiveConfig(conf)
		return o.opampClient.UpdateEffectiveConfig(ctx)
	}
	return nil
}

func (o *opampAgent) updateEffectiveConfig(conf *confmap.Conf) {
	o.eclk.Lock()
	defer o.eclk.Unlock()

	o.effectiveConfig = conf
}

func newOpampAgent(cfg *Config, logger *zap.Logger, build component.BuildInfo, res pcommon.Resource) (*opampAgent, error) {
	agentType := build.Command

	sn, ok := res.Attributes().Get(semconv.AttributeServiceName)
	if ok {
		agentType = sn.AsString()
	}

	agentVersion := build.Version

	sv, ok := res.Attributes().Get(semconv.AttributeServiceVersion)
	if ok {
		agentVersion = sv.AsString()
	}

	uid := ulid.Make()

	if cfg.InstanceUID != "" {
		puid, err := ulid.Parse(cfg.InstanceUID)
		if err != nil {
			return nil, err
		}
		uid = puid
	} else {
		sid, ok := res.Attributes().Get(semconv.AttributeServiceInstanceID)
		if ok {
			uuid, err := uuid.Parse(sid.AsString())
			if err != nil {
				return nil, err
			}
			uid = ulid.ULID(uuid)
		}
	}

	agent := &opampAgent{
		cfg:          cfg,
		logger:       logger,
		agentType:    agentType,
		agentVersion: agentVersion,
		instanceID:   uid,
		capabilities: cfg.Capabilities,
	}

	return agent, nil
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

	nonIdent := []*protobufs.KeyValue{
		stringKeyValue(semconv.AttributeOSType, runtime.GOOS),
		stringKeyValue(semconv.AttributeHostArch, runtime.GOARCH),
		stringKeyValue(semconv.AttributeHostName, hostname),
	}

	o.agentDescription = &protobufs.AgentDescription{
		IdentifyingAttributes:    ident,
		NonIdentifyingAttributes: nonIdent,
	}

	return nil
}

func (o *opampAgent) updateAgentIdentity(instanceID ulid.ULID) {
	o.logger.Debug("OpAMP agent identity is being changed",
		zap.String("old_id", o.instanceID.String()),
		zap.String("new_id", instanceID.String()))
	o.instanceID = instanceID
}

func (o *opampAgent) composeEffectiveConfig() *protobufs.EffectiveConfig {
	o.eclk.RLock()
	defer o.eclk.RUnlock()

	if !o.capabilities.ReportsEffectiveConfig || o.effectiveConfig == nil {
		return nil
	}

	conf, err := yaml.Marshal(o.effectiveConfig.ToStringMap())
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
	if msg.AgentIdentification == nil {
		return
	}

	instanceID, err := ulid.Parse(msg.AgentIdentification.NewInstanceUid)
	if err != nil {
		o.logger.Error("Failed to parse a new agent identity", zap.Error(err))
		return
	}

	o.updateAgentIdentity(instanceID)
}
