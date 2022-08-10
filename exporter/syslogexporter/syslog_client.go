package syslogexporter

import (
	"fmt"
	"net"
	"sync"
)

type sysLogClient interface {
	SendLog(s string) error
	Close() error
}

type sysLogNetClient struct {
	endpoint    string
	netProtocol string
	conn        net.Conn

	mu sync.Mutex // guards conn
}

func (c *sysLogNetClient) connect() error {
	if c.conn != nil {
		// ignore err from close, it makes sense to continue anyway
		c.conn.Close()
		c.conn = nil
	}

	//todo not fail if can't connect, setup retry
	conn, err := net.Dial(c.netProtocol, c.endpoint)
	if err != nil {
		fmt.Printf("Connection error: %s\n", err.Error())
		return err
	}

	c.conn = conn
	return nil
}

func (c *sysLogNetClient) SendLog(s string) error {
	if c.conn != nil {
		if _, err := c.conn.Write([]byte(s)); err == nil {
			return err
		}
	}

	if err := c.connect(); err != nil {
		return err
	}

	_, err := c.conn.Write([]byte(s))
	if err != nil {
		fmt.Printf("Send error: %s\n", err.Error())
		return err
	}

	return nil
}

func (c *sysLogNetClient) Close() error {
	err := c.conn.Close()
	if err != nil {
		return err
	}
	return nil
}
