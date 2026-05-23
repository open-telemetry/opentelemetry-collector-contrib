// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsutil

import (
	"context"
	"net/http"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	ststypes "github.com/aws/aws-sdk-go-v2/service/sts/types"
	smithymiddleware "github.com/aws/smithy-go/middleware"
	smithyhttp "github.com/aws/smithy-go/transport/http"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	testAccountID   = "012345678912"
	testInstanceARN = "arn:aws:ec2:us-east-1:012345678912:instance/i-0123a456700123456"
	testRegion      = "us-east-1"
	testRoleARN     = "arn:aws:iam::012345678912:role/XXXXXXXX"
)

var (
	testCredentials = aws.Credentials{
		AccessKeyID:     "AKIAIOSFODNN7EXAMPLE",
		SecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
		SessionToken:    "SessionToken",
	}
	testAWSConfig = aws.Config{
		Region: testRegion,
		Credentials: credentials.NewStaticCredentialsProvider(
			testCredentials.AccessKeyID,
			testCredentials.SecretAccessKey,
			testCredentials.SessionToken,
		),
	}
)

type mockCredentialsProvider struct {
	mock.Mock
}

var _ aws.CredentialsProvider = (*mockCredentialsProvider)(nil)

func (m *mockCredentialsProvider) Retrieve(ctx context.Context) (aws.Credentials, error) {
	args := m.Called(ctx)
	return args.Get(0).(aws.Credentials), args.Error(1)
}

func TestStsCredentialsProvider_Retrieve(t *testing.T) {
	t.Run("Regional/Success", func(t *testing.T) {
		regional := new(mockCredentialsProvider)
		regional.On("Retrieve", mock.Anything).Return(testCredentials, nil).Once()
		partitional := new(mockCredentialsProvider)

		provider := &stsCredentialsProvider{
			regional:    regional,
			partitional: partitional,
		}

		got, err := provider.Retrieve(context.Background())
		require.NoError(t, err)
		assert.Equal(t, testCredentials, got)
		regional.AssertExpectations(t)
		partitional.AssertNotCalled(t, "Retrieve", mock.Anything)
	})

	t.Run("Regional/OtherErrorPropagates", func(t *testing.T) {
		// Errors that aren't RegionDisabledException must not trigger fallback.
		regional := new(mockCredentialsProvider)
		regional.On("Retrieve", mock.Anything).Return(aws.Credentials{}, assert.AnError).Once()
		partitional := new(mockCredentialsProvider)

		provider := &stsCredentialsProvider{
			regional:    regional,
			partitional: partitional,
		}

		_, err := provider.Retrieve(context.Background())
		assert.ErrorIs(t, err, assert.AnError)
		regional.AssertExpectations(t)
		partitional.AssertNotCalled(t, "Retrieve", mock.Anything)
	})

	t.Run("Fallback/RegionDisabledExceptionLatchesPartitional", func(t *testing.T) {
		regional := new(mockCredentialsProvider)
		regional.On("Retrieve", mock.Anything).Return(aws.Credentials{}, &ststypes.RegionDisabledException{}).Once()
		partitional := new(mockCredentialsProvider)
		partitional.On("Retrieve", mock.Anything).Return(testCredentials, nil)

		provider := &stsCredentialsProvider{
			regional:    regional,
			partitional: partitional,
		}
		assert.Nil(t, provider.fallback)

		got, err := provider.Retrieve(context.Background())
		require.NoError(t, err)
		assert.Equal(t, testCredentials, got)
		assert.NotNil(t, provider.fallback)

		// Second call goes directly through fallback; regional must not be
		// consulted (Once() above would fail if it were).
		got, err = provider.Retrieve(context.Background())
		require.NoError(t, err)
		assert.Equal(t, testCredentials, got)

		regional.AssertExpectations(t)
		partitional.AssertExpectations(t)
	})

	t.Run("Regional/RegionDisabledExceptionWithoutPartitional", func(t *testing.T) {
		// When partitional is nil, RegionDisabledException must propagate
		// rather than triggering an arbitrary fallback.
		regional := new(mockCredentialsProvider)
		regional.On("Retrieve", mock.Anything).Return(aws.Credentials{}, &ststypes.RegionDisabledException{}).Once()

		provider := &stsCredentialsProvider{regional: regional}

		_, err := provider.Retrieve(context.Background())
		var rde *ststypes.RegionDisabledException
		assert.ErrorAs(t, err, &rde)
		assert.Nil(t, provider.fallback)
	})
}

func TestNewStsCredentialsProvider_KnownPartition(t *testing.T) {
	provider := newStsCredentialsProvider(testAWSConfig, testRoleARN, testRegion, "")
	stsProvider, ok := provider.(*stsCredentialsProvider)
	require.True(t, ok)

	require.NotNil(t, stsProvider.regional)
	assert.NotNil(t, stsProvider.partitional)

	_, ok = stsProvider.regional.(*stscreds.AssumeRoleProvider)
	assert.True(t, ok)
}

func TestConfusedDeputyHeaders(t *testing.T) {
	testCases := map[string]struct {
		envSourceArn          string
		envSourceAccount      string
		expectedHeaderArn     string
		expectedHeaderAccount string
	}{
		"unpopulated": {
			envSourceArn:          "",
			envSourceAccount:      "",
			expectedHeaderArn:     "",
			expectedHeaderAccount: "",
		},
		"both populated": {
			envSourceArn:          testInstanceARN,
			envSourceAccount:      testAccountID,
			expectedHeaderArn:     testInstanceARN,
			expectedHeaderAccount: testAccountID,
		},
		"only source arn populated": {
			envSourceArn:          testInstanceARN,
			envSourceAccount:      "",
			expectedHeaderArn:     "",
			expectedHeaderAccount: "",
		},
		"only source account populated": {
			envSourceArn:          "",
			envSourceAccount:      testAccountID,
			expectedHeaderArn:     "",
			expectedHeaderAccount: "",
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Setenv(AmzSourceAccount, tc.envSourceAccount)
			t.Setenv(AmzSourceArn, tc.envSourceArn)

			client := newStsClient(testAWSConfig)

			input := &sts.AssumeRoleInput{
				RoleArn:         aws.String(testRoleARN),
				RoleSessionName: aws.String("MockSession"),
			}

			// Capture headers via a Finalize/After short-circuit middleware
			// that returns a fake AssumeRoleOutput before the request is
			// sent. By Finalize/After, signing has run, so the captured
			// request is the fully-signed wire form.
			var capturedHeaders http.Header
			_, err := client.AssumeRole(context.Background(), input, func(o *sts.Options) {
				o.APIOptions = append(o.APIOptions, func(s *smithymiddleware.Stack) error {
					return s.Finalize.Add(smithymiddleware.FinalizeMiddlewareFunc("CaptureHeaders",
						func(_ context.Context, in smithymiddleware.FinalizeInput, _ smithymiddleware.FinalizeHandler) (smithymiddleware.FinalizeOutput, smithymiddleware.Metadata, error) {
							if req, ok := in.Request.(*smithyhttp.Request); ok {
								capturedHeaders = req.Header.Clone()
							}
							return smithymiddleware.FinalizeOutput{Result: &sts.AssumeRoleOutput{}}, smithymiddleware.Metadata{}, nil
						}), smithymiddleware.After)
				})
			})
			require.NoError(t, err)

			assert.Equal(t, tc.expectedHeaderArn, capturedHeaders.Get(SourceArnHeaderKey))
			assert.Equal(t, tc.expectedHeaderAccount, capturedHeaders.Get(SourceAccountHeaderKey))
		})
	}
}
