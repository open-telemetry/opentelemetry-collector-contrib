// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opampextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/opampextension"

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"maps"
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
	"github.com/shirou/gopsutil/v4/host"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/extension/extensioncapabilities"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/service"
	"go.opentelemetry.io/collector/service/hostcapabilities"
	conventions "go.opentelemetry.io/otel/semconv/v1.38.0"
	"go.uber.org/zap"
	expmaps "golang.org/x/exp/maps"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"gopkg.in/yaml.v3"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/opampcustommessages"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/status"
)

type statusAggregator interface {
	Subscribe(scope status.Scope, verbosity status.Verbosity) (<-chan *status.AggregateStatus, status.UnsubscribeFunc)
	RecordStatus(source *componentstatus.InstanceID, event *componentstatus.Event)
}

type eventSourcePair struct {
	source *componentstatus.InstanceID
	event  *componentstatus.Event
}

type opampAgent struct {
	cfg    *Config
	logger *zap.Logger

	agentType     string
	agentVersion  string
	resourceAttrs map[string]string

	instanceID uuid.UUID

	eclk            sync.RWMutex
	effectiveConfig *confmap.Conf

	// lifetimeCtx is canceled on Stop of the component
	lifetimeCtx       context.Context
	lifetimeCtxCancel context.CancelFunc

	reportFunc func(*componentstatus.Event)

	capabilities Capabilities

	agentDescription    *protobufs.AgentDescription
	availableComponents *protobufs.AvailableComponents

	opampClient client.OpAMPClient

	customCapabilityRegistry *customCapabilityRegistry

	statusAggregator     statusAggregator
	statusSubscriptionWg *sync.WaitGroup
	componentHealthWg    *sync.WaitGroup
	startTimeUnixNano    uint64
	componentStatusCh    chan *eventSourcePair
	readyCh              chan struct{}
}

var (
	_ opampcustommessages.CustomCapabilityRegistry = (*opampAgent)(nil)
	_ extensioncapabilities.Dependent              = (*opampAgent)(nil)
	_ extensioncapabilities.ConfigWatcher          = (*opampAgent)(nil)
	_ extensioncapabilities.PipelineWatcher        = (*opampAgent)(nil)
	_ componentstatus.Watcher                      = (*opampAgent)(nil)

	// identifyingAttributes is the list of semantic convention keys that are used
	// for the agent description's identifying attributes.
	identifyingAttributes = map[string]struct{}{
		string(conventions.ServiceNameKey):       {},
		string(conventions.ServiceVersionKey):    {},
		string(conventions.ServiceInstanceIDKey): {},
	}
)

func (o *opampAgent) Start(ctx context.Context, host component.Host) error {
	o.reportFunc = func(event *componentstatus.Event) {
		componentstatus.ReportStatus(host, event)
	}

	header := http.Header{}
	for k, v := range o.cfg.Server.GetHeaders() {
		header.Set(k, string(v))
	}

	tls, err := o.cfg.Server.GetTLSConfig(ctx)
	if err != nil {
		return err
	}

	if o.cfg.PPID != 0 {
		go monitorPPID(o.lifetimeCtx, o.cfg.PPIDPollInterval, o.cfg.PPID, o.reportFunc)
	}

	headerFunc, err := makeHeadersFunc(o.logger, o.cfg.Server, host)
	if err != nil {
		return err
	}

	settings := types.StartSettings{
		Header:         header,
		HeaderFunc:     headerFunc,
		TLSConfig:      tls,
		OpAMPServerURL: o.cfg.Server.GetEndpoint(),
		InstanceUid:    types.InstanceUid(o.instanceID),
		Callbacks: types.Callbacks{
			OnConnect: func(_ context.Context) {
				o.logger.Debug("Connected to the OpAMP server")
			},
			OnConnectFailed: func(_ context.Context, err error) {
				o.logger.Error("Failed to connect to the OpAMP server", zap.Error(err))
			},
			OnError: func(_ context.Context, err *protobufs.ServerErrorResponse) {
				o.logger.Error("OpAMP server returned an error response", zap.String("message", err.ErrorMessage))
			},
			GetEffectiveConfig: func(_ context.Context) (*protobufs.EffectiveConfig, error) {
				return o.composeEffectiveConfig(), nil
			},
			OnMessage: o.onMessage,
		},
	}

	if err := o.createAgentDescription(); err != nil {
		return err
	}

	if err := o.opampClient.SetAgentDescription(o.agentDescription); err != nil {
		return err
	}

	if mi, ok := host.(hostcapabilities.ModuleInfo); ok {
		o.initAvailableComponents(mi.GetModuleInfos())
	} else if o.capabilities.ReportsAvailableComponents {
		// init empty availableComponents to not get an error when starting the opampClient
		o.initAvailableComponents(service.ModuleInfos{})
	}

	if o.availableComponents != nil {
		if err := o.opampClient.SetAvailableComponents(o.availableComponents); err != nil {
			return err
		}
	}

	capabilities := o.capabilities.toAgentCapabilities()
	if err := o.opampClient.SetCapabilities(&capabilities); err != nil {
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

	o.statusSubscriptionWg.Wait()
	o.componentHealthWg.Wait()
	if o.componentStatusCh != nil {
		close(o.componentStatusCh)
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

// Dependencies implements extensioncapabilities.Dependent
func (o *opampAgent) Dependencies() []component.ID {
	if o.cfg.Server == nil {
		return nil
	}

	var emptyComponentID component.ID
	authID := o.cfg.Server.GetAuthExtensionID()
	if authID == emptyComponentID {
		return nil
	}

	return []component.ID{authID}
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

func (o *opampAgent) Ready() error {
	o.setHealth(&protobufs.ComponentHealth{Healthy: true})
	close(o.readyCh)
	return nil
}

func (o *opampAgent) NotReady() error {
	o.setHealth(&protobufs.ComponentHealth{Healthy: false})
	return nil
}

// ComponentStatusChanged implements the componentstatus.Watcher interface.
func (o *opampAgent) ComponentStatusChanged(
	source *componentstatus.InstanceID,
	event *componentstatus.Event,
) {
	// There can be late arriving events after shutdown. We need to close
	// the event channel so that this function doesn't block and we release all
	// goroutines, but attempting to write to a closed channel will panic; log
	// and recover.
	defer func() {
		if r := recover(); r != nil {
			o.logger.Info(
				"discarding event received after shutdown",
				zap.Any("source", source),
				zap.Any("event", event),
			)
		}
	}()
	o.componentStatusCh <- &eventSourcePair{source: source, event: event}
}

func (o *opampAgent) updateEffectiveConfig(conf *confmap.Conf) {
	o.eclk.Lock()
	defer o.eclk.Unlock()

	o.effectiveConfig = conf
}

func newOpampAgent(cfg *Config, set extension.Settings) (*opampAgent, error) {
	agentType := set.BuildInfo.Command

	sn, ok := set.Resource.Attributes().Get(string(conventions.ServiceNameKey))
	if ok {
		agentType = sn.AsString()
	}

	agentVersion := set.BuildInfo.Version

	sv, ok := set.Resource.Attributes().Get(string(conventions.ServiceVersionKey))
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
		sid, ok := set.Resource.Attributes().Get(string(conventions.ServiceInstanceIDKey))
		if ok {
			uid, err = uuid.Parse(sid.AsString())
			if err != nil {
				return nil, err
			}
		}
	}
	resourceAttrs := make(map[string]string, set.Resource.Attributes().Len())
	set.Resource.Attributes().Range(func(k string, v pcommon.Value) bool {
		resourceAttrs[k] = v.Str()
		return true
	})

	opampClient := cfg.Server.GetClient(set.Logger)
	agent := &opampAgent{
		cfg:                      cfg,
		logger:                   set.Logger,
		agentType:                agentType,
		agentVersion:             agentVersion,
		instanceID:               uid,
		capabilities:             cfg.Capabilities,
		opampClient:              opampClient,
		resourceAttrs:            resourceAttrs,
		statusSubscriptionWg:     &sync.WaitGroup{},
		componentHealthWg:        &sync.WaitGroup{},
		readyCh:                  make(chan struct{}),
		customCapabilityRegistry: newCustomCapabilityRegistry(set.Logger, opampClient),
	}

	agent.lifetimeCtx, agent.lifetimeCtxCancel = context.WithCancel(context.Background())

	if agent.capabilities.ReportsHealth {
		agent.initHealthReporting()
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
	description := getOSDescription(o.logger)

	ident := []*protobufs.KeyValue{
		stringKeyValue(string(conventions.ServiceInstanceIDKey), o.instanceID.String()),
		stringKeyValue(string(conventions.ServiceNameKey), o.agentType),
		stringKeyValue(string(conventions.ServiceVersionKey), o.agentVersion),
	}

	// Initially construct using a map to properly deduplicate any keys that
	// are both automatically determined and defined in the config
	nonIdentifyingAttributeMap := map[string]string{}
	nonIdentifyingAttributeMap[string(conventions.OSTypeKey)] = runtime.GOOS
	nonIdentifyingAttributeMap[string(conventions.HostArchKey)] = runtime.GOARCH
	nonIdentifyingAttributeMap[string(conventions.HostNameKey)] = hostname
	nonIdentifyingAttributeMap[string(conventions.OSDescriptionKey)] = description

	maps.Copy(nonIdentifyingAttributeMap, o.cfg.AgentDescription.NonIdentifyingAttributes)
	if o.cfg.AgentDescription.IncludeResourceAttributes {
		for k, v := range o.resourceAttrs {
			// skip the attributes that are being used in the identifying attributes.
			if _, ok := identifyingAttributes[k]; ok {
				continue
			}
			nonIdentifyingAttributeMap[k] = v
		}
	}

	// Sort the non identifying attributes to give them a stable order for tests
	keys := expmaps.Keys(nonIdentifyingAttributeMap)
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

func (o *opampAgent) composeEffectiveConfig() *protobufs.EffectiveConfig {
	o.eclk.RLock()
	defer o.eclk.RUnlock()

	if !o.capabilities.ReportsEffectiveConfig || o.effectiveConfig == nil {
		return nil
	}

	m := o.effectiveConfig.ToStringMap()
	conf, err := yaml.Marshal(m)
	if err != nil {
		o.logger.Error("cannot unmarshal effectiveConfig", zap.Any("conf", o.effectiveConfig), zap.Error(err))
		return nil
	}

	return &protobufs.EffectiveConfig{
		ConfigMap: &protobufs.AgentConfigMap{
			ConfigMap: map[string]*protobufs.AgentConfigFile{
				"": {
					Body:        conf,
					ContentType: "text/yaml",
				},
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

func (o *opampAgent) setHealth(ch *protobufs.ComponentHealth) {
	if o.capabilities.ReportsHealth && o.opampClient != nil {
		if ch.Healthy && o.startTimeUnixNano == 0 {
			ch.StartTimeUnixNano = ch.StatusTimeUnixNano
		} else {
			ch.StartTimeUnixNano = o.startTimeUnixNano
		}
		if err := o.opampClient.SetHealth(ch); err != nil {
			o.logger.Error("Could not report health to OpAMP server", zap.Error(err))
		}
	}
}

func getOSDescription(logger *zap.Logger) string {
	info, err := host.Info()
	if err != nil {
		logger.Error("failed getting host info", zap.Error(err))
		return runtime.GOOS
	}
	switch runtime.GOOS {
	case "darwin":
		return "macOS " + info.PlatformVersion
	case "linux":
		return cases.Title(language.English).String(info.Platform) + " " + info.PlatformVersion
	case "windows":
		return info.Platform + " " + info.PlatformVersion
	default:
		return runtime.GOOS
	}
}

func (o *opampAgent) initHealthReporting() {
	if !o.capabilities.ReportsHealth {
		return
	}
	o.setHealth(&protobufs.ComponentHealth{Healthy: false})

	if o.statusAggregator == nil {
		o.statusAggregator = status.NewAggregator(status.PriorityPermanent)
	}
	statusChan, unsubscribeFunc := o.statusAggregator.Subscribe(status.ScopeAll, status.Verbose)
	o.statusSubscriptionWg.Add(1)
	go o.statusAggregatorEventLoop(unsubscribeFunc, statusChan)

	// Start processing events in the background so that our status watcher doesn't
	// block others before the extension starts.
	o.componentStatusCh = make(chan *eventSourcePair)
	o.componentHealthWg.Add(1)
	go o.componentHealthEventLoop()
}

func (o *opampAgent) initAvailableComponents(moduleInfos service.ModuleInfos) {
	if !o.capabilities.ReportsAvailableComponents {
		return
	}

	o.availableComponents = &protobufs.AvailableComponents{
		Hash: generateAvailableComponentsHash(moduleInfos),
		Components: map[string]*protobufs.ComponentDetails{
			"receivers": {
				SubComponentMap: createComponentTypeAvailableComponentDetails(moduleInfos.Receiver),
			},
			"processors": {
				SubComponentMap: createComponentTypeAvailableComponentDetails(moduleInfos.Processor),
			},
			"exporters": {
				SubComponentMap: createComponentTypeAvailableComponentDetails(moduleInfos.Exporter),
			},
			"extensions": {
				SubComponentMap: createComponentTypeAvailableComponentDetails(moduleInfos.Extension),
			},
			"connectors": {
				SubComponentMap: createComponentTypeAvailableComponentDetails(moduleInfos.Connector),
			},
		},
	}
}

func generateAvailableComponentsHash(moduleInfos service.ModuleInfos) []byte {
	var builder strings.Builder

	addComponentTypeComponentsToStringBuilder(&builder, moduleInfos.Receiver, "receiver")
	addComponentTypeComponentsToStringBuilder(&builder, moduleInfos.Processor, "processor")
	addComponentTypeComponentsToStringBuilder(&builder, moduleInfos.Exporter, "exporter")
	addComponentTypeComponentsToStringBuilder(&builder, moduleInfos.Extension, "extension")
	addComponentTypeComponentsToStringBuilder(&builder, moduleInfos.Connector, "connector")

	// Compute the SHA-256 hash of the serialized representation.
	hash := sha256.Sum256([]byte(builder.String()))
	return hash[:]
}

func addComponentTypeComponentsToStringBuilder(builder *strings.Builder, componentTypeComponents map[component.Type]service.ModuleInfo, componentType string) {
	// Collect components and sort them to ensure deterministic ordering.
	components := make([]component.Type, 0, len(componentTypeComponents))
	for k := range componentTypeComponents {
		components = append(components, k)
	}
	sort.Slice(components, func(i, j int) bool {
		return components[i].String() < components[j].String()
	})

	// Append the component type and its sorted key-value pairs.
	builder.WriteString(componentType + ":")
	for _, k := range components {
		builder.WriteString(k.String() + "=" + componentTypeComponents[k].BuilderRef + ";")
	}
}

func createComponentTypeAvailableComponentDetails(componentTypeComponents map[component.Type]service.ModuleInfo) map[string]*protobufs.ComponentDetails {
	availableComponentDetails := map[string]*protobufs.ComponentDetails{}
	for componentType, r := range componentTypeComponents {
		availableComponentDetails[componentType.String()] = &protobufs.ComponentDetails{
			Metadata: []*protobufs.KeyValue{
				{
					Key: "code.namespace",
					Value: &protobufs.AnyValue{
						Value: &protobufs.AnyValue_StringValue{
							StringValue: r.BuilderRef,
						},
					},
				},
			},
		}
	}
	return availableComponentDetails
}

func (o *opampAgent) componentHealthEventLoop() {
	// Record events with component.StatusStarting, but queue other events until
	// PipelineWatcher.Ready is called. This prevents aggregate statuses from
	// flapping between StatusStarting and StatusOK as components are started
	// individually by the service.
	var eventQueue []*eventSourcePair

	defer o.componentHealthWg.Done()
	for loop := true; loop; {
		select {
		case esp, ok := <-o.componentStatusCh:
			if !ok {
				return
			}
			if esp.event.Status() != componentstatus.StatusStarting {
				eventQueue = append(eventQueue, esp)
				continue
			}
			o.statusAggregator.RecordStatus(esp.source, esp.event)
		case <-o.readyCh:
			for _, esp := range eventQueue {
				o.statusAggregator.RecordStatus(esp.source, esp.event)
			}
			eventQueue = nil
			loop = false
		case <-o.lifetimeCtx.Done():
			return
		}
	}

	// After PipelineWatcher.Ready, record statuses as they are received.
	for {
		select {
		case esp, ok := <-o.componentStatusCh:
			if !ok {
				return
			}
			o.statusAggregator.RecordStatus(esp.source, esp.event)
		case <-o.lifetimeCtx.Done():
			return
		}
	}
}

func (o *opampAgent) statusAggregatorEventLoop(unsubscribeFunc status.UnsubscribeFunc, statusChan <-chan *status.AggregateStatus) {
	defer func() {
		unsubscribeFunc()
		o.statusSubscriptionWg.Done()
	}()
	for {
		select {
		case <-o.lifetimeCtx.Done():
			return
		case statusUpdate, ok := <-statusChan:
			if !ok {
				return
			}

			if statusUpdate == nil || statusUpdate.Status() == componentstatus.StatusNone {
				continue
			}

			componentHealth := convertComponentHealth(statusUpdate)

			o.setHealth(componentHealth)
		}
	}
}

func convertComponentHealth(statusUpdate *status.AggregateStatus) *protobufs.ComponentHealth {
	var isHealthy bool
	if statusUpdate.Status() == componentstatus.StatusOK {
		isHealthy = true
	} else {
		isHealthy = false
	}

	componentHealth := &protobufs.ComponentHealth{
		Healthy:            isHealthy,
		Status:             statusUpdate.Status().String(),
		StatusTimeUnixNano: uint64(statusUpdate.Timestamp().UnixNano()),
	}

	if statusUpdate.Err() != nil {
		componentHealth.LastError = statusUpdate.Err().Error()
	}

	if len(statusUpdate.ComponentStatusMap) > 0 {
		componentHealth.ComponentHealthMap = map[string]*protobufs.ComponentHealth{}
		for comp, compState := range statusUpdate.ComponentStatusMap {
			componentHealth.ComponentHealthMap[comp] = convertComponentHealth(compState)
		}
	}

	return componentHealth
}
