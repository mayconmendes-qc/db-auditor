package loganomaly

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"sync"
)

// TemplateStore clusters masked messages into stable templates (Drain-like).
// Max MaxTemplates distinct ids; overflow goes to OthersTemplateID.
type TemplateStore struct {
	mu       sync.Mutex
	byExact  map[string]*templateEntry // masked exact → entry
	order    []string                  // insertion order of template ids (deterministic walk)
	overflow int64
}

type templateEntry struct {
	ID      string
	Pattern string
	Count   int64
}

// NewTemplateStore creates an empty store.
func NewTemplateStore() *TemplateStore {
	return &TemplateStore{byExact: make(map[string]*templateEntry)}
}

// Observe masks msg, assigns a stable template id, and returns id + pattern.
// Raw msg must not be retained by the caller after this call.
func (s *TemplateStore) Observe(msg string) (id, pattern string, unclassifiable bool) {
	masked := MaskMessage(msg)
	if masked == "" || !HasFixedToken(masked) {
		return UnclassifiableID, "", true
	}
	// Drain-like: tokenize and collapse adjacent pure mask tokens already done by Mask.
	// Further: normalize "user=<*>" style is already stable. Exact masked string is the template.
	pattern = normalizeTemplate(masked)
	id = TemplateID(pattern)

	s.mu.Lock()
	defer s.mu.Unlock()
	if e, ok := s.byExact[pattern]; ok {
		e.Count++
		return e.ID, e.Pattern, false
	}
	if len(s.byExact) >= MaxTemplates {
		s.overflow++
		return OthersTemplateID, "outros", false
	}
	e := &templateEntry{ID: id, Pattern: pattern, Count: 1}
	s.byExact[pattern] = e
	s.order = append(s.order, pattern)
	return id, pattern, false
}

// Len returns distinct template count (excluding unclassifiable/others tracking).
func (s *TemplateStore) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.byExact)
}

// Overflow returns how many events fell into "others".
func (s *TemplateStore) Overflow() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.overflow
}

// Snapshot returns templates sorted by pattern for deterministic reports.
func (s *TemplateStore) Snapshot() []TemplateInfo {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]TemplateInfo, 0, len(s.byExact))
	for _, p := range s.order {
		e := s.byExact[p]
		out = append(out, TemplateInfo{ID: e.ID, Pattern: e.Pattern, Count: e.Count})
	}
	return out
}

// TemplateInfo is a masked template summary (never contains raw lines).
type TemplateInfo struct {
	ID      string `json:"id"`
	Pattern string `json:"pattern"`
	Count   int64  `json:"count"`
}

// TemplateID is a stable hash of the masked pattern (RNF-06).
func TemplateID(pattern string) string {
	sum := sha256.Sum256([]byte(pattern))
	return hex.EncodeToString(sum[:8])
}

func normalizeTemplate(masked string) string {
	toks := strings.Fields(masked)
	if len(toks) == 0 {
		return ""
	}
	// Drain depth-1 style: collapse runs of MaskToken.
	out := make([]string, 0, len(toks))
	prevMask := false
	for _, t := range toks {
		isMask := t == MaskToken
		if isMask && prevMask {
			continue
		}
		out = append(out, t)
		prevMask = isMask
	}
	return strings.Join(out, " ")
}
