package display

// ANSI color codes — can be disabled via DisableColors().
var (
	ColorReset  = "\033[0m"
	ColorRed    = "\033[31m"
	ColorGreen  = "\033[32m"
	ColorYellow = "\033[33m"
	ColorGray   = "\033[90m"
	ColorBold   = "\033[1m"
)

// DisableColors sets all color variables to empty strings,
// effectively stripping ANSI codes from output.
func DisableColors() {
	ColorReset = ""
	ColorRed = ""
	ColorGreen = ""
	ColorYellow = ""
	ColorGray = ""
	ColorBold = ""
}
