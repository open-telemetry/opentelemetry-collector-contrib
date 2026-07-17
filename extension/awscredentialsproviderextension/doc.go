// Licensed to Elasticsearch B.V. under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Elasticsearch B.V. licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

//go:generate mdatagen metadata.yaml

// Package awscredentialsproviderextension resolves AWS credentials and exposes them to other
// components through the Provider interface. AWS-SDK-based components (e.g. the
// awscloudwatch receiver) reference the extension by ID and use the resolved
// aws.CredentialsProvider for their SDK clients.
package awscredentialsproviderextension // import "github.com/elastic/opentelemetry-collector-components/extension/awscredentialsproviderextension"
