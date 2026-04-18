package llm

// Float64Ptr returns a pointer to v (for optional Request.Temperature / provider params).
func Float64Ptr(v float64) *float64 {
	return &v
}

// IntPtr returns a pointer to v (for optional Request.MaxTokens).
func IntPtr(v int) *int {
	return &v
}
