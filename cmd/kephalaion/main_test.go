package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunHelpAndVersion(t *testing.T) {
	cases := []struct {
		args []string
		code int
		want string
	}{
		{nil, 0, "Kommandos:"},
		{[]string{"help"}, 0, "Kommandos:"},
		{[]string{"version"}, 0, "Plattform:"},
	}
	for _, c := range cases {
		var out, errOut bytes.Buffer
		if got := run(c.args, &out, &errOut); got != c.code {
			t.Errorf("run(%v) = %d, erwartet %d", c.args, got, c.code)
		}
		if !strings.Contains(out.String(), c.want) {
			t.Errorf("run(%v): Ausgabe ohne %q:\n%s", c.args, c.want, out.String())
		}
	}
}

func TestRunUnknown(t *testing.T) {
	var out, errOut bytes.Buffer
	if got := run([]string{"gibtsnicht"}, &out, &errOut); got != 2 {
		t.Fatalf("Exit-Code %d, erwartet 2", got)
	}
	if !strings.Contains(errOut.String(), "Unbekanntes Kommando: gibtsnicht") {
		t.Fatalf("Fehlermeldung fehlt:\n%s", errOut.String())
	}
}
