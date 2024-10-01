// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package snmpreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/snmpreceiver"

import (
	"time"

	"github.com/gosnmp/gosnmp"
)

// goSNMPWrapper is mostly to assist with mocking
type goSNMPWrapper interface {
	// Connect creates and opens a socket. Because UDP is a connectionless
	// protocol, you won't know if the remote host is responding until you send
	// packets. And if the host is regularly disappearing and reappearing, you won't
	// know if you've only done a Connect().
	//
	// For historical reasons (ie this is part of the public API), the method won't
	// be renamed.
	Connect() error

	// Close closes the connection
	Close() error

	// Get sends an SNMP GET request
	Get(oids []string) (result *gosnmp.SnmpPacket, err error)

	// WalkAll is similar to Walk but returns a filled array of all values rather
	// than using a callback function to stream results. Caution: if you have set
	// x.AppOpts to 'c', WalkAll may loop indefinitely and cause an Out Of Memory -
	// use Walk instead.
	WalkAll(rootOid string) (results []gosnmp.SnmpPDU, err error)

	// BulkWalkAll is similar to BulkWalk but returns a filled array of all values
	// rather than using a callback function to stream results. Caution: if you
	// have set x.AppOpts to 'c', BulkWalkAll may loop indefinitely and cause an
	// Out Of Memory - use BulkWalk instead.
	BulkWalkAll(rootOid string) (results []gosnmp.SnmpPDU, err error)

	// GetTransport gets the Transport
	GetTransport() string

	// SetTransport sets the Transport
	SetTransport(transport string)

	// GetTarget gets the Target
	GetTarget() string

	// SetTarget sets the Target
	SetTarget(target string)

	// GetPort gets the Port
	GetPort() uint16

	// SetPort sets the Port
	SetPort(port uint16)

	// GetCommunity gets the Community
	GetCommunity() string

	// SetCommunity sets the Community
	SetCommunity(community string)

	// GetVersion gets the Version
	GetVersion() gosnmp.SnmpVersion

	// SetVersion sets the Version
	SetVersion(version gosnmp.SnmpVersion)

	// GetTimeout gets the Timeout
	GetTimeout() time.Duration

	// SetTimeout sets the Timeout
	SetTimeout(timeout time.Duration)

	// GetMaxOids gets the MaxOids
	GetMaxOids() int

	// SetMaxOids sets the MaxOids
	SetMaxOids(maxOids int)

	// GetMsgFlags gets the MsgFlags
	GetMsgFlags() gosnmp.SnmpV3MsgFlags

	// SetMsgFlags sets the MsgFlags
	SetMsgFlags(msgFlags gosnmp.SnmpV3MsgFlags)

	// GetSecurityModel gets the SecurityModel
	GetSecurityModel() gosnmp.SnmpV3SecurityModel

	// SetSecurityModel sets the SecurityModel
	SetSecurityModel(securityModel gosnmp.SnmpV3SecurityModel)

	// GetSecurityParameters gets the SecurityParameters
	GetSecurityParameters() gosnmp.SnmpV3SecurityParameters

	// SetSecurityParameters sets the SecurityParameters
	SetSecurityParameters(securityParameters gosnmp.SnmpV3SecurityParameters)
}

// otelGoSNMPWrapper is a wrapper around gosnmp
type otelGoSNMPWrapper struct {
	gosnmp.GoSNMP
}

// newGoSNMPWrapper creates a new goSNMPWrapper using gosnmp
func newGoSNMPWrapper() goSNMPWrapper {
	return &otelGoSNMPWrapper{
		gosnmp.GoSNMP{
			MaxOids: gosnmp.Default.MaxOids,
		},
	}
}

// Close closes the GoSNMP connection
func (w *otelGoSNMPWrapper) Close() error {
	return w.GoSNMP.Conn.Close()
}

// GetTransport gets the Transport
func (w *otelGoSNMPWrapper) GetTransport() string {
	return w.GoSNMP.Transport
}

// SetTransport sets the Transport
func (w *otelGoSNMPWrapper) SetTransport(transport string) {
	w.GoSNMP.Transport = transport
}

// GetTarget gets the Target
func (w *otelGoSNMPWrapper) GetTarget() string {
	return w.GoSNMP.Target
}

// SetTarget sets the Target
func (w *otelGoSNMPWrapper) SetTarget(target string) {
	w.GoSNMP.Target = target
}

// GetPort gets the Port
func (w *otelGoSNMPWrapper) GetPort() uint16 {
	return w.GoSNMP.Port
}

// SetPort sets the Port
func (w *otelGoSNMPWrapper) SetPort(port uint16) {
	w.GoSNMP.Port = port
}

// GetCommunity gets the Community
func (w *otelGoSNMPWrapper) GetCommunity() string {
	return w.GoSNMP.Community
}

// SetCommunity sets the Community
func (w *otelGoSNMPWrapper) SetCommunity(community string) {
	w.GoSNMP.Community = community
}

// GetVersion gets the Version
func (w *otelGoSNMPWrapper) GetVersion() gosnmp.SnmpVersion {
	return w.GoSNMP.Version
}

// SetVersion sets the Version
func (w *otelGoSNMPWrapper) SetVersion(version gosnmp.SnmpVersion) {
	w.GoSNMP.Version = version
}

// GetTimeout gets the Timeout
func (w *otelGoSNMPWrapper) GetTimeout() time.Duration {
	return w.GoSNMP.Timeout
}

// SetTimeout sets the Timeout
func (w *otelGoSNMPWrapper) SetTimeout(timeout time.Duration) {
	w.GoSNMP.Timeout = timeout
}

// GetMaxOids gets the MaxOids
func (w *otelGoSNMPWrapper) GetMaxOids() int {
	return w.GoSNMP.MaxOids
}

// SetMaxOids sets the MaxOids
func (w *otelGoSNMPWrapper) SetMaxOids(maxOids int) {
	w.GoSNMP.MaxOids = maxOids
}

// GetMsgFlags gets the MsgFlags
func (w *otelGoSNMPWrapper) GetMsgFlags() gosnmp.SnmpV3MsgFlags {
	return w.GoSNMP.MsgFlags
}

// SetMsgFlags sets the MsgFlags
func (w *otelGoSNMPWrapper) SetMsgFlags(msgFlags gosnmp.SnmpV3MsgFlags) {
	w.GoSNMP.MsgFlags = msgFlags
}

// GetSecurityModel gets the SecurityModel
func (w *otelGoSNMPWrapper) GetSecurityModel() gosnmp.SnmpV3SecurityModel {
	return w.GoSNMP.SecurityModel
}

// SetSecurityModel sets the SecurityModel
func (w *otelGoSNMPWrapper) SetSecurityModel(securityModel gosnmp.SnmpV3SecurityModel) {
	w.GoSNMP.SecurityModel = securityModel
}

// GetSecurityParameters gets the SecurityParameters
func (w *otelGoSNMPWrapper) GetSecurityParameters() gosnmp.SnmpV3SecurityParameters {
	return w.GoSNMP.SecurityParameters
}

// SetSecurityParameters sets the SecurityParameters
func (w *otelGoSNMPWrapper) SetSecurityParameters(securityParameters gosnmp.SnmpV3SecurityParameters) {
	w.GoSNMP.SecurityParameters = securityParameters
}
