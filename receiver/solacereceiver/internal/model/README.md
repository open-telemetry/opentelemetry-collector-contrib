# model

The model directory contains the protobuf models used for payload unmarshalling in the Solace receiver. Go models can be generated based on the protobuf using `protoc`. To generate the protobuf model, the [protoc-gen-go](https://developers.google.com/protocol-buffers/docs/reference/go-generated) package must be installed. To format the code correctly, we call [gci](https://github.com/daixiang0/gci).

To generate the V1 model from the model directory:
```
protoc --go_out=../ --go_opt=paths=import --go_opt=Mreceive_v1.proto=model/receive/v1 receive_v1.proto
protoc --go_out=../ --go_opt=paths=import --go_opt=Megress_v1.proto=model/egress/v1 egress_v1.proto
protoc --go_out=../ --go_opt=paths=import --go_opt=Mmove_v1.proto=model/move/v1 move_v1.proto
gci write -s standard -s default -s "prefix(github.com/open-telemetry/opentelemetry-collector-contrib)" .
```
