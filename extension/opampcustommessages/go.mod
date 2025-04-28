module github.com/open-telemetry/opentelemetry-collector-contrib/extension/opampcustommessages

go 1.23.0

require github.com/open-telemetry/opamp-go v0.19.0

require google.golang.org/protobuf v1.36.5 // indirect

replace go.opentelemetry.io/collector/extension/extensionmiddleware/extensionmiddlewaretest v0.0.0-00010101000000-000000000000 => go.opentelemetry.io/collector/extension/extensionmiddleware/extensionmiddlewaretest v0.0.0-20250424234037-ac7c0f2f4cd8

replace go.opentelemetry.io/collector/config/configmiddleware v0.0.0-00010101000000-000000000000 => go.opentelemetry.io/collector/config/configmiddleware v0.0.0-20250424234037-ac7c0f2f4cd8

replace go.opentelemetry.io/collector/extension/extensionmiddleware v1.30.0 => go.opentelemetry.io/collector/extension/extensionmiddleware v0.0.0-20250424234037-ac7c0f2f4cd8
