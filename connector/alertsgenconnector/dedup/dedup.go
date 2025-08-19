
package dedup

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"time"
)

type Deduper struct {
	window  time.Duration
	include []string
	exclude map[string]struct{}
	last    map[string]time.Time
}

func NewDeduper(window time.Duration, include []string, exclude []string) *Deduper {
	ex := make(map[string]struct{}, len(exclude))
	for _, l := range exclude {
		ex[l] = struct{}{}
	}
	return &Deduper{
		window:  window,
		include: include,
		exclude: ex,
		last:    map[string]time.Time{},
	}
}

func (d *Deduper) fp(rule string, labels map[string]string) string {
	keys := make([]string, 0, len(labels))
	if len(d.include) > 0 {
		keys = append(keys, d.include...)
	} else {
		for k := range labels {
			if _, bad := d.exclude[k]; bad {
				continue
			}
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	h := sha256.New()
	h.Write([]byte(rule))
	for _, k := range keys {
		h.Write([]byte{0})
		h.Write([]byte(k))
		h.Write([]byte{1})
		h.Write([]byte(labels[k]))
	}
	return hex.EncodeToString(h.Sum(nil))
}

func (d *Deduper) Allow(rule string, labels map[string]string) bool {
	fp := d.fp(rule, labels)
	now := time.Now()
	if t, ok := d.last[fp]; ok {
		if now.Sub(t) < d.window {
			return false
		}
	}
	d.last[fp] = now
	return true
}
