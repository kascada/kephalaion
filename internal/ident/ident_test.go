package ident

import (
	"strings"
	"testing"
)

func TestCheckName(t *testing.T) {
	good := []string{"a", "team-x", "laptop.1", "0_a", strings.Repeat("a", 63), "sys", "mysystem"}
	for _, n := range good {
		if err := CheckName("Collection", n); err != nil {
			t.Errorf("%q: %v", n, err)
		}
	}
	bad := []string{"", "A", "Team", "-a", ".a", "_a", "a:b", "a b", "ä", strings.Repeat("a", 64),
		"system", "systemx", "SYSTEM", "System-a", "sYsTeM.1"}
	for _, n := range bad {
		if err := CheckName("Collection", n); err == nil {
			t.Errorf("%q hätte abgelehnt werden sollen", n)
		}
	}
	if err := CheckName("Node", "SYSTEM"); err == nil || !strings.Contains(err.Error(), "reserviert") {
		t.Errorf("SYSTEM: %v", err)
	}
}

func TestParseAddress(t *testing.T) {
	h, c, err := ParseAddress("privat:team-x")
	if err != nil || h != "privat" || c != "team-x" {
		t.Fatalf("ParseAddress = %q, %q, %v", h, c, err)
	}
	for _, a := range []string{"privat", "privat:", ":team", "Privat:team", "privat:Team", "privat:system", "a:b:c"} {
		if _, _, err := ParseAddress(a); err == nil {
			t.Errorf("%q hätte abgelehnt werden sollen", a)
		}
	}
	if Address("privat", "team-x") != "privat:team-x" {
		t.Error("Address")
	}
}

func TestToken(t *testing.T) {
	a, err := NewToken()
	if err != nil {
		t.Fatal(err)
	}
	b, _ := NewToken()
	if a == b {
		t.Fatal("zwei gleiche Token")
	}
	if !strings.HasPrefix(a, "keph_") || len(a) != len("keph_")+43 {
		t.Errorf("Token %q: Länge %d", a, len(a))
	}
	if err := CheckToken(a); err != nil {
		t.Errorf("CheckToken(neu): %v", err)
	}
	for _, bad := range []string{"", "keph_", "abc", "keph_abc", a + "A", "KEPH_" + a[5:], a[:len(a)-1] + "=", a[:len(a)-1] + "+"} {
		if err := CheckToken(bad); err == nil {
			t.Errorf("CheckToken(%q) hätte scheitern sollen", bad)
		} else if len(bad) > 10 && strings.Contains(err.Error(), bad) {
			t.Errorf("Fehler nennt das Token: %v", err)
		}
	}
	h := HashToken(a)
	if len(h) != 64 || h == a || HashToken(a) != h || HashToken(b) == h {
		t.Errorf("HashToken = %q", h)
	}
	m := MaskToken(a)
	if m != "keph_…"+a[len(a)-4:] || strings.Contains(m, a[5:len(a)-4]) {
		t.Errorf("MaskToken = %q", m)
	}
	if MaskToken("") != "" {
		t.Error("MaskToken(leer)")
	}
}
