package display

import "testing"

func TestDisableColors(t *testing.T) {
	// Ensure colors are set before disabling
	if ColorReset == "" {
		t.Fatal("ColorReset should not be empty before DisableColors")
	}

	DisableColors()

	vars := map[string]string{
		"ColorReset":  ColorReset,
		"ColorRed":    ColorRed,
		"ColorGreen":  ColorGreen,
		"ColorYellow": ColorYellow,
		"ColorGray":   ColorGray,
		"ColorBold":   ColorBold,
	}

	for name, val := range vars {
		if val != "" {
			t.Errorf("%s should be empty after DisableColors, got %q", name, val)
		}
	}
}
