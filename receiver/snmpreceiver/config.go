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
// TODO: FIX ALL COMMENTS
package snmpreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/snmpreceiver"

import (
	"go.opentelemetry.io/collector/receiver/scraperhelper"
)

// Config defines the configuration for the various elements of the receiver agent.
type Config struct {
	scraperhelper.ScraperControllerSettings `mapstructure:",squash"`

	// The SNMP target to send data to. Must be formatted as {host}:{port}
	Endpoint string `mapstructure:"endpoint"`

	// The version of SNMP. Valid options: 1, v2c, 3. Default: v2c
	Version string `mapstructure:"version"`

	// The SNMP community string to use. Default: public
	Community string `mapstructure:"community"`

	// Only valid for version “v3”
	User string `mapstructure:"user"`

	// Only valid for version “v3”
	// Valid options: “no_auth_no_priv”, “auth_no_priv”, “auth_priv”
	// Default: "no_auth_no_priv"
	SecurityLevel string `mapstructure:"security_level"`

	// Only valid for version “v3” and if “no_auth_no_priv” is not selected for security_level
	// Valid options: “md5”, “sha”, “sha224”, “sha256”, “sha384”, “sha512”
	AuthenticationType string `mapstructure:"authentication_type"`

	// Only valid if authentication_type specified
	AuthPassword string `mapstructure:"auth_password"`

	// Only valid for version “v3” and if “auth_priv” selected for security_level
	// Valid options: “des”, “aes”, “aes192”, “aes256”, “aes192c”, “aes256c”
	PrivacyType string `mapstructure:"privacy_type"`

	// Only valid if privacy_type specified
	PrivacyPassword string `mapstructure:"privacy_password"`

	// Resource attributes. The attribute names are used for the keys
	ResourceAttributes map[string]ResourceAttributeConfig `mapstructure:"resource_attributes"`

	// Attributes. The attribute names are used for the keys
	Attributes map[string]AttributeConfig `mapstructure:"attributes"`

	// Metrics. The metric names are used for the keys
	Metrics map[string]MetricConfig `mapstructure:"metrics"`
}

// ResourceAttributeConfig defines
type ResourceAttributeConfig struct {
	Description string `mapstructure:"description"`
	Type        string `mapstructure:"type"`
	// An optional value used to map returned values from a table column OID to a resource attribute values
	// This OID must be related to the same table that is used to match a different column OID a defined metric
	// In addition, the resource attribute that this OID belongs to must be assigned to the same metric
	// If OID is not used, then IndexedValuePrefix must be used instead
	OID string `mapstructure:"oid"` // Used to assign return values from one or more indexed OIDs to a resource attribute using
	// a new indexed OID within the same table as one or more metric indexed OIDs

	//# used to assign return values from one or more indexed OIDs to a simple resource attribute
	//#   when no alternate indexed OID attributes are used for resource attributes
	//#   Example:
	//#   tech.name:
	//#     indexed_value_prefix: probe
	//#
	//#   might result in a resource attributes of “tech.name: probe1”, “tech.name: probe2”, and
	//#  “tech.name: probe3” being assigned to one each of a metric’s values
	IndexedValuePrefix string `mapstructure:"indexed_value_prefix"` // required and valid if no oid field
}

type AttributeConfig struct {
	// optional, will match <attribute_name> if not included
	Value string `mapstructure:"value"`

	// used to uniquely label overlapping multiple scalar or indexed oid return values belonging to
	// a single metric
	//Enum Enum `mapstructure:"value"` // required and valid if no oid or indexed_value_prefix field

	Description string   `mapstructure:"description"`
	Enum        []string `mapstructure:"enum"`
	// used to uniquely label indexed oid return values belonging to a single metric using a new
	//  indexed OID within the same table
	OID                string `mapstructure:"oid"`                  // required and valid if no enum or indexed_value_prefix fields
	IndexedValuePrefix string `mapstructure:"indexed_value_prefix"` // required and valid if no oid field
}

type MetricConfig struct {
	Description string      `mapstructure:"description"`
	Unit        string      `mapstructure:"unit"`
	Gauge       GaugeMetric `mapstructure:"gauge"`
	Sum         SumMetric   `mapstructure:"sum"`
	// this would allow for a single metric which can handle multiple OID values, each with a
	// different set of assigned attribute key/values
	ScalarOIDs  []ScalarOID  `mapstructure:"scalar_oids"`
	IndexedOIDs []IndexedOID `mapstructure:"indexed_oids"`
}

type GaugeMetric struct {
	ValueType string `mapstructure:"value_type"`
}

type SumMetric struct {
	// Cumulative or delta
	Aggregation string `mapstructure:"aggregation"`
	// True or false
	Monotonic bool   `mapstructure:"monotonic"`
	ValueType string `mapstructure:"value_type"`
}

type ScalarOID struct {
	OID        string      `mapstructure:"oid"`
	Attributes []Attribute `mapstructure:"attributes"`
}

type IndexedOID struct {
	OID                string      `mapstructure:"oid"`
	ResourceAttributes []string    `mapstructure:"resource_attributes"`
	Attributes         []Attribute `mapstructure:"attributes"`
}

type Attribute struct {
	Name  string `mapstructure:"name"`
	Value string `mapstructure:"value"`
}

// TODO validate attributes, resource attributes, metrics configuration
// func (config Config) Validate() error {
// 	var err error

// 	_, _, err = net.SplitHostPort(config.Endpoint)
// 	if err != nil {
// 		return err
// 	}

// 	if config.CollectionInterval <= 0 {
// 		return errors.New("collection_interval must be a positive duration")
// 	}

// 	err = validateVersion(config)
// 	if err != nil {
// 		return err
// 	}

// 	//err = validateSNMPFields(config.SnmpFields)
// 	//if err != nil {
// 	//	return err
// 	//}
// 	//
// 	//err = validateSNMPTables(config.SnmpTables)
// 	//if err != nil {
// 	//	return err
// 	//}

// 	return nil
// }

// func validateVersion(config Config) error {
// 	snmpVersions := []string{"v1", "v2c", "v3"}
// 	var err error
// 	if matchStringInlist(snmpVersions, config.Version) {
// 		if config.Version == "v3" {
// 			err = validateAuthCredentials(config)
// 		}
// 	} else {
// 		err = fmt.Errorf("Invalid version, must be one of: v1,v2c,v3")
// 	}

// 	return err
// }

// // TODO add check for privacy password, user, auth_password
// func validateAuthCredentials(config Config) error {
// 	authenticationType := []string{"MD5", "SHA", "SHA224", "SHA256", "SHA384", "SHA512"}
// 	privacyType := []string{"DES", "AES", "AES192", "AES192C", "AES256", "AES256C"}
// 	securityLevel := []string{"no_auth_no_priv", "auth_no_priv", "auth_priv"}

// 	if !matchStringInlist(authenticationType, strings.ToUpper(config.AuthenticationType)) {
// 		return fmt.Errorf("Invalid auth_protocol, must be one of %v: ", authenticationType)
// 	} else if config.SecurityLevel == "no_auth_no_priv" {
// 		return fmt.Errorf("Cannot use authentication protocol with security level %v: ", config.SecurityLevel)
// 	}

// 	if !matchStringInlist(privacyType, strings.ToUpper(config.PrivacyType)) {
// 		return fmt.Errorf("Invalid privacy_protocol, must be one of %v: ", privacyType)
// 	} else if config.SecurityLevel != "auth_priv" {
// 		return fmt.Errorf("Privacy type is only valid for security level %v: ", "auth_priv")
// 	}

// 	if !matchStringInlist(securityLevel, strings.ToUpper(config.SecurityLevel)) {
// 		return fmt.Errorf("Invalid security level, must be one of %v: ", securityLevel)
// 	}

// 	return nil
// }

// func matchStringInlist(stringList []string, lookup string) bool {
// 	for _, listLookup := range stringList {
// 		if listLookup == lookup {
// 			return true
// 		}
// 	}

// 	return false
// }
