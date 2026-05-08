package display

import (
	"fmt"
	"strings"
	"time"
)

// FormatDuration formats a duration as "Xms" or "X.XXs".
func FormatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return fmt.Sprintf("%.2fs", d.Seconds())
}

// TruncateCmd truncates a string to n characters, appending "..." if truncated.
func TruncateCmd(s string, n int) string {
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}

// CountLines counts the number of newlines in a string.
func CountLines(s string) int {
	return strings.Count(s, "\n")
}
