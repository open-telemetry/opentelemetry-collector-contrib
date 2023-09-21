// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
package snmpreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/snmpreceiver"

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
)

// Config Defaults
const (
	defaultCollectionInterval = 10 * time.Second // In seconds
	defaultTimeout            = 5 * time.Second  // In seconds
	defaultEndpoint           = "udp://localhost:161"
	defaultVersion            = "v2c"
	defaultCommunity          = "public"
	defaultSecurityLevel      = "no_auth_no_priv"
	defaultAuthType           = "MD5"
	defaultPrivacyType        = "DES"
)

var (
	// Config error messages
	errMsgInvalidEndpointWError                     = `invalid endpoint '%s': must be in '[scheme]://[host]:[port]' format: %w`
	errMsgInvalidEndpoint                           = `invalid endpoint '%s': must be in '[scheme]://[host]:[port]' format`
	errMsgAttributeConfigNoEnumOIDOrPrefix          = `attribute '%s' must contain one of either an enum, oid, or indexed_value_prefix`
	errMsgResourceAttributeNoOIDOrScalarOIDOrPrefix = `resource_attribute '%s' must contain one of either an oid, scalar_oid, or indexed_value_prefix`
	errMsgMetricNoUnit                              = `metric '%s' must have a unit`
	errMsgMetricNoGaugeOrSum                        = `metric '%s' must have one of either a gauge or sum`
	errMsgMetricNoOIDs                              = `metric '%s' must have one of either scalar_oids or indexed_oids`
	errMsgGaugeBadValueType                         = `metric '%s' gauge value_type must be either int or double`
	errMsgSumBadValueType                           = `metric '%s' sum value_type must be either int or double`
	errMsgSumBadAggregation                         = `metric '%s' sum aggregation value must be either cumulative or delta`
	errMsgScalarOIDNoOID                            = `metric '%s' scalar_oid must contain an oid`
	errMsgScalarAttributeNoName                     = `metric '%s' scalar_oid attribute must contain a name`
	errMsgScalarAttributeBadName                    = `metric '%s' scalar_oid attribute name '%s' must match an attribute config`
	errMsgScalarOIDBadAttribute                     = `metric '%s' scalar_oid attribute name '%s' must match attribute config with enum values`
	errMsgScalarAttributeBadValue                   = `metric '%s' scalar_oid attribute '%s' value '%s' must match one of the possible enum values for the attribute config`
	errMsgScalarMetricHasIndexedResourceAttribute   = `scalar oid metric '%s' has resource attribute '%s' which has indexed value`
	errMsgColumnOIDNoOID                            = `metric '%s' column_oid must contain an oid`
	errMsgColumnAttributeNoName                     = `metric '%s' column_oid attribute must contain a name`
	errMsgColumnAttributeBadName                    = `metric '%s' column_oid attribute name '%s' must match an attribute config`
	errMsgColumnAttributeBadValue                   = `metric '%s' column_oid attribute '%s' value '%s' must match one of the possible enum values for the attribute config`
	errMsgColumnResourceAttributeBadName            = `metric '%s' column_oid resource_attribute '%s' must match a resource_attribute config`
	errMsgColumnIndexedIdentifierRequired           = `metric '%s' column_oid must either have an indexed resource_attribute or an indexed_value_prefix/oid attribute`
	errMsgMultipleKeysSetOnResourceAttribute        = `resource attribute '%s' must have only one of oid, scalar_oid, or indexed_value_prefix`
	errScalarOIDResourceAttributeEndsInNonzeroDigit = `resource attribute '%s' has scalar_oid '%s' that ends in a nonzero digit (scalar oids should not be indexed)`
	errColumnOIDResourceAttributeEndsInZero         = `resource attribute '%s' has oid '%s' that ends in a zero (column oids should be indexed)`

	// Config errors
	errEmptyEndpoint        = errors.New("endpoint must be specified")
	errEndpointBadScheme    = errors.New("endpoint scheme must be either tcp, tcp4, tcp6, udp, udp4, or udp6")
	errEmptyVersion         = errors.New("version must specified")
	errBadVersion           = errors.New("version must be either v1, v2c, or v3")
	errEmptyUser            = errors.New("user must be specified when version is v3")
	errEmptySecurityLevel   = errors.New("security_level must be specified when version is v3")
	errBadSecurityLevel     = errors.New("security_level must be either no_auth_no_priv, auth_no_priv, or auth_priv")
	errEmptyAuthType        = errors.New("auth_type must be specified when security_level is auth_no_priv or auth_priv")
	errBadAuthType          = errors.New("auth_type must be either MD5, SHA, SHA224, SHA256, SHA384, SHA512")
	errEmptyAuthPassword    = errors.New("auth_password must be specified when security_level is auth_no_priv or auth_priv")
	errEmptyPrivacyType     = errors.New("privacy_type must be specified when security_level is auth_priv")
	errBadPrivacyType       = errors.New("privacy_type must be either DES, AES, AES192, AES192C, AES256, AES256C")
	errEmptyPrivacyPassword = errors.New("privacy_password must be specified when security_level is auth_priv")
	errMetricRequired       = errors.New("must have at least one config under metrics")
)

// Config defines the configuration for the various elements of the receiver.
type Config struct {
	scraperhelper.ScraperControllerSettings `mapstructure:",squash"`

	// Endpoint is the SNMP target to request data from. Must be formatted as [udp|tcp|][4|6|]://{host}:{port}.
	// Default: udp://localhost:161
	// If no scheme is given, udp4 is assumed.
	// If no port is given, 161 is assumed.
	Endpoint string `mapstructure:"endpoint"`

	// Version is the version of SNMP to use for this connection.
	// Valid options: v1, v2c, v3.
	// Default: v2c
	Version string `mapstructure:"version"`

	// Community is the SNMP community string to use.
	// Only valid for versions "v1" and "v2c"
	// Default: public
	Community string `mapstructure:"community"`

	// User is the SNMP User for this connection.
	// Only valid for version “v3”
	User string `mapstructure:"user"`

	// SecurityLevel is the security level to use for this SNMP connection.
	// Only valid for version “v3”
	// Valid options: “no_auth_no_priv”, “auth_no_priv”, “auth_priv”
	// Default: "no_auth_no_priv"
	SecurityLevel string `mapstructure:"security_level"`

	// AuthType is the type of authentication protocol to use for this SNMP connection.
	// Only valid for version “v3” and if “no_auth_no_priv” is not selected for SecurityLevel
	// Valid options: “md5”, “sha”, “sha224”, “sha256”, “sha384”, “sha512”
	// Default: "md5"
	AuthType string `mapstructure:"auth_type"`

	// AuthPassword is the authentication password used for this SNMP connection.
	// Only valid for version "v3" and if "no_auth_no_priv" is not selected for SecurityLevel
	AuthPassword configopaque.String `mapstructure:"auth_password"`

	// PrivacyType is the type of privacy protocol to use for this SNMP connection.
	// Only valid for version “v3” and if "auth_priv" is selected for SecurityLevel
	// Valid options: “des”, “aes”, “aes192”, “aes256”, “aes192c”, “aes256c”
	// Default: "des"
	PrivacyType string `mapstructure:"privacy_type"`

	// PrivacyPassword is the authentication password used for this SNMP connection.
	// Only valid for version “v3” and if "auth_priv" is selected for SecurityLevel
	PrivacyPassword configopaque.String `mapstructure:"privacy_password"`

	// ResourceAttributes defines what resource attributes will be used for this receiver and is composed
	// of resource attribute names along with their resource attribute configurations
	ResourceAttributes map[string]*ResourceAttributeConfig `mapstructure:"resource_attributes"`

	// Attributes defines what attributes will be used on metrics for this receiver and is composed of
	// attribute names along with their attribute configurations
	Attributes map[string]*AttributeConfig `mapstructure:"attributes"`

	// Metrics defines what SNMP metrics will be collected for this receiver and is composed of metric
	// names along with their metric configurations
	Metrics map[string]*MetricConfig `mapstructure:"metrics"`
}

// ResourceAttributeConfig contains config info about all of the resource attributes that will be used by this receiver.
type ResourceAttributeConfig struct {
	// Description is optional and describes what the resource attribute represents
	Description string `mapstructure:"description"`
	// OID is required only if ScalarOID or IndexedValuePrefix is not set.
	// This is the column OID which will provide indexed values to be used for this resource attribute. These indexed values
	// will ultimately each be associated with a different "resource" as an attribute on that resource. Indexed metric values
	// will then be used to associate metric datapoints to the matching "resource" (based on matching indexes).
	OID string `mapstructure:"oid"`
	// ScalarOID is required only if OID or IndexedValuePrefix is not set.
	// This is the scalar OID which will provide a value to be used for this resource attribute.
	// Single or indexed metrics can then be associated with the resource. (Indexed metrics also need an indexed attribute or resource attribute to associate with a scalar metric resource attribute)
	ScalarOID string `mapstructure:"scalar_oid"`
	// IndexedValuePrefix is required only if OID or ScalarOID is not set.
	// This will be used alongside indexed metric values for this resource attribute. The prefix value concatenated with
	// specific indexes of metric indexed values (Ex: prefix.1.2) will ultimately each be associated with a different "resource"
	// as an attribute on that resource. The related indexed metric values will then be used to associate metric datapoints to
	// those resources.
	IndexedValuePrefix string `mapstructure:"indexed_value_prefix"` // required and valid if no oid or scalar_oid field
}

// AttributeConfig contains config info about all of the metric attributes that will be used by this receiver.
type AttributeConfig struct {
	// Value is optional, and will allow for a different attribute key other than the attribute name
	Value string `mapstructure:"value"`
	// Description is optional and describes what the attribute represents
	Description string `mapstructure:"description"`
	// Enum is required only if OID and IndexedValuePrefix are not defined.
	// This contains a list of possible values that can be associated with this attribute
	Enum []string `mapstructure:"enum"`
	// OID is required only if Enum and IndexedValuePrefix are not defined.
	// This is the column OID which will provide indexed values to be uased for this attribute (alongside a metric with ColumnOIDs)
	OID string `mapstructure:"oid"`
	// IndexedValuePrefix is required only if Enum and OID are not defined.
	// This is used alongside metrics with ColumnOIDs to assign attribute values using this prefix + the OID index of the metric value
	IndexedValuePrefix string `mapstructure:"indexed_value_prefix"`
}

// MetricConfig contains config info about a given metric
type MetricConfig struct {
	// Description is optional and describes what this metric represents
	Description string `mapstructure:"description"`
	// Unit is required
	Unit string `mapstructure:"unit"`
	// Either Gauge or Sum config is required
	Gauge *GaugeMetric `mapstructure:"gauge"`
	Sum   *SumMetric   `mapstructure:"sum"`
	// Either ScalarOIDs or ColumnOIDs is required.
	// ScalarOIDs is used if one or more scalar OID values is used for this metric.
	// ColumnOIDs is used if one or more column OID indexed set of values is used
	// for this metric.
	ScalarOIDs []ScalarOID `mapstructure:"scalar_oids"`
	ColumnOIDs []ColumnOID `mapstructure:"column_oids"`
}

// GaugeMetric contains info about the value of the gauge metric
type GaugeMetric struct {
	// ValueType is required can can be either int or double
	ValueType string `mapstructure:"value_type"`
}

// SumMetric contains info about the value of the sum metric
type SumMetric struct {
	// Aggregation is required and can be cumulative or delta
	Aggregation string `mapstructure:"aggregation"`
	// Monotonic is required and can be true or false
	Monotonic bool `mapstructure:"monotonic"`
	// ValueType is required can can be either int or double
	ValueType string `mapstructure:"value_type"`
}

// ScalarOID holds OID info for a scalar metric as well as any {resource} attributes
// that are attached to it
type ScalarOID struct {
	// OID is required and is the scalar OID that is associated with a metric
	OID string `mapstructure:"oid"`
	// ResourceAttributes is optional and may contain only scalar OID values to associate this metric with
	ResourceAttributes []string `mapstructure:"resource_attributes"`
	// Attributes is optional and may contain names and values associated with enum
	// AttributeConfigs to associate with the value of the scalar OID
	Attributes []Attribute `mapstructure:"attributes"`
}

// ColumnOID holds OID info for an indexed metric as well as any attributes
// or resource attributes that are attached to it
type ColumnOID struct {
	// OID is required and is the column OID that is associated with a metric
	OID string `mapstructure:"oid"`
	// ResourceAttributes is required only if there are no Attributes associated with non enum
	// AttributeConfigs defined here. Valid values are ResourceAttributeConfig names that will
	// be used to differentiate the indexed values for the column OID
	ResourceAttributes []string `mapstructure:"resource_attributes"`
	// Attributes is required only if there are no ResourceAttributes associated defined here.
	// Valid values are non enum AttributeConfig names that will be used to differentiate the
	// indexed values for the column OID
	Attributes []Attribute `mapstructure:"attributes"`
}

// Attribute is a connection between a metric configuration and an AttributeConfig
type Attribute struct {
	// Name is required and should match the key for an AttributeConfig
	Name string `mapstructure:"name"`
	// Value is optional and is only needed for a matched AttributeConfig's with enum value.
	// Value should match one of the AttributeConfig's enum values in this case
	Value string `mapstructure:"value"`
}

// Validate validates the given config, returning an error specifying any issues with the config.
func (cfg *Config) Validate() error {
	var combinedErr error

	combinedErr = errors.Join(combinedErr, validateEndpoint(cfg))
	combinedErr = errors.Join(combinedErr, validateVersion(cfg))
	if strings.ToUpper(cfg.Version) == "V3" {
		combinedErr = errors.Join(combinedErr, validateSecurity(cfg))
	}
	combinedErr = errors.Join(combinedErr, validateMetricConfigs(cfg))

	return combinedErr
}

// validateEndpoint validates the Endpoint
func validateEndpoint(cfg *Config) error {
	if cfg.Endpoint == "" {
		return errEmptyEndpoint
	}

	// Ensure valid endpoint
	u, err := url.Parse(cfg.Endpoint)
	if err != nil {
		return fmt.Errorf(errMsgInvalidEndpointWError, cfg.Endpoint, err)
	}
	if u.Host == "" || u.Port() == "" {
		return fmt.Errorf(errMsgInvalidEndpoint, cfg.Endpoint)
	}

	// Ensure valid scheme
	switch strings.ToUpper(u.Scheme) {
	case "TCP", "TCP4", "TCP6", "UDP", "UDP4", "UDP6": // ok
	default:
		return errEndpointBadScheme
	}

	return nil
}

// validateVersion validates the Version
func validateVersion(cfg *Config) error {
	if cfg.Version == "" {
		return errEmptyVersion
	}

	// Ensure valid version
	switch strings.ToUpper(cfg.Version) {
	case "V1", "V2C", "V3": // ok
	default:
		return errBadVersion
	}

	return nil
}

// validateSecurity validates all v3 related security configs
func validateSecurity(cfg *Config) error {
	var combinedErr error

	// Ensure valid user
	if cfg.User == "" {
		combinedErr = errors.Join(combinedErr, errEmptyUser)
	}

	if cfg.SecurityLevel == "" {
		return errors.Join(combinedErr, errEmptySecurityLevel)
	}

	// Ensure valid security level
	switch strings.ToUpper(cfg.SecurityLevel) {
	case "NO_AUTH_NO_PRIV":
		return combinedErr
	case "AUTH_NO_PRIV":
		// Ensure valid auth configs
		return errors.Join(combinedErr, validateAuth(cfg))
	case "AUTH_PRIV": // ok
		// Ensure valid auth and privacy configs
		combinedErr = errors.Join(combinedErr, validateAuth(cfg))
		return errors.Join(combinedErr, validatePrivacy(cfg))
	default:
		return errors.Join(combinedErr, errBadSecurityLevel)
	}
}

// validateAuth validates the AuthType and AuthPassword
func validateAuth(cfg *Config) error {
	var combinedErr error

	// Ensure valid auth password
	if cfg.AuthPassword == "" {
		combinedErr = errors.Join(combinedErr, errEmptyAuthPassword)
	}

	// Ensure valid auth type
	if cfg.AuthType == "" {
		return errors.Join(combinedErr, errEmptyAuthType)
	}

	switch strings.ToUpper(cfg.AuthType) {
	case "MD5", "SHA", "SHA224", "SHA256", "SHA384", "SHA512": // ok
	default:
		combinedErr = errors.Join(combinedErr, errBadAuthType)
	}

	return combinedErr
}

// validatePrivacy validates the PrivacyType and PrivacyPassword
func validatePrivacy(cfg *Config) error {
	var combinedErr error

	// Ensure valid privacy password
	if cfg.PrivacyPassword == "" {
		combinedErr = errors.Join(combinedErr, errEmptyPrivacyPassword)
	}

	// Ensure valid privacy type
	if cfg.PrivacyType == "" {
		return errors.Join(combinedErr, errEmptyPrivacyType)
	}

	switch strings.ToUpper(cfg.PrivacyType) {
	case "DES", "AES", "AES192", "AES192C", "AES256", "AES256C": // ok
	default:
		combinedErr = errors.Join(combinedErr, errBadPrivacyType)
	}

	return combinedErr
}

// validateMetricConfigs validates all MetricConfigs, AttributeConfigs, and ResourceAttributeConfigs
func validateMetricConfigs(cfg *Config) error {
	var combinedErr error

	// Validate the Attribute and ResourceAttribute configs up front
	combinedErr = errors.Join(combinedErr, validateAttributeConfigs(cfg))
	combinedErr = errors.Join(combinedErr, validateResourceAttributeConfigs(cfg))

	// Ensure there is at least one MetricConfig
	metrics := cfg.Metrics
	if len(metrics) == 0 {
		return errors.Join(combinedErr, errMetricRequired)
	}

	// Make sure each MetricConfig has valid info
	for metricName, metricCfg := range metrics {
		if metricCfg.Unit == "" {
			combinedErr = errors.Join(combinedErr, fmt.Errorf(errMsgMetricNoUnit, metricName))
		}

		if metricCfg.Gauge == nil && metricCfg.Sum == nil {
			combinedErr = errors.Join(combinedErr, fmt.Errorf(errMsgMetricNoGaugeOrSum, metricName))
		}

		if len(metricCfg.ScalarOIDs) == 0 && len(metricCfg.ColumnOIDs) == 0 {
			combinedErr = errors.Join(combinedErr, fmt.Errorf(errMsgMetricNoOIDs, metricName))
		}

		if metricCfg.Gauge != nil {
			combinedErr = errors.Join(combinedErr, validateGauge(metricName, metricCfg.Gauge))
		}

		if metricCfg.Sum != nil {
			combinedErr = errors.Join(combinedErr, validateSum(metricName, metricCfg.Sum))
		}

		for _, scalarOID := range metricCfg.ScalarOIDs {
			combinedErr = errors.Join(combinedErr, validateScalarOID(metricName, scalarOID, cfg))
		}

		for _, columnOID := range metricCfg.ColumnOIDs {
			combinedErr = errors.Join(combinedErr, validateColumnOID(metricName, columnOID, cfg))
		}
	}

	return combinedErr
}

// validateColumnOID validates a ColumnOID
func validateColumnOID(metricName string, columnOID ColumnOID, cfg *Config) error {
	var combinedErr error

	// Ensure that it contains an OID
	if columnOID.OID == "" {
		combinedErr = errors.Join(combinedErr, fmt.Errorf(errMsgColumnOIDNoOID, metricName))
	}

	// Keep track of whether the different indexed values can be differentiated by either attribute within the same metric
	// or by different resource attributes (in different resources)
	hasIndexedIdentifier := false

	// Check that any Attributes have a valid Name and a valid Value (if applicable)
	if len(columnOID.Attributes) > 0 {
		for _, attribute := range columnOID.Attributes {
			if attribute.Name == "" {
				combinedErr = errors.Join(combinedErr, fmt.Errorf(errMsgColumnAttributeNoName, metricName))
				continue
			}

			attrCfg, ok := cfg.Attributes[attribute.Name]
			if !ok {
				combinedErr = errors.Join(combinedErr, fmt.Errorf(errMsgColumnAttributeBadName, metricName, attribute.Name))
				continue
			}

			if len(attrCfg.Enum) > 0 {
				if !contains(attrCfg.Enum, attribute.Value) {
					combinedErr = errors.Join(combinedErr, fmt.Errorf(errMsgColumnAttributeBadValue, metricName, attribute.Name, attribute.Value))
				}
				continue
			}

			hasIndexedIdentifier = true
		}
	}

	// Check that any ResourceAttributes have a valid value
	for _, name := range columnOID.ResourceAttributes {
		resourceAttribute, ok := cfg.ResourceAttributes[name]
		if !ok {
			combinedErr = errors.Join(combinedErr, fmt.Errorf(errMsgColumnResourceAttributeBadName, metricName, name))
			continue
		}

		if resourceAttribute.OID != "" || resourceAttribute.IndexedValuePrefix != "" {
			hasIndexedIdentifier = true
		}
	}

	// Check that there is either a column based attribute or resource attribute associated with it
	if !hasIndexedIdentifier {
		combinedErr = errors.Join(combinedErr, fmt.Errorf(errMsgColumnIndexedIdentifierRequired, metricName))
	}

	return combinedErr
}

// validateScalarOID validates a ScalarOID
func validateScalarOID(metricName string, scalarOID ScalarOID, cfg *Config) error {
	var combinedErr error

	// Ensure that it contains an OID
	if scalarOID.OID == "" {
		combinedErr = errors.Join(combinedErr, fmt.Errorf(errMsgScalarOIDNoOID, metricName))
	}

	// Check that any Resource Attributes have a valid Value
	for _, name := range scalarOID.ResourceAttributes {
		resourceAttribute, ok := cfg.ResourceAttributes[name]
		if !ok {
			combinedErr = errors.Join(combinedErr, fmt.Errorf(errMsgColumnResourceAttributeBadName, metricName, name))
			continue
		}

		// Scalar OID metrics should only have Scalar OID resource attributes
		// ResourceAttributeConfig validation ensures that (only) one of ScalarOID, OID, or IndexedValuePrefix is set before reaching this
		if resourceAttribute.OID != "" || resourceAttribute.IndexedValuePrefix != "" {
			combinedErr = errors.Join(combinedErr, fmt.Errorf(errMsgScalarMetricHasIndexedResourceAttribute, metricName, name))
			continue
		}

	}

	if len(scalarOID.Attributes) == 0 {
		return combinedErr
	}

	// Check that any Attributes have a valid Name and a valid Value
	for _, attribute := range scalarOID.Attributes {
		if attribute.Name == "" {
			combinedErr = errors.Join(combinedErr, fmt.Errorf(errMsgScalarAttributeNoName, metricName))
			continue
		}

		attrCfg, ok := cfg.Attributes[attribute.Name]
		if !ok {
			combinedErr = errors.Join(combinedErr, fmt.Errorf(errMsgScalarAttributeBadName, metricName, attribute.Name))
			continue
		}

		if len(attrCfg.Enum) == 0 {
			combinedErr = errors.Join(combinedErr, fmt.Errorf(errMsgScalarOIDBadAttribute, metricName, attribute.Name))
			continue
		}

		if !contains(attrCfg.Enum, attribute.Value) {
			combinedErr = errors.Join(combinedErr, fmt.Errorf(errMsgScalarAttributeBadValue, metricName, attribute.Name, attribute.Value))
		}
	}

	return combinedErr
}

// validateGauge validates a GaugeMetric
func validateGauge(metricName string, gauge *GaugeMetric) error {
	// Ensure valid values for ValueType
	upperValType := strings.ToUpper(gauge.ValueType)
	if upperValType != "INT" && upperValType != "DOUBLE" {
		return fmt.Errorf(errMsgGaugeBadValueType, metricName)
	}

	return nil
}

// validateSum validates a SumMetric
func validateSum(metricName string, sum *SumMetric) error {
	var combinedErr error

	// Ensure valid values for ValueType
	upperValType := strings.ToUpper(sum.ValueType)
	if upperValType != "INT" && upperValType != "DOUBLE" {
		combinedErr = errors.Join(combinedErr, fmt.Errorf(errMsgSumBadValueType, metricName))
	}

	// Ensure valid values for Aggregation
	upperAggregation := strings.ToUpper(sum.Aggregation)
	if upperAggregation != "CUMULATIVE" && upperAggregation != "DELTA" {
		combinedErr = errors.Join(combinedErr, fmt.Errorf(errMsgSumBadAggregation, metricName))
	}

	return combinedErr
}

// validateAttributeConfigs validates the AttributeConfigs
func validateAttributeConfigs(cfg *Config) error {
	var combinedErr error

	attributes := cfg.Attributes
	if len(attributes) == 0 {
		return nil
	}

	// Make sure each Attribute has either an OID, Enum, or IndexedValuePrefix
	for attrName, attrCfg := range attributes {
		if len(attrCfg.Enum) == 0 && attrCfg.OID == "" && attrCfg.IndexedValuePrefix == "" {
			combinedErr = errors.Join(combinedErr, fmt.Errorf(errMsgAttributeConfigNoEnumOIDOrPrefix, attrName))
		}
	}

	return combinedErr
}

// validateResourceAttributeConfigs validates the ResourceAttributeConfigs
func validateResourceAttributeConfigs(cfg *Config) error {
	var combinedErr error

	resourceAttributes := cfg.ResourceAttributes
	if len(resourceAttributes) == 0 {
		return nil
	}

	// Make sure each Resource Attribute has exactly one of OID or ScalarOID or IndexedValuePrefix, and check that scalar and column OIDs end in the right digit
	for attrName, attrCfg := range resourceAttributes {

		hasOID := attrCfg.OID != ""
		hasScalarOID := attrCfg.ScalarOID != ""
		hasIVP := attrCfg.IndexedValuePrefix != ""

		switch {
		case hasOID:
			if hasScalarOID || hasIVP {
				combinedErr = errors.Join(combinedErr, fmt.Errorf(errMsgMultipleKeysSetOnResourceAttribute, attrName))
			}
			nums := strings.Split(attrCfg.OID, ".")
			if nums[len(nums)-1] == "0" {
				combinedErr = errors.Join(combinedErr, fmt.Errorf(errColumnOIDResourceAttributeEndsInZero, attrName, attrCfg.OID))
			}
		case hasScalarOID:
			if hasOID || hasIVP {
				combinedErr = errors.Join(combinedErr, fmt.Errorf(errMsgMultipleKeysSetOnResourceAttribute, attrName))
			}
			nums := strings.Split(attrCfg.ScalarOID, ".")
			if nums[len(nums)-1] != "0" {
				combinedErr = errors.Join(combinedErr, fmt.Errorf(errScalarOIDResourceAttributeEndsInNonzeroDigit, attrName, attrCfg.ScalarOID))
			}
		case hasIVP:
			if hasScalarOID || hasOID {
				combinedErr = errors.Join(combinedErr, fmt.Errorf(errMsgMultipleKeysSetOnResourceAttribute, attrName))
			}
		default:
			combinedErr = errors.Join(combinedErr, fmt.Errorf(errMsgResourceAttributeNoOIDOrScalarOIDOrPrefix, attrName))
		}
	}
	return combinedErr
}

// contains checks if string slice contains a string value
func contains(elements []string, value string) bool {
	for _, element := range elements {
		if value == element {
			return true
		}
	}
	return false
}
