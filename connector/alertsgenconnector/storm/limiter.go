package storm

import "time"

type Limiter struct {
    maxPerMinute int
    slots [60]int
    idx int
    lastTick time.Time
}

func NewLimiter(maxPerMinute int) *Limiter {
    if maxPerMinute <= 0 { maxPerMinute = 100 }
    return &Limiter{maxPerMinute: maxPerMinute, lastTick: time.Now()}
}

func (l *Limiter) Allow() bool {
    now := time.Now()
    if now.Sub(l.lastTick) >= time.Second {
        steps := int(now.Sub(l.lastTick)/time.Second)
        for i:=0; i<steps && i<60; i++ {
            l.idx = (l.idx+1)%60
            l.slots[l.idx]=0
        }
        l.lastTick = now
    }
    total := 0
    for _,v := range l.slots { total += v }
    if total >= l.maxPerMinute {
        return false
    }
    l.slots[l.idx]++
    return true
}
