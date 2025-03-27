package translator

import (
	"github.com/elastic/apm-data/model/modelpb"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func parseMessage(message *modelpb.Message, attrs pcommon.Map) {
	if message == nil {
		return
	}

	PutOptionalStr(attrs, "message.body", &message.Body)
	PutOptionalInt(attrs, "message.age_millis", message.AgeMillis)
	PutOptionalStr(attrs, "message.queue_name", &message.QueueName)
	PutOptionalStr(attrs, "message.routing_key", &message.RoutingKey)

	for _, header := range message.Headers {
		arr := attrs.PutEmptyMap("message.headers")
		for _, item := range header.Value {
			arr.PutStr(header.Key, item)
		}
	}
}
