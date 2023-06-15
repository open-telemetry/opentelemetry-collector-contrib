// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package snmpreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/snmpreceiver"

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gosnmp/gosnmp"
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

// SNMPData used for processFunc and is a simpler version of gosnmp.SnmpPDU
type SNMPData struct {
	columnOID string // optional
	oid       string
	value     interface{}
	valueType oidDataType
}

// client is used for retrieving data from a SNMP environment
type client interface {
	// GetScalarData retrieves SNMP scalar data from a list of passed in OIDS,
	// then returns the retrieved data
	GetScalarData(oids []string, scraperErrors *scrapererror.ScrapeErrors) []SNMPData
	// GetIndexedData retrieves SNMP indexed data from a list of passed in OIDS,
	// then returns the retrieved data
	GetIndexedData(oids []string, scraperErrors *scrapererror.ScrapeErrors) []SNMPData
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
func newClient(cfg *Config, logger *zap.Logger) (client, error) {
	// Create goSNMP client
	goSNMP := newGoSNMPWrapper()
	goSNMP.SetTimeout(5 * time.Second)

	// Set goSNMP version based on config
	switch cfg.Version {
	case "v3":
		goSNMP.SetVersion(gosnmp.Version3)
	case "v1":
		goSNMP.SetVersion(gosnmp.Version1)
	default:
		goSNMP.SetVersion(gosnmp.Version2c)
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
	default:
		client.SetMsgFlags(gosnmp.NoAuthNoPriv)
	}
	client.SetSecurityParameters(securityParams)
}

// getAuthProtocol gets gosnmp auth protocol based on config auth type
func getAuthProtocol(authType string) gosnmp.SnmpV3AuthProtocol {
	switch strings.ToUpper(authType) {
	case "SHA":
		return gosnmp.SHA
	case "SHA224":
		return gosnmp.SHA224
	case "SHA256":
		return gosnmp.SHA256
	case "SHA384":
		return gosnmp.SHA384
	case "SHA512":
		return gosnmp.SHA512
	default:
		return gosnmp.MD5
	}
}

// getPrivacyProtocol gets gosnmp privacy protocol based on config privacy type
func getPrivacyProtocol(privacyType string) gosnmp.SnmpV3PrivProtocol {
	switch strings.ToUpper(privacyType) {
	case "AES":
		return gosnmp.AES
	case "AES192":
		return gosnmp.AES192
	case "AES192C":
		return gosnmp.AES192C
	case "AES256":
		return gosnmp.AES256
	case "AES256C":
		return gosnmp.AES256C
	default:
		return gosnmp.DES
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

// GetScalarData retrieves and returns scalar data from passed in scalar OIDs.
// Note: These OIDs must all end in ".0" for the SNMP GET to work correctly
func (c *snmpClient) GetScalarData(oids []string, scraperErrors *scrapererror.ScrapeErrors) []SNMPData {
	scalarData := []SNMPData{}

	// Nothing to do if there are no OIDs
	if len(oids) == 0 {
		return scalarData
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
					return scalarData
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

			// Add the data to be returned
			scalarData = append(scalarData, clientSNMPData)
		}
	}

	return scalarData
}

// GetIndexedData retrieves indexed metrics from passed in column OIDs. The returned data
// is then also passed into the provided function.
func (c *snmpClient) GetIndexedData(oids []string, scraperErrors *scrapererror.ScrapeErrors) []SNMPData {
	indexedData := []SNMPData{}

	// Nothing to do if there are no OIDs
	if len(oids) == 0 {
		return indexedData
	}

	// For each column based OID
	for _, oid := range oids {
		// Call the correct gosnmp Walk function based on SNMP version
		var err error
		var snmpPDUs []gosnmp.SnmpPDU
		if c.client.GetVersion() == gosnmp.Version1 {
			snmpPDUs, err = c.client.WalkAll(oid)
		} else {
			snmpPDUs, err = c.client.BulkWalkAll(oid)
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
					return indexedData
				}
			}
		}

		for _, snmpPDU := range snmpPDUs {
			// If there is no value, then stop processing
			if snmpPDU.Value == nil {
				scraperErrors.AddPartial(1, fmt.Errorf("problem with getting indexed data: data for OID '%s' not found", snmpPDU.Name))
				continue
			}
			// Convert data into the more simplified data type
			clientSNMPData := c.convertSnmpPDUToSnmpData(snmpPDU)
			// Keep track of which column OID this data came from as well
			clientSNMPData.columnOID = oid
			// If the value type is not supported, then ignore
			if clientSNMPData.valueType == notSupportedVal {
				scraperErrors.AddPartial(1, fmt.Errorf("problem with getting indexed data: data for OID '%s' not a supported type", snmpPDU.Name))
				continue
			}

			// Add the data to be returned
			indexedData = append(indexedData, clientSNMPData)
		}
	}

	return indexedData
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
// client's SNMPData type.
func (c *snmpClient) convertSnmpPDUToSnmpData(pdu gosnmp.SnmpPDU) SNMPData {
	clientSNMPData := SNMPData{
		oid: pdu.Name,
	}

	// Condense gosnmp data types to our client's simplified data types
	switch pdu.Type { // nolint:exhaustive
	// Integer types
	case gosnmp.Counter32, gosnmp.Gauge32, gosnmp.Uinteger32, gosnmp.TimeTicks, gosnmp.Integer:
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
	case gosnmp.IPAddress, gosnmp.ObjectIdentifier, gosnmp.OctetString:
		clientSNMPData.valueType = stringVal
		clientSNMPData.value = toString(pdu.Value)
		return clientSNMPData

	// Float types
	case gosnmp.OpaqueFloat, gosnmp.OpaqueDouble:
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
	switch value := value.(type) { // shadow
	case uint:
		return int64(value), nil
	case int:
		return int64(value), nil
	case int8:
		return int64(value), nil
	case int16:
		return int64(value), nil
	case int32:
		return int64(value), nil
	case int64:
		return value, nil
	case uint8:
		return int64(value), nil
	case uint16:
		return int64(value), nil
	case uint32:
		return int64(value), nil
	default:
		return 0, fmt.Errorf("incompatible type while converting OID '%s' data to int64", name)
	}
}

// toFloat64 converts SnmpPDU.Value to float64, or returns an error for non
// float like types.
//
// This is a convenience function to make working with SnmpPDU's easier - it
// reduces the need for type assertions. A float64 is convenient, as SNMP can
// return float32 and float64.
func (c snmpClient) toFloat64(name string, value interface{}) (float64, error) {
	switch value := value.(type) { // shadow
	case float32:
		return float64(value), nil
	case float64:
		return value, nil
	case string:
		// for testing and other apps - numbers may appear as strings
		val, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return 0, fmt.Errorf("problem converting OID '%s' data to float64: %w", name, err)
		}

		return val, nil
	default:
		return 0, fmt.Errorf("incompatible type while converting OID '%s' data to float64", name)
	}
}

// toString converts SnmpPDU.Value to string
//
// This is a convenience function to make working with SnmpPDU's easier - it
// reduces the need for type assertions.
func toString(value interface{}) string {
	switch value := value.(type) { // shadow
	case []byte:
		return string(value)
	case string:
		return value
	default:
		return fmt.Sprintf("%v", value)
	}
}
