// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package oracledbreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/oracledbreceiver"

import (
	"encoding/binary"
	"fmt"
	"sync"

	"github.com/jcmturner/gokrb5/v8/client"
	"github.com/jcmturner/gokrb5/v8/config"
	"github.com/jcmturner/gokrb5/v8/credentials"
	"github.com/jcmturner/gokrb5/v8/gssapi"
	"github.com/jcmturner/gokrb5/v8/iana/chksumtype"
	"github.com/jcmturner/gokrb5/v8/iana/flags"
	"github.com/jcmturner/gokrb5/v8/keytab"
	"github.com/jcmturner/gokrb5/v8/messages"
	"github.com/jcmturner/gokrb5/v8/types"
)

// kerberosAuth implements go-ora's configurations.KerberosAuthInterface using
// the pure-Go gokrb5 library. go-ora invokes Authenticate during Oracle Net's
// advanced negotiation (ANO) Kerberos handshake.
//
// For the keytab and password credential types, gokrb5 starts a background
// goroutine that renews the ticket-granting ticket before it expires, so a
// single client works for the lifetime of the receiver. The ccache credential
// type has no such renewal: gokrb5 loads the ticket from the cache once and
// never refreshes it. To keep long-running collectors working when an external
// process (kinit, SSSD, ...) refreshes the cache file, Authenticate reloads the
// client from the cache on the fly if acquiring a service ticket fails.
type kerberosAuth struct {
	// cfg is retained so the ccache credential type can rebuild the client by
	// re-reading the cache file. mu guards cl because go-ora may call
	// Authenticate concurrently for different pooled connections.
	cfg *KerberosConfig
	mu  sync.Mutex
	cl  *client.Client
}

// close releases the underlying gokrb5 client. For the keytab and password
// credential types this stops the background TGT-renewal goroutine and timer
// that Login started; it is a safe no-op for the ccache type, which has no such
// goroutine. Taking the lock ensures we destroy whichever client is current,
// even if the ccache reload path replaced it.
func (k *kerberosAuth) close() {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.cl != nil {
		k.cl.Destroy()
	}
}

// newKerberosClient builds a gokrb5 client from the receiver's Kerberos config
// and acquires a ticket-granting ticket (except for the ccache credential type,
// which already holds a TGT).
func newKerberosClient(cfg *KerberosConfig) (*client.Client, error) {
	krbCfg, err := config.Load(cfg.ConfigFile)
	if err != nil {
		return nil, fmt.Errorf("load krb5 config %q: %w", cfg.ConfigFile, err)
	}

	disableFAST := client.DisablePAFXFAST(cfg.DisableFASTNegotiation)

	var cl *client.Client
	switch cfg.CredentialType {
	case KerberosCredentialKeytab:
		kt, ktErr := keytab.Load(cfg.KeytabFile)
		if ktErr != nil {
			return nil, fmt.Errorf("load keytab %q: %w", cfg.KeytabFile, ktErr)
		}
		cl = client.NewWithKeytab(cfg.Principal, cfg.Realm, kt, krbCfg, disableFAST)
	case KerberosCredentialPassword:
		cl = client.NewWithPassword(cfg.Principal, cfg.Realm, string(cfg.Password), krbCfg, disableFAST)
	case KerberosCredentialCache:
		cc, ccErr := credentials.LoadCCache(cfg.CredentialCache)
		if ccErr != nil {
			return nil, fmt.Errorf("load credential cache %q: %w", cfg.CredentialCache, ccErr)
		}
		cl, err = client.NewFromCCache(cc, krbCfg, disableFAST)
		if err != nil {
			return nil, fmt.Errorf("create client from credential cache: %w", err)
		}
	default:
		return nil, fmt.Errorf("%w: %q", errInvalidCredentialType, cfg.CredentialType)
	}

	// A credential cache already holds a TGT; keytab and password must log in to
	// acquire one.
	if cfg.CredentialType != KerberosCredentialCache {
		if err = cl.Login(); err != nil {
			return nil, fmt.Errorf("kerberos login: %w", err)
		}
	}

	return cl, nil
}

// Authenticate is called by go-ora during the ANO Kerberos handshake. go-ora
// passes the negotiated server host name and service name; the target service
// principal name (SPN) is service/host. It returns a bare Kerberos AP-REQ
// (ASN.1 APPLICATION 14), which is what Oracle Net expects — not a GSS-API- or
// SPNEGO-wrapped token.
func (k *kerberosAuth) Authenticate(server, service string) ([]byte, error) {
	spn := service + "/" + server

	k.mu.Lock()
	defer k.mu.Unlock()

	tkt, key, err := k.cl.GetServiceTicket(spn)
	if err != nil {
		// gokrb5 does not renew a ticket loaded from a credential cache. If the
		// cached TGT has expired (or the cache was replaced out of band), reload
		// the client from the cache file and try once more. keytab and password
		// clients renew themselves, so there is nothing to reload for them.
		if k.cfg.CredentialType != KerberosCredentialCache {
			return nil, fmt.Errorf("get service ticket for %q: %w", spn, err)
		}
		cl, reloadErr := newKerberosClient(k.cfg)
		if reloadErr != nil {
			return nil, fmt.Errorf("get service ticket for %q: %w; reload credential cache: %w", spn, err, reloadErr)
		}
		k.cl = cl
		tkt, key, err = k.cl.GetServiceTicket(spn)
		if err != nil {
			return nil, fmt.Errorf("get service ticket for %q after credential cache reload: %w", spn, err)
		}
	}

	auth, err := types.NewAuthenticator(k.cl.Credentials.Domain(), k.cl.Credentials.CName())
	if err != nil {
		return nil, fmt.Errorf("new authenticator: %w", err)
	}
	// The GSSAPI-type checksum carries the RFC 4121 context flags Oracle expects
	// even though the AP-REQ itself is sent bare (not GSS-wrapped).
	auth.Cksum = types.Checksum{
		CksumType: chksumtype.GSSAPI,
		Checksum:  gssAPIChecksum([]int{gssapi.ContextFlagMutual, gssapi.ContextFlagInteg}),
	}

	apreq, err := messages.NewAPReq(tkt, key, auth)
	if err != nil {
		return nil, fmt.Errorf("new AP-REQ: %w", err)
	}
	types.SetFlag(&apreq.APOptions, flags.APOptionMutualRequired)

	b, err := apreq.Marshal()
	if err != nil {
		return nil, fmt.Errorf("marshal AP-REQ: %w", err)
	}
	return b, nil
}

// gssAPIChecksum builds the RFC 4121 §4.1.1 GSSAPI authenticator checksum: a
// 24-byte value with the channel-binding length (16) in the first four bytes
// and the context flags in the last four, both little-endian.
func gssAPIChecksum(gflags []int) []byte {
	c := make([]byte, 24)
	binary.LittleEndian.PutUint32(c[:4], 16)
	var f uint32
	for _, i := range gflags {
		f |= uint32(i)
	}
	binary.LittleEndian.PutUint32(c[20:24], f)
	return c
}
