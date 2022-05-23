// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package sigv4authextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/sigv4authextension"

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/aws-sdk-go-v2/service/sts/types"
)

type mockAssumeRole struct {
	TestInput func(*sts.AssumeRoleInput, []func(*sts.Options))
}

func (s *mockAssumeRole) AssumeRole(ctx context.Context, params *sts.AssumeRoleInput, optFns ...func(*sts.Options)) (*sts.AssumeRoleOutput, error) {
	if s.TestInput != nil {
		s.TestInput(params, optFns)
	}
	expiry := time.Now().Add(60 * time.Minute)

	return &sts.AssumeRoleOutput{
		Credentials: &types.Credentials{
			// Just reflect the role arn to the provider.
			AccessKeyId:     params.RoleArn,
			SecretAccessKey: aws.String("assumedSecretAccessKey"),
			SessionToken:    aws.String("assumedSessionToken"),
			Expiration:      &expiry,
		},
	}, nil
}

var roleARNs = []string{"00000000000000000000000000000000000", "00000000000000000000000000000000001"}

func TestAssumeChainedRoleProvider(t *testing.T) {
	callIndex := 0
	stub := &mockAssumeRole{
		TestInput: func(in *sts.AssumeRoleInput, optFns []func(*sts.Options)) {
			if callIndex == 0 {
				if *in.RoleArn != roleARNs[0] {
					t.Errorf("Expect 0th role ARN to be %s", roleARNs[0])
				}
				opts := &sts.Options{}
				for _, optFn := range optFns {
					optFn(opts)
				}
				if opts.Credentials != nil {
					t.Error("Expect 0th call credentials to be unmodified")
				}
			} else if callIndex == 1 {
				if *in.RoleArn != roleARNs[1] {
					t.Errorf("Expect 1st role ARN to be %s", roleARNs[1])
				}
				opts := &sts.Options{}
				for _, optFn := range optFns {
					optFn(opts)
				}
				if opts.Credentials == nil {
					t.Error("Expect 1st call credentials to be modified")
				}
				creds, err := opts.Credentials.Retrieve(context.TODO())
				if err != nil {
					t.Fatalf("Expect no error, %v", err)
				}
				if creds.AccessKeyID != roleARNs[0] {
					t.Error("Expect 1st call source credentials to match")
				}
			}
			callIndex++
		},
	}
	p := NewAssumeChainedRoleProvider(stub, roleARNs)

	creds, err := p.Retrieve(context.Background())
	if err != nil {
		t.Fatalf("Expect no error, %v", err)
	}
	if callIndex != 2 {
		t.Error("Expect exactly two calls")
	}

	if e, a := roleARNs[1], creds.AccessKeyID; e != a {
		t.Errorf("Expect access key ID to be final role ARN")
	}
	if e, a := "assumedSecretAccessKey", creds.SecretAccessKey; e != a {
		t.Errorf("Expect secret access key to match")
	}
	if e, a := "assumedSessionToken", creds.SessionToken; e != a {
		t.Errorf("Expect session token to match")
	}
}
