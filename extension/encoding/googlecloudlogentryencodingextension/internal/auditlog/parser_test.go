// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package auditlog

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func TestIsValid(t *testing.T) {
	tests := map[string]struct {
		log        auditLog
		expectsErr string
	}{
		"invalid @type": {
			log:        auditLog{Type: "invalid"},
			expectsErr: "expected @type to be",
		},
		"empty service name": {
			log:        auditLog{Type: auditLogType},
			expectsErr: "missing service name",
		},
		"empty method name": {
			log: auditLog{
				Type:        auditLogType,
				ServiceName: "test",
			},
			expectsErr: "missing method name",
		},
		"valid": {
			log: auditLog{
				Type:        auditLogType,
				ServiceName: "test",
				MethodName:  "test",
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := isValid(tt.log)
			if tt.expectsErr != "" {
				require.ErrorContains(t, err, tt.expectsErr)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestHandleResourceLocation(t *testing.T) {
	tests := map[string]struct {
		resourceLoc  *resourceLocation
		expectedAttr map[string]any
	}{
		"nil": {
			resourceLoc:  nil,
			expectedAttr: map[string]any{},
		},
		"current locations": {
			resourceLoc: &resourceLocation{
				CurrentLocations: []string{"europe-west1-a", "us-east1"},
			},
			expectedAttr: map[string]any{
				gcpAuditResourceLocationCurrent: []any{"europe-west1-a", "us-east1"},
			},
		},
		"original locations": {
			resourceLoc: &resourceLocation{
				OriginalLocations: []string{"europe-west1-a", "us-east1"},
			},
			expectedAttr: map[string]any{
				gcpAuditResourceLocationOriginal: []any{"europe-west1-a", "us-east1"},
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			attr := pcommon.NewMap()
			handleResourceLocation(tt.resourceLoc, attr)
			require.Equal(t, tt.expectedAttr, attr.AsRaw())
		})
	}
}

func TestHandleStatus(t *testing.T) {
	tests := map[string]struct {
		status       *status
		expectedAttr map[string]any
	}{
		"nil": {
			status:       nil,
			expectedAttr: map[string]any{},
		},
		"not nil": {
			status: &status{
				Code: func() *int64 {
					v := int64(6)
					return &v
				}(),
				Message: "RESOURCE_ALREADY_EXISTS",
			},
			expectedAttr: map[string]any{
				"rpc.jsonrpc.error_code":    int64(6),
				"rpc.jsonrpc.error_message": "RESOURCE_ALREADY_EXISTS",
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			attr := pcommon.NewMap()
			handleStatus(tt.status, attr)
			require.Equal(t, tt.expectedAttr, attr.AsRaw())
		})
	}
}

func TestHandleAuthenticationInfo(t *testing.T) {
	tests := map[string]struct {
		auth         *authenticationInfo
		expectedAttr map[string]any
	}{
		"nil": {
			auth:         nil,
			expectedAttr: map[string]any{},
		},
		"not nil": {
			auth: &authenticationInfo{
				PrincipalEmail:        "test@opentelemetry.iam.gserviceaccount.com",
				PrincipalSubject:      "serviceAccount:test@opentelemetry.iam.gserviceaccount.com",
				ServiceAccountKeyName: "//iam.googleapis.com/projects/test/serviceAccounts/test@opentelemetry.iam.gserviceaccount.com/keys/7ebb40c681d9b7d29fd1942bfc2bd65bf5c56b06",
				AuthoritySelector:     "https://www.googleapis.com/auth/cloud-platform",
			},
			expectedAttr: map[string]any{
				"user.id":                               "serviceAccount:test@opentelemetry.iam.gserviceaccount.com",
				"user.email":                            "test@opentelemetry.iam.gserviceaccount.com",
				gcpAuditAuthenticationAuthoritySelector: "https://www.googleapis.com/auth/cloud-platform",
				gcpAuditAuthenticationServiceAccountKeyName: "//iam.googleapis.com/projects/test/serviceAccounts/test@opentelemetry.iam.gserviceaccount.com/keys/7ebb40c681d9b7d29fd1942bfc2bd65bf5c56b06",
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			attr := pcommon.NewMap()
			handleAuthenticationInfo(tt.auth, attr)
			require.Equal(t, tt.expectedAttr, attr.AsRaw())
		})
	}
}

func TestAuthorizationInfo(t *testing.T) {
	tests := map[string]struct {
		auth         []authorizationInfo
		expectedAttr map[string]any
	}{
		"empty": {
			auth:         []authorizationInfo{},
			expectedAttr: map[string]any{},
		},
		"single auth": {
			auth: func() []authorizationInfo {
				b := true
				return []authorizationInfo{
					{
						Granted:    &b,
						Permission: "io.k8s.core.v1.configmaps.update",
						Resource:   "core/v1/namespaces/kube-system/configmaps/cluster-autoscaler-status",
					},
				}
			}(),
			expectedAttr: map[string]any{
				gcpAuditAuthorization: []any{
					map[string]any{
						gcpAuditAuthorizationPermission: "io.k8s.core.v1.configmaps.update",
						gcpAuditAuthorizationGranted:    true,
						gcpAuditAuthorizationResource:   "core/v1/namespaces/kube-system/configmaps/cluster-autoscaler-status",
					},
				},
			},
		},
		"multiple auth": {
			auth: func() []authorizationInfo {
				b := true
				return []authorizationInfo{
					{
						Granted:    &b,
						Permission: "io.k8s.core.v1.configmaps.update",
						Resource:   "core/v1/namespaces/kube-system/configmaps/cluster-autoscaler-status",
					},
					{
						Granted:    &b,
						Permission: "io.k8s.coordination.v1.leases.update",
						Resource:   "coordination.k8s.io/v1/namespaces/kube-system/leases/cloud-controller-manager",
					},
				}
			}(),
			expectedAttr: map[string]any{
				gcpAuditAuthorization: []any{
					map[string]any{
						gcpAuditAuthorizationPermission: "io.k8s.core.v1.configmaps.update",
						gcpAuditAuthorizationGranted:    true,
						gcpAuditAuthorizationResource:   "core/v1/namespaces/kube-system/configmaps/cluster-autoscaler-status",
					},
					map[string]any{
						gcpAuditAuthorizationPermission: "io.k8s.coordination.v1.leases.update",
						gcpAuditAuthorizationGranted:    true,
						gcpAuditAuthorizationResource:   "coordination.k8s.io/v1/namespaces/kube-system/leases/cloud-controller-manager",
					},
				},
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			attr := pcommon.NewMap()
			handleAuthorizationInfo(tt.auth, attr)
			require.Equal(t, tt.expectedAttr, attr.AsRaw())
		})
	}
}

func TestHandleRequestMetadata(t *testing.T) {
	tests := map[string]struct {
		metadata     *requestMetadata
		expectedAttr map[string]any
		expectsErr   string
	}{
		"nil": {
			metadata:     nil,
			expectedAttr: map[string]any{},
		},
		"not nil": {
			metadata: &requestMetadata{
				CallerIP:                "104.197.128.48",
				CallerSuppliedUserAgent: "Python-httplib2/0.22.0 (gzip), custodian-gcp/0.1 (gzip),gzip(gfe)",
				CallerNetwork:           "//compute.googleapis.com/projects/elastic-apps-163815/global/networks/__unknown__",
			},
			expectedAttr: map[string]any{
				"client.address":             "104.197.128.48",
				"user_agent.original":        "Python-httplib2/0.22.0 (gzip), custodian-gcp/0.1 (gzip),gzip(gfe)",
				gcpAuditRequestCallerNetwork: "//compute.googleapis.com/projects/elastic-apps-163815/global/networks/__unknown__",
			},
		},
		"request attributes": {
			metadata: &requestMetadata{
				RequestAttributes: &requestAttributes{
					ID:     "req-12345",
					Method: "GET",
					Headers: map[string]string{
						"User-Agent": "test-client/1.0",
						"Accept":     "application/json",
					},
					Path:     "/test/path",
					Host:     "example.com",
					Scheme:   "https",
					Query:    "foo=bar&baz=qux",
					Time:     "2025-08-21T12:34:56Z",
					Size:     "1234",
					Protocol: "HTTP/1.1",
					Reason:   "test-reason",
					Auth: auth{
						Principal:    "user@example.com",
						Audiences:    []string{"test-service", "another-service"},
						Presenter:    "test-presenter",
						AccessLevels: []string{"level1", "level2"},
					},
				},
			},
			expectedAttr: map[string]any{
				"http.request.size":              int64(1234),
				"http.request.method":            "GET",
				"url.query":                      "foo=bar&baz=qux",
				"url.path":                       "/test/path",
				"url.scheme":                     "https",
				gcpAuditRequestTime:              "2025-08-21T12:34:56Z",
				"http.request.header.host":       "example.com",
				"http.request.header.user-agent": "test-client/1.0",
				"http.request.header.accept":     "application/json",
				"network.protocol.name":          "http/1.1",
				gcpAuditRequestReason:            "test-reason",
				httpRequestID:                    "req-12345",
				gcpAuditRequestAuthPrincipal:     "user@example.com",
				gcpAuditRequestAuthPresenter:     "test-presenter",
				gcpAuditRequestAuthAccessLevels:  []any{"level1", "level2"},
				gcpAuditRequestAuthAudiences:     []any{"test-service", "another-service"},
			},
		},
		"request attributes - invalid request size format": {
			metadata: &requestMetadata{
				RequestAttributes: &requestAttributes{
					Size: "invalid",
				},
			},
			expectsErr: "failed to add http request size",
		},
		"destination attributes": {
			metadata: &requestMetadata{
				DestinationAttributes: &destinationAttributes{
					IP:         "10.0.0.1",
					Port:       "8080",
					Principal:  "serviceAccount:my-svc@project.iam.gserviceaccount.com",
					RegionCode: "us-central1",
					Labels: map[string]string{
						"env":        "staging",
						"team.owner": "devops",
					},
				},
			},
			expectedAttr: map[string]any{
				"server.port":                 int64(8080),
				"server.address":              "10.0.0.1",
				gcpAuditDestinationPrincipal:  "serviceAccount:my-svc@project.iam.gserviceaccount.com",
				gcpAuditDestinationRegionCode: "us-central1",
				gcpAuditDestinationLabels: map[string]any{
					"env":        "staging",
					"team.owner": "devops",
				},
			},
		},
		"destination attributes - invalid port format": {
			metadata: &requestMetadata{
				DestinationAttributes: &destinationAttributes{
					Port: "invalid",
				},
			},
			expectsErr: "failed to add destination port",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			attr := pcommon.NewMap()
			err := handleRequestMetadata(tt.metadata, attr)
			if tt.expectsErr != "" {
				require.ErrorContains(t, err, tt.expectsErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.expectedAttr, attr.AsRaw())
		})
	}
}

func TestHandlePolicyViolationInfo(t *testing.T) {
	tests := map[string]struct {
		info         *policyViolationInfo
		expectedAttr map[string]any
	}{
		"nil": {
			info:         nil,
			expectedAttr: map[string]any{},
		},
		"nil policy violation": {
			info:         &policyViolationInfo{},
			expectedAttr: map[string]any{},
		},
		"not nil": {
			info: &policyViolationInfo{
				OrgPolicyViolationInfo: &orgPolicyViolationInfo{
					ResourceType: "store.googleapis.com/bucket",
					ResourceTags: map[string]string{
						"name":  "wrench",
						"mass":  "1.3kg",
						"count": "3",
					},
					ViolationInfo: []violationInfo{
						{
							Constraint:   "constraints/compute.vmExternalIpAccess",
							ErrorMessage: "External IP access requires security approval",
							CheckedValue: "true",
							PolicyType:   "BOOLEAN",
						},
					},
				},
			},
			expectedAttr: map[string]any{
				gcpAuditPolicyViolationResourceType: "store.googleapis.com/bucket",
				gcpAuditPolicyViolationResourceTags: map[string]any{
					"name":  "wrench",
					"mass":  "1.3kg",
					"count": "3",
				},
				gcpAuditPolicyViolationInfo: []any{
					map[string]any{
						gcpAuditPolicyViolationInfoConstraint:   "constraints/compute.vmExternalIpAccess",
						gcpAuditPolicyViolationInfoErrorMessage: "External IP access requires security approval",
						gcpAuditPolicyViolationInfoPolicyType:   "BOOLEAN",
						gcpAuditPolicyViolationInfoCheckedValue: "true",
					},
				},
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			attr := pcommon.NewMap()
			handlePolicyViolationInfo(tt.info, attr)
			require.Equal(t, tt.expectedAttr, attr.AsRaw())
		})
	}
}

func TestParsePayloadIntoAttributes(t *testing.T) {
	tests := map[string]struct {
		payload      []byte
		expectedAttr map[string]any
		expectsErr   string
	}{
		"invalid payload": {
			payload:    []byte("invalid"),
			expectsErr: "failed to unmarshal audit log payload",
		},
		"invalid audit log": {
			payload:    []byte(`{}`),
			expectsErr: "failed to validate audit log payload",
		},
		"invalid num response items": {
			payload: []byte(`{
  "@type": "type.googleapis.com/google.cloud.audit.AuditLog",
  "methodName": "io.k8s.core.v1.configmaps.update",
  "serviceName": "k8s.io",
  "numResponseItems": "invalid"
}`),
			expectsErr: "failed to add number of response items",
		},
		"valid": {
			payload: []byte(`{
  "@type": "type.googleapis.com/google.cloud.audit.AuditLog",
  "methodName": "io.k8s.core.v1.configmaps.update",
  "serviceName": "k8s.io"
}`),
			expectedAttr: map[string]any{
				gcpAuditMethodName:  "io.k8s.core.v1.configmaps.update",
				gcpAuditServiceName: "k8s.io",
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			attr := pcommon.NewMap()
			err := ParsePayloadIntoAttributes(tt.payload, attr)
			if tt.expectsErr != "" {
				require.ErrorContains(t, err, tt.expectsErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.expectedAttr, attr.AsRaw())
		})
	}
}
