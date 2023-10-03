// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package opampextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/opampextension"

import (
	"context"
	"net/http"
	"os"
	"runtime"

	"github.com/oklog/ulid/v2"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	semconv "go.opentelemetry.io/collector/semconv/v1.18.0"
	"go.uber.org/zap"

	"github.com/open-telemetry/opamp-go/client"
	"github.com/open-telemetry/opamp-go/client/types"
	"github.com/open-telemetry/opamp-go/protobufs"
)

// TODO: Replace with https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/27293
const localConfig = `
exporters:
  otlp:
    endpoint: localhost:1111
receivers:
  otlp:
    protocols:
      grpc: {}
      http: {}
service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: []
      exporters: [otlp]
`

type opampAgent struct {
	cfg    *Config
	logger *zap.Logger

	agentType    string
	agentVersion string

	instanceId ulid.ULID

	effectiveConfig string

	agentDescription *protobufs.AgentDescription

	opampClient client.OpAMPClient

	remoteConfigStatus *protobufs.RemoteConfigStatus
}

func (o *opampAgent) Start(_ context.Context, _ component.Host) error {
	o.opampClient = client.NewWebSocket(o.logger.Sugar())

	header := http.Header{}
	for k, v := range o.cfg.Headers {
		header.Set(k, string(v))
	}

	tls, err := o.cfg.TLSSetting.LoadTLSConfig()
	if err != nil {
		return err
	}

	settings := types.StartSettings{
		Header:         header,
		TLSConfig:      tls,
		OpAMPServerURL: o.cfg.Endpoint,
		InstanceUid:    o.instanceId.String(),
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
		// TODO: Include ReportsEffectiveConfig once the extension has access to the
		// collector's effective configuration.
		Capabilities: protobufs.AgentCapabilities_AgentCapabilities_ReportsStatus,
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

	sid, ok := res.Attributes().Get(semconv.AttributeServiceInstanceID)
	if ok {
		puid, err := ulid.Parse(sid.AsString())
		if err != nil {
			return nil, err
		}
		uid = puid
	}

	if cfg.InstanceUID != "" {
		puid, err := ulid.Parse(cfg.InstanceUID)
		if err != nil {
			return nil, err
		}
		uid = puid
	}

	agent := &opampAgent{
		cfg:             cfg,
		logger:          logger,
		agentType:       agentType,
		agentVersion:    agentVersion,
		instanceId:      uid,
		effectiveConfig: localConfig, // TODO: Replace with https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/27293
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
		stringKeyValue("service.instance.id", o.instanceId.String()),
		stringKeyValue("service.name", o.agentType),
		stringKeyValue("service.version", o.agentVersion),
	}

	nonIdent := []*protobufs.KeyValue{
		stringKeyValue("os.arch", runtime.GOARCH),
		stringKeyValue("os.family", runtime.GOOS),
		stringKeyValue("host.name", hostname),
	}

	o.agentDescription = &protobufs.AgentDescription{
		IdentifyingAttributes:    ident,
		NonIdentifyingAttributes: nonIdent,
	}

	return nil
}

func (o *opampAgent) updateAgentIdentity(instanceId ulid.ULID) {
	o.logger.Debug("OpAMP agent identity is being changed",
		zap.String("old_id", o.instanceId.String()),
		zap.String("new_id", instanceId.String()))
	o.instanceId = instanceId
}

func (o *opampAgent) composeEffectiveConfig() *protobufs.EffectiveConfig {
	return &protobufs.EffectiveConfig{
		ConfigMap: &protobufs.AgentConfigMap{
			ConfigMap: map[string]*protobufs.AgentConfigFile{
				"": {Body: []byte(o.effectiveConfig)},
			},
		},
	}
}

func (o *opampAgent) onMessage(ctx context.Context, msg *types.MessageData) {
	if msg.AgentIdentification == nil {
		return
	}

	instanceId, err := ulid.Parse(msg.AgentIdentification.NewInstanceUid)
	if err != nil {
		o.logger.Error("Failed to parse a new agent identity", zap.Error(err))
		return
	}

	o.updateAgentIdentity(instanceId)
}
