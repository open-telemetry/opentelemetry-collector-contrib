This folder contains raw X-Ray segment documents.

# sampleapp folder
The `ddbSample.txt` was generated from the sample application instrumented with the Go X-Ray SDK while the rest were synthesized manually. The sample app assumes:
1. there is a DynamoDB table with the name "xray_sample_table" in the us-west-2 region
2. there is no DynamoDB table with the name "does_not_exist" in the us-west-2 region

The segments can be captured using `tcpdump` via the following command:
```
$ sudo tcpdump -i any -A -c100 -v -nn udp port 2000 > xray_instrumented_client.txt
```
You can safely interrupt the process after getting enough samples.

You can run the sample app by:
```
go run sampleapp/sample.go
```
Provide AWS credentials via environment variables and run the sample app.

# sampleserver folder
The `serverSample.txt` was generated from the sample server with handler instrumented with Go X-Ray SDK.

Again, the segments can be captured via the `tcpdump` command:
```
$ sudo tcpdump -i any -A -c100 -v -nn udp port 2000 > xray_instrumented_server.txt
```

You can run the sample server by:
```
go run sampleserver/sample.go
```