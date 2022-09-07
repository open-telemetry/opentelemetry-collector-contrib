package internal

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/gosnmp/gosnmp"
)

// GosnmpWrapper wraps a *gosnmp.GoSNMP object so we can use it as a snmpConnection.
type GosnmpWrapper struct {
	*gosnmp.GoSNMP
}

// Host returns the value of GoSNMP.Target.
func (gs GosnmpWrapper) Host() string {
	return gs.Target
}

// Walk wraps GoSNMP.Walk() or GoSNMP.BulkWalk(), depending on whether the
// connection is using SNMPv1 or newer.
func (gs GosnmpWrapper) Walk(oid string, fn gosnmp.WalkFunc) error {
	if gs.Version == gosnmp.Version1 {
		return gs.GoSNMP.Walk(oid, fn)
	}
	return gs.GoSNMP.BulkWalk(oid, fn)
}

func NewWrapper(s *Config) (GosnmpWrapper, error) {
	gs := GosnmpWrapper{&gosnmp.GoSNMP{}}

	gs.Timeout = s.CollectionInterval

	//gs.Retries = s.Retries
	//
	//gs.UseUnconnectedUDPSocket = s.UnconnectedUDPSocket

	switch s.Version {
	case "v3":
		gs.Version = gosnmp.Version3
	case "v2":
		gs.Version = gosnmp.Version2c
	case "v1":
		gs.Version = gosnmp.Version1
	default:
		return GosnmpWrapper{}, fmt.Errorf("invalid version")
	}

	// TODO is community needed with v3?
	//if s.Version < 3 {
	//	if s.Community == "" {
	//		gs.Community = "public"
	//	} else {
	//		gs.Community = s.Community
	//	}
	//}

	// TODO max repetitions as parameter? is Context name needed?
	//gs.MaxRepetitions = s.MaxRepetitions

	if s.Version == "v3" {
		//gs.ContextName = s.ContextName

		sp := &gosnmp.UsmSecurityParameters{}
		gs.SecurityParameters = sp
		gs.SecurityModel = gosnmp.UserSecurityModel

		switch strings.ToLower(s.SecurityLevel) {
		case "no_auth_no_priv", "":
			gs.MsgFlags = gosnmp.NoAuthNoPriv
		case "auth_no_priv":
			gs.MsgFlags = gosnmp.AuthNoPriv
		case "auth_priv":
			gs.MsgFlags = gosnmp.AuthPriv
		default:
			return GosnmpWrapper{}, fmt.Errorf("invalid security_level")
		}

		sp.UserName = s.User

		switch strings.ToLower(s.AuthenticationType) {
		case "md5":
			sp.AuthenticationProtocol = gosnmp.MD5
		case "sha":
			sp.AuthenticationProtocol = gosnmp.SHA
		case "sha224":
			sp.AuthenticationProtocol = gosnmp.SHA224
		case "sha256":
			sp.AuthenticationProtocol = gosnmp.SHA256
		case "sha384":
			sp.AuthenticationProtocol = gosnmp.SHA384
		case "sha512":
			sp.AuthenticationProtocol = gosnmp.SHA512
		case "":
			sp.AuthenticationProtocol = gosnmp.NoAuth
		default:
			return GosnmpWrapper{}, fmt.Errorf("invalid authentication_type")
		}

		sp.AuthenticationPassphrase = s.AuthPassword

		switch strings.ToLower(s.PrivacyType) {
		case "des":
			sp.PrivacyProtocol = gosnmp.DES
		case "aes":
			sp.PrivacyProtocol = gosnmp.AES
		case "aes192":
			sp.PrivacyProtocol = gosnmp.AES192
		case "aes192c":
			sp.PrivacyProtocol = gosnmp.AES192C
		case "aes256":
			sp.PrivacyProtocol = gosnmp.AES256
		case "aes256c":
			sp.PrivacyProtocol = gosnmp.AES256C
		case "":
			sp.PrivacyProtocol = gosnmp.NoPriv
		default:
			return GosnmpWrapper{}, fmt.Errorf("invalid privacy_type")
		}

		sp.PrivacyPassphrase = s.PrivacyPassword

		// TODO engine parameters are optional?

		//sp.AuthoritativeEngineID = s.EngineID
		//
		//sp.AuthoritativeEngineBoots = s.EngineBoots
		//
		//sp.AuthoritativeEngineTime = s.EngineTime
	}
	return gs, nil
}

// SetAgent takes a url (scheme://host:port) and sets the wrapped
// GoSNMP struct's corresponding fields.  This shouldn't be called
// after using the wrapped GoSNMP struct, for example after
// connecting.
func (gs *GosnmpWrapper) SetAgent(agent string) error {
	if !strings.Contains(agent, "://") {
		agent = "udp://" + agent
	}

	u, err := url.Parse(agent)
	if err != nil {
		return err
	}

	// Only allow udp{4,6} and tcp{4,6}.
	// Allowing ip{4,6} does not make sense as specifying a port
	// requires the specification of a protocol.
	// gosnmp does not handle these errors well, which is why
	// they can result in cryptic errors by net.Dial.
	switch u.Scheme {
	case "tcp", "tcp4", "tcp6", "udp", "udp4", "udp6":
		gs.Transport = u.Scheme
	default:
		return fmt.Errorf("unsupported scheme: %v", u.Scheme)
	}

	gs.Target = u.Hostname()

	portStr := u.Port()
	if portStr == "" {
		portStr = "161"
	}
	port, err := strconv.ParseUint(portStr, 10, 16)
	if err != nil {
		return fmt.Errorf("parsing port: %w", err)
	}
	gs.Port = uint16(port)
	return nil
}

func (gs GosnmpWrapper) Reconnect() error {
	if gs.Conn == nil {
		return gs.Connect()
	}

	return nil
}
