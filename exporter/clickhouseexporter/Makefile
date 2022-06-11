include ../../Makefile.Common

local-run-example:
	cd ../../ && GOOS=linux go build -o ./local/otelcontribcol ./cmd/otelcontribcol
	cd example && docker-compose up -d
recreate-otel-collector:
	cd ../../ && GOOS=linux go build -o ./local/otelcontribcol ./cmd/otelcontribcol
	cd example && docker-compose up --build otel-collector
