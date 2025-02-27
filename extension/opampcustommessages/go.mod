module github.com/open-telemetry/opentelemetry-collector-contrib/extension/opampcustommessages

go 1.23.0

require github.com/open-telemetry/opamp-go v0.18.0

require google.golang.org/protobuf v1.36.5 // indirect

replace go.opentelemetry.io/collector/extension/extensionauth => go.opentelemetry.io/collector/extension/extensionauth v0.0.0-20250227125643-49a6e963bf87

replace go.opentelemetry.io/collector/extension/extensionauth/extensionauthtest => go.opentelemetry.io/collector/extension/extensionauth/extensionauthtest v0.0.0-20250227125643-49a6e963bf87

replace go.opentelemetry.io/collector/service/hostcapabilities => go.opentelemetry.io/collector/service/hostcapabilities v0.0.0-20250227134758-6a98ee8b14a2
