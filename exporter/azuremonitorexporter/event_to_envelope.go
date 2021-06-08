package azuremonitorexporter

// Contains code common to both trace and metrics exporters
import (
	"github.com/microsoft/ApplicationInsights-Go/appinsights/contracts"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.uber.org/zap"
)

// Wraps a telemetry item in an envelope with the information found in this  context.
func  eventToEnvelope(span pdata.Span,item  pdata.SpanEvent, logger *zap.Logger) *contracts.Envelope {

	eventData := NewEventData(item)
	data := contracts.NewData()
	data.BaseType = eventData.BaseType()
	data.BaseData = eventData

	envelope := contracts.NewEnvelope()
	envelope.Name = eventData.EnvelopeName(item.Name())//context.nameIKey)
	envelope.Data = data

	timestamp := item.Timestamp()
	
	envelope.Time = timestamp.AsTime().UTC().Format("2006-01-02T15:04:05.999999Z")
	envelope.Tags = make(contracts.ContextTags)
	envelope.Tags[contracts.OperationId] = span.TraceID().HexString()
	envelope.Tags[contracts.OperationParentId] = span.SpanID().HexString()

	// copy attributes to envelope
    attrMap := item.Attributes()
		attrMap.Range(func(k string, v pdata.AttributeValue) bool {
			eventData.Properties[k] =  v.StringVal()
			return true
		})

	// Sanitize.
	for _, warningMsg := range eventData.Sanitize() {
		logger.Warn("Telemetry data warning: "+ warningMsg)
	}
	for _, warningMsg := range contracts.SanitizeTags(envelope.Tags) {
		logger.Warn("Telemetry tag warning: "+ warningMsg)
	}

	return envelope
}

func NewEventData(event pdata.SpanEvent) *contracts.EventData {
	data := contracts.NewEventData()
	data.Name = event.Name() 
	data.Properties = make(map[string]string) 
	data.Measurements = make(map[string]float64)

	return data
}