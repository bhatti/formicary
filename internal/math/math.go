package math

import "time"

// Min function for two integers
func Min(a, b int) int {
	if a <= b {
		return a
	}
	return b
}

// Max function for two integers
func Max(a, b int) int {
	if a >= b {
		return a
	}
	return b
}

// Min64 function for two integers
func Min64(a, b int64) int64 {
	if a <= b {
		return a
	}
	return b
}

// Max64 function for two integers
func Max64(a, b int64) int64 {
	if a >= b {
		return a
	}
	return b
}

// MinDuration function for two duration
func MinDuration(a, b time.Duration) time.Duration {
	if a <= b {
		return a
	}
	return b
}

// MaxDuration function for two duration
func MaxDuration(a, b time.Duration) time.Duration {
	if a >= b {
		return a
	}
	return b
}
