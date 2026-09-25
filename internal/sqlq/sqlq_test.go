package sqlq

import "testing"

func TestBind(t *testing.T) {
	in := "SELECT value FROM t WHERE key = $1 AND x = '$2' AND y = $2"
	if got, want := Bind(SQLite, in), "SELECT value FROM t WHERE key = ?1 AND x = '$2' AND y = ?2"; got != want {
		t.Errorf("Bind(SQLite) = %q, erwartet %q", got, want)
	}
	if got := Bind(Postgres, in); got != in {
		t.Errorf("Bind(Postgres) = %q, erwartet unverändert", got)
	}
}

func TestCheck(t *testing.T) {
	for _, bad := range []string{
		"INSERT OR REPLACE INTO t VALUES ($1)",
		"insert  or ignore into t values ($1)",
		"REPLACE INTO t VALUES ($1)",
		"PRAGMA user_version",
		"CREATE TABLE t (id INTEGER PRIMARY KEY AUTOINCREMENT)",
		"SELECT name FROM sqlite_master",
		"SELECT value FROM t WHERE key = ?",
		"SELECT value FROM t WHERE key = ?1",
	} {
		if Check(bad) == nil {
			t.Errorf("Check(%q) hätte scheitern sollen", bad)
		}
	}
	if err := Check("INSERT INTO t (key, value) VALUES ($1, $2)"); err != nil {
		t.Errorf("Check: %v", err)
	}
}

func TestTexts(t *testing.T) {
	v := struct {
		A, B string
		n    int
	}{"a", "b", 1}
	got := Texts(&v)
	if len(got) != 2 || got["A"] != "a" || got["B"] != "b" {
		t.Errorf("Texts = %v", got)
	}
}
