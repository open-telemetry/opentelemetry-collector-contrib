module github.com/open-telemetry/opentelemetry-collector-contrib/extension/opampcustommessages

go 1.24.0

require github.com/open-telemetry/opamp-go v0.22.0

require google.golang.org/protobuf v1.36.10 // indirect

// Can be removed after 0.144.0 release
replace go.opentelemetry.io/collector/internal/componentalias => go.opentelemetry.io/collector/internal/componentalias v0.0.0-20260115162016-5e41fb551263
