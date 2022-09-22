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

// custom errors
var (
	errNoGetOIDs        = errors.New(`all GET OIDs requests have failed`)
	errNoProcessGetOIDs = errors.New(`all attempts to process GET OIDs have failed`)
	errNoWalkOIDs       = errors.New(`all WALK OIDs requests have failed`)
)

type oidDataType byte

const (
	NotSupported oidDataType = 0x00
	Integer      oidDataType = 0x01
	UInteger     oidDataType = 0x02
	Float        oidDataType = 0x03
	String       oidDataType = 0x04
)

type snmpData struct {
	parentOID string // optional
	oid       string
	value     interface{}
	valueType oidDataType
}

type processFunc func(data snmpData) error

// client is used for retrieving data about a Big-IP environment
type client interface {
	GetScalarData(oids []string, processFn processFunc) error
	GetIndexedData(oids []string, processFn processFunc) error
	Connect() error
}

// snmpClient implements the client interface and retrieves data through the iControl REST API
type snmpClient struct {
	client *gosnmp.GoSNMP
	logger *zap.Logger
}

// Verify snmpClient implements client interface
var _ client = (*snmpClient)(nil)

// newClient creates an initialized client (but with no token)
func newClient(cfg *Config, host component.Host, settings component.TelemetrySettings, logger *zap.Logger) (client, error) {
	var snmpVersion gosnmp.SnmpVersion
	switch cfg.Version {
	case "v3":
		snmpVersion = gosnmp.Version3
	case "v2c":
		snmpVersion = gosnmp.Version2c
	case "v1":
		snmpVersion = gosnmp.Version1
	default:
		return nil, fmt.Errorf("invalid version")
	}

	endpoint := cfg.Endpoint
	if !strings.Contains(endpoint, "://") {
		endpoint = "udp://" + endpoint
	}

	snmpUrl, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}

	var scheme string
	switch snmpUrl.Scheme {
	case "tcp", "tcp4", "tcp6", "udp", "udp4", "udp6":
		scheme = snmpUrl.Scheme
	default:
		return nil, fmt.Errorf("unsupported scheme: %v", snmpUrl.Scheme)
	}

	portStr := snmpUrl.Port()
	if portStr == "" {
		portStr = "161"
	}

	port, err := strconv.ParseUint(portStr, 10, 16)
	if err != nil {
		return nil, fmt.Errorf("parsing port: %w", err)
	}

	goSNMP := &gosnmp.GoSNMP{
		Transport: scheme,
		Target:    snmpUrl.Hostname(),
		Port:      uint16(port),
		Version:   snmpVersion,
		Timeout:   cfg.CollectionInterval,
		MaxOids:   gosnmp.Default.MaxOids,
	}

	if snmpVersion == gosnmp.Version3 {
		if err != nil {
			return nil, err
		}
		goSNMP.SecurityModel = gosnmp.UserSecurityModel
		securityParams := &gosnmp.UsmSecurityParameters{
			UserName: cfg.User,
		}
		switch strings.ToUpper(cfg.SecurityLevel) {
		case "NO_AUTH_NO_PRIV":
			goSNMP.MsgFlags = gosnmp.NoAuthNoPriv
		case "AUTH_NO_PRIV":
			goSNMP.MsgFlags = gosnmp.AuthNoPriv
			securityParams.AuthenticationProtocol = getAuthProtocol(cfg.AuthType)
			securityParams.AuthenticationPassphrase = cfg.AuthPassword
		case "AUTH_PRIV":
			goSNMP.MsgFlags = gosnmp.AuthPriv
			securityParams.AuthenticationProtocol = getAuthProtocol(cfg.AuthType)
			securityParams.AuthenticationPassphrase = cfg.AuthPassword
			securityParams.PrivacyProtocol = getPrivacyProtocol(cfg.PrivacyType)
			securityParams.PrivacyPassphrase = cfg.PrivacyPassword
		default:
			goSNMP.MsgFlags = gosnmp.NoAuthNoPriv
		}
		goSNMP.SecurityParameters = securityParams
	} else {
		goSNMP.Community = cfg.Community
	}

	return &snmpClient{
		client: goSNMP,
		logger: logger,
	}, nil
}

func getAuthProtocol(authType string) gosnmp.SnmpV3AuthProtocol {
	switch strings.ToUpper(authType) {
	case "MD5":
		return gosnmp.MD5
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

func getPrivacyProtocol(privacyType string) gosnmp.SnmpV3PrivProtocol {
	switch strings.ToUpper(privacyType) {
	case "DES":
		return gosnmp.DES
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

func (c *snmpClient) Connect() error {
	return c.client.Connect()
}

// GetScalarData expects OIDs to end with ".0" for scalar metrics
// We can just say this is required in the config for now
func (c *snmpClient) GetScalarData(oids []string, processFn processFunc) error {
	// Make sure no single call has over our max limit of input OIDs
	chunkedOIDs := chunkArray(oids, c.client.MaxOids)
	getOIDsSuccess := false
	processOIDSuccess := false

	for _, oids := range chunkedOIDs {
		// Not sure if GetBulk would work right based on how the receiver and configuration are currently working
		packets, err := c.client.Get(oids)
		if err != nil {
			c.logger.Warn("Problem with GET oids", zap.Error(err))
			continue
		} else {
			getOIDsSuccess = true
		}

		for _, data := range packets.Variables {
			if data.Value == nil {
				c.logger.Warn(fmt.Sprintf("Data for OID: %s not found", data.Name))
				continue
			}
			snmpData := c.convertSnmpPDUToSnmpData(data)
			if err := processFn(snmpData); err != nil {
				c.logger.Warn(fmt.Sprintf("Problem with processing data for OID: %s", snmpData.oid), zap.Error(err))
			} else {
				processOIDSuccess = true
			}
		}
	}

	if !getOIDsSuccess {
		return errNoGetOIDs
	}

	if !processOIDSuccess {
		return errNoProcessGetOIDs
	}

	return nil
}

func (c *snmpClient) GetIndexedData(oids []string, processFn processFunc) error {
	walkOIDsSuccess := false

	for _, oid := range oids {
		walkFn := func(data gosnmp.SnmpPDU) error {
			snmpData := c.convertSnmpPDUToSnmpData(data)
			snmpData.parentOID = oid

			return processFn(snmpData)
		}

		var err error
		if c.client.Version == gosnmp.Version1 {
			err = c.client.Walk(oid, walkFn)
		} else {
			err = c.client.BulkWalk(oid, walkFn)
		}
		if err != nil {
			c.logger.Warn("Problem with WALK oids", zap.Error(err))
			continue
		} else {
			walkOIDsSuccess = true
		}
	}

	if !walkOIDsSuccess {
		return errNoWalkOIDs
	}

	return nil
}

func (c *snmpClient) convertSnmpPDUToSnmpData(pdu gosnmp.SnmpPDU) snmpData {
	snmpData := snmpData{
		oid: pdu.Name,
	}

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
