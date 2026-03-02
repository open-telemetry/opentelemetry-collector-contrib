// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logs // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/azureencodingextension/internal/unmarshaler/logs"

import (
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	conventions "go.opentelemetry.io/otel/semconv/v1.38.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/azureencodingextension/internal/unmarshaler"
)

// Constants for Identity > claims (JWT standard claims)
const (
	identityClaimIssuer    = "iss"
	identityClaimSubject   = "sub"
	identityClaimAudience  = "aud"
	identityClaimExpires   = "exp"
	identityClaimNotBefore = "nbf"
	identityClaimIssuedAt  = "iat"
)

// Constants for Identity > claims (Azure-specific claims)
const (
	identityClaimScope                 = "http://schemas.microsoft.com/identity/claims/scope"
	identityClaimType                  = "idtyp"
	identityClaimApplicationID         = "appid"
	identityClaimAuthMethodsReferences = "http://schemas.microsoft.com/claims/authnmethodsreferences"
	identityClaimProvider              = "http://schemas.microsoft.com/identity/claims/identityprovider"
	identityClaimIdentifierObject      = "http://schemas.microsoft.com/identity/claims/objectidentifier"
	identityClaimIdentifierName        = "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/nameidentifier"
	identityClaimEmailAddress          = "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/emailaddress"
)

// OpenTelemetry attribute names for Activity Log identity fields
const (
	// Identity > authorization
	attributeIdentityAuthorizationScope  = "azure.identity.authorization.scope"
	attributeIdentityAuthorizationAction = "azure.identity.authorization.action"
	// Identity > authorization > evidence
	attributeIdentityAuthorizationEvidenceRole                = "azure.identity.authorization.evidence.role"
	attributeIdentityAuthorizationEvidenceRoleAssignmentScope = "azure.identity.authorization.evidence.role.assignment.scope"
	attributeIdentityAuthorizationEvidenceRoleAssignmentID    = "azure.identity.authorization.evidence.role.assignment.id"
	attributeIdentityAuthorizationEvidenceRoleDefinitionID    = "azure.identity.authorization.evidence.role.definition.id"
	attributeIdentityAuthorizationEvidencePrincipalID         = "azure.identity.authorization.evidence.principal.id"
	attributeIdentityAuthorizationEvidencePrincipalType       = "azure.identity.authorization.evidence.principal.type"
	// Identity > claims (standard JWT claims)
	attributeIdentityClaimsAudience  = "azure.identity.audience"
	attributeIdentityClaimsIssuer    = "azure.identity.issuer"
	attributeIdentityClaimsSubject   = "azure.identity.subject"
	attributeIdentityClaimsNotAfter  = "azure.identity.not_after"
	attributeIdentityClaimsNotBefore = "azure.identity.not_before"
	attributeIdentityClaimsCreated   = "azure.identity.created"
	// Identity > claims (Azure-specific claims)
	attributeIdentityClaimsScope                 = "azure.identity.scope"
	attributeIdentityClaimsType                  = "azure.identity.type"
	attributeIdentityClaimsApplicationID         = "azure.identity.application.id"
	attributeIdentityClaimsAuthMethodsReferences = "azure.identity.auth.methods.references"
	attributeIdentityClaimsIdentifierObject      = "azure.identity.identifier.object"
	attributeIdentityClaimsIdentifierID          = "user.id"
	attributeIdentityClaimsProvider              = "azure.identity.provider"
)

// OpenTelemetry attribute name for Azure Identity as a nested map,
// used for identity structures that are not Activity Log style
// (e.g., Storage logs, KeyVault logs)
const attributeAzureIdentity = "azure.identity"

// azureIdentityRecord is an interface for category-specific identity parsing.
// Each category that has identity data implements this interface with
// a type-safe struct that is parsed directly from JSON (no double parsing).
type azureIdentityRecord interface {
	PutIdentityAttributes(attrs pcommon.Map)
}

// Compile-time interface satisfaction checks
var (
	_ azureIdentityRecord = (*azureIdentityActivity)(nil)
	_ azureIdentityRecord = (*azureIdentityStorage)(nil)
)

// azureIdentityBase contains the common `claims` field shared across
// identity structures that include Azure AD/Entra ID JWT token claims.
// Category-specific identity types embed this to inherit claims parsing.
type azureIdentityBase struct {
	Claims map[string]string `json:"claims"`
}

// PutIdentityAttributes extracts known identity fields into flat OTel attributes.
// Only specific fields are extracted to minimize the risk of including sensitive data.
func (id *azureIdentityBase) PutIdentityAttributes(attrs pcommon.Map) {
	// Claims: simple string claims
	claimAttributesMap := map[string]string{
		identityClaimIssuer:                attributeIdentityClaimsIssuer,
		identityClaimSubject:               attributeIdentityClaimsSubject,
		identityClaimAudience:              attributeIdentityClaimsAudience,
		identityClaimScope:                 attributeIdentityClaimsScope,
		identityClaimType:                  attributeIdentityClaimsType,
		identityClaimApplicationID:         attributeIdentityClaimsApplicationID,
		identityClaimAuthMethodsReferences: attributeIdentityClaimsAuthMethodsReferences,
		identityClaimProvider:              attributeIdentityClaimsProvider,
		identityClaimIdentifierObject:      attributeIdentityClaimsIdentifierObject,
		identityClaimIdentifierName:        attributeIdentityClaimsIdentifierID,
		identityClaimEmailAddress:          string(conventions.UserEmailKey),
	}
	for claimKey, attrName := range claimAttributesMap {
		unmarshaler.AttrPutStrIf(attrs, attrName, id.Claims[claimKey])
	}

	// Claims: timestamp fields (Unix epoch -> RFC3339)
	timestampClaimsMap := map[string]string{
		identityClaimExpires:   attributeIdentityClaimsNotAfter,
		identityClaimNotBefore: attributeIdentityClaimsNotBefore,
		identityClaimIssuedAt:  attributeIdentityClaimsCreated,
	}
	for claimKey, attrName := range timestampClaimsMap {
		if ts := id.Claims[claimKey]; ts != "" {
			if parsedTime, err := parseUnixTimestamp(ts); err == nil {
				attrs.PutStr(attrName, parsedTime.Format(time.RFC3339))
			}
		}
	}
}
