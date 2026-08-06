// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package signingprocessor

import (
	"context"
	"crypto"
	"hash"
	"testing"

	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
)

type logSink struct{ logs []plog.Logs }

func (s *logSink) ConsumeLogs(_ context.Context, ld plog.Logs) error {
	s.logs = append(s.logs, ld)
	return nil
}
func (s *logSink) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func TestConsumeLogsResourceAttrs(t *testing.T) {
	prov := newTestProvider(t)
	sink := &logSink{}
	p := &signingProcessor{
		config:       &Config{Algorithm: "RS256", CertificateRef: CertificateRefFingerprint},
		provider:     prov,
		nextLogs:     sink,
		hashFunc:     func() hash.Hash { return crypto.SHA256.New() },
		jwaAlgorithm: "RS256",
		certRef:      "sha256:abc",
	}

	ld := plog.NewLogs()
	rl := ld.ResourceLogs().AppendEmpty()
	sl := rl.ScopeLogs().AppendEmpty()
	lr := sl.LogRecords().AppendEmpty()
	lr.Body().SetStr("test")
	lr.Attributes().PutStr("audit.actor.id", "u1")

	if err := p.ConsumeLogs(context.Background(), ld); err != nil {
		t.Fatalf("ConsumeLogs: %v", err)
	}

	if len(sink.logs) != 1 {
		t.Fatalf("expected 1 batch, got %d", len(sink.logs))
	}
	res := sink.logs[0].ResourceLogs().At(0).Resource().Attributes()
	algo, ok := res.Get("audit.integrity.algorithm")
	if !ok || algo.Str() != "RS256" {
		t.Errorf("audit.integrity.algorithm: got %q, want RS256", algo.Str())
	}
	certRef, ok2 := res.Get("audit.integrity.certificate")
	if !ok2 || certRef.Str() != "sha256:abc" {
		t.Errorf("audit.integrity.certificate: got %q, want sha256:abc", certRef.Str())
	}
	t.Logf("✅ resource attrs: algorithm=%s certificate=%s", algo.Str(), certRef.Str())

	rec := sink.logs[0].ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
	if _, ok4 := rec.Attributes().Get("audit.integrity.value"); !ok4 {
		t.Error("audit.integrity.value missing from log record")
	}
	t.Logf("✅ record integrity attrs present")

	// Full sign+verify end-to-end through ConsumeLogs
	verifyRecord(t, rec, &prov.key.PublicKey)
	t.Logf("✅ signature verifies against public key")
}
