// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package testutil // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/testutil"

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

// PData specifies the telemetry found in a log file.
type PData struct {
	Metrics []pmetric.Metrics
	Traces  []ptrace.Traces
}

// ReadLoggingExporters reads a Collector log file from r, returning all the metrics, traces and logs found in it
// as "loggingexporter" output.
//
// Supports only Metrics (Sums & Gauges) for now.
func ReadLoggingExporter(r io.Reader) (PData, error) {
	var pd PData
	scn := newScanner(r)
	for scn.Scan() {
		txt := scn.Text()
		switch {
		case strings.Contains(txt, `MetricsExporter	{"#metrics":`):
			metrics := pmetric.NewMetrics()
			rmetrics := metrics.ResourceMetrics().AppendEmpty()
			if err := readResourceMetrics(scn, rmetrics); err != nil {
				return pd, fmt.Errorf("error parsing resource: %w", err)
			}
			pd.Metrics = append(pd.Metrics, metrics)
		case strings.Contains(txt, `TracesExporter	{"#spans":`):
			// TODO
		case strings.Contains(txt, `LogsExporter	{"#logs":`):
			// TODO
		default:
			continue
		}
	}
	if err := scn.Err(); err != nil {
		return pd, fmt.Errorf("error scanning file: %w", err)
	}
	return pd, nil
}

// scanner is a wrapper on top of bufio.Scanner which has all the same
// methods, but is enhanced with some additional features such as being
// able to "unscan", remember line numbers, better error reporting and
// advancing lines while checking prefixes.
type scanner struct {
	line      int
	last      string
	unscanned bool
	scn       *bufio.Scanner
	str       strings.Builder
}

func newScanner(r io.Reader) *scanner {
	return &scanner{scn: bufio.NewScanner(r)}
}

// Scan advances the scanner and reports whether there are more lines left.
func (s *scanner) Scan() bool {
	if s.unscanned {
		s.unscanned = false
		return true
	}
	s.last = s.scn.Text()
	ok := s.scn.Scan()
	if ok {
		s.line++
		s.str.WriteString(fmt.Sprintf("%d: ", s.line))
		s.str.WriteString(s.scn.Text())
		s.str.WriteByte('\n')
	}
	return ok
}

// Debug exits to the OS and reports the last 1500 characters that were scanned.
func (s *scanner) Debug() {
	fmt.Println(s.Tail(1500))
	os.Exit(0)
}

// Tail returns the last n characters that were scanned, including their starting
// line numbers.
func (s *scanner) Tail(n int) string {
	return string([]byte(s.str.String())[s.str.Len()-n:])
}

// Line reports the current line number.
func (s *scanner) Line() int { return s.line }

// Text returns the last scanned line.
func (s *scanner) Text() string {
	if s.unscanned {
		return s.last
	}
	return s.scn.Text()
}

// Unscan reverts the last scan.
func (s *scanner) Unscan() { s.unscanned = true }

// Err reports scanning errors.
func (s *scanner) Err() error {
	if err := s.scn.Err(); err != nil {
		return fmt.Errorf("#%d: %w", s.line, s.scn.Err())
	}
	return nil
}

// Advance moves to the next line. If the current line does not have the given
// prefix, it returns an error. If the end is reached, it returns io.EOF.
func (s *scanner) Advance(prefix string) error {
	if !strings.HasPrefix(s.Text(), prefix) {
		return fmt.Errorf("line %d: expected %q [...], got %s", s.Line(), prefix, s.Text())
	}
	ok := s.Scan()
	if !ok {
		return io.EOF
	}
	return nil
}

func readResource(scn *scanner, rsrc pcommon.Resource) error {
	switch scn.Text() {
	case "Resource labels:", "Resource attributes:":
		// ok
		return readAttributes(scn, rsrc.Attributes())
	}
	return nil
}

func readAttributes(scn *scanner, m pcommon.Map) error {
	for scn.Scan() {
		txt := scn.Text()
		attrPrefix := "     -> "
		if !strings.HasPrefix(txt, attrPrefix) {
			break
		}
		parts := strings.Split(strings.TrimPrefix(txt, attrPrefix), ": ")
		if len(parts) != 2 {
			return errors.New("invalid attribute format; could not split by ':'")
		}
		key, v1 := parts[0], parts[1]
		eot := bytes.Index([]byte(v1), []byte("("))
		if eot == -1 {
			return errors.New("invalid attribute value format; expected '('")
		}
		typ := string([]byte(v1)[:eot])
		val := string([]byte(v1)[eot+1 : len(v1)-1])
		switch typ {
		case "EMPTY":
			m.PutEmpty(key)
		case "STRING":
			m.PutStr(key, val)
		case "BOOL":
			v, err := strconv.ParseBool(val)
			if err != nil {
				return fmt.Errorf("error parsing boolean value %q: %w", key, err)
			}
			m.PutBool(key, v)
		case "INT":
			v, err := strconv.ParseInt(val, 10, 64)
			if err != nil {
				return fmt.Errorf("error parsing int value %q: %w", key, err)
			}
			m.PutInt(key, v)
		case "DOUBLE":
			v, err := strconv.ParseFloat(val, 64)
			if err != nil {
				return fmt.Errorf("error parsing float value %q: %w", key, err)
			}
			m.PutDouble(key, v)
		case "MAP":
			mp := m.PutEmptyMap(key)
			var v map[string]interface{}
			if err := json.Unmarshal([]byte(val), &v); err != nil {
				return fmt.Errorf("error parsing map %q: %w", key, err)
			}
			mp.FromRaw(v)
		case "SLICE":
			s := m.PutEmptySlice(key)
			var v []interface{}
			if err := json.Unmarshal([]byte(val), &v); err != nil {
				return fmt.Errorf("error parsing map %q: %w", key, err)
			}
			s.FromRaw(v)
		case "BYTES":
			bs := m.PutEmptyBytes(key)
			b, err := base64.StdEncoding.DecodeString(val)
			if err != nil {
				return fmt.Errorf("error parsing byte slice %q: %w", key, err)
			}
			bs.FromRaw(b)
		}
	}
	scn.Unscan()
	return nil
}

func readResourceMetrics(scn *scanner, rmetrics pmetric.ResourceMetrics) error {
	for scn.Scan() {
		if !strings.Contains(scn.Text(), "ResourceMetrics #") {
			break
		}
		if !scn.Scan() {
			return io.ErrUnexpectedEOF
		}
		if err := scn.Advance("Resource SchemaURL:"); err != nil {
			return err
		}
		if err := readResource(scn, rmetrics.Resource()); err != nil {
			return fmt.Errorf("error parsing resource: %w", err)
		}
		scopem := rmetrics.ScopeMetrics()
		if err := readScopeMetrics(scn, scopem); err != nil {
			return fmt.Errorf("error parsing scope metrics: %w", err)
		}
	}
	return nil
}

func readScopeMetrics(scn *scanner, scopem pmetric.ScopeMetricsSlice) error {
	for scn.Scan() {
		if !strings.HasPrefix(scn.Text(), "ScopeMetrics #") {
			break
		}
		mm := scopem.AppendEmpty()
		if !scn.Scan() {
			return io.ErrUnexpectedEOF
		}
		if err := scn.Advance("ScopeMetrics SchemaURL:"); err != nil {
			return err
		}

		parts := strings.Split(scn.Text(), " ")
		var scopename, scopever string
		if len(parts) == 3 {
			scopename, scopever = parts[1], parts[2]
		}
		mm.Scope().SetName(scopename)
		mm.Scope().SetVersion(scopever)
		if err := scn.Advance("InstrumentationScope"); err != nil {
			return err
		}
		scn.Unscan()
		if err := readMetrics(scn, mm); err != nil {
			return err
		}
	}
	return nil
}

func readMetrics(scn *scanner, sm pmetric.ScopeMetrics) error {
	for scn.Scan() {
		if !strings.HasPrefix(scn.Text(), "Metric #") {
			break
		}
		if !scn.Scan() {
			return io.ErrUnexpectedEOF
		}
		mm := sm.Metrics().AppendEmpty()
		if err := scn.Advance("Descriptor:"); err != nil {
			return err
		}
		mm.SetName(strings.TrimPrefix(scn.Text(), "     -> Name: "))
		if err := scn.Advance("     -> Name: "); err != nil {
			return err
		}
		mm.SetDescription(strings.TrimPrefix(scn.Text(), "     -> Description: "))
		if err := scn.Advance("     -> Description: "); err != nil {
			return err
		}
		mm.SetUnit(strings.TrimPrefix(scn.Text(), "     -> Unit: "))
		if err := scn.Advance("     -> Unit: "); err != nil {
			return err
		}
		typ := strings.TrimPrefix(scn.Text(), "     -> DataType: ")
		switch typ {
		case "Gauge":
			g := mm.SetEmptyGauge()
			if err := readNumberDataPoints(scn, g.DataPoints()); err != nil {
				return fmt.Errorf("%d: error parsing data points: %w", scn.Line(), err)
			}
		case "Sum":
			s := mm.SetEmptySum()
			if err := readSum(scn, s); err != nil {
				return fmt.Errorf("error reading sum: %w", err)
			}
		case "Histogram":
			mm.SetEmptyHistogram()
			fallthrough
		case "ExponentialHistogram":
			mm.SetEmptyExponentialHistogram()
			fallthrough
		case "Summary":
			mm.SetEmptySummary()
			fallthrough
		default:
			return fmt.Errorf("metric data type %q not implemented", typ)
		}
	}
	scn.Unscan()
	return nil
}

func readSum(scn *scanner, s pmetric.Sum) error {
	if !scn.Scan() {
		return io.ErrUnexpectedEOF
	}
	s.SetIsMonotonic(strings.TrimPrefix(scn.Text(), "     -> IsMonotonic: ") == "true")
	if err := scn.Advance("     -> IsMonotonic: "); err != nil {
		return err
	}
	switch strings.TrimPrefix(scn.Text(), "     -> AggregationTemporality: ") {
	case "AGGREGATION_TEMPORALITY_DELTA":
		s.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
	case "AGGREGATION_TEMPORALITY_CUMULATIVE":
		s.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
	}
	if err := scn.Advance("     -> AggregationTemporality: "); err != nil {
		return err
	}
	scn.Unscan()
	return readNumberDataPoints(scn, s.DataPoints())
}

func readNumberDataPoints(scn *scanner, dps pmetric.NumberDataPointSlice) error {
	for scn.Scan() {
		if !strings.HasPrefix(scn.Text(), "NumberDataPoints #") {
			break
		}
		dp := dps.AppendEmpty()
		if err := scn.Advance("NumberDataPoints #"); err != nil {
			return err
		}
		if scn.Text() == "Data point attributes:" {
			if err := readAttributes(scn, dp.Attributes()); err != nil {
				return fmt.Errorf("error reading data points: %w", err)
			}
			if !scn.Scan() {
				return io.ErrUnexpectedEOF
			}
		}
		ssts := strings.TrimPrefix(scn.Text(), "StartTimestamp: ")
		sts, err := time.Parse("2006-01-02 15:04:05.999999999 -0700 MST", ssts)
		if err != nil {
			return fmt.Errorf("error parsing start timestamp: %w", err)
		}
		dp.SetStartTimestamp(pcommon.NewTimestampFromTime(sts))
		if err = scn.Advance("StartTimestamp: "); err != nil {
			return err
		}
		ts, err := time.Parse("2006-01-02 15:04:05.999999999 -0700 MST", strings.TrimPrefix(scn.Text(), "Timestamp: "))
		if err != nil {
			return fmt.Errorf("error parsing timestamp: %w", err)
		}
		dp.SetTimestamp(pcommon.NewTimestampFromTime(ts))
		if err := scn.Advance("Timestamp: "); err != nil {
			return err
		}
		v := strings.TrimPrefix(scn.Text(), "Value: ")
		if strings.Contains(v, ".") {
			// float64
			vv, err := strconv.ParseFloat(v, 64)
			if err != nil {
				return fmt.Errorf("error parsing float %q: %w", v, err)
			}
			dp.SetDoubleValue(vv)
		} else {
			// int64
			vv, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				return fmt.Errorf("error parsing float %q: %w", v, err)
			}
			dp.SetIntValue(vv)
		}
	}
	scn.Unscan()
	return nil
}
