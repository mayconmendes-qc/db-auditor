package loganomaly

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"
)

func TestMaskingRemovesEmailsIPsParams(t *testing.T) {
	corpus := []struct {
		msg     string
		needles []string
	}{
		{
			msg:     `connection authorized: user=alice database=app SSL from 192.168.1.50`,
			needles: []string{"alice", "app", "192.168.1.50"},
		},
		{
			msg:     `ERROR: duplicate key for email person@example.com id=4242`,
			needles: []string{"person@example.com", "4242"},
		},
		{
			msg:     `login from 2001:db8::1 path=/var/lib/postgresql/data/pg_wal`,
			needles: []string{"2001:db8::1", "/var/lib/postgresql/data/pg_wal"},
		},
		{
			msg:     `uuid=550e8400-e29b-41d4-a716-446655440000 hash=abcdef0123456789abcdef0123456789`,
			needles: []string{"550e8400-e29b-41d4-a716-446655440000", "abcdef0123456789abcdef0123456789"},
		},
	}
	for _, c := range corpus {
		got := MaskMessage(c.msg)
		if ContainsSensitiveLeak(got, c.needles...) {
			t.Fatalf("leak in %q → %q", c.msg, got)
		}
		if !HasFixedToken(got) && strings.Contains(c.msg, "connection authorized") {
			t.Fatalf("expected fixed tokens: %q", got)
		}
	}
}

func TestUnclassifiableAllVariable(t *testing.T) {
	got := MaskMessage("person@example.com")
	if HasFixedToken(got) {
		t.Fatalf("expected unclassifiable, got %q", got)
	}
	id, _, un := NewTemplateStore().Observe("person@example.com")
	if !un || id != UnclassifiableID {
		t.Fatalf("id=%s un=%v", id, un)
	}
}

func FuzzMasking(f *testing.F) {
	f.Add("user=alice host=10.0.0.1")
	f.Add("mail=a@b.co")
	f.Fuzz(func(t *testing.T, seed string) {
		r := rand.New(rand.NewSource(int64(len(seed)) + 42))
		email := fmt.Sprintf("user%d@example-%d.com", r.Intn(1e6), r.Intn(1e6))
		ip := fmt.Sprintf("%d.%d.%d.%d", r.Intn(256), r.Intn(256), r.Intn(256), r.Intn(256))
		msg := seed + " " + email + " " + ip + " user=secretrole"
		got := MaskMessage(msg)
		if strings.Contains(got, email) || strings.Contains(strings.ToLower(got), strings.ToLower(email)) {
			t.Fatalf("email survived: %s", got)
		}
		if strings.Contains(got, ip) {
			t.Fatalf("ip survived: %s", got)
		}
		if strings.Contains(got, "secretrole") {
			t.Fatalf("role param survived: %s", got)
		}
	})
}
