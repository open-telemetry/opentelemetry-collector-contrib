package datasenders

import (
	"bytes"
	"context"
	"fmt"
	"net"

	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/testbed/testbed"
)

type SyslogWriter struct {
	testbed.DataSenderBase
	conn    net.Conn
	count   int
	buf     []string
	bufSize int
	network string
}

var _ testbed.LogDataSender = (*SyslogWriter)(nil)

func NewSyslogWriter(network string, host string, port int, batchSize int) *SyslogWriter {
	f := &SyslogWriter{
		network: network,
		bufSize: batchSize,
		DataSenderBase: testbed.DataSenderBase{
			Port: port,
			Host: host,
		},
	}
	return f
}

func (f *SyslogWriter) GetEndpoint() net.Addr {
	var addr net.Addr
	switch f.network {
	case "udp":
		addr, _ = net.ResolveUDPAddr(f.network, fmt.Sprintf("%s:%d", f.Host, f.Port))

	default:
		addr, _ = net.ResolveTCPAddr(f.network, fmt.Sprintf("%s:%d", f.Host, f.Port))
	}
	return addr
}

func (f *SyslogWriter) Start() (err error) {
	f.conn, err = net.Dial(f.GetEndpoint().Network(), f.GetEndpoint().String())
	return err
}

func (f *SyslogWriter) ConsumeLogs(_ context.Context, logs pdata.Logs) error {

	for i := 0; i < logs.ResourceLogs().Len(); i++ {
		for j := 0; j < logs.ResourceLogs().At(i).InstrumentationLibraryLogs().Len(); j++ {
			ills := logs.ResourceLogs().At(i).InstrumentationLibraryLogs().At(j)
			for k := 0; k < ills.Logs().Len(); k++ {
				err := f.Send(ills.Logs().At(k))
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (f *SyslogWriter) GenConfigYAMLStr() string {
	return fmt.Sprintf(`
  syslog:
    protocol: rfc5424
    %s:
      listen_address: "%s"
`, f.network, f.GetEndpoint())
}
func (f *SyslogWriter) Send(lr pdata.LogRecord) error {
	msg := "<86>1 2021-02-28T00:01:02.003Z 192.168.1.1 SecureAuth0 23108 ID52020 [SecureAuth@27389] test msg\n"
	f.buf = append(f.buf, msg)
	return f.SendCheck()
}

func (f *SyslogWriter) SendCheck() error {
	if len(f.buf) == f.bufSize {
		b := bytes.NewBufferString("")
		for _, v := range f.buf {
			b.WriteString(v)
		}
		_, err := f.conn.Write(b.Bytes())
		f.buf = []string{}
		if err != nil {
			return err
		}
	}
	return nil
}

func (f *SyslogWriter) Flush() {
}

func (f *SyslogWriter) ProtocolName() string {
	return "syslog"
}
