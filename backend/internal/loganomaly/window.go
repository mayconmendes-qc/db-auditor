package loganomaly

import (
	"sort"
	"time"
)

// WindowKey is the start of a fixed-size window (UTC).
type WindowKey int64 // unix nano of window start

// WindowCounts holds per-template and per-level counts for one window.
type WindowCounts struct {
	Start      time.Time
	ByTemplate map[string]int    // templateID → count
	ByLevel    map[string]int    // level bucket → count
	Templates  map[string]string // id → pattern (masked only)
}

// Counter aggregates events into windows.
type Counter struct {
	size     time.Duration
	windows  map[WindowKey]*WindowCounts
	minStart time.Time
	maxStart time.Time
	hasData  bool
}

// NewCounter creates a window counter.
func NewCounter(window time.Duration) *Counter {
	if window <= 0 {
		window = 5 * time.Minute
	}
	return &Counter{
		size:    window,
		windows: make(map[WindowKey]*WindowCounts),
	}
}

func (c *Counter) key(t time.Time) WindowKey {
	unix := t.UTC().UnixNano()
	size := c.size.Nanoseconds()
	start := unix - (unix % size)
	if unix < 0 && unix%size != 0 {
		start -= size
	}
	return WindowKey(start)
}

// Add records one event into its window.
func (c *Counter) Add(ev Event) {
	if ev.Unclassifiable || ev.TemplateID == "" {
		return
	}
	k := c.key(ev.Time)
	w, ok := c.windows[k]
	if !ok {
		start := time.Unix(0, int64(k)).UTC()
		w = &WindowCounts{
			Start:      start,
			ByTemplate: make(map[string]int),
			ByLevel:    make(map[string]int),
			Templates:  make(map[string]string),
		}
		c.windows[k] = w
		if !c.hasData || start.Before(c.minStart) {
			c.minStart = start
		}
		if !c.hasData || start.After(c.maxStart) {
			c.maxStart = start
		}
		c.hasData = true
	}
	w.ByTemplate[ev.TemplateID]++
	w.ByLevel[ev.Level]++
	if ev.Template != "" {
		w.Templates[ev.TemplateID] = ev.Template
	}
}

// SortedKeys returns window starts ascending.
func (c *Counter) SortedKeys() []WindowKey {
	keys := make([]WindowKey, 0, len(c.windows))
	for k := range c.windows {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	return keys
}

// Windows returns counts in chronological order.
func (c *Counter) Windows() []*WindowCounts {
	keys := c.SortedKeys()
	out := make([]*WindowCounts, 0, len(keys))
	for _, k := range keys {
		out = append(out, c.windows[k])
	}
	return out
}

// Len returns number of distinct windows.
func (c *Counter) Len() int { return len(c.windows) }

// Range returns min/max window starts.
func (c *Counter) Range() (time.Time, time.Time, bool) {
	return c.minStart, c.maxStart, c.hasData
}

// Size returns the window duration.
func (c *Counter) Size() time.Duration { return c.size }
