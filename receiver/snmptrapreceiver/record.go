// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package snmptrapreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/snmptrapreceiver"

import (
	"encoding/json"
	"time"
)

// Record is one trap or inform, ready to marshal as an OTel log body.
type Record struct {
	Time         time.Time `json:"time"`
	Source       string    `json:"source"`
	Version      string    `json:"version"`
	PDUType      string    `json:"pdu_type"`
	TrapOID      string    `json:"trap_oid"`
	TrapName     string    `json:"trap_name,omitempty"`
	MIB          string    `json:"mib,omitempty"`
	Community    string    `json:"community,omitempty"`
	EngineID     string    `json:"engine_id,omitempty"`
	ContextName  string    `json:"context_name,omitempty"`
	AgentAddress string    `json:"agent_address,omitempty"`
	Enterprise   string    `json:"enterprise,omitempty"`
	GenericTrap  *int      `json:"generic_trap,omitempty"`
	SpecificTrap *int      `json:"specific_trap,omitempty"`
	SysUpTime    uint      `json:"sys_up_time,omitempty"`
	Varbinds     []Varbind `json:"varbinds"`
}

// Varbind is one decoded PDU variable.
type Varbind struct {
	OID   string `json:"oid"`
	Name  string `json:"name,omitempty"`
	Type  string `json:"type"`
	Value any    `json:"value"`
}

// Lookup is a best-effort MIB name for an OID.
type Lookup struct {
	Name string
	MIB  string
}

// Translator resolves OIDs to textual names. Misses must not be fatal.
type Translator interface {
	Lookup(oid string) (Lookup, error)
}

// NoopTranslator never resolves names.
type NoopTranslator struct{}

// Lookup implements Translator.
func (NoopTranslator) Lookup(oid string) (Lookup, error) {
	return Lookup{Name: NormalizeOID(oid)}, errUnresolved
}

// Resolved reports whether trap identity was enriched past a numeric OID.
func (r Record) Resolved() bool {
	if r.TrapOID == "" {
		return false
	}
	return r.TrapName != "" && r.TrapName != r.TrapOID
}

// MarshalJSONLine is compact JSON for the OTel log body.
func (r Record) MarshalJSONLine() ([]byte, error) {
	return json.Marshal(r)
}
