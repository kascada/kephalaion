// Package loopback ist neutral, für Hub und Node: die Prüfung, dass eine
// HTTP-Anfrage diesen Rechner meint. Solange serve nur auf Loopback lauscht,
// lassen beide Rollen nur Anfragen durch, deren Host localhost, 127.0.0.1
// oder [::1] mit dem Port ist, auf dem sie ankamen — so erreicht eine
// Webseite im Browser den Dienst nicht über DNS-Rebinding.
package loopback

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
)

var hosts = map[string]bool{"localhost": true, "127.0.0.1": true, "::1": true}

// IsHost sagt, ob host (ohne Port, ohne Klammern) dieser Rechner ist:
// localhost, 127.0.0.1 oder ::1, ohne Unterschied der Schreibweise.
func IsHost(host string) bool { return hosts[strings.ToLower(host)] }

// CheckHost verlangt als Host localhost, 127.0.0.1 oder [::1] mit dem Port,
// auf dem die Anfrage ankam. Ein Tunnel geht damit nur mit gleichem Port
// (ssh -L 8080:localhost:8080).
func CheckHost(r *http.Request) error {
	host, port, err := net.SplitHostPort(r.Host)
	if err != nil || !IsHost(host) {
		return fmt.Errorf("Host %q ist nicht dieser Rechner", r.Host)
	}
	local, ok := r.Context().Value(http.LocalAddrContextKey).(net.Addr)
	if !ok || local == nil {
		return errors.New("eigene Adresse unbekannt")
	}
	_, ownPort, err := net.SplitHostPort(local.String())
	if err != nil || ownPort != port {
		return fmt.Errorf("Host %q nennt nicht den Port, auf dem die Anfrage ankam", r.Host)
	}
	return nil
}

// Forbid beantwortet eine Anfrage, die eine Prüfung nicht bestand, mit 403.
func Forbid(w http.ResponseWriter, err error) {
	http.Error(w, "Forbidden: "+err.Error(), http.StatusForbidden)
}

// Guard lässt nur Anfragen durch, die CheckHost bestehen; sonst 403.
func Guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := CheckHost(r); err != nil {
			Forbid(w, err)
			return
		}
		next.ServeHTTP(w, r)
	})
}
