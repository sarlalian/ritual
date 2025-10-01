// ABOUTME: Tests for core types validation functions
// ABOUTME: Validates concurrency constraints and error handling

package types

import (
	"strings"
	"testing"
)

func TestValidateConcurrency(t *testing.T) {
	tests := []struct {
		name        string
		value       int
		expected    int
		shouldError bool
		errorText   string
	}{
		{
			name:        "Zero value returns default",
			value:       0,
			expected:    DefaultConcurrency,
			shouldError: false,
		},
		{
			name:        "Valid minimum value",
			value:       MinConcurrency,
			expected:    MinConcurrency,
			shouldError: false,
		},
		{
			name:        "Valid maximum value",
			value:       MaxConcurrency,
			expected:    MaxConcurrency,
			shouldError: false,
		},
		{
			name:        "Valid middle value",
			value:       50,
			expected:    50,
			shouldError: false,
		},
		{
			name:        "Negative value returns error",
			value:       -1,
			expected:    0,
			shouldError: true,
			errorText:   "must be at least",
		},
		{
			name:        "Below minimum returns error",
			value:       0,
			expected:    DefaultConcurrency,
			shouldError: false,
		},
		{
			name:        "Above maximum returns error",
			value:       MaxConcurrency + 1,
			expected:    0,
			shouldError: true,
			errorText:   "cannot exceed",
		},
		{
			name:        "Way above maximum returns error",
			value:       10000,
			expected:    0,
			shouldError: true,
			errorText:   "cannot exceed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ValidateConcurrency(tt.value)

			if tt.shouldError {
				if err == nil {
					t.Errorf("Expected error for value %d, got nil", tt.value)
				} else if tt.errorText != "" && !strings.Contains(err.Error(), tt.errorText) {
					t.Errorf("Expected error to contain '%s', got: %v", tt.errorText, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error for value %d, got: %v", tt.value, err)
				}
			}

			if result != tt.expected {
				t.Errorf("Expected result %d, got %d", tt.expected, result)
			}
		})
	}
}

func TestConcurrencyConstants(t *testing.T) {
	// Verify constants have sensible values
	if MinConcurrency < 1 {
		t.Errorf("MinConcurrency must be at least 1, got %d", MinConcurrency)
	}

	if MaxConcurrency < MinConcurrency {
		t.Errorf("MaxConcurrency (%d) must be >= MinConcurrency (%d)", MaxConcurrency, MinConcurrency)
	}

	if DefaultConcurrency < MinConcurrency || DefaultConcurrency > MaxConcurrency {
		t.Errorf("DefaultConcurrency (%d) must be between Min (%d) and Max (%d)",
			DefaultConcurrency, MinConcurrency, MaxConcurrency)
	}

	// Verify reasonable default
	if DefaultConcurrency < 1 || DefaultConcurrency > 100 {
		t.Errorf("DefaultConcurrency (%d) seems unreasonable", DefaultConcurrency)
	}
}
