module github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/xray/testdata/sampleserver

go 1.22.0

require github.com/aws/aws-xray-sdk-go v1.8.5

require (
	github.com/andybalholm/brotli v1.1.0 // indirect
	github.com/aws/aws-sdk-go v1.47.10 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/klauspost/compress v1.17.6 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/valyala/bytebufferpool v1.0.0 // indirect
	github.com/valyala/fasthttp v1.52.0 // indirect
	golang.org/x/net v0.26.0 // indirect
	golang.org/x/sys v0.21.0 // indirect
	golang.org/x/text v0.16.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240318140521-94a12d6c2237 // indirect
	google.golang.org/grpc v1.64.1 // indirect
	google.golang.org/protobuf v1.33.0 // indirect
)

retract (
	v0.76.2
	v0.76.1
	v0.65.0
)
