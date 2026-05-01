module github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/xray/testdata/sampleserver

go 1.25.0

require github.com/aws/aws-xray-sdk-go/v2 v2.0.1

require (
	github.com/andybalholm/brotli v1.1.0 // indirect
	github.com/klauspost/compress v1.17.6 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/stretchr/testify v1.11.1 // indirect
	github.com/valyala/bytebufferpool v1.0.0 // indirect
	github.com/valyala/fasthttp v1.52.0 // indirect
	golang.org/x/net v0.51.0 // indirect
	golang.org/x/sys v0.41.0 // indirect
	golang.org/x/text v0.34.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20251202230838-ff82c1b0f217 // indirect
	google.golang.org/grpc v1.79.3 // indirect
	google.golang.org/protobuf v1.36.10 // indirect
)

retract (
	v0.76.2
	v0.76.1
	v0.65.0
)
