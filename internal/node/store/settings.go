package store

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/kephalaion/kephalaion/internal/sqlitedb"
)

// Schlüssel der settings des Nodes, die er kennt und prüft.
const (
	// SettingSyncInterval ist der Abstand des Abgleichs im Hintergrund, eine
	// Go-Dauer (30s, 2m); 0 schaltet ihn ab.
	SettingSyncInterval = "sync_interval"
)

// Grenzen des Abstands.
const (
	DefaultSyncInterval = 30 * time.Second
	MinSyncInterval     = time.Second
)

// ErrUnknownSetting meldet einen Schlüssel, den der Node nicht kennt.
var ErrUnknownSetting = errors.New("unbekannter Schlüssel")

// settingChecks prüft den Wert je bekanntem Schlüssel.
var settingChecks = map[string]func(string) error{
	SettingSyncInterval: func(v string) error { _, err := ParseSyncInterval(v); return err },
}

// SettingKeys sind die bekannten Schlüssel, sortiert.
func SettingKeys() []string {
	keys := make([]string, 0, len(settingChecks))
	for k := range settingChecks {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// CheckSetting prüft Schlüssel und Wert, wie config set sie verlangt.
func CheckSetting(key, value string) error {
	check, ok := settingChecks[key]
	if !ok {
		return fmt.Errorf("%w %q; der Node kennt: %v", ErrUnknownSetting, key, SettingKeys())
	}
	if err := check(value); err != nil {
		return fmt.Errorf("%s: %w", key, err)
	}
	return nil
}

// ParseSyncInterval liest den Abstand: eine Go-Dauer ab 1s, oder 0 für
// „aus“.
func ParseSyncInterval(v string) (time.Duration, error) {
	if v == "0" {
		return 0, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0, fmt.Errorf("%q ist keine Dauer; erwartet etwa 30s, 2m oder 0 (aus)", v)
	}
	if d == 0 {
		return 0, nil
	}
	if d < MinSyncInterval {
		return 0, fmt.Errorf("%s ist kürzer als %s; 0 schaltet den Abgleich im Hintergrund ab", d, MinSyncInterval)
	}
	return d, nil
}

// SyncInterval liest den Abstand aus den settings; fehlt er, gilt
// DefaultSyncInterval. Ein ungültiger Wert — etwa aus einem Import — ist ein
// Fehler, der Aufrufer entscheidet, was dann gilt.
func SyncInterval(settings map[string]string) (time.Duration, error) {
	v, ok := settings[SettingSyncInterval]
	if !ok {
		return DefaultSyncInterval, nil
	}
	d, err := ParseSyncInterval(v)
	if err != nil {
		return DefaultSyncInterval, fmt.Errorf("%s: %w", SettingSyncInterval, err)
	}
	return d, nil
}

func (s *sqliteStore) SetSetting(ctx context.Context, key, value string) error {
	if err := CheckSetting(key, value); err != nil {
		return err
	}
	return sqlitedb.SetSetting(ctx, s.db, key, value)
}

func (s *sqliteStore) UnsetSetting(ctx context.Context, key string) (bool, error) {
	if _, ok := settingChecks[key]; !ok {
		return false, fmt.Errorf("%w %q; der Node kennt: %v", ErrUnknownSetting, key, SettingKeys())
	}
	return sqlitedb.UnsetSetting(ctx, s.db, key)
}
