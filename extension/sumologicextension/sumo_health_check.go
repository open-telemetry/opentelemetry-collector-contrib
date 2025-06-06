package sumologicextension

import (
	"encoding/json"
	"fmt"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/status"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.uber.org/zap"
	"sync"
)

type SumoHealthCheck struct {
	logger *zap.Logger

	statusAggregator     statusAggregator
	statusSubscriptionWg *sync.WaitGroup
	componentHealthWg    *sync.WaitGroup
	startTimeUnixNano    uint64
	componentStatusCh    chan *eventSourcePair
	readyCh              chan struct{}
	closeChan            chan struct{}
}

type statusAggregator interface {
	Subscribe(scope status.Scope, verbosity status.Verbosity) (<-chan *status.AggregateStatus, status.UnsubscribeFunc)
	RecordStatus(source *componentstatus.InstanceID, event *componentstatus.Event)
}

type eventSourcePair struct {
	source *componentstatus.InstanceID
	event  *componentstatus.Event
}

func (hc *SumoHealthCheck) initHealthReporting() {
	hc.setHealth(map[string]any{"Healthy": false})

	if hc.statusAggregator == nil {
		hc.statusAggregator = status.NewAggregator(status.PriorityPermanent)
	}
	statusChan, unsubscribeFunc := hc.statusAggregator.Subscribe(status.ScopeAll, status.Verbose)
	hc.statusSubscriptionWg.Add(1)
	go hc.statusAggregatorEventLoop(unsubscribeFunc, statusChan)

	// Start processing events in the background so that our status watcher doesn't
	// block others before the extension starts.
	hc.componentStatusCh = make(chan *eventSourcePair)
	hc.componentHealthWg.Add(1)
	go hc.componentHealthEventLoop()
}

func (hc *SumoHealthCheck) statusAggregatorEventLoop(unsubscribeFunc status.UnsubscribeFunc, statusChan <-chan *status.AggregateStatus) {
	defer func() {
		unsubscribeFunc()
		hc.logger.Info("unsubscribed from health event aggregator")
		hc.statusSubscriptionWg.Done()
	}()
	for {
		select {
		case <-hc.closeChan:
			hc.logger.Info("stopping health aggregator event loop")
			return
		case statusUpdate, ok := <-statusChan:
			if !ok {
				return
			}

			if statusUpdate == nil || statusUpdate.Status() == componentstatus.StatusNone {
				continue
			}

			componentHealth := convertComponentHealth(statusUpdate)

			hc.setHealth(componentHealth)
		}
	}
}

func (hc *SumoHealthCheck) componentHealthEventLoop() {
	// Record events with component.StatusStarting, but queue other events until
	// PipelineWatcher.Ready is called. This prevents aggregate statuses from
	// flapping between StatusStarting and StatusOK as components are started
	// individually by the service.
	var eventQueue []*eventSourcePair

	defer func() {
		hc.logger.Info("componentHealthEventLoop finished")
		hc.componentHealthWg.Done()
	}()
	for loop := true; loop; {
		select {
		case esp, ok := <-hc.componentStatusCh:
			if !ok {
				hc.logger.Info("componentStatusCh closed")
				return
			}
			if esp.event.Status() != componentstatus.StatusStarting {
				eventQueue = append(eventQueue, esp)
				continue
			}
			hc.statusAggregator.RecordStatus(esp.source, esp.event)
		case <-hc.readyCh:
			hc.logger.Info("ready closed")
			for _, esp := range eventQueue {
				hc.statusAggregator.RecordStatus(esp.source, esp.event)
			}
			eventQueue = nil
			loop = false
		case <-hc.closeChan:
			hc.logger.Info("returning componentHealthEventLoop, closeChan stopped")
			return
		}
	}

	// After PipelineWatcher.Ready, record statuses as they are received.
	hc.logger.Info("start component wise  event loop")
	for {
		select {
		case esp, ok := <-hc.componentStatusCh:
			if !ok {
				return
			}
			hc.statusAggregator.RecordStatus(esp.source, esp.event)
		case <-hc.closeChan:
			hc.logger.Info("stopping component health event loop, closeChan stopped")
			return
		}
	}
}

func convertComponentHealth(statusUpdate *status.AggregateStatus) map[string]any {
	var isHealthy bool
	if statusUpdate.Status() == componentstatus.StatusOK {
		isHealthy = true
	} else {
		isHealthy = false
	}

	componentHealth := map[string]any{
		"Healthy":            isHealthy,
		"Status":             statusUpdate.Status().String(),
		"StatusTimeUnixNano": uint64(statusUpdate.Timestamp().UnixNano()),
	}

	if statusUpdate.Err() != nil {
		componentHealth["LastError"] = statusUpdate.Err().Error()
	}

	if len(statusUpdate.ComponentStatusMap) > 0 {
		componentHealth["componentHealth"] = map[string]map[string]any{}
		subMap := componentHealth["componentHealth"].(map[string]map[string]any)
		for comp, compState := range statusUpdate.ComponentStatusMap {
			subMap[comp] = convertComponentHealth(compState)
		}
	}

	return componentHealth
}

func (o *SumoHealthCheck) setHealth(healthMap map[string]any) {
	b, _ := json.MarshalIndent(healthMap, "", "  ")
	fmt.Println("healthMap:", string(b))
}

func (hc *SumoHealthCheck) shutdown() {
	hc.statusSubscriptionWg.Wait()
	hc.componentHealthWg.Wait()
	if hc.componentStatusCh != nil {
		close(hc.componentStatusCh)
	}
}

func (hc *SumoHealthCheck) ready() {
	hc.setHealth(map[string]any{"Healthy": true})
	close(hc.readyCh)
}

func (hc *SumoHealthCheck) notReady() {
	hc.setHealth(map[string]any{"Healthy": false})
}
