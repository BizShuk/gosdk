package utils

import (
	"testing"
	"time"
)

func TestParseTimeDuration(t *testing.T) {
	tests := []struct {
		input    string
		expected time.Duration
		wantErr  bool
	}{
		{
			input:    "01:00:00",
			expected: 1 * time.Hour,
			wantErr:  false,
		},
		{
			input:    "00:05:30",
			expected: 5*time.Minute + 30*time.Second,
			wantErr:  false,
		},
		{
			input:    "00:00:15",
			expected: 15 * time.Second,
			wantErr:  false,
		},
		{
			input:    "invalid",
			expected: 0,
			wantErr:  true,
		},
		{
			input:    "12:abc:34",
			expected: 0,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseTimeDuration(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseTimeDuration() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.expected {
				t.Errorf("ParseTimeDuration() = %v, expected %v", got, tt.expected)
			}
		})
	}
}
