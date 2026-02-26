package fuzz

import "gocv.io/x/gocv"

// FilterConfig is a human-readable description of the current state.
type FilterConfig struct {
	Name   string            `json:"name"`
	Params map[string]string `json:"params"`
}

// Filter is a stateful iterator over parameter variations.
type Filter interface {
	// Name returns the identifier of the filter.
	Name() string
	
	// Reset re-initializes the iterator to position 0.
	Reset()
	
	// Next advances to the next parameter variation.
	// Returns false when exhausted.
	Next() bool
	
	// Apply applies the current parameter variation to the image.
	// Returns a NEW Mat (caller must Close it).
	// If it fails, it can return an empty Mat.
	Apply(src gocv.Mat) gocv.Mat
	
	// Config returns the current parameter description.
	Config() FilterConfig
	
	// Total returns the total number of variations for this filter.
	Total() int
}
