package k8sobjectreceiver

import (
	"go.opentelemetry.io/collector/pdata/plog"
	semconv "go.opentelemetry.io/collector/semconv/v1.9.0"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/watch"
)

func watchEventToLogData(event watch.Event) plog.Logs {
	udata := event.Object.(*unstructured.Unstructured)
	out := plog.NewLogs()
	rl := out.ResourceLogs().AppendEmpty()
	sl := rl.ScopeLogs().AppendEmpty()
	lr := sl.LogRecords().AppendEmpty()
	dest := lr.Body()

	attrs := lr.Attributes()
	attrs.EnsureCapacity(3)

	attrs.PutString("event.domain", "k8s")
	attrs.PutString("event.name", udata.GetKind())

	if namespace := udata.GetNamespace(); namespace != "" {
		attrs.PutString(semconv.AttributeK8SNamespaceName, namespace)
	}

	destMap := dest.SetEmptyMap()
	obj := map[string]interface{}{
		"type":   string(event.Type),
		"object": udata.Object,
	}
	destMap.FromRaw(obj)
	return out
}

func unstructuredListToLogData(event *unstructured.UnstructuredList) plog.Logs {
	out := plog.NewLogs()
	rl := out.ResourceLogs().AppendEmpty()
	sl := rl.ScopeLogs().AppendEmpty()

	logSlice := sl.LogRecords()
	logSlice.EnsureCapacity(len(event.Items))
	for _, e := range event.Items {
		record := logSlice.AppendEmpty()
		attrs := record.Attributes()
		attrs.EnsureCapacity(3)

		attrs.PutString("event.domain", "k8s")
		attrs.PutString("event.name", e.GetKind())
		if namespace := e.GetNamespace(); namespace != "" {
			attrs.PutString(semconv.AttributeK8SNamespaceName, namespace)
		}
		dest := record.Body()
		destMap := dest.SetEmptyMap()
		destMap.FromRaw(e.Object)
	}
	return out
}
