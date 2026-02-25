include ../../Makefile.Common

local-run-example:
	cd ../../cmd/otelcontribcol && GOOS=linux go build -o ../../local/otelcontribcol
	cd example && docker compose up -d
recreate-otel-collector:
	cd ../../ && make otelcontribcol
	cd example && docker compose up --build otelcollector
