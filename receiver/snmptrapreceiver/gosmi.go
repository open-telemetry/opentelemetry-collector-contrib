// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package snmptrapreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/snmptrapreceiver"

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/sleepinggenius2/gosmi"
	"github.com/sleepinggenius2/gosmi/types"
)

var errUnresolved = errors.New("oid not in mib dictionary")

var (
	gosmiOnce    sync.Once
	gosmiInitErr error
	gosmiMu      sync.Mutex
)

// GosmiTranslator looks up OIDs with gosmi. Init and module load are process-global.
type GosmiTranslator struct {
	log *slog.Logger
}

// NewGosmiTranslator initializes gosmi (once) and loads modules from dirs.
// Load failures are logged and skipped. An empty path list returns NoopTranslator.
func NewGosmiTranslator(paths []string, log *slog.Logger) (Translator, error) {
	if log == nil {
		log = slog.Default()
	}
	cleaned := cleanPaths(paths)
	if len(cleaned) == 0 {
		return NoopTranslator{}, nil
	}
	if err := ensureGosmi(); err != nil {
		return NoopTranslator{}, err
	}

	gosmiMu.Lock()
	defer gosmiMu.Unlock()
	loaded := 0
	for _, p := range cleaned {
		gosmi.AppendPath(p)
		n, err := loadDir(p)
		if err != nil {
			log.Warn("snmptrap: skipping mib path", "path", p, "err", err)
			continue
		}
		loaded += n
	}
	log.Info("snmptrap: gosmi dictionary loaded", "paths", len(cleaned), "modules", loaded)
	return &GosmiTranslator{log: log}, nil
}

func cleanPaths(paths []string) []string {
	out := make([]string, 0, len(paths))
	seen := map[string]struct{}{}
	for _, p := range paths {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	return out
}

func ensureGosmi() error {
	gosmiOnce.Do(func() {
		defer func() {
			if rec := recover(); rec != nil {
				gosmiInitErr = fmt.Errorf("gosmi init panic: %v", rec)
			}
		}()
		gosmi.Init()
	})
	return gosmiInitErr
}

func loadDir(path string) (int, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		ext := strings.ToLower(filepath.Ext(name))
		switch ext {
		case "", ".mib", ".my", ".txt", ".smi", ".in":
		default:
			continue
		}
		candidates := []string{strings.TrimSuffix(name, filepath.Ext(name)), name}
		for _, c := range candidates {
			if c == "" {
				continue
			}
			if _, err := gosmi.LoadModule(c); err == nil {
				n++
				break
			}
		}
	}
	return n, nil
}

// Lookup implements Translator.
func (g *GosmiTranslator) Lookup(oid string) (Lookup, error) {
	oid = NormalizeOID(oid)
	if oid == "" {
		return Lookup{}, errUnresolved
	}
	parsed, err := types.OidFromString(oid)
	if err != nil {
		return Lookup{}, err
	}

	gosmiMu.Lock()
	defer gosmiMu.Unlock()
	node, err := gosmi.GetNodeByOID(parsed)
	if err != nil {
		return Lookup{}, err
	}
	if node.Name == "" {
		return Lookup{}, errUnresolved
	}
	mib := ""
	if mod := node.GetModule(); mod.Name != "" {
		mib = mod.Name
	}
	name := node.Name
	if mib != "" {
		name = mib + "::" + node.Name
	}
	return Lookup{Name: name, MIB: mib}, nil
}
