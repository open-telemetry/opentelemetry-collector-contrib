package aerospikereceiver

import (
	"time"

	as "github.com/aerospike/aerospike-client-go/v5"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/aerospikereceiver/internal/model"
)

type defaultASClient struct {
	conn    *as.Connection
	timeout time.Duration
}

const (
	namespaceKey  = "namespace"
	namespacesKey = "namespaces"
)

// Aerospike is the interface that provides information about a given node
type Aerospike interface {
	// NamespaceInfo gets information about a specific namespace
	NamespaceInfo(namespace string) (*model.NamespaceInfo, error)
	// Info gets high-level information about the node/system.
	Info() (*model.NodeInfo, error)
	// Close closes the connection to the Aerospike node
	Close()
}

// newASClient creates a new defaultASClient connected to the given host and port
func newASClient(host string, port int, timeout time.Duration) (*defaultASClient, error) {
	policy := as.NewClientPolicy()
	policy.Timeout = timeout

	conn, err := as.NewConnection(policy, as.NewHost(host, port))
	if err != nil {
		return nil, err.Unwrap()
	}

	return &defaultASClient{
		conn:    conn,
		timeout: timeout,
	}, nil
}

func (c *defaultASClient) NamespaceInfo(namespace string) (*model.NamespaceInfo, error) {
	c.conn.SetTimeout(time.Now().Add(c.timeout), c.timeout)
	var response model.InfoResponse
	response, err := c.conn.RequestInfo(model.NamespaceKey(namespace))
	if err != nil {
		return nil, err
	}

	return model.ParseNamespaceInfo(response, namespace), nil
}

func (c *defaultASClient) Info() (*model.NodeInfo, error) {
	c.conn.SetTimeout(time.Now().Add(c.timeout), c.timeout)
	var response model.InfoResponse
	response, err := c.conn.RequestInfo("namespaces", "node", "statistics", "services")
	if err != nil {
		return nil, err.Unwrap()
	}

	return model.ParseInfo(response), nil
}

func (c *defaultASClient) Close() {
	c.conn.Close()
}
