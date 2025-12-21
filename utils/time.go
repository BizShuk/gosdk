package utils

import (
	"fmt"
	"time"
)

// ParseHHMMSS converts a string of "HH:MM:SS" format to a time.Duration.
func ParseTimeDuration(s string) (time.Duration, error) {
	var hours, minutes, seconds int
	// Use Sscanf to parse the hours, minutes, and seconds from the string.
	_, err := fmt.Sscanf(s, "%d:%d:%d", &hours, &minutes, &seconds)
	if err != nil {
		return 0, fmt.Errorf("failed to parse HH:MM:SS string: %w", err)
	}

	// Calculate the total duration in seconds and convert to time.Duration.
	duration := time.Duration(hours)*time.Hour +
		time.Duration(minutes)*time.Minute +
		time.Duration(seconds)*time.Second

	return duration, nil
}
