// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package model // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbatlasreceiver/internal/model"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
)

// LogEntry represents a MongoDB Atlas JSON log entry
type LogEntry struct {
	Timestamp  LogTimestamp           `json:"t"`
	Severity   string                 `json:"s"`
	Component  string                 `json:"c"`
	ID         int64                  `json:"id"`
	Context    string                 `json:"ctx"`
	Message    string                 `json:"msg"`
	Attributes map[string]interface{} `json:"attr"`
	// Raw is the original log line. It is not a part of the payload, but transient data added during decoding.
	Raw string `json:"-"`
}

// AuditLog represents a MongoDB Atlas JSON audit log entry
type AuditLog struct {
	Type      string         `json:"atype"`
	Timestamp LogTimestamp   `json:"ts"`
	ID        *ID            `json:"uuid,omitempty"`
	Local     Address        `json:"local"`
	Remote    Address        `json:"remote"`
	Users     []AuditUser    `json:"users"`
	Roles     []AuditRole    `json:"roles"`
	Result    int            `json:"result"`
	Param     map[string]any `json:"param"`
	// Raw is the original log line. It is not a part of the payload, but transient data added during decoding.
	Raw string `json:"-"`
}

// logTimestamp is the structure that represents a Log Timestamp
type LogTimestamp struct {
	Date string `json:"$date"`
}

type ID struct {
	Binary string `json:"$binary"`
	Type   string `json:"$type"`
}

type Address struct {
	IP         *string `json:"ip,omitempty"`
	Port       *int    `json:"port,omitempty"`
	SystemUser *bool   `json:"isSystemUser,omitempty"`
	UnixSocket *string `json:"unix,omitempty"`
}

type AuditRole struct {
	Role     string `json:"role"`
	Database string `json:"db"`
}

func (ar AuditRole) Pdata() pcommon.Map {
	m := pcommon.NewMap()
	m.EnsureCapacity(2)
	m.PutStr("role", ar.Role)
	m.PutStr("db", ar.Database)
	return m
}

type AuditUser struct {
	User     string `json:"user"`
	Database string `json:"db"`
}

func (ar AuditUser) Pdata() pcommon.Map {
	m := pcommon.NewMap()
	m.EnsureCapacity(2)
	m.PutStr("user", ar.User)
	m.PutStr("db", ar.Database)
	return m
}
