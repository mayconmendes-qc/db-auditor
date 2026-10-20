package loganomaly

import (
	"fmt"
	"testing"
)

func TestTemplateCapOthers(t *testing.T) {
	s := NewTemplateStore()
	// Distinct letter-only patterns so number masking does not collapse them.
	for i := 0; i < MaxTemplates; i++ {
		msg := fmt.Sprintf("fixed marker alpha event kind_%s happened", base26(i))
		id, _, un := s.Observe(msg)
		if un || id == OthersTemplateID {
			t.Fatalf("early overflow at %d id=%s", i, id)
		}
	}
	if s.Len() != MaxTemplates {
		t.Fatalf("len=%d", s.Len())
	}
	id, pat, un := s.Observe("fixed marker alpha event kind_overflowzz happened")
	if un || id != OthersTemplateID || pat != "outros" {
		t.Fatalf("id=%s pat=%s un=%v", id, pat, un)
	}
	if s.Overflow() != 1 {
		t.Fatalf("overflow=%d", s.Overflow())
	}
}

func base26(n int) string {
	if n == 0 {
		return "a"
	}
	var b []byte
	for n > 0 {
		b = append(b, byte('a'+n%26))
		n /= 26
	}
	return string(b)
}

func TestTemplateIDStable(t *testing.T) {
	a := TemplateID("connection authorized: user=<*> database=<*>")
	b := TemplateID("connection authorized: user=<*> database=<*>")
	if a != b || len(a) != 16 {
		t.Fatalf("%s vs %s", a, b)
	}
}
