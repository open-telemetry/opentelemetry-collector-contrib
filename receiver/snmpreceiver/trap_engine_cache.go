// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package snmpreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/snmpreceiver"

import (
	"container/list"
	"errors"
	"time"

	"github.com/gosnmp/gosnmp"
)

// trapEngineEntry keeps localized keys and timeliness state under one lifetime.
// Only the receive goroutine accesses entries and their activity order.
type trapEngineEntry struct {
	auth         []byte
	privacy      []byte
	clock        trapEngineTime
	synchronized bool
	lastSeen     time.Time
	activity     *list.Element
}

// trapUSMKeys names the entry passed through the USM parser while authentication
// and notification validation are still in progress.
type trapUSMKeys = trapEngineEntry

type trapEngineCache struct {
	entries     map[string]trapEngineEntry
	activity    list.List
	maxEntries  int
	idleTimeout time.Duration
}

type trapEngineCandidate struct {
	packet    *gosnmp.SnmpPacket
	engine    string
	clock     trapEngineTime
	validated bool
}

func newTrapEngineCache(maxEntries int, idleTimeout time.Duration) *trapEngineCache {
	return &trapEngineCache{
		entries:     make(map[string]trapEngineEntry),
		maxEntries:  maxEntries,
		idleTimeout: idleTimeout,
	}
}

// expire removes only idle entries. The activity list orders entries by their
// last accepted notification, so cleanup visits expired entries and at most one
// live entry, rather than scanning the cache for every datagram.
func (c *trapEngineCache) expire(now time.Time) int {
	if c.idleTimeout == 0 {
		return 0
	}
	expired := 0
	for element := c.activity.Front(); element != nil; element = c.activity.Front() {
		engine := element.Value.(string)
		entry := c.entries[engine]
		if now.Sub(entry.lastSeen) < c.idleTimeout {
			break
		}
		delete(c.entries, engine)
		c.activity.Remove(element)
		expired++
	}
	return expired
}

func (p *trapParser) expireEngines(now time.Time) int {
	return p.cache.expire(now)
}

// nextEngineExpiration lets the receive goroutine wake for idle cleanup even
// when no datagrams arrive, without a separate cache-maintenance goroutine.
func (p *trapParser) nextEngineExpiration() time.Time {
	if p.cache.idleTimeout == 0 {
		return time.Time{}
	}
	oldest := p.cache.activity.Front()
	if oldest == nil {
		return time.Time{}
	}
	entry := p.engines[oldest.Value.(string)]
	return entry.lastSeen.Add(p.cache.idleTimeout)
}

// admitTrap stages a timeliness update. A successfully parsed engine can still
// fail notification decoding, so its clock and activity remain unchanged here.
func (p *trapParser) admitTrap(packet *gosnmp.SnmpPacket, now time.Time) error {
	if err := validateTrap(p.cfg, packet); err != nil {
		return err
	}
	if packet.Version != gosnmp.Version3 || packet.MsgFlags&gosnmp.AuthNoPriv == 0 {
		return nil
	}
	if p.pending == nil || p.pending.packet != packet {
		return errors.New("authenticated SNMP trap has no pending engine state")
	}
	params := packet.SecurityParameters.(*gosnmp.UsmSecurityParameters)
	entry := p.engines[p.pending.engine]
	clock, err := nextTrapEngineTime(entry.clock, entry.synchronized, params, now)
	if err != nil {
		return err
	}
	p.pending.clock = clock
	p.pending.validated = true
	return nil
}

// commitEngine runs after security and notification decoding succeed, including
// when downstream queue admission subsequently fails. Activity is independent
// of the clock anchor: an accepted duplicate refreshes lastSeen, not received.
func (p *trapParser) commitEngine(packet *gosnmp.SnmpPacket, now time.Time) bool {
	if p.pending == nil || p.pending.packet != packet || !p.pending.validated {
		return false
	}
	candidate := p.pending
	p.pending = nil
	entry := p.engines[candidate.engine]
	added := !entry.synchronized
	entry.clock = candidate.clock
	entry.synchronized = true
	entry.lastSeen = now
	if entry.activity == nil {
		entry.activity = p.cache.activity.PushBack(candidate.engine)
	} else {
		p.cache.activity.MoveToBack(entry.activity)
	}
	p.engines[candidate.engine] = entry
	return added
}

// discardEngine rolls back keys provisionally inserted by successful parsing.
// Previously admitted engines retain their keys, clock, and activity unchanged.
func (p *trapParser) discardEngine(packet *gosnmp.SnmpPacket) {
	if p.pending == nil || p.pending.packet != packet {
		return
	}
	if entry, exists := p.engines[p.pending.engine]; exists && !entry.synchronized {
		delete(p.engines, p.pending.engine)
	}
	p.pending = nil
}
