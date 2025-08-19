package cardinality

import (
    "crypto/sha256"
    "encoding/hex"
)

type Limiter struct {
    maxLabels int
    hashAt int
}

func NewLimiter(maxLabels, hashAt int) *Limiter {
    if maxLabels <= 0 { maxLabels = 20 }
    if hashAt <= 0 { hashAt = 128 }
    return &Limiter{maxLabels: maxLabels, hashAt: hashAt}
}

func (l *Limiter) Enforce(labels map[string]string) map[string]string {
    out := map[string]string{}
    count := 0
    for k, v := range labels {
        if count >= l.maxLabels {
            break
        }
        if len(v) > l.hashAt {
            out[k] = hashPrefix8(v)
        } else {
            out[k] = v
        }
        count++
    }
    return out
}

func hashPrefix8(s string) string {
    h := sha256.Sum256([]byte(s))
    return hex.EncodeToString(h[:])[:8]
}
