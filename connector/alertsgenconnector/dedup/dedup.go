package dedup

import (
    "crypto/sha256"
    "encoding/hex"
    "sort"
    "time"
)

type Deduper struct {
    window time.Duration
    include []string
    exclude map[string]struct{}

    last map[string]time.Time
}

func NewDeduper(window time.Duration, include, exclude []string) *Deduper {
    ex := map[string]struct{}{}
    for _, k := range exclude { ex[k]=struct{}{} }
    return &Deduper{
        window: window,
        include: include,
        exclude: ex,
        last: map[string]time.Time{},
    }
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

func (d *Deduper) fp(rule string, labels map[string]string) string {
    keys := make([]string,0,len(d.include))
    for _, k := range d.include {
        if _, ok := labels[k]; ok {
            keys = append(keys, k)
        }
    }
    sort.Strings(keys)
    h := sha256.New()
    h.Write([]byte(rule))
    for _, k := range keys {
        if _, drop := d.exclude[k]; drop { continue }
        h.Write([]byte(k)); h.Write([]byte("=")); h.Write([]byte(labels[k])); h.Write([]byte(","))
    }
    return hex.EncodeToString(h.Sum(nil))
}
