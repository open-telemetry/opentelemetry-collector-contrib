// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package vpcflowlog // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/googlecloudlogentryencodingextension/internal/vpcflowlog"

import (
	"fmt"

	gojson "github.com/goccy/go-json"
	"go.opentelemetry.io/collector/pdata/pcommon"
	semconv "go.opentelemetry.io/otel/semconv/v1.34.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/googlecloudlogentryencodingextension/internal/shared"
)

const (
	// VPC flow log name suffixes for detection
	VPCFlowLogNameSuffix        = "networkmanagement.googleapis.com%2Fvpc_flows"
	ComputeVPCFlowLogNameSuffix = "compute.googleapis.com%2Fvpc_flows"

	// Connection fields
	gcpVPCFlowReporter       = "gcp.vpc.flow.reporter"
	gcpVPCFlowBytesSent      = "gcp.vpc.flow.bytes_sent"
	gcpVPCFlowPacketsSent    = "gcp.vpc.flow.packets_sent"
	gcpVPCFlowStartTime      = "gcp.vpc.flow.start_time"
	gcpVPCFlowEndTime        = "gcp.vpc.flow.end_time"
	gcpVPCFlowNetworkRTTMsec = "gcp.vpc.flow.network.rtt_msec"

	// Network service fields
	gcpVPCFlowNetworkServiceDSCP = "gcp.vpc.flow.network_service.dscp"

	// Source instance fields
	gcpVPCFlowSourceInstanceProjectID = "gcp.vpc.flow.source.instance.project.id"
	gcpVPCFlowSourceInstanceVMRegion  = "gcp.vpc.flow.source.instance.vm.region"
	gcpVPCFlowSourceInstanceVMName    = "gcp.vpc.flow.source.instance.vm.name"
	gcpVPCFlowSourceInstanceVMZone    = "gcp.vpc.flow.source.instance.vm.zone"
	gcpVPCFlowSourceInstanceMIGName   = "gcp.vpc.flow.source.instance.managed_instance_group.name"
	gcpVPCFlowSourceInstanceMIGZone   = "gcp.vpc.flow.source.instance.managed_instance_group.zone"

	// Destination instance fields
	gcpVPCFlowDestInstanceProjectID = "gcp.vpc.flow.destination.instance.project.id"
	gcpVPCFlowDestInstanceVMRegion  = "gcp.vpc.flow.destination.instance.vm.region"
	gcpVPCFlowDestInstanceVMName    = "gcp.vpc.flow.destination.instance.vm.name"
	gcpVPCFlowDestInstanceVMZone    = "gcp.vpc.flow.destination.instance.vm.zone"
	gcpVPCFlowDestInstanceMIGName   = "gcp.vpc.flow.destination.instance.managed_instance_group.name"
	gcpVPCFlowDestInstanceMIGZone   = "gcp.vpc.flow.destination.instance.managed_instance_group.zone"

	// Source location fields
	gcpVPCFlowSourceASN          = "gcp.vpc.flow.source.asn"
	gcpVPCFlowSourceGeoCity      = "gcp.vpc.flow.source.geo.city"
	gcpVPCFlowSourceGeoContinent = "gcp.vpc.flow.source.geo.continent"
	gcpVPCFlowSourceGeoCountry   = "gcp.vpc.flow.source.geo.country.iso_code.alpha3"
	gcpVPCFlowSourceGeoRegion    = "gcp.vpc.flow.source.geo.region"

	// Destination location fields
	gcpVPCFlowDestASN          = "gcp.vpc.flow.destination.asn"
	gcpVPCFlowDestGeoCity      = "gcp.vpc.flow.destination.geo.city"
	gcpVPCFlowDestGeoContinent = "gcp.vpc.flow.destination.geo.continent"
	gcpVPCFlowDestGeoCountry   = "gcp.vpc.flow.destination.geo.country.iso_code.alpha3"
	gcpVPCFlowDestGeoRegion    = "gcp.vpc.flow.destination.geo.region"

	// Source VPC fields
	gcpVPCFlowSourceProjectID    = "gcp.vpc.flow.source.project.id"
	gcpVPCFlowSourceSubnetName   = "gcp.vpc.flow.source.subnet.name"
	gcpVPCFlowSourceSubnetRegion = "gcp.vpc.flow.source.subnet.region"
	gcpVPCFlowSourceVPCName      = "gcp.vpc.flow.source.vpc.name"

	// Destination VPC fields
	gcpVPCFlowDestProjectID    = "gcp.vpc.flow.destination.project.id"
	gcpVPCFlowDestSubnetName   = "gcp.vpc.flow.destination.subnet.name"
	gcpVPCFlowDestSubnetRegion = "gcp.vpc.flow.destination.subnet.region"
	gcpVPCFlowDestVPCName      = "gcp.vpc.flow.destination.vpc.name"

	// Internet routing details
	gcpVPCFlowEgressASPaths = "gcp.vpc.flow.egress.as_paths"
)

type vpcFlowLog struct {
	Connection  *connection `json:"connection"`
	Reporter    string      `json:"reporter"`
	BytesSent   string      `json:"bytes_sent"`
	PacketsSent string      `json:"packets_sent"`
	StartTime   string      `json:"start_time"`
	EndTime     string      `json:"end_time"`
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
		if protocolStr, exists := protocolNames[uint32(*conn.Protocol)]; exists {
			attr.PutStr(string(semconv.NetworkTransportKey), protocolStr)
		}
	}

	shared.PutStr(string(semconv.SourceAddressKey), conn.SrcIP, attr)
	shared.PutStr(string(semconv.DestinationAddressKey), conn.DestIP, attr)

	// Only add port attributes if ports are present
	shared.PutInt(string(semconv.SourcePortKey), conn.SrcPort, attr)
	shared.PutInt(string(semconv.DestinationPortKey), conn.DestPort, attr)
}

func handleNetworkService(ns *networkService, attr pcommon.Map) {
	if ns == nil {
		return
	}

	shared.PutInt(gcpVPCFlowNetworkServiceDSCP, ns.DSCP, attr)
}

func handleInstance(inst *instance, prefix string, attr pcommon.Map) {
	if inst == nil {
		return
	}

	switch prefix {
	case "gcp.vpc.flow.source.instance":
		shared.PutStr(gcpVPCFlowSourceInstanceProjectID, inst.ProjectID, attr)
		shared.PutStr(gcpVPCFlowSourceInstanceVMRegion, inst.Region, attr)
		shared.PutStr(gcpVPCFlowSourceInstanceVMName, inst.VMName, attr)
		shared.PutStr(gcpVPCFlowSourceInstanceVMZone, inst.Zone, attr)

		if inst.ManagedInstanceGroup != nil {
			shared.PutStr(gcpVPCFlowSourceInstanceMIGName, inst.ManagedInstanceGroup.Name, attr)
			shared.PutStr(gcpVPCFlowSourceInstanceMIGZone, inst.ManagedInstanceGroup.Zone, attr)
		}
	case "gcp.vpc.flow.destination.instance":
		shared.PutStr(gcpVPCFlowDestInstanceProjectID, inst.ProjectID, attr)
		shared.PutStr(gcpVPCFlowDestInstanceVMRegion, inst.Region, attr)
		shared.PutStr(gcpVPCFlowDestInstanceVMName, inst.VMName, attr)
		shared.PutStr(gcpVPCFlowDestInstanceVMZone, inst.Zone, attr)

		if inst.ManagedInstanceGroup != nil {
			shared.PutStr(gcpVPCFlowDestInstanceMIGName, inst.ManagedInstanceGroup.Name, attr)
			shared.PutStr(gcpVPCFlowDestInstanceMIGZone, inst.ManagedInstanceGroup.Zone, attr)
		}
	}
}

func handleLocation(loc *location, prefix string, attr pcommon.Map) {
	if loc == nil {
		return
	}

	switch prefix {
	case "gcp.vpc.flow.source":
		shared.PutInt(gcpVPCFlowSourceASN, loc.ASN, attr)
		shared.PutStr(gcpVPCFlowSourceGeoCity, loc.City, attr)
		shared.PutStr(gcpVPCFlowSourceGeoContinent, loc.Continent, attr)
		shared.PutStr(gcpVPCFlowSourceGeoCountry, loc.Country, attr)
		shared.PutStr(gcpVPCFlowSourceGeoRegion, loc.Region, attr)
	case "gcp.vpc.flow.destination":
		shared.PutInt(gcpVPCFlowDestASN, loc.ASN, attr)
		shared.PutStr(gcpVPCFlowDestGeoCity, loc.City, attr)
		shared.PutStr(gcpVPCFlowDestGeoContinent, loc.Continent, attr)
		shared.PutStr(gcpVPCFlowDestGeoCountry, loc.Country, attr)
		shared.PutStr(gcpVPCFlowDestGeoRegion, loc.Region, attr)
	}
}

func handleVPC(vpc *vpc, prefix string, attr pcommon.Map) {
	if vpc == nil {
		return
	}

	switch prefix {
	case "gcp.vpc.flow.source":
		shared.PutStr(gcpVPCFlowSourceProjectID, vpc.ProjectID, attr)
		shared.PutStr(gcpVPCFlowSourceSubnetName, vpc.SubnetworkName, attr)
		shared.PutStr(gcpVPCFlowSourceSubnetRegion, vpc.SubnetworkRegion, attr)
		shared.PutStr(gcpVPCFlowSourceVPCName, vpc.VPCName, attr)
	case "gcp.vpc.flow.destination":
		shared.PutStr(gcpVPCFlowDestProjectID, vpc.ProjectID, attr)
		shared.PutStr(gcpVPCFlowDestSubnetName, vpc.SubnetworkName, attr)
		shared.PutStr(gcpVPCFlowDestSubnetRegion, vpc.SubnetworkRegion, attr)
		shared.PutStr(gcpVPCFlowDestVPCName, vpc.VPCName, attr)
	}
}

func handleInternetRoutingDetails(ird *internetRoutingDetails, attr pcommon.Map) {
	if ird == nil || len(ird.EgressASPath) == 0 {
		return
	}

	asPaths := attr.PutEmptySlice(gcpVPCFlowEgressASPaths)
	for _, path := range ird.EgressASPath {
		pathMap := asPaths.AppendEmpty().SetEmptyMap()
		asDetails := pathMap.PutEmptySlice("as_details")
		for _, detail := range path.ASDetails {
			detailMap := asDetails.AppendEmpty().SetEmptyMap()
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

	shared.PutStr(gcpVPCFlowStartTime, log.StartTime, attr)
	shared.PutStr(gcpVPCFlowEndTime, log.EndTime, attr)

	// Handle RTT
	if err := shared.AddStrAsInt(gcpVPCFlowNetworkRTTMsec, log.RTTMsec, attr); err != nil {
		return fmt.Errorf("failed to add RTT: %w", err)
	}

	// Handle network service
	handleNetworkService(log.NetworkService, attr)

	// Handle instance details
	handleInstance(log.SrcInstance, "gcp.vpc.flow.source.instance", attr)
	handleInstance(log.DestInstance, "gcp.vpc.flow.destination.instance", attr)

	// Handle location details
	handleLocation(log.SrcLocation, "gcp.vpc.flow.source", attr)
	handleLocation(log.DestLocation, "gcp.vpc.flow.destination", attr)

	// Handle VPC details
	handleVPC(log.SrcVPC, "gcp.vpc.flow.source", attr)
	handleVPC(log.DestVPC, "gcp.vpc.flow.destination", attr)

	// Handle internet routing details
	handleInternetRoutingDetails(log.InternetRoutingDetails, attr)

	return nil
}
