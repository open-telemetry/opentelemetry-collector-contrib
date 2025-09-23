// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package models provides data structures for system-level metrics and information.
// This file defines the data models used to represent SQL Server system and host information
// that is automatically collected and added as resource attributes to all metrics.
package models

// SystemInformation represents host and SQL Server system information
// This provides essential host/system context for all metrics collected from SQL Server
type SystemInformation struct {
	// SQL Server Instance Information
	ServerName   *string `db:"sql_instance" description:"SQL Server instance name"`
	ComputerName *string `db:"computer_name" description:"Physical machine/host name"`
	ServiceName  *string `db:"service_name" description:"SQL Server service name"`

	// SQL Server Edition and Version
	Edition        *string `db:"sku" description:"SQL Server edition (Standard, Enterprise, etc.)"`
	EngineEdition  *int    `db:"engine_edition" description:"SQL Server engine edition ID"`
	ProductVersion *string `db:"sql_version" description:"SQL Server product version"`
	VersionDesc    *string `db:"sql_version_desc" description:"SQL Server version description"`

	// Hardware Information
	CPUCount          *int   `db:"cpu_count" description:"Number of logical processors"`
	ServerMemoryKB    *int64 `db:"server_memory" description:"Total physical memory in KB"`
	AvailableMemoryKB *int64 `db:"available_server_memory" description:"Available physical memory in KB"`

	// Instance Configuration
	IsClustered    *bool `db:"instance_type" description:"Whether SQL Server instance is clustered"`
	IsHadrEnabled  *bool `db:"is_hadr_enabled" description:"Whether AlwaysOn Availability Groups is enabled"`
	Uptime         *int  `db:"uptime" description:"SQL Server uptime in minutes"`
	ComputerUptime *int  `db:"computer_uptime" description:"Computer uptime in seconds"`

	// Network Configuration
	Port            *string `db:"Port" description:"SQL Server port number"`
	PortType        *string `db:"PortType" description:"Port type (Static or Dynamic)"`
	ForceEncryption *int    `db:"ForceEncryption" description:"Whether connection encryption is forced"`
}
