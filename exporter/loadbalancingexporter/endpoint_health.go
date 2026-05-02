// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadbalancingexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter"

import (
	"context"
	"errors"
	"net"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"go.opentelemetry.io/collector/consumer/consumererror"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type endpointFailureReason string

const (
	endpointFailureTimeout           endpointFailureReason = "timeout"
	endpointFailureUnavailable       endpointFailureReason = "unavailable"
	endpointFailureConnectionRefused endpointFailureReason = "connection_refused"
	endpointFailureConnectionReset   endpointFailureReason = "connection_reset"
	endpointFailureNoRoute           endpointFailureReason = "no_route"
	endpointFailureDNS               endpointFailureReason = "dns"
	endpointFailureUnknownTransport  endpointFailureReason = "unknown_transport"
	endpointFailureExporterStopping  endpointFailureReason = "exporter_stopping"
	endpointFailureActiveProbe       endpointFailureReason = "active_probe"
)

type endpointHealthSettings struct {
	enabled            bool
	quarantineDuration time.Duration
	rerouteOnFailure   bool
	maxRerouteAttempts int
	activeProbe        endpointHealthActiveProbeSettings
	now                func() time.Time
}

type endpointHealthActiveProbeSettings struct {
	enabled bool
	fall    int
	rise    int
}

type endpointHealthManager struct {
	mu                 sync.RWMutex
	settings           endpointHealthSettings
	endpoints          map[string]*endpointHealthState
	failOpenActive     bool
	nextExpiryUnixNano atomic.Int64
}

type endpointHealthState struct {
	endpoint          string
	present           bool
	quarantinedUntil  time.Time
	failureReason     endpointFailureReason
	lastSeenAt        time.Time
	lastFailedAt      time.Time
	lastStateChangeAt time.Time
	probeUnhealthy    bool
	probeFailures     int
	probeSuccesses    int
}

type endpointHealthReconcileResult struct {
	eligible        []string
	removed         []string
	failOpen        bool
	failOpenStarted bool
}

type endpointHealthFailureDecision struct {
	endpointLocal   bool
	quarantined     bool
	reason          endpointFailureReason
	eligible        []string
	failOpen        bool
	failOpenStarted bool
}

type endpointHealthSuccessDecision struct {
	recovered bool
	reason    endpointFailureReason
	eligible  []string
}

type endpointHealthRecovered struct {
	endpoint string
	reason   endpointFailureReason
}

type endpointHealthRefreshResult struct {
	eligible        []string
	recovered       []endpointHealthRecovered
	failOpenStarted bool
}

func newEndpointHealthManager(settings endpointHealthSettings) *endpointHealthManager {
	if settings.now == nil {
		settings.now = time.Now
	}
	if settings.quarantineDuration <= 0 {
		settings.quarantineDuration = defaultEndpointHealthQuarantineDuration
	}
	return &endpointHealthManager{
		settings:  settings,
		endpoints: make(map[string]*endpointHealthState),
	}
}

func endpointHealthSettingsFromConfig(cfg EndpointHealthConfig) endpointHealthSettings {
	quarantineDuration := cfg.QuarantineDuration
	if quarantineDuration <= 0 {
		quarantineDuration = defaultEndpointHealthQuarantineDuration
	}
	return endpointHealthSettings{
		enabled:            cfg.Enabled,
		quarantineDuration: quarantineDuration,
		rerouteOnFailure:   cfg.RerouteOnFailure,
		maxRerouteAttempts: cfg.MaxRerouteAttempts,
		activeProbe: endpointHealthActiveProbeSettings{
			enabled: cfg.ActiveProbe.Enabled,
			fall:    cfg.ActiveProbe.Fall,
			rise:    cfg.ActiveProbe.Rise,
		},
		now: time.Now,
	}
}

func (m *endpointHealthManager) enabled() bool {
	return m != nil && m.settings.enabled
}

func (m *endpointHealthManager) rerouteOnFailure() bool {
	return m != nil && m.settings.enabled && m.settings.rerouteOnFailure
}

func (m *endpointHealthManager) reconcile(resolved []string) endpointHealthReconcileResult {
	if !m.enabled() {
		eligible := append([]string(nil), resolved...)
		return endpointHealthReconcileResult{eligible: eligible}
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	now := m.settings.now()
	present := make(map[string]struct{}, len(resolved))
	for _, endpoint := range resolved {
		present[endpoint] = struct{}{}
		state := m.stateLocked(endpoint)
		state.lastSeenAt = now
		if !state.present {
			state.present = true
			state.lastStateChangeAt = now
		}
	}

	var removed []string
	for endpoint, state := range m.endpoints {
		if _, ok := present[endpoint]; ok || !state.present {
			continue
		}
		delete(m.endpoints, endpoint)
		removed = append(removed, endpoint)
	}
	sort.Strings(removed)

	eligible, failOpen, failOpenStarted := m.eligibleEndpointsLocked(now)
	return endpointHealthReconcileResult{
		eligible:        eligible,
		removed:         removed,
		failOpen:        failOpen,
		failOpenStarted: failOpenStarted,
	}
}

func (m *endpointHealthManager) markFailure(endpoint string, err error) endpointHealthFailureDecision {
	if !m.enabled() {
		return endpointHealthFailureDecision{}
	}

	reason, ok := classifyEndpointFailure(err)
	m.mu.Lock()
	defer m.mu.Unlock()

	now := m.settings.now()
	decision := endpointHealthFailureDecision{endpointLocal: ok, reason: reason}
	if ok {
		if state, exists := m.endpoints[endpoint]; exists && state.present {
			state.lastFailedAt = now
			state.failureReason = reason
			if state.quarantinedUntil.IsZero() || !state.quarantinedUntil.After(now) {
				state.quarantinedUntil = now.Add(m.settings.quarantineDuration)
				state.lastStateChangeAt = now
				decision.quarantined = true
			}
		}
	}
	decision.eligible, decision.failOpen, decision.failOpenStarted = m.eligibleEndpointsLocked(now)
	return decision
}

func (m *endpointHealthManager) markProbeFailure(endpoint string) endpointHealthFailureDecision {
	if !m.enabled() || !m.settings.activeProbe.enabled {
		return endpointHealthFailureDecision{}
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	state, exists := m.endpoints[endpoint]
	if !exists || !state.present {
		return endpointHealthFailureDecision{}
	}

	now := m.settings.now()
	state.lastFailedAt = now
	state.probeFailures++
	state.probeSuccesses = 0

	decision := endpointHealthFailureDecision{
		endpointLocal: true,
		reason:        endpointFailureActiveProbe,
	}
	if state.probeUnhealthy {
		if !state.hasActiveTransportQuarantine(now) {
			state.failureReason = endpointFailureActiveProbe
		}
	} else if state.probeFailures >= m.settings.activeProbe.fall {
		state.probeUnhealthy = true
		if !state.hasActiveTransportQuarantine(now) {
			state.failureReason = endpointFailureActiveProbe
		}
		state.lastStateChangeAt = now
		decision.quarantined = true
	}
	decision.eligible, decision.failOpen, decision.failOpenStarted = m.eligibleEndpointsLocked(now)
	return decision
}

func (m *endpointHealthManager) markSuccess(endpoint string) bool {
	return m.markSuccessDecision(endpoint).recovered
}

func (m *endpointHealthManager) markSuccessDecision(endpoint string) endpointHealthSuccessDecision {
	if !m.enabled() {
		return endpointHealthSuccessDecision{}
	}

	m.mu.RLock()
	state, ok := m.endpoints[endpoint]
	if !ok || !state.present || (state.quarantinedUntil.IsZero() && state.failureReason == "" && !state.probeUnhealthy) {
		m.mu.RUnlock()
		return endpointHealthSuccessDecision{}
	}
	m.mu.RUnlock()

	m.mu.Lock()
	defer m.mu.Unlock()

	state, ok = m.endpoints[endpoint]
	if !ok || !state.present {
		return endpointHealthSuccessDecision{}
	}
	if state.probeUnhealthy {
		return endpointHealthSuccessDecision{}
	}
	if !state.quarantinedUntil.IsZero() || state.failureReason != "" {
		reason := state.failureReason
		state.quarantinedUntil = time.Time{}
		state.failureReason = ""
		state.probeFailures = 0
		state.probeSuccesses = 0
		state.lastStateChangeAt = m.settings.now()
		eligible, _, _ := m.eligibleEndpointsLocked(state.lastStateChangeAt)
		return endpointHealthSuccessDecision{
			recovered: true,
			reason:    reason,
			eligible:  eligible,
		}
	}
	return endpointHealthSuccessDecision{}
}

func (m *endpointHealthManager) markProbeSuccess(endpoint string) endpointHealthSuccessDecision {
	if !m.enabled() || !m.settings.activeProbe.enabled {
		return endpointHealthSuccessDecision{}
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	state, ok := m.endpoints[endpoint]
	if !ok || !state.present {
		return endpointHealthSuccessDecision{}
	}
	state.probeFailures = 0
	if !state.probeUnhealthy {
		state.probeSuccesses = 0
		return endpointHealthSuccessDecision{}
	}

	state.probeSuccesses++
	if state.probeSuccesses < m.settings.activeProbe.rise {
		return endpointHealthSuccessDecision{}
	}

	now := m.settings.now()
	reason := state.failureReason
	state.probeUnhealthy = false
	state.probeSuccesses = 0
	state.lastStateChangeAt = now
	if !state.quarantinedUntil.IsZero() && state.quarantinedUntil.After(now) {
		_, _, _ = m.eligibleEndpointsLocked(now)
		return endpointHealthSuccessDecision{}
	}
	state.failureReason = ""
	eligible, _, _ := m.eligibleEndpointsLocked(now)
	return endpointHealthSuccessDecision{
		recovered: true,
		reason:    reason,
		eligible:  eligible,
	}
}

func (m *endpointHealthManager) isPresent(endpoint string) bool {
	if !m.enabled() {
		return true
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	state, ok := m.endpoints[endpoint]
	return ok && state.present
}

func (m *endpointHealthManager) failOpen() bool {
	if !m.enabled() {
		return false
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	_, _, _ = m.eligibleEndpointsLocked(m.settings.now())
	return m.failOpenActive
}

func (m *endpointHealthManager) eligibleEndpoints() []string {
	if !m.enabled() {
		return nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	eligible, _, _ := m.eligibleEndpointsLocked(m.settings.now())
	return eligible
}

func (m *endpointHealthManager) eligibleEndpointsNoRefresh() []string {
	if !m.enabled() {
		return nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	eligible, _, _ := m.eligibleEndpointsLockedWithRefresh(m.settings.now(), false)
	return eligible
}

func (m *endpointHealthManager) presentEndpoints() []string {
	if !m.enabled() {
		return nil
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	var endpoints []string
	for _, state := range m.endpoints {
		if state.present {
			endpoints = append(endpoints, state.endpoint)
		}
	}
	sort.Strings(endpoints)
	return endpoints
}

func (m *endpointHealthManager) refreshExpiredQuarantines() endpointHealthRefreshResult {
	if !m.enabled() {
		return endpointHealthRefreshResult{}
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	now := m.settings.now()
	var recovered []endpointHealthRecovered
	for _, state := range m.endpoints {
		if !state.present || state.quarantinedUntil.IsZero() || state.quarantinedUntil.After(now) {
			continue
		}
		if state.probeUnhealthy {
			state.quarantinedUntil = time.Time{}
			state.failureReason = endpointFailureActiveProbe
			continue
		}
		recovered = append(recovered, endpointHealthRecovered{endpoint: state.endpoint, reason: state.failureReason})
		state.quarantinedUntil = time.Time{}
		state.failureReason = ""
		state.lastStateChangeAt = now
	}

	eligible, _, failOpenStarted := m.eligibleEndpointsLocked(now)
	if len(recovered) == 0 && !failOpenStarted {
		return endpointHealthRefreshResult{}
	}

	return endpointHealthRefreshResult{
		eligible:        eligible,
		recovered:       recovered,
		failOpenStarted: failOpenStarted,
	}
}

func (m *endpointHealthManager) quarantineRefreshDue() bool {
	if !m.enabled() {
		return false
	}
	nextExpiryUnixNano := m.nextExpiryUnixNano.Load()
	if nextExpiryUnixNano == 0 {
		return false
	}
	return !m.settings.now().Before(time.Unix(0, nextExpiryUnixNano))
}

func (m *endpointHealthManager) stateLocked(endpoint string) *endpointHealthState {
	state, ok := m.endpoints[endpoint]
	if !ok {
		state = &endpointHealthState{endpoint: endpoint}
		m.endpoints[endpoint] = state
	}
	return state
}

func (m *endpointHealthManager) eligibleEndpointsLocked(now time.Time) ([]string, bool, bool) {
	return m.eligibleEndpointsLockedWithRefresh(now, true)
}

func (m *endpointHealthManager) eligibleEndpointsLockedWithRefresh(now time.Time, refreshExpired bool) ([]string, bool, bool) {
	var eligible []string
	var present []string
	var nextExpiry time.Time
	for _, state := range m.endpoints {
		if !state.present {
			continue
		}
		present = append(present, state.endpoint)
		if !state.quarantinedUntil.IsZero() {
			if !state.quarantinedUntil.After(now) && refreshExpired {
				state.quarantinedUntil = time.Time{}
				if state.probeUnhealthy {
					state.failureReason = endpointFailureActiveProbe
				} else {
					state.failureReason = ""
					state.lastStateChangeAt = now
				}
			}
			if !state.quarantinedUntil.IsZero() && state.quarantinedUntil.After(now) && (nextExpiry.IsZero() || state.quarantinedUntil.Before(nextExpiry)) {
				nextExpiry = state.quarantinedUntil
			}
		}
		if state.quarantinedUntil.IsZero() && !state.probeUnhealthy {
			eligible = append(eligible, state.endpoint)
		}
	}
	if nextExpiry.IsZero() {
		m.nextExpiryUnixNano.Store(0)
	} else {
		m.nextExpiryUnixNano.Store(nextExpiry.UnixNano())
	}

	sort.Strings(present)
	sort.Strings(eligible)
	failOpen := len(present) > 0 && len(eligible) == 0
	failOpenStarted := failOpen && !m.failOpenActive
	m.failOpenActive = failOpen
	if failOpen {
		return present, true, failOpenStarted
	}
	return eligible, false, false
}

func (s *endpointHealthState) hasActiveTransportQuarantine(now time.Time) bool {
	return !s.quarantinedUntil.IsZero() &&
		s.quarantinedUntil.After(now) &&
		s.failureReason != "" &&
		s.failureReason != endpointFailureActiveProbe
}

func classifyEndpointFailure(err error) (endpointFailureReason, bool) {
	if err == nil || consumererror.IsPermanent(err) || errors.Is(err, context.Canceled) {
		return "", false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return endpointFailureTimeout, true
	}
	if isEndpointDNSFailure(err) {
		return endpointFailureDNS, true
	}
	switch status.Code(err) {
	case codes.DeadlineExceeded:
		return endpointFailureTimeout, true
	case codes.Unavailable:
		return endpointFailureUnavailable, true
	}

	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return endpointFailureTimeout, true
	}

	switch {
	case errors.Is(err, syscall.ECONNREFUSED):
		return endpointFailureConnectionRefused, true
	case errors.Is(err, syscall.ECONNRESET), errors.Is(err, syscall.EPIPE):
		return endpointFailureConnectionReset, true
	case errors.Is(err, syscall.EHOSTUNREACH), errors.Is(err, syscall.ENETUNREACH):
		return endpointFailureNoRoute, true
	}

	errText := strings.ToLower(err.Error())
	switch {
	case strings.Contains(errText, "i/o timeout"):
		return endpointFailureTimeout, true
	case strings.Contains(errText, "connection refused"):
		return endpointFailureConnectionRefused, true
	case strings.Contains(errText, "connection reset"), strings.Contains(errText, "broken pipe"):
		return endpointFailureConnectionReset, true
	case strings.Contains(errText, "no route to host"),
		strings.Contains(errText, "host unreachable"),
		strings.Contains(errText, "network unreachable"):
		return endpointFailureNoRoute, true
	case strings.Contains(errText, "transport: error while dialing"),
		strings.Contains(errText, "transport is closing"),
		strings.Contains(errText, "dial tcp"),
		strings.Contains(errText, "read tcp"),
		strings.Contains(errText, "write tcp"):
		return endpointFailureUnknownTransport, true
	default:
		return "", false
	}
}

func isEndpointDNSFailure(err error) bool {
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return true
	}
	errText := strings.ToLower(err.Error())
	return strings.HasPrefix(errText, "lookup ") || strings.Contains(errText, "no such host")
}
