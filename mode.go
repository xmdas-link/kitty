package kitty

import "os"

// EnvKittyMode indicates environment name for kitty mode.
const EnvKittyMode = "KITTY_MODE"

const (
	// DebugMode indicates kitty mode is debug.
	DebugMode = "debug"
	// ReleaseMode indicates kitty mode is release.
	ReleaseMode = "release"
	// TestMode indicates kitty mode is test.
	TestMode = "test"
)

const (
	debugCode = iota
	releaseCode
	testCode
)

var kittyMode = debugCode
var modeName = DebugMode

func init() {
	mode := os.Getenv(EnvKittyMode)
	SetMode(mode)
}

// SetMode sets kitty mode according to input string.
func SetMode(value string) {
	switch value {
	case DebugMode, "":
		kittyMode = debugCode
	case ReleaseMode:
		kittyMode = releaseCode
	case TestMode:
		kittyMode = testCode
	default:
		panic("kitty mode unknown: " + value)
	}
	if value == "" {
		value = DebugMode
	}
	modeName = value
}
