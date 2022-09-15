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

type snmpData struct {
	parentOID string // optional
	oid       string
	value     interface{}
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
	endpoint := cfg.Endpoint
	if !strings.Contains(endpoint, "://") {
		endpoint = "udp://" + endpoint
	}

	snmpUrl, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}

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

	// Only allow udp{4,6} and tcp{4,6}.
	// Allowing ip{4,6} does not make sense as specifying a port
	// requires the specification of a protocol.
	// gosnmp does not handle these errors well, which is why
	// they can result in cryptic errors by net.Dial.
	var transport string
	switch snmpUrl.Scheme {
	case "tcp", "tcp4", "tcp6", "udp", "udp4", "udp6":
		transport = snmpUrl.Scheme
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
		Transport: transport,
		Target:    snmpUrl.Hostname(),
		Port:      uint16(port),
		Community: cfg.Community,
		Version:   snmpVersion,
		Timeout:   cfg.CollectionInterval,
		MaxOids:   gosnmp.Default.MaxOids,
	}

	return &snmpClient{
		client: goSNMP,
		logger: logger,
	}, nil
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
		// Having issues trying to get BulkGet to work right
		packets, err := c.client.Get(oids)
		if err != nil {
			c.logger.Warn("Problem with GET oids", zap.Error(err))
			continue
		} else {
			getOIDsSuccess = true
		}

		for _, data := range packets.Variables {
			snmpData := snmpData{
				oid:   data.Name,
				value: data.Value,
			}
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
			snmpData := snmpData{
				parentOID: oid,
				oid:       data.Name,
				value:     data.Value,
			}

			return processFn(snmpData)
		}
		// TODO: Determine if we can use BulkWalk
		err := c.client.Walk(oid, walkFn)
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
