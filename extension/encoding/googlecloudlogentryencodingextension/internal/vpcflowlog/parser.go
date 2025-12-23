// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package vpcflowlog // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/googlecloudlogentryencodingextension/internal/vpcflowlog"

import (
	"fmt"
	"time"

	gojson "github.com/goccy/go-json"
	"go.opentelemetry.io/collector/pdata/pcommon"
	conventions "go.opentelemetry.io/otel/semconv/v1.38.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/googlecloudlogentryencodingextension/internal/shared"
)

// Template type for OTEL attribute names needing the side (source or destination) to be added
type attrNameTmpl string

const (
	// VPC flow log name suffixes for detection
	NetworkManagementNameSuffix = "networkmanagement.googleapis.com%2Fvpc_flows"
	ComputeNameSuffix           = "compute.googleapis.com%2Fvpc_flows"

	// Connection fields
	gcpVPCFlowReporter     = "gcp.vpc.flow.reporter"
	gcpVPCFlowBytesSent    = "gcp.vpc.flow.bytes_sent"
	gcpVPCFlowPacketsSent  = "gcp.vpc.flow.packets_sent"
	gcpVPCFlowStartTime    = "gcp.vpc.flow.start_time"
	gcpVPCFlowEndTime      = "gcp.vpc.flow.end_time"
	gcpVPCFlowNetworkRTTMs = "gcp.vpc.flow.network.rtt_ms"

	// Network service fields
	gcpVPCFlowNetworkServiceDSCP = "gcp.vpc.flow.network_service.dscp"

	// Instance fields
	gcpVPCFlowInstanceProjectIDTemplate attrNameTmpl = "gcp.vpc.flow.%s.instance.project.id"
	gcpVPCFlowInstanceVMRegionTemplate  attrNameTmpl = "gcp.vpc.flow.%s.instance.vm.region"
	gcpVPCFlowInstanceVMNameTemplate    attrNameTmpl = "gcp.vpc.flow.%s.instance.vm.name"
	gcpVPCFlowInstanceVMZoneTemplate    attrNameTmpl = "gcp.vpc.flow.%s.instance.vm.zone"
	gcpVPCFlowInstanceMIGNameTemplate   attrNameTmpl = "gcp.vpc.flow.%s.instance.managed_instance_group.name"
	gcpVPCFlowInstanceMIGRegionTemplate attrNameTmpl = "gcp.vpc.flow.%s.instance.managed_instance_group.region"
	gcpVPCFlowInstanceMIGZoneTemplate   attrNameTmpl = "gcp.vpc.flow.%s.instance.managed_instance_group.zone"
	gcpVPCFlowGoogleServiceTypeTemplate attrNameTmpl = "gcp.vpc.flow.%s.google_service.type"
	gcpVPCFlowGoogleServiceNameTemplate attrNameTmpl = "gcp.vpc.flow.%s.google_service.name"
	gcpVPCFlowGoogleServiceConnTemplate attrNameTmpl = "gcp.vpc.flow.%s.google_service.connectivity"

	// Location fields
	gcpVPCFlowASNTemplate          attrNameTmpl = "gcp.vpc.flow.%s.asn"
	gcpVPCFlowGeoCityTemplate      attrNameTmpl = "gcp.vpc.flow.%s.geo.city"
	gcpVPCFlowGeoContinentTemplate attrNameTmpl = "gcp.vpc.flow.%s.geo.continent"
	gcpVPCFlowGeoCountryTemplate   attrNameTmpl = "gcp.vpc.flow.%s.geo.country.iso_code.alpha3"
	gcpVPCFlowGeoRegionTemplate    attrNameTmpl = "gcp.vpc.flow.%s.geo.region"

	// VPC fields
	gcpVPCFlowProjectIDTemplate    attrNameTmpl = "gcp.vpc.flow.%s.project.id"
	gcpVPCFlowSubnetNameTemplate   attrNameTmpl = "gcp.vpc.flow.%s.subnet.name"
	gcpVPCFlowSubnetRegionTemplate attrNameTmpl = "gcp.vpc.flow.%s.subnet.region"
	gcpVPCFlowVPCNameTemplate      attrNameTmpl = "gcp.vpc.flow.%s.vpc.name"

	// Internet routing details
	gcpVPCFlowEgressASPaths = "gcp.vpc.flow.egress.as_paths"
)

// Flow side (source or destination)
type flowSide string

const (
	src  flowSide = "source"
	dest flowSide = "destination"
)

var validSides = []flowSide{src, dest}

// fmtAttributeNameUsingSide formats a template string with the given side
func fmtAttributeNameUsingSide(template attrNameTmpl, side flowSide) string {
	return fmt.Sprintf(string(template), string(side))
}

type vpcFlowLog struct {
	Connection  *connection `json:"connection"`
	Reporter    string      `json:"reporter"`
	BytesSent   string      `json:"bytes_sent"`
	PacketsSent string      `json:"packets_sent"`
	StartTime   *time.Time  `json:"start_time"`
	EndTime     *time.Time  `json:"end_time"`
	RTTMsec     string      `json:"rtt_msec"`

	// Network service
	NetworkService *networkService `json:"network_service"`

	// Instance details
	SrcInstance  *instance `json:"src_instance"`
	DestInstance *instance `json:"dest_instance"`

	// Location details
	SrcLocation  *location `json:"src_location"`
	DestLocation *location `json:"dest_location"`

	// VPC details
	SrcVPC  *vpc `json:"src_vpc"`
	DestVPC *vpc `json:"dest_vpc"`

	// Google service details
	SrcGoogleService  *googleService `json:"src_google_service"`
	DestGoogleService *googleService `json:"dest_google_service"`

	// Internet routing details
	InternetRoutingDetails *internetRoutingDetails `json:"internet_routing_details"`
}

type connection struct {
	Protocol *int64 `json:"protocol"`
	SrcIP    string `json:"src_ip"`
	DestIP   string `json:"dest_ip"`
	SrcPort  *int64 `json:"src_port"`
	DestPort *int64 `json:"dest_port"`
}

type networkService struct {
	DSCP *int64 `json:"dscp"`
}

type instance struct {
	ProjectID            string                `json:"project_id"`
	Region               string                `json:"region"`
	VMName               string                `json:"vm_name"`
	Zone                 string                `json:"zone"`
	ManagedInstanceGroup *managedInstanceGroup `json:"managed_instance_group"`
}

type managedInstanceGroup struct {
	Name   string `json:"name"`
	Region string `json:"region"`
	Zone   string `json:"zone"`
}

type googleService struct {
	Type         string `json:"type"`
	ServiceName  string `json:"service_name"`
	Connectivity string `json:"connectivity"`
}

type location struct {
	ASN       *int64 `json:"asn"`
	City      string `json:"city"`
	Continent string `json:"continent"`
	Country   string `json:"country"`
	Region    string `json:"region"`
}

type vpc struct {
	ProjectID        string `json:"project_id"`
	SubnetworkName   string `json:"subnetwork_name"`
	SubnetworkRegion string `json:"subnetwork_region"`
	VPCName          string `json:"vpc_name"`
}

type internetRoutingDetails struct {
	EgressASPath []egressASPath `json:"egress_as_path"`
}

type egressASPath struct {
	ASDetails []asDetails `json:"as_details"`
}

type asDetails struct {
	ASN *int64 `json:"asn"`
}

func handleConnection(conn *connection, attr pcommon.Map) {
	if conn == nil {
		return
	}

	// Map protocol number to string
	if conn.Protocol != nil {
		if protocolStr, exists := shared.ProtocolName(uint32(*conn.Protocol)); exists {
			attr.PutStr(string(conventions.NetworkTransportKey), protocolStr)
		}
	}

	shared.PutStr(string(conventions.SourceAddressKey), conn.SrcIP, attr)
	shared.PutStr(string(conventions.DestinationAddressKey), conn.DestIP, attr)

	// Only add port attributes if ports are present
	shared.PutInt(string(conventions.SourcePortKey), conn.SrcPort, attr)
	shared.PutInt(string(conventions.DestinationPortKey), conn.DestPort, attr)
}

func handleNetworkService(ns *networkService, attr pcommon.Map) {
	if ns == nil {
		return
	}

	shared.PutInt(gcpVPCFlowNetworkServiceDSCP, ns.DSCP, attr)
}

func handleInstance(inst *instance, side flowSide, attr pcommon.Map) error {
	if inst == nil {
		return nil
	}

	// Validate that side is one of our constants
	if side != src && side != dest {
		return fmt.Errorf("unsupported side %q passed to handleInstance. Must be one of %v", side, validSides)
	}

	shared.PutStr(fmtAttributeNameUsingSide(gcpVPCFlowInstanceProjectIDTemplate, side), inst.ProjectID, attr)
	shared.PutStr(fmtAttributeNameUsingSide(gcpVPCFlowInstanceVMRegionTemplate, side), inst.Region, attr)
	shared.PutStr(fmtAttributeNameUsingSide(gcpVPCFlowInstanceVMNameTemplate, side), inst.VMName, attr)
	shared.PutStr(fmtAttributeNameUsingSide(gcpVPCFlowInstanceVMZoneTemplate, side), inst.Zone, attr)

	if inst.ManagedInstanceGroup != nil {
		shared.PutStr(fmtAttributeNameUsingSide(gcpVPCFlowInstanceMIGNameTemplate, side), inst.ManagedInstanceGroup.Name, attr)
		shared.PutStr(fmtAttributeNameUsingSide(gcpVPCFlowInstanceMIGRegionTemplate, side), inst.ManagedInstanceGroup.Region, attr)
		shared.PutStr(fmtAttributeNameUsingSide(gcpVPCFlowInstanceMIGZoneTemplate, side), inst.ManagedInstanceGroup.Zone, attr)
	}

	return nil
}

func handleLocation(loc *location, side flowSide, attr pcommon.Map) error {
	if loc == nil {
		return nil
	}

	// Validate that side is one of our constants
	if side != src && side != dest {
		return fmt.Errorf("unsupported side %q passed to handleLocation. Must be one of %v", side, validSides)
	}

	shared.PutInt(fmtAttributeNameUsingSide(gcpVPCFlowASNTemplate, side), loc.ASN, attr)
	shared.PutStr(fmtAttributeNameUsingSide(gcpVPCFlowGeoCityTemplate, side), loc.City, attr)
	shared.PutStr(fmtAttributeNameUsingSide(gcpVPCFlowGeoContinentTemplate, side), loc.Continent, attr)
	shared.PutStr(fmtAttributeNameUsingSide(gcpVPCFlowGeoCountryTemplate, side), loc.Country, attr)
	shared.PutStr(fmtAttributeNameUsingSide(gcpVPCFlowGeoRegionTemplate, side), loc.Region, attr)

	return nil
}

func handleVPC(vpc *vpc, side flowSide, attr pcommon.Map) error {
	if vpc == nil {
		return nil
	}

	// Validate that side is one of our constants
	if side != src && side != dest {
		return fmt.Errorf("unsupported side %q passed to handleVPC. Must be one of %v", side, validSides)
	}

	shared.PutStr(fmtAttributeNameUsingSide(gcpVPCFlowProjectIDTemplate, side), vpc.ProjectID, attr)
	shared.PutStr(fmtAttributeNameUsingSide(gcpVPCFlowSubnetNameTemplate, side), vpc.SubnetworkName, attr)
	shared.PutStr(fmtAttributeNameUsingSide(gcpVPCFlowSubnetRegionTemplate, side), vpc.SubnetworkRegion, attr)
	shared.PutStr(fmtAttributeNameUsingSide(gcpVPCFlowVPCNameTemplate, side), vpc.VPCName, attr)

	return nil
}

func handleGoogleService(service *googleService, side flowSide, attr pcommon.Map) error {
	if service == nil {
		return nil
	}

	if side != src && side != dest {
		return fmt.Errorf("unsupported side %q passed to handleGoogleService. Must be one of %v", side, validSides)
	}

	shared.PutStr(fmtAttributeNameUsingSide(gcpVPCFlowGoogleServiceTypeTemplate, side), service.Type, attr)
	shared.PutStr(fmtAttributeNameUsingSide(gcpVPCFlowGoogleServiceNameTemplate, side), service.ServiceName, attr)
	shared.PutStr(fmtAttributeNameUsingSide(gcpVPCFlowGoogleServiceConnTemplate, side), service.Connectivity, attr)

	return nil
}

func handleInternetRoutingDetails(ird *internetRoutingDetails, attr pcommon.Map) {
	if ird == nil || len(ird.EgressASPath) == 0 {
		return
	}

	asPaths := attr.PutEmptySlice(gcpVPCFlowEgressASPaths)
	for _, path := range ird.EgressASPath {
		pathMap := asPaths.AppendEmpty().SetEmptyMap()
		asDetailsSlice := pathMap.PutEmptySlice("as_details")
		for _, detail := range path.ASDetails {
			detailMap := asDetailsSlice.AppendEmpty().SetEmptyMap()
			if detail.ASN != nil {
				detailMap.PutInt("asn", *detail.ASN)
			}
		}
	}
}

func ParsePayloadIntoAttributes(payload []byte, attr pcommon.Map) error {
	var log vpcFlowLog
	if err := gojson.Unmarshal(payload, &log); err != nil {
		return fmt.Errorf("failed to unmarshal VPC flow log payload: %w", err)
	}

	// Handle connection details
	handleConnection(log.Connection, attr)

	// Handle basic flow information
	shared.PutStr(gcpVPCFlowReporter, log.Reporter, attr)

	if err := shared.AddStrAsInt(gcpVPCFlowBytesSent, log.BytesSent, attr); err != nil {
		return fmt.Errorf("failed to add bytes sent: %w", err)
	}

	if err := shared.AddStrAsInt(gcpVPCFlowPacketsSent, log.PacketsSent, attr); err != nil {
		return fmt.Errorf("failed to add packets sent: %w", err)
	}

	if log.StartTime != nil {
		startTime := *log.StartTime
		attr.PutStr(gcpVPCFlowStartTime, startTime.Format(time.RFC3339Nano))
	}
	if log.EndTime != nil {
		endTime := *log.EndTime
		attr.PutStr(gcpVPCFlowEndTime, endTime.Format(time.RFC3339Nano))
	}

	// Handle RTT
	if err := shared.AddStrAsInt(gcpVPCFlowNetworkRTTMs, log.RTTMsec, attr); err != nil {
		return fmt.Errorf("failed to add RTT: %w", err)
	}

	// Handle network service
	handleNetworkService(log.NetworkService, attr)

	// Handle instance details
	if err := handleInstance(log.SrcInstance, src, attr); err != nil {
		return fmt.Errorf("failed to handle source instance: %w", err)
	}
	if err := handleInstance(log.DestInstance, dest, attr); err != nil {
		return fmt.Errorf("failed to handle destination instance: %w", err)
	}

	// Handle location details
	if err := handleLocation(log.SrcLocation, src, attr); err != nil {
		return fmt.Errorf("failed to handle source location: %w", err)
	}
	if err := handleLocation(log.DestLocation, dest, attr); err != nil {
		return fmt.Errorf("failed to handle destination location: %w", err)
	}

	// Handle VPC details
	if err := handleVPC(log.SrcVPC, src, attr); err != nil {
		return fmt.Errorf("failed to handle source VPC: %w", err)
	}
	if err := handleVPC(log.DestVPC, dest, attr); err != nil {
		return fmt.Errorf("failed to handle destination VPC: %w", err)
	}

	// Handle Google service details
	if err := handleGoogleService(log.SrcGoogleService, src, attr); err != nil {
		return fmt.Errorf("failed to handle source Google service: %w", err)
	}
	if err := handleGoogleService(log.DestGoogleService, dest, attr); err != nil {
		return fmt.Errorf("failed to handle destination Google service: %w", err)
	}

	// Handle internet routing details
	handleInternetRoutingDetails(log.InternetRoutingDetails, attr)

	return nil
}
