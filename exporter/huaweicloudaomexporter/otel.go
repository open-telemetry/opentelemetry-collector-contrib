package huaweicloudaomexporter

import (
	"encoding/json"
	"os"
)

const (
	traceIDField       = "traceID"
	spanIDField        = "spanID"
	parentSpanIDField  = "parentSpanID"
	nameField          = "name"
	kindField          = "kind"
	linksField         = "links"
	timeField          = "time"
	startTimeField     = "start"
	endTimeField       = "end"
	traceStateField    = "traceState"
	durationField      = "duration"
	attributeField     = "attribute"
	statusCodeField    = "statusCode"
	statusMessageField = "statusMessage"
	logsField          = "logs"
)

type logKeyValuePair struct {
	Key   string
	Value string
}

type logKeyValuePairs []logKeyValuePair

func (kv logKeyValuePairs) Len() int           { return len(kv) }
func (kv logKeyValuePairs) Swap(i, j int)      { kv[i], kv[j] = kv[j], kv[i] }
func (kv logKeyValuePairs) Less(i, j int) bool { return kv[i].Key < kv[j].Key }

func loadFromJSON(file string, obj any) error {
	blob, err := os.ReadFile(file)
	if err == nil {
		err = json.Unmarshal(blob, obj)
	}
	return err
}
