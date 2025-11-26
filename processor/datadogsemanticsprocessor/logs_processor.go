package datadogsemanticsprocessor

import (
	"context"
	"fmt"
	"github.com/DataDog/datadog-agent/pkg/opentelemetry-mapping-go/otlp/attributes/source"
	"github.com/DataDog/datadog-agent/pkg/trace/traceutil"
	"github.com/DataDog/datadog-agent/pkg/trace/transform"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/datadogsemanticsprocessor/internal/attributes"
	"go.opentelemetry.io/collector/pdata/plog"
	semconv "go.opentelemetry.io/otel/semconv/v1.32.0"
)

func (lp *logsProcessor) processLogs(ctx context.Context, logs plog.Logs) (output plog.Logs, err error) {
	resourceLogs := logs.ResourceLogs()
	for i := 0; i < resourceLogs.Len(); i++ {
		resourceLog := resourceLogs.At(i)
		otelResource := resourceLog.Resource()
		resourceAttrs := otelResource.Attributes()
		for j := 0; j < resourceLog.ScopeLogs().Len(); j++ {
			serviceVersion := ""
			if serviceVersionAttr, ok := resourceAttrs.Get(string(semconv.ServiceVersionKey)); ok {
				serviceVersion = serviceVersionAttr.AsString()
			}
			err = attributes.InsertAttrIfMissingOrShouldOverride(lp.overrideIncomingDatadogFields, resourceAttrs, "datadog.version", serviceVersion)
			if err != nil {
				return plog.Logs{}, err
			}

			if lp.overrideIncomingDatadogFields {
				resourceAttrs.Remove("datadog.host.name")
			}

			datadogHostName := ""
			if src, ok := lp.attrsTranslator.ResourceToSource(ctx, otelResource, attributes.SignalTypeLogs, nil); ok && src.Kind == source.HostnameKind {
				datadogHostName = src.Identifier
			}

			err = attributes.InsertAttrIfMissingOrShouldOverride(lp.overrideIncomingDatadogFields, resourceAttrs, "datadog.host.name", datadogHostName)
			if err != nil {
				return plog.Logs{}, err
			}

			// TODO: maybe this stuff?
			scopeLog := resourceLog.ScopeLogs().At(j)
			for k := 0; k < scopeLog.LogRecords().Len(); k++ {
				otelLog := scopeLog.LogRecords().At(k)
				attrs := otelLog.Attributes()

				// Note: default value from GetOTelService is "otlpresourcenoservicename"
				err = attributes.InsertAttrIfMissingOrShouldOverride(lp.overrideIncomingDatadogFields, resourceAttrs, "datadog.service", attrs.)
				if err != nil {
					return plog.Logs{}, err
				}

				env := "default"
				if envFromAttr := transform.GetOTelEnv(otelspan, otelres, false); envFromAttr != "" {
					env = envFromAttr
				}
				err = attributes.InsertAttrIfMissingOrShouldOverride(lp.overrideIncomingDatadogFields, rattr, "datadog.env", env)
				if err != nil {
					return plog.Logs{}, err
				}

				err = attributes.InsertAttrIfMissingOrShouldOverride(lp.overrideIncomingDatadogFields, sattr, "datadog.name", traceutil.GetOTelOperationNameV2(otelspan, otelres))
				if err != nil {
					return plog.Logs{}, err
				}
				err = attributes.InsertAttrIfMissingOrShouldOverride(lp.overrideIncomingDatadogFields, sattr, "datadog.resource", traceutil.GetOTelResourceV2(otelspan, otelres))
				if err != nil {
					return plog.Logs{}, err
				}
				err = attributes.InsertAttrIfMissingOrShouldOverride(lp.overrideIncomingDatadogFields, sattr, "datadog.type", traceutil.GetOTelSpanType(otelspan, otelres))
				if err != nil {
					return plog.Logs{}, err
				}
				spanKind := otelspan.Kind()
				err = attributes.InsertAttrIfMissingOrShouldOverride(lp.overrideIncomingDatadogFields, sattr, "datadog.span.kind", traceutil.OTelSpanKindName(spanKind))
				if err != nil {
					return plog.Logs{}, err
				}

				// Map VCS (version control system) attributes for source code integration
				if vcsRevision, ok := sattr.Get(string(semconv.VCSRefHeadRevisionKey)); ok {
					err = attributes.InsertAttrIfMissingOrShouldOverride(lp.overrideIncomingDatadogFields, sattr, "git.commit.sha", vcsRevision.AsString())
					if err != nil {
						return plog.Logs{}, err
					}
				}
				if vcsRepoURL, ok := sattr.Get(string(semconv.VCSRepositoryURLFullKey)); ok {
					// Strip protocol from repository URL as required by Datadog
					cleanURL := stripProtocolFromURL(vcsRepoURL.AsString())
					err = attributes.InsertAttrIfMissingOrShouldOverride(lp.overrideIncomingDatadogFields, sattr, "git.repository_url", cleanURL)
					if err != nil {
						return plog.Logs{}, err
					}
				}

				metaMap := make(map[string]string)
				code := transform.GetOTelStatusCode(otelspan, otelres, false)
				if code != 0 {
					err = attributes.InsertAttrIfMissingOrShouldOverride(lp.overrideIncomingDatadogFields, sattr, "datadog.http_status_code", fmt.Sprintf("%d", code))
					if err != nil {
						return plog.Logs{}, err
					}
				}
				ddError := int64(status2Error(otelspan.Status(), otelspan.Events(), metaMap))
				err = attributes.InsertAttrIfMissingOrShouldOverride(lp.overrideIncomingDatadogFields, sattr, "datadog.error", ddError)
				if err != nil {
					return plog.Logs{}, err
				}
				if ddError == 1 {
					err = attributes.InsertAttrIfMissingOrShouldOverride(lp.overrideIncomingDatadogFields, sattr, "datadog.error.msg", metaMap["error.msg"])
					if err != nil {
						return plog.Logs{}, err
					}
					err = attributes.InsertAttrIfMissingOrShouldOverride(lp.overrideIncomingDatadogFields, sattr, "datadog.error.type", metaMap["error.type"])
					if err != nil {
						return plog.Logs{}, err
					}
					err = attributes.InsertAttrIfMissingOrShouldOverride(lp.overrideIncomingDatadogFields, sattr, "datadog.error.stack", metaMap["error.stack"])
					if err != nil {
						return plog.Logs{}, err
					}
				}
			}
		}
	}
	return logs, nil
}
