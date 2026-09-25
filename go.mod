module github.com/kascada/kephalaion

go 1.26

// Festgenagelt: lokal und in CI baut genau diese Toolchain. CI läuft mit
// GOTOOLCHAIN=local und prüft sie, bevor es baut; lokal lädt GOTOOLCHAIN=auto
// sie bei Bedarf nach.
toolchain go1.27.1
