package upgrade

import (
	"fmt"
	"strconv"
	"strings"
)

// Version ist eine Versionsangabe nach Semver mit führendem v, etwa v0.1.0
// oder v0.2.0-rc1. Build-Metadaten (+…) kommen in Tags nicht vor und werden
// nicht angenommen.
type Version struct {
	Major, Minor, Patch int
	// Pre ist das Suffix ohne Bindestrich; leer bei einem Release ohne Suffix.
	Pre string
	raw string
}

// ParseVersion liest eine Version der Form vX.Y.Z oder vX.Y.Z-suffix.
func ParseVersion(s string) (Version, error) {
	bad := func() (Version, error) {
		return Version{}, fmt.Errorf("keine gültige Version: %q (erwartet vX.Y.Z oder vX.Y.Z-suffix)", s)
	}
	rest, ok := strings.CutPrefix(s, "v")
	if !ok {
		return bad()
	}
	core, pre, hasPre := strings.Cut(rest, "-")
	if hasPre && !validPre(pre) {
		return bad()
	}
	parts := strings.Split(core, ".")
	if len(parts) != 3 {
		return bad()
	}
	var nums [3]int
	for i, p := range parts {
		n, ok := parseNumeric(p)
		if !ok {
			return bad()
		}
		nums[i] = n
	}
	return Version{Major: nums[0], Minor: nums[1], Patch: nums[2], Pre: pre, raw: s}, nil
}

// String liefert die Version so, wie sie gelesen wurde.
func (v Version) String() string { return v.raw }

// IsPrerelease sagt, ob die Version ein Suffix trägt.
func (v Version) IsPrerelease() bool { return v.Pre != "" }

// Compare liefert -1, 0 oder 1, je nachdem ob v kleiner, gleich oder größer
// als w ist — nach den Vorrangregeln von Semver 2.0.0.
func (v Version) Compare(w Version) int {
	for _, d := range [3]int{v.Major - w.Major, v.Minor - w.Minor, v.Patch - w.Patch} {
		if d != 0 {
			return sign(d)
		}
	}
	switch {
	case v.Pre == w.Pre:
		return 0
	case v.Pre == "":
		return 1 // ohne Suffix hat Vorrang vor jeder Vorabversion
	case w.Pre == "":
		return -1
	}
	a, b := strings.Split(v.Pre, "."), strings.Split(w.Pre, ".")
	for i := 0; i < len(a) && i < len(b); i++ {
		if c := comparePreIdent(a[i], b[i]); c != 0 {
			return c
		}
	}
	return sign(len(a) - len(b))
}

func comparePreIdent(a, b string) int {
	na, aNum := parseNumeric(a)
	nb, bNum := parseNumeric(b)
	switch {
	case aNum && bNum:
		return sign(na - nb)
	case aNum:
		return -1 // numerische Kennungen haben geringeren Vorrang
	case bNum:
		return 1
	}
	return strings.Compare(a, b)
}

// parseNumeric nimmt nur Ziffern ohne führende Null (außer „0“ selbst).
func parseNumeric(s string) (int, bool) {
	if s == "" || (len(s) > 1 && s[0] == '0') {
		return 0, false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, false
		}
	}
	n, err := strconv.Atoi(s)
	return n, err == nil
}

func validPre(s string) bool {
	if s == "" {
		return false
	}
	for _, ident := range strings.Split(s, ".") {
		if ident == "" {
			return false
		}
		for _, r := range ident {
			if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r == '-') {
				return false
			}
		}
	}
	return true
}

func sign(n int) int {
	switch {
	case n < 0:
		return -1
	case n > 0:
		return 1
	}
	return 0
}
