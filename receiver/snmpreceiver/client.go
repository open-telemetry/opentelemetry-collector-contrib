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
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/gosnmp/gosnmp"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/receiver/scrapererror"
	"go.uber.org/zap"
)

type oidDataType byte

// Trimmed down the larger list of gosnmp data types to make retrieving data a
// little more simple when using this client
const (
	notSupportedVal oidDataType = 0x00
	integerVal      oidDataType = 0x01 // value will be int64
	floatVal        oidDataType = 0x02 // value will be float64
	stringVal       oidDataType = 0x03 // value will be string
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
	GetScalarData(oids []string, processFn processFunc, scraperErrors *scrapererror.ScrapeErrors)
	// GetIndexedData retrieves SNMP indexed data from a list of passed in OIDS,
	// then the passed in function is performed on each piece of data
	GetIndexedData(oids []string, processFn processFunc, scraperErrors *scrapererror.ScrapeErrors)
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
// Relies on config being validated thoroughly
func newClient(cfg *Config, _ component.Host, _ component.TelemetrySettings, logger *zap.Logger) (client, error) {
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
	}

	// Checked in config
	snmpURL, _ := url.Parse(cfg.Endpoint)

	// Set goSNMP transport based on config
	lCaseScheme := strings.ToLower(snmpURL.Scheme)
	switch lCaseScheme {
	case "tcp", "tcp4", "tcp6", "udp", "udp4", "udp6":
		goSNMP.SetTransport(lCaseScheme)
	}

	// Checked in config that it exists
	portStr := snmpURL.Port()

	port, err := strconv.ParseUint(portStr, 10, 16)
	if err != nil {
		return nil, fmt.Errorf("failed to create goSNMP client: issue parsing port '%s'. %w", portStr, err)
	}
	goSNMP.SetPort(uint16(port))

	// Set goSNMP target based on config
	goSNMP.SetTarget(snmpURL.Hostname())

	if goSNMP.GetVersion() == gosnmp.Version3 {
		// Set goSNMP v3 configs
		setV3ClientConfigs(goSNMP, cfg)
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
func setV3ClientConfigs(client goSNMPWrapper, cfg *Config) {
	client.SetSecurityModel(gosnmp.UserSecurityModel)
	// Set goSNMP user based on config
	securityParams := &gosnmp.UsmSecurityParameters{
		UserName: cfg.User,
	}
	// Set goSNMP security level & auth/privacy details based on config
	switch strings.ToUpper(cfg.SecurityLevel) {
	case "NO_AUTH_NO_PRIV":
		client.SetMsgFlags(gosnmp.NoAuthNoPriv)
	case "AUTH_NO_PRIV":
		client.SetMsgFlags(gosnmp.AuthNoPriv)
		protocol := getAuthProtocol(cfg.AuthType)
		securityParams.AuthenticationProtocol = protocol
		securityParams.AuthenticationPassphrase = cfg.AuthPassword
	case "AUTH_PRIV":
		client.SetMsgFlags(gosnmp.AuthPriv)

		authProtocol := getAuthProtocol(cfg.AuthType)
		securityParams.AuthenticationProtocol = authProtocol
		securityParams.AuthenticationPassphrase = cfg.AuthPassword

		privProtocol := getPrivacyProtocol(cfg.PrivacyType)
		securityParams.PrivacyProtocol = privProtocol
		securityParams.PrivacyPassphrase = cfg.PrivacyPassword
	}
	client.SetSecurityParameters(securityParams)
}

// getAuthProtocol gets gosnmp auth protocol based on config auth type
func getAuthProtocol(authType string) gosnmp.SnmpV3AuthProtocol {
	var authProtocol gosnmp.SnmpV3AuthProtocol

	switch strings.ToUpper(authType) {
	case "MD5":
		authProtocol = gosnmp.MD5
	case "SHA":
		authProtocol = gosnmp.SHA
	case "SHA224":
		authProtocol = gosnmp.SHA224
	case "SHA256":
		authProtocol = gosnmp.SHA256
	case "SHA384":
		authProtocol = gosnmp.SHA384
	case "SHA512":
		authProtocol = gosnmp.SHA512
	}

	return authProtocol
}

// getPrivacyProtocol gets gosnmp privacy protocol based on config privacy type
func getPrivacyProtocol(privacyType string) gosnmp.SnmpV3PrivProtocol {
	var privacyProtocol gosnmp.SnmpV3PrivProtocol

	switch strings.ToUpper(privacyType) {
	case "DES":
		privacyProtocol = gosnmp.DES
	case "AES":
		privacyProtocol = gosnmp.AES
	case "AES192":
		privacyProtocol = gosnmp.AES192
	case "AES192C":
		privacyProtocol = gosnmp.AES192C
	case "AES256":
		privacyProtocol = gosnmp.AES256
	case "AES256C":
		privacyProtocol = gosnmp.AES256C
	}

	return privacyProtocol
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
func (c *snmpClient) GetScalarData(oids []string, processFn processFunc, scraperErrors *scrapererror.ScrapeErrors) {
	// Nothing to do if there are no OIDs
	if len(oids) == 0 {
		return
	}

	// Group OIDs into chunks based on the max amount allowed in a single SNMP GET
	chunkedOIDs := chunkArray(oids, c.client.GetMaxOids())

	// For each group of OIDs
	for _, oidChunk := range chunkedOIDs {
		// Note: Not implementing GetBulk as I don't think it would work correctly for the current design
		packets, err := c.client.Get(oidChunk)
		if err != nil {
			scraperErrors.AddPartial(len(oidChunk), fmt.Errorf("problem with getting scalar data: problem with SNMP GET for OIDs '%v': %w", oidChunk, err))
			// Prevent getting stuck in a failure where we can't recover
			if strings.Contains(err.Error(), "request timeout (after ") {
				if err = c.Close(); err != nil {
					c.logger.Warn("Problem with closing connection while trying to reset it", zap.Error(err))
				}
				if err = c.Connect(); err != nil {
					scraperErrors.AddPartial(len(oidChunk), fmt.Errorf("problem with getting scalar data: problem connecting while trying to reset connection: %w", err))
					return
				}
			}
			continue
		}

		// For each piece of data in a returned packet
		for _, data := range packets.Variables {
			// If there is no value, then ignore
			if data.Value == nil {
				scraperErrors.AddPartial(1, fmt.Errorf("problem with getting scalar data: data for OID '%s' not found", data.Name))
				continue
			}
			// Convert data into the more simplified data type
			clientSNMPData := c.convertSnmpPDUToSnmpData(data)
			// If the value type is not supported, then ignore
			if clientSNMPData.valueType == notSupportedVal {
				scraperErrors.AddPartial(1, fmt.Errorf("problem with getting scalar data: data for OID '%s' not a supported type", data.Name))
				continue
			}
			// Process the data
			if err := processFn(clientSNMPData); err != nil {
				scraperErrors.AddPartial(1, fmt.Errorf("problem with getting scalar data: problem with processing data for OID '%s': %w", data.Name, err))
				continue
			}
		}
	}
}

// GetIndexedData retrieves indexed metrics from passed in column OIDs. The returned data
// is then also passed into the provided function.
func (c *snmpClient) GetIndexedData(oids []string, processFn processFunc, scraperErrors *scrapererror.ScrapeErrors) {
	// Nothing to do if there are no OIDs
	if len(oids) == 0 {
		return
	}

	// For each column based OID
	for _, oid := range oids {
		// Because BulkWalk and Walk do not return errors if the given OID doesn't exist, we need to keep track of whether
		// BulkWalk called the walkFn or not. This will allow us to know if there was a problem with given OID
		walkFnCalled := false
		// Create a walkFunc which is a required argument for the gosnmp Walk functions
		walkFn := func(data gosnmp.SnmpPDU) error {
			walkFnCalled = true
			// If there is no value, then stop processing
			if data.Value == nil {
				scraperErrors.AddPartial(1, fmt.Errorf("problem with getting indexed data: data for OID '%s' not found", data.Name))
				return nil
			}
			// Convert data into the more simplified data type
			clientSNMPData := c.convertSnmpPDUToSnmpData(data)
			// Keep track of which column OID this data came from as well
			clientSNMPData.parentOID = oid
			// If the value type is not supported, then ignore
			if clientSNMPData.valueType == notSupportedVal {
				scraperErrors.AddPartial(1, fmt.Errorf("problem with getting indexed data: data for OID '%s' not a supported type", data.Name))
				return nil
			}

			// Process the data
			if err := processFn(clientSNMPData); err != nil {
				scraperErrors.AddPartial(1, fmt.Errorf("problem with getting indexed data: problem with processing data for OID '%s': %w", data.Name, err))
			}

			return nil
		}

		// Call the correct gosnmp Walk function based on SNMP version
		var err error
		if c.client.GetVersion() == gosnmp.Version1 {
			err = c.client.Walk(oid, walkFn)
		} else {
			err = c.client.BulkWalk(oid, walkFn)
		}
		if err != nil {
			scraperErrors.AddPartial(1, fmt.Errorf("problem with getting indexed data: problem with SNMP WALK for OID '%v': %w", oid, err))
			// Allows for quicker recovery rather than timing out for each WALK OID and waiting for the next GET to fix it
			if strings.Contains(err.Error(), "request timeout (after ") {
				if err = c.Close(); err != nil {
					c.logger.Warn("Problem with closing connection while trying to reset it", zap.Error(err))
				}
				if err = c.Connect(); err != nil {
					scraperErrors.AddPartial(len(oids), fmt.Errorf("problem with getting indexed data: problem connecting while trying to reset connection: %w", err))
					return
				}
			}
		} else if !walkFnCalled {
			scraperErrors.AddPartial(1, fmt.Errorf("problem with getting indexed data: problem with SNMP WALK for OID '%v': Could not find any data for given OID", oid))
		}
	}
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
	clientSNMPData := snmpData{
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
		value, err := c.toInt64(pdu.Name, pdu.Value)
		if err != nil {
			clientSNMPData.valueType = notSupportedVal
			clientSNMPData.value = value
			return clientSNMPData
		}

		clientSNMPData.valueType = integerVal
		clientSNMPData.value = value
		return clientSNMPData

	// String types
	case gosnmp.IPAddress:
		fallthrough
	case gosnmp.ObjectIdentifier:
		fallthrough
	case gosnmp.OctetString:
		clientSNMPData.valueType = stringVal
		clientSNMPData.value = toString(pdu.Value)
		return clientSNMPData

	// Float types
	case gosnmp.OpaqueFloat:
		fallthrough
	case gosnmp.OpaqueDouble:
		value, err := c.toFloat64(pdu.Name, pdu.Value)
		if err != nil {
			clientSNMPData.valueType = notSupportedVal
			clientSNMPData.value = value
			return clientSNMPData
		}

		clientSNMPData.valueType = floatVal
		clientSNMPData.value = value
		return clientSNMPData

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
		clientSNMPData.valueType = notSupportedVal
		clientSNMPData.value = pdu.Value
		return clientSNMPData
	}
}

// toInt64 converts SnmpPDU.Value to int64, or returns an error for
// non int-like types or a uint64.
//
// This is a convenience function to make working with SnmpPDU's easier - it
// reduces the need for type assertions. A int64 is convenient, as SNMP can
// return int32, uint32, and int64.
func (c snmpClient) toInt64(name string, value interface{}) (int64, error) {
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
		return 0, fmt.Errorf("incompatible type while converting OID '%s' data to int64", name)
	}

	return val, nil
}

// toFloat64 converts SnmpPDU.Value to float64, or returns an error for non
// float like types.
//
// This is a convenience function to make working with SnmpPDU's easier - it
// reduces the need for type assertions. A float64 is convenient, as SNMP can
// return float32 and float64.
func (c snmpClient) toFloat64(name string, value interface{}) (float64, error) {
	var val float64

	switch value := value.(type) {
	case float32:
		val = float64(value)
	case float64:
		val = value
	case string:
		// for testing and other apps - numbers may appear as strings
		var err error
		if val, err = strconv.ParseFloat(value, 64); err != nil {
			return 0, fmt.Errorf("problem converting OID '%s' data to float64: %w", name, err)
		}
	default:
		return 0, fmt.Errorf("incompatible type while converting OID '%s' data to float64", name)
	}

	return val, nil
}

// toString converts SnmpPDU.Value to string
//
// This is a convenience function to make working with SnmpPDU's easier - it
// reduces the need for type assertions.
func toString(value interface{}) string {
	var val string

	switch value := value.(type) {
	case []byte:
		val = string(value)
	case string:
		val = value
	default:
		val = fmt.Sprintf("%v", value)
	}

	return val
}
