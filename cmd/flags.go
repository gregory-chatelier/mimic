package cmd

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// parseDurationWithDays parses a duration string, supporting Go's standard units
// plus 'd' for days (e.g., '24h', '1d', '30m')
func parseDurationWithDays(s string) (time.Duration, error) {
	// Replace 'd' with hours equivalent (e.g., "1d" -> "24h")
	if strings.Contains(s, "d") {
		// Use a simple regex to find 'd' suffixes
		parts := strings.Split(s, "d")
		if len(parts) == 2 && parts[1] == "" {
			// Format: "Nd" where N is a number
			numStr := strings.TrimSpace(parts[0])
			num, err := strconv.ParseFloat(numStr, 64)
			if err != nil {
				return 0, fmt.Errorf("invalid number in duration: %s", s)
			}
			s = fmt.Sprintf("%fh", num*24)
		}
	}
	return time.ParseDuration(s)
}
