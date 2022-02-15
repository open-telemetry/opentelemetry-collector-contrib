// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package awsfirehosereceiver implements a receiver that can be used to
// receive requests from the AWS Kinesis Data Firehose and transform them
// into formats usable by the Opentelemetry collector. The configuration
// determines which unmarshaler to use. Each unmarshaler is responsible for
// processing a Firehose record format that can be sent through the delivery
// stream.
//
// More details can be found at:
// https://docs.aws.amazon.com/firehose/latest/dev/httpdeliveryrequestresponse.html
package awsfirehosereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsfirehosereceiver"
