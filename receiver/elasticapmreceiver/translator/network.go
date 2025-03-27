package translator

import (
	"github.com/elastic/apm-data/model/modelpb"
	"go.opentelemetry.io/collector/pdata/pcommon"
	conventions "go.opentelemetry.io/collector/semconv/v1.18.0"
)

func parseNetwork(network *modelpb.Network, attrs pcommon.Map) {
	if network == nil {
		return
	}
	parseNetworkConnection(network.Connection, attrs)
	parseNetworkCarrier(network.Carrier, attrs)
}

func parseNetworkConnection(connection *modelpb.NetworkConnection, attrs pcommon.Map) {
	if connection == nil {
		return
	}
	PutOptionalStr(attrs, conventions.AttributeNetHostConnectionSubtype, &connection.Subtype)
	PutOptionalStr(attrs, conventions.AttributeNetHostConnectionType, &connection.Type)
}

func parseNetworkCarrier(carrier *modelpb.NetworkCarrier, attrs pcommon.Map) {
	if carrier == nil {
		return
	}
	PutOptionalStr(attrs, conventions.AttributeNetHostCarrierName, &carrier.Name)
	PutOptionalStr(attrs, conventions.AttributeNetHostCarrierMcc, &carrier.Mcc)
	PutOptionalStr(attrs, conventions.AttributeNetHostCarrierMnc, &carrier.Mnc)
	PutOptionalStr(attrs, conventions.AttributeNetHostCarrierIcc, &carrier.Icc)
}
