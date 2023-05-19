// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cluster // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/aerospikereceiver/cluster"

import (
	"testing"
	"time"

	as "github.com/aerospike/aerospike-client-go/v6"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/aerospikereceiver/cluster/mocks"
)

func TestNode_NewConnNode(t *testing.T) {
	conn := mocks.NewAsconn(t)
	nodeName := "BB990C28F270008"

	cPolicy := as.NewClientPolicy()
	host := as.NewHost("testip", 3000)
	authEnabled := true

	conn.On("SetTimeout", time.Time{}, cPolicy.Timeout).Return(nil)
	conn.On("RequestInfo", "node").Return(map[string]string{"node": nodeName}, nil)
	conn.On("Login", cPolicy).Return(nil)

	expectedConnNode := &connNode{
		conn:   conn,
		policy: cPolicy,
		name:   nodeName,
	}

	connFactoryPos := newMockConnFactoryFunc(t)
	connFactoryPos.On("Execute", cPolicy, host).Return(conn, nil)

	factoryFuncPos := func(policy *as.ClientPolicy, host *as.Host) (asconn, as.Error) {
		return connFactoryPos.Execute(policy, host)
	}

	testNode, err := _newConnNode(cPolicy, host, authEnabled, factoryFuncPos)
	require.NoError(t, err)
	connFactoryPos.AssertExpectations(t)
	conn.AssertExpectations(t)
	testConnNode := testNode.(*connNode)
	require.Equal(t, expectedConnNode, testConnNode)

	connFactoryNegHost := newMockConnFactoryFunc(t)
	connFactoryNegHost.On("Execute", cPolicy, host).Return(nil, as.ErrTimeout)

	factoryFuncNegHost := func(policy *as.ClientPolicy, host *as.Host) (asconn, as.Error) {
		return connFactoryNegHost.Execute(policy, host)
	}

	_, err = _newConnNode(cPolicy, host, authEnabled, factoryFuncNegHost)
	connFactoryNegHost.AssertExpectations(t)
	require.ErrorContains(t, err, "ResultCode: TIMEOUT")

	connBadLogin := mocks.NewAsconn(t)
	connBadLogin.On("Login", cPolicy).Return(as.ErrNotAuthenticated)
	connBadLogin.On("SetTimeout", time.Time{}, cPolicy.Timeout).Return(nil)

	connFactoryNegLogin := newMockConnFactoryFunc(t)
	connFactoryNegLogin.On("Execute", cPolicy, host).Return(connBadLogin, nil)

	factoryFuncNegLogin := func(policy *as.ClientPolicy, host *as.Host) (asconn, as.Error) {
		return connFactoryNegLogin.Execute(policy, host)
	}

	_, err = _newConnNode(cPolicy, host, authEnabled, factoryFuncNegLogin)
	connFactoryNegLogin.AssertExpectations(t)
	require.ErrorContains(t, err, "ResultCode: NOT_AUTHENTICATED")

}

func TestNode_RequestInfo(t *testing.T) {
	conn := mocks.NewAsconn(t)
	nodeName := "BB990C28F270008"
	commands := []interface{}{"node", "statistics"}

	cPolicy := as.NewClientPolicy()

	expectedRes := map[string]string{
		"node":       "BB990C28F270008",
		"statistics": "mem_use=1;objects=2",
	}

	conn.On("RequestInfo", commands...).Return(expectedRes, nil)

	connNodePos := &connNode{
		conn:   conn,
		policy: cPolicy,
		name:   nodeName,
	}

	actualRes, err := connNodePos.RequestInfo(nil, "node", "statistics")
	require.NoError(t, err)
	conn.AssertExpectations(t)
	require.Equal(t, expectedRes, actualRes)

	connBadSession := mocks.NewAsconn(t)
	connBadSession.On("RequestInfo", commands...).Return(nil, as.ErrNotAuthenticated)

	connNodeNeg := &connNode{
		conn:   connBadSession,
		policy: cPolicy,
		name:   nodeName,
	}

	_, err = connNodeNeg.RequestInfo(nil, "node", "statistics")
	require.ErrorContains(t, err, "ResultCode: NOT_AUTHENTICATED")
	connBadSession.AssertExpectations(t)
}
