module github.com/open-telemetry/opentelemetry-collector-contrib/extension/opampcustommessages

go 1.22.0

require github.com/open-telemetry/opamp-go v0.17.0

require google.golang.org/protobuf v1.34.2 // indirect

replace go.opentelemetry.io/collector/scraper/scraperhelper v0.116.0 => go.opentelemetry.io/collector/scraper/scraperhelper v0.0.0-20250106214556-67fdcd1f4267

replace go.opentelemetry.io/collector/extension/xextension v0.116.0 => go.opentelemetry.io/collector/extension/xextension v0.0.0-20250106214556-67fdcd1f4267
