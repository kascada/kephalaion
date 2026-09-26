// Package buildinfo trägt die Werte, die beim Bauen in das Binary gestempelt
// werden.
package buildinfo

import "runtime"

// DevVersion ist die Version eines Binarys, das nicht aus einem Release stammt.
const DevVersion = "dev"

// Version und Commit werden beim Bauen über -ldflags gesetzt:
//
//	-X github.com/kephalaion/kephalaion/internal/buildinfo.Version=v0.1.0
//	-X github.com/kephalaion/kephalaion/internal/buildinfo.Commit=…
//
// Ein Ad-hoc-`go build` ohne diese Flags ergibt einen dev build.
var (
	Version = DevVersion
	Commit  = ""
)

// Info fasst zusammen, was `kephalaion version` ausgibt.
type Info struct {
	Version   string
	Commit    string
	GoVersion string
	OS        string
	Arch      string
}

// Get liefert die Werte dieses Binarys. Eine leer gestempelte Version zählt
// als dev build, damit `upgrade` sie nie für ein Release hält.
func Get() Info {
	v := Version
	if v == "" {
		v = DevVersion
	}
	return Info{
		Version:   v,
		Commit:    Commit,
		GoVersion: runtime.Version(),
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
	}
}

// IsDev sagt, ob dieses Binary ein dev build ist.
func (i Info) IsDev() bool {
	return i.Version == DevVersion
}

// Platform liefert `<os>/<arch>`.
func (i Info) Platform() string {
	return i.OS + "/" + i.Arch
}
