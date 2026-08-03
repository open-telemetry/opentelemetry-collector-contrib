package configfile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeFixture(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return path
}

func TestParseFileFormats(t *testing.T) {
	dir := t.TempDir()
	sshd := writeFixture(t, dir, "sshd_config", "Port 22\nPermitRootLogin no\n")
	ini := writeFixture(t, dir, "app.ini", "[Unit]\nAfter=network.target\n")
	yamlPath := writeFixture(t, dir, "cfg.yaml", "debug: true\nport: 9001\n")
	jsonPath := writeFixture(t, dir, "cfg.json", `{"enabled":true,"retries":3}`)

	cases := []struct {
		path   string
		format string
		want   map[string]string
	}{
		{sshd, "generic", map[string]string{"Port": "22", "PermitRootLogin": "no"}},
		{ini, "ini", map[string]string{"Unit.After": "network.target"}},
		{yamlPath, "yaml", map[string]string{"debug": "true", "port": "9001"}},
		{jsonPath, "json", map[string]string{"enabled": "true", "retries": "3"}},
	}
	opts := Options{ExcludeKeys: DefaultExcludeKeyGlobs, MaxKeysPerFile: 500}
	for _, tc := range cases {
		_, got, err := ParseFile(tc.path, tc.format, opts)
		if err != nil {
			t.Fatalf("ParseFile(%q): %v", tc.path, err)
		}
		for k, v := range tc.want {
			if got[k] != v {
				t.Fatalf("ParseFile(%q) key %q = %q, want %q", tc.path, k, got[k], v)
			}
		}
	}
}

func TestChecksumStableAndRedaction(t *testing.T) {
	dir := t.TempDir()
	path := writeFixture(t, dir, "cfg.yaml", "debug: true\npassword: secret\nPort: 22\n")
	opts := Options{ExcludeKeys: DefaultExcludeKeyGlobs, MaxKeysPerFile: 500}

	_, keys1, err := ParseFile(path, "yaml", opts)
	if err != nil {
		t.Fatal(err)
	}
	_, keys2, err := ParseFile(path, "yaml", opts)
	if err != nil {
		t.Fatal(err)
	}
	if Checksum(keys1) != Checksum(keys2) {
		t.Fatal("checksum not stable")
	}
	for k := range keys1 {
		if strings.Contains(strings.ToLower(k), "password") {
			t.Fatalf("redacted key leaked: %q", k)
		}
	}
}

func TestProcessEntrySkipsUnchangedChecksum(t *testing.T) {
	dir := t.TempDir()
	path := writeFixture(t, dir, "sshd_config", "Port 22\n")
	opts := Options{MaxKeysPerFile: 500}

	st := &State{Files: make(map[string]FileState)}
	entry := FileEntry{Path: path}

	snap1, emit1, err := ProcessEntry(entry, st, opts, true)
	if err != nil || !emit1 || snap1 == nil {
		t.Fatalf("first run: snap=%v emit=%v err=%v", snap1, emit1, err)
	}

	snap2, emit2, err := ProcessEntry(entry, st, opts, false)
	if err != nil || emit2 || snap2 != nil {
		t.Fatalf("second run should skip: snap=%v emit=%v err=%v", snap2, emit2, err)
	}

	info, _ := os.Stat(path)
	past := info.ModTime().Add(-time.Hour)
	if err := os.Chtimes(path, past, past); err != nil {
		t.Fatal(err)
	}
	snap3, emit3, err := ProcessEntry(entry, st, opts, false)
	if err != nil || emit3 || snap3 != nil {
		t.Fatalf("touch same content should skip emit: snap=%v emit=%v err=%v", snap3, emit3, err)
	}
}

func TestSnapshotsToLogsAttributes(t *testing.T) {
	snap := &Snapshot{
		File:      "/etc/ssh/sshd_config",
		Format:    "generic",
		Checksum:  "abc123",
		KeysTotal: 1,
		Event:     EventInitial,
		Keys:      map[string]string{"Port": "22"},
	}
	ld := SnapshotsToLogs([]*Snapshot{snap})
	if ld.LogRecordCount() != 1 {
		t.Fatalf("expected 1 record, got %d", ld.LogRecordCount())
	}
	rec := ld.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
	attrs := LogRecordAttributes(rec)
	if attrs["config.file"] != snap.File {
		t.Fatalf("config.file = %q", attrs["config.file"])
	}
	if attrs["config.key.Port"] != "22" {
		t.Fatalf("config.key.Port = %q", attrs["config.key.Port"])
	}
	svc, ok := ld.ResourceLogs().At(0).Resource().Attributes().Get("service.name")
	if !ok || svc.Str() != ServiceName {
		t.Fatalf("service.name = %v ok=%v", svc.AsString(), ok)
	}
}
