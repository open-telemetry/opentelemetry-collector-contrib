# model

The model directory contains the protobuf models used for payload unmarshalling in the Solace receiver. Go models can be generated based on the protobuf using `protoc`. To generate the protobuf model, the [protoc-gen-go](https://developers.google.com/protocol-buffers/docs/reference/go-generated) package must be installed. To format the code correctly, we call [goimports](https://pkg.go.dev/golang.org/x/tools/cmd/goimports).

To generate the V1 model from the model directory:
```
protoc --go_out=../ --go_opt=paths=import --go_opt=Mreceive_v1.proto=model/v1 receive_v1.proto
goimports -w v1/
```
