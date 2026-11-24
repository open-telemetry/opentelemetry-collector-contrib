// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package execc // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/execc"

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"os/exec"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
)

type execcReceiver struct {
	cfg      *Config
	consumer consumer.Logs
	logger   *zap.Logger

	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func newExeccReceiver(cfg *Config, next consumer.Logs, logger *zap.Logger) *execcReceiver {
	return &execcReceiver{
		cfg:      cfg,
		consumer: next,
		logger:   logger,
	}
}

func (r *execcReceiver) Start(ctx context.Context, _ component.Host) error {
	runCtx, cancel := context.WithCancel(context.Background())
	r.cancel = cancel

	delay := r.cfg.Schedule.InitialDelay
	if delay <= 0 {
		delay = r.cfg.Schedule.Interval / 2
	}

	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		t := time.NewTimer(delay)
		defer t.Stop()
		for {
			select {
			case <-runCtx.Done():
				return
			case <-t.C:
				r.runOnce(runCtx)
				t.Reset(r.cfg.Schedule.Interval)
			}
		}
	}()
	return nil
}

func (r *execcReceiver) Shutdown(_ context.Context) error {
	if r.cancel != nil {
		r.cancel()
	}
	r.wg.Wait()
	return nil
}

// runOnce executes the command, splits stdout into lines, and emits each
// line as one log record.
func (r *execcReceiver) runOnce(ctx context.Context) {
	cctx, cancel := context.WithTimeout(ctx, r.timeout())
	defer cancel()

	var out bytes.Buffer
	cmd := exec.CommandContext(cctx, r.cfg.Command, r.cfg.Args...)
	if r.cfg.WorkingDir != "" {
		cmd.Dir = r.cfg.WorkingDir
	}
	if len(r.cfg.Env) > 0 {
		cmd.Env = append(cmd.Environ(), envSlice(r.cfg.Env)...)
	}
	cmd.Stdout = &out
	cmd.Stderr = nil // stdout only for now; stderr goes to process logs

	if err := cmd.Start(); err != nil {
		r.logger.Error("execc: failed to start command", zap.String("command", r.cfg.Command), zap.Error(err))
		return
	}
	if err := cmd.Wait(); err != nil {
		// Non-zero exit does not invalidate partial output.
		r.logger.Warn("execc: command did not exit cleanly",
			zap.String("command", r.cfg.Command), zap.Error(err))
	}

	logs := r.buildLogs(&out)
	if logs.LogRecordCount() == 0 {
		return
	}
	if err := r.consumer.ConsumeLogs(ctx, logs); err != nil {
		r.logger.Error("execc: failed to consume logs", zap.Error(err))
	}
}

func (r *execcReceiver) buildLogs(out *bytes.Buffer) plog.Logs {
	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutStr("command", r.cfg.Command)
	sl := rl.ScopeLogs().AppendEmpty()
	sl.Scope().SetName("otelcol/execc")
	now := pcommon.NewTimestampFromTime(time.Now())

	scanner := bufio.NewScanner(out)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024) // allow long lines to 4MiB
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		rec := sl.LogRecords().AppendEmpty()
		rec.SetObservedTimestamp(now)
		for k, v := range r.cfg.Attributes {
			rec.Attributes().PutStr(k, toStr(v))
		}
		r.parseLine(rec, line, now)
	}
	return logs
}

func (r *execcReceiver) parseLine(rec plog.LogRecord, line []byte, now pcommon.Timestamp) {
	switch r.cfg.Parser.Type {
	case "json":
		var m map[string]interface{}
		if err := json.Unmarshal(line, &m); err == nil && m != nil {
			bodyKey := r.cfg.Parser.BodyKey
			if bodyKey == "" {
				bodyKey = "message"
			}
			if v, ok := m[bodyKey]; ok {
				rec.Body().SetStr(toStr(v))
			} else {
				rec.Body().SetStr(string(line))
			}
			for k, v := range m {
				rec.Attributes().PutStr(k, toStr(v))
			}
			if r.cfg.Parser.Layout != "" && m["timestamp"] != nil {
				if t, err := time.Parse(r.cfg.Parser.Layout, toStr(m["timestamp"])); err == nil {
					rec.SetTimestamp(pcommon.NewTimestampFromTime(t))
					return
				}
			}
			rec.SetTimestamp(now)
			return
		}
		// fall through: unparseable JSON becomes a plain line record
		rec.Body().SetStr(string(line))
		rec.SetTimestamp(now)
		rec.SetSeverityText("WARN")
	default:
		rec.Body().SetStr(string(line))
		rec.SetTimestamp(now)
	}
}

func (r *execcReceiver) timeout() time.Duration {
	if r.cfg.Timeout > 0 {
		return r.cfg.Timeout
	}
	return r.cfg.Schedule.Interval
}

func envSlice(in map[string]string) []string {
	out := make([]string, 0, len(in))
	for k, v := range in {
		out = append(out, k+"="+v)
	}
	return out
}

func toStr(v interface{}) string {
	switch t := v.(type) {
	case string:
		return t
	default:
		b, _ := json.Marshal(t)
		return string(b)
	}
}
