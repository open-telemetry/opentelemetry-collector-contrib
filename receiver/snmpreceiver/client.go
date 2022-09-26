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

package snmpreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/snmpreceiver"

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"

	"github.com/gosnmp/gosnmp"
)

// Custom errors
var (
	errNoGetOIDs        = errors.New(`all GET OIDs requests have failed`)
	errNoProcessGetOIDs = errors.New(`all attempts to process GET OIDs have failed`)
	errNoWalkOIDs       = errors.New(`all WALK OIDs requests have failed`)
)

// Trimmed down list of SNMP data types to make things a little more simple for the scraper
type oidDataType byte

const (
	NotSupported oidDataType = 0x00
	Integer      oidDataType = 0x01 // value will be int64
	Float        oidDataType = 0x02 // value will be float64
	String       oidDataType = 0x03 // value will be string
)

// snmpData used for processFunc and is a simpler version of gosnmp.SnmpPDU
type snmpData struct {
	parentOID string // optional
	oid       string
	value     interface{}
	valueType oidDataType
}

// processFunc our own function type to better control what data can be passed in
type processFunc func(data snmpData) error

// client is used for retrieving data from a SNMP environment
type client interface {
	// GetScalarData retrieves SNMP scalar data from a list of passed in OIDS,
	// then the passed in function is performed on each piece of data
	GetScalarData(oids []string, processFn processFunc) error
	// GetIndexedData retrieves SNMP indexed data from a list of passed in OIDS,
	// then the passed in function is performed on each piece of data
	GetIndexedData(oids []string, processFn processFunc) error
	// Connect makes a connection to the SNMP host
	Connect() error
	// Close closes a connection to the SNMP host
	Close() error
}

// snmpClient implements the client interface and retrieves data through SNMP
type snmpClient struct {
	client goSNMPWrapper
	logger *zap.Logger
}

// Verify snmpClient implements client interface
var _ client = (*snmpClient)(nil)

// newClient creates an initialized client
func newClient(cfg *Config, host component.Host, settings component.TelemetrySettings, logger *zap.Logger) (client, error) {
	// Create goSNMP client
	goSNMP := newGoSNMPWrapper()
	goSNMP.SetTimeout(cfg.CollectionInterval)

	// Set goSNMP version based on config
	switch cfg.Version {
	case "v3":
		goSNMP.SetVersion(gosnmp.Version3)
	case "v2c":
		goSNMP.SetVersion(gosnmp.Version2c)
	case "v1":
		goSNMP.SetVersion(gosnmp.Version1)
	default:
		return nil, errors.New("failed to create goSNMP client: invalid version")
	}

	// Verify config endpoint is good
	endpoint := cfg.Endpoint
	if !strings.Contains(endpoint, "://") {
		endpoint = "udp://" + endpoint
	}

	snmpUrl, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to create goSNMP client: invalid endpoint. %w", err)
	}

	// Set goSNMP transport based on config
	switch snmpUrl.Scheme {
	case "tcp", "tcp4", "tcp6", "udp", "udp4", "udp6":
		goSNMP.SetTransport(snmpUrl.Scheme)
	default:
		return nil, fmt.Errorf("failed to create goSNMP client: unsupported scheme '%s'", snmpUrl.Scheme)
	}

	// Set goSNMP port based on config
	portStr := snmpUrl.Port()
	if portStr == "" {
		portStr = "161"
	}

	port, err := strconv.ParseUint(portStr, 10, 16)
	if err != nil {
		return nil, fmt.Errorf("failed to create goSNMP client: issue parsing port '%s'. %w", portStr, err)
	}
	goSNMP.SetPort(uint16(port))

	// Set goSNMP target based on config
	goSNMP.SetTarget(snmpUrl.Hostname())

	if goSNMP.GetVersion() == gosnmp.Version3 {
		// Set goSNMP v3 configs
		if err := setV3ClientConfigs(&goSNMP, cfg); err != nil {
			return nil, fmt.Errorf("failed to create goSNMP client: %w", err)
		}
	} else {
		// Set goSNMP community string
		goSNMP.SetCommunity(cfg.Community)
	}

	// return client
	return &snmpClient{
		client: goSNMP,
		logger: logger,
	}, nil
}

// setV3ClientConfigs sets SNMP v3 related configurations on gosnmp client based on config
func setV3ClientConfigs(client *goSNMPWrapper, cfg *Config) error {
	(*client).SetSecurityModel(gosnmp.UserSecurityModel)
	// Set goSNMP user based on config
	securityParams := &gosnmp.UsmSecurityParameters{
		UserName: cfg.User,
	}
	// Set goSNMP security level & auth/privacy details based on config
	switch strings.ToUpper(cfg.SecurityLevel) {
	case "NO_AUTH_NO_PRIV":
		(*client).SetMsgFlags(gosnmp.NoAuthNoPriv)
	case "AUTH_NO_PRIV":
		(*client).SetMsgFlags(gosnmp.AuthNoPriv)
		protocol, err := getAuthProtocol(cfg.AuthType)
		if err != nil {
			return err
		}
		securityParams.AuthenticationProtocol = protocol
		securityParams.AuthenticationPassphrase = cfg.AuthPassword
	case "AUTH_PRIV":
		(*client).SetMsgFlags(gosnmp.AuthPriv)
		authProtocol, err := getAuthProtocol(cfg.AuthType)
		if err != nil {
			return err
		}
		securityParams.AuthenticationProtocol = authProtocol
		securityParams.AuthenticationPassphrase = cfg.AuthPassword

		privProtocol, err := getPrivacyProtocol(cfg.PrivacyType)
		if err != nil {
			return err
		}
		securityParams.PrivacyProtocol = privProtocol
		securityParams.PrivacyPassphrase = cfg.PrivacyPassword
	default:
		return fmt.Errorf("invalid security protocol '%s'", cfg.SecurityLevel)
	}
	(*client).SetSecurityParameters(securityParams)

	return nil
}

// getAuthProtocol gets gosnmp auth protocol based on config auth type
func getAuthProtocol(authType string) (gosnmp.SnmpV3AuthProtocol, error) {
	switch strings.ToUpper(authType) {
	case "MD5":
		return gosnmp.MD5, nil
	case "SHA":
		return gosnmp.SHA, nil
	case "SHA224":
		return gosnmp.SHA224, nil
	case "SHA256":
		return gosnmp.SHA256, nil
	case "SHA384":
		return gosnmp.SHA384, nil
	case "SHA512":
		return gosnmp.SHA512, nil
	default:
		return gosnmp.MD5, fmt.Errorf("invalid auth protocol '%s'", authType)
	}
}

// getPrivacyProtocol gets gosnmp privacy protocol based on config privacy type
func getPrivacyProtocol(privacyType string) (gosnmp.SnmpV3PrivProtocol, error) {
	switch strings.ToUpper(privacyType) {
	case "DES":
		return gosnmp.DES, nil
	case "AES":
		return gosnmp.AES, nil
	case "AES192":
		return gosnmp.AES192, nil
	case "AES192C":
		return gosnmp.AES192C, nil
	case "AES256":
		return gosnmp.AES256, nil
	case "AES256C":
		return gosnmp.AES256C, nil
	default:
		return gosnmp.DES, fmt.Errorf("invalid privacy protocol '%s'", privacyType)
	}
}

// Connect uses the goSNMP client's connect
func (c *snmpClient) Connect() error {
	return c.client.Connect()
}

// Close uses the goSNMP client's close
func (c *snmpClient) Close() error {
	return c.client.Close()
}

// GetScalarData retrieves scalar metrics from passed in scalar OIDs. The returned data
// is then also passed into the provided function.
// Note: These OIDs must all end in ".0" for the SNMP GET to work correctly
func (c *snmpClient) GetScalarData(oids []string, processFn processFunc) error {
	// Nothing to do if there are no OIDs
	if len(oids) == 0 {
		return nil
	}

	// Group OIDs into chunks based on the max amount allowed in a single SNMP GET
	chunkedOIDs := chunkArray(oids, c.client.GetMaxOids())
	getOIDsSuccess := false
	processOIDSuccess := false

	// For each group of OIDs
	for _, oids := range chunkedOIDs {
		// Note: Not implementing GetBulk as I don't think it would work correctly for the current design
		packets, err := c.client.Get(oids)
		if err != nil {
			c.logger.Warn("Problem with GET OIDs", zap.Error(err))
			// Prevent getting stuck in a failure where we can't recover
			if strings.Contains(err.Error(), "request timeout (after ") {
				c.Close()
				c.Connect()
			}
			continue
		}
		// Keep track of if at least one GET successfully returned data
		getOIDsSuccess = true

		// For each piece of data in a returned packet
		for _, data := range packets.Variables {
			// If there is no value, then ignore
			if data.Value == nil {
				c.logger.Warn(fmt.Sprintf("Data for OID: %s not found", data.Name))
				continue
			}
			// Convert data into the more simplified data type
			snmpData := c.convertSnmpPDUToSnmpData(data)
			// If the value type is not supported, then ignore
			if snmpData.valueType == NotSupported {
				c.logger.Warn(fmt.Sprintf("Data for OID: %s not a supported type", data.Name))
				continue
			}
			// Process the data
			if err := processFn(snmpData); err != nil {
				c.logger.Warn(fmt.Sprintf("Problem with processing data for OID: %s", snmpData.oid), zap.Error(err))
				continue
			}
			// Keep track of if at least one set of GET data was successfully processed
			processOIDSuccess = true
		}
	}

	// Return specialized error messages if we failed to retrieve or process any data here.
	if !getOIDsSuccess {
		return errNoGetOIDs
	}

	if !processOIDSuccess {
		return errNoProcessGetOIDs
	}

	return nil
}

// GetIndexedData retrieves indexed metrics from passed in column OIDs. The returned data
// is then also passed into the provided function.
func (c *snmpClient) GetIndexedData(oids []string, processFn processFunc) error {
	// Nothing to do if there are no OIDs
	if len(oids) == 0 {
		return nil
	}

	walkOIDsSuccess := false

	// For each column based OID
	for _, oid := range oids {
		// Create a walkFunc which is a required argument for the gosnmp Walk functions
		walkFn := func(data gosnmp.SnmpPDU) error {
			// If there is no value, then stop processing
			if data.Value == nil {
				return fmt.Errorf("Data for OID: %s not found", data.Name)
			}
			// Convert data into the more simplified data type
			snmpData := c.convertSnmpPDUToSnmpData(data)
			// Keep track of which column OID this data came from as well
			snmpData.parentOID = oid
			// If the value type is not supported, then ignore
			if snmpData.valueType == NotSupported {
				return fmt.Errorf("Data for OID: %s not a supported type", data.Name)
			}

			// Process the data with the provided function
			return processFn(snmpData)
		}

		// Call the correct gosnmp Walk function based on SNMP version
		var err error
		if c.client.GetVersion() == gosnmp.Version1 {
			err = c.client.Walk(oid, walkFn)
		} else {
			err = c.client.BulkWalk(oid, walkFn)
		}
		if err != nil {
			c.logger.Warn("Problem with WALK OID", zap.Error(err))
			// Allows for quicker recovery rather than timing out for each WALK OID and waiting for the next GET to fix it
			if strings.Contains(err.Error(), "request timeout (after ") {
				c.Close()
				c.Connect()
			}
			continue
		} else {
			// Keep track of if at least one set of GET data was successfully processed
			walkOIDsSuccess = true
		}
	}

	// Return specialized error messages if we failed to retrieve or process any data here.
	if !walkOIDsSuccess {
		return errNoWalkOIDs
	}

	return nil
}

// chunkArray takes an initial array and splits it into a number of smaller
// arrays of a given size.
func chunkArray(initArray []string, chunkSize int) [][]string {
	var chunkedArrays [][]string

	for i := 0; i < len(initArray); i += chunkSize {
		end := i + chunkSize

		if end > len(initArray) {
			end = len(initArray)
		}

		chunkedArrays = append(chunkedArrays, initArray[i:end])
	}
	return chunkedArrays
}

// convertSnmpPDUToSnmpData takes a piece of SnmpPDU data and converts it to the
// client's snmpData type.
func (c *snmpClient) convertSnmpPDUToSnmpData(pdu gosnmp.SnmpPDU) snmpData {
	snmpData := snmpData{
		oid: pdu.Name,
	}

	// Condense gosnmp data types to our client's simplified data types
	switch pdu.Type {
	// Integer types
	case gosnmp.Counter32:
		fallthrough
	case gosnmp.Gauge32:
		fallthrough
	case gosnmp.Uinteger32:
		fallthrough
	case gosnmp.TimeTicks:
		fallthrough
	case gosnmp.Integer:
		snmpData.valueType = Integer
		snmpData.value = c.toInt64(pdu.Name, pdu.Value)
		return snmpData

	// String types
	case gosnmp.IPAddress:
		fallthrough
	case gosnmp.ObjectIdentifier:
		fallthrough
	case gosnmp.OctetString:
		snmpData.valueType = String
		snmpData.value = toString(pdu.Value)
		return snmpData

	// Float types
	case gosnmp.OpaqueFloat:
		fallthrough
	case gosnmp.OpaqueDouble:
		snmpData.valueType = Float
		snmpData.value = c.toFloat64(pdu.Name, pdu.Value)
		return snmpData

	// Not supported types either because gosnmp doesn't support them
	// or they are a type that doesn't translate well to OTEL
	case gosnmp.UnknownType:
		fallthrough
	case gosnmp.Counter64:
		fallthrough
	case gosnmp.NsapAddress:
		fallthrough
	case gosnmp.ObjectDescription:
		fallthrough
	case gosnmp.BitString:
		fallthrough
	case gosnmp.NoSuchObject:
		fallthrough
	case gosnmp.NoSuchInstance:
		fallthrough
	case gosnmp.EndOfMibView:
		fallthrough
	case gosnmp.Opaque:
		fallthrough
	case gosnmp.Null:
		fallthrough
	case gosnmp.Boolean:
		fallthrough
	default:
		snmpData.valueType = NotSupported
		snmpData.value = pdu.Value
		return snmpData
	}
}

// toInt64 converts SnmpPDU.Value to int64, or returns a zero int64 for
// non int-like types (eg strings) or a uint64.
//
// This is a convenience function to make working with SnmpPDU's easier - it
// reduces the need for type assertions. A int64 is convenient, as SNMP can
// return int32, uint32, and int64.
func (c snmpClient) toInt64(name string, value interface{}) int64 {
	var val int64

	switch value := value.(type) { // shadow
	case int:
		val = int64(value)
	case int8:
		val = int64(value)
	case int16:
		val = int64(value)
	case int32:
		val = int64(value)
	case int64:
		val = value
	case uint:
		val = int64(value)
	case uint8:
		val = int64(value)
	case uint16:
		val = int64(value)
	case uint32:
		val = int64(value)
	default:
		c.logger.Warn(fmt.Sprintf("Unexpected type while converting OID: %s data to int64. Returning 0", name))
		val = 0
	}

	return val
}

// toFloat64 converts SnmpPDU.Value to float64, or returns a zero float64 for
// non float-like types (eg strings).
//
// This is a convenience function to make working with SnmpPDU's easier - it
// reduces the need for type assertions. A float64 is convenient, as SNMP can
// return float32 and float64.
func (c snmpClient) toFloat64(name string, value interface{}) float64 {
	var val float64

	switch value := value.(type) { // shadow
	case float32:
		val = float64(value)
	case float64:
		val = value
	case string:
		// for testing and other apps - numbers may appear as strings
		var err error
		if val, err = strconv.ParseFloat(value, 64); err != nil {
			c.logger.Warn(fmt.Sprintf("Problem converting OID: %s data to float64. Returning 0", name), zap.Error(err))
			val = 0
		}
	default:
		c.logger.Warn(fmt.Sprintf("Unexpected type while converting OID: %s data to float64. Returning 0", name))
		val = 0
	}

	return val
}

// toString converts SnmpPDU.Value to string
//
// This is a convenience function to make working with SnmpPDU's easier - it
// reduces the need for type assertions.
func toString(value interface{}) string {
	var val string

	switch value := value.(type) { // shadow
	case []byte:
		val = string(value)
	case string:
		val = value
	default:
		val = fmt.Sprintf("%v", value)
	}

	return val
}
