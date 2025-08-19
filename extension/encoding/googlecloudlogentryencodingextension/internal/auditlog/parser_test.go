// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package auditlog

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	semconv "go.opentelemetry.io/otel/semconv/v1.34.0"
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
				string(semconv.RPCJSONRPCErrorCodeKey):    int64(6),
				string(semconv.RPCJSONRPCErrorMessageKey): "RESOURCE_ALREADY_EXISTS",
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
				string(semconv.UserIDKey):                   "serviceAccount:test@opentelemetry.iam.gserviceaccount.com",
				string(semconv.UserEmailKey):                "test@opentelemetry.iam.gserviceaccount.com",
				gcpAuditAuthenticationAuthoritySelector:     "https://www.googleapis.com/auth/cloud-platform",
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
				string(semconv.ClientAddressKey):     "104.197.128.48",
				string(semconv.UserAgentOriginalKey): "Python-httplib2/0.22.0 (gzip), custodian-gcp/0.1 (gzip),gzip(gfe)",
				gcpAuditRequestCallerNetwork:         "//compute.googleapis.com/projects/elastic-apps-163815/global/networks/__unknown__",
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			attr := pcommon.NewMap()
			handleRequestMetadata(tt.metadata, attr)
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
