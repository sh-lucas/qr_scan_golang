package fuzz

import (
	"encoding/json"
	"strings"

	"gocv.io/x/gocv"
)

// Pipeline represents an ordered sequence of filters
type Pipeline struct {
	Filters []Filter
}

// Configs returns the JSON representation of current filter configurations
func (p *Pipeline) Configs() string {
	var confs []FilterConfig
	for _, f := range p.Filters {
		confs = append(confs, f.Config())
	}
	b, _ := json.Marshal(confs)
	return string(b)
}

// Names returns the pipe-separated names of the filters
func (p *Pipeline) Names() string {
	var names []string
	for _, f := range p.Filters {
		names = append(names, f.Name())
	}
	return strings.Join(names, "|")
}

// Apply runs the image through all filters in sequence.
// Returns a new Mat (must be closed by caller) and a boolean indicating success of the pipeline (false if a filter failed and returned empty).
func (p *Pipeline) Apply(src gocv.Mat) (gocv.Mat, bool) {
	current := src

	for i, f := range p.Filters {
		next := f.Apply(current)
		// If it's not the original source, close the intermediate mat.
		// Note: do NOT guard with !current.Empty() – a non-nil native cv::Mat
		// pointer exists even when the Mat is logically empty, so it must be
		// closed to avoid a memory leak.
		if i > 0 {
			current.Close()
		}
		if next.Empty() {
			next.Close()
			return gocv.NewMat(), false
		}
		current = next
	}

	// If the pipeline is empty, current == src.
	// But we must return a *new* mat or a clone so the caller always closes it.
	if len(p.Filters) == 0 {
		clone := gocv.NewMat()
		src.CopyTo(&clone)
		return clone, true
	}

	return current, true
}

func (p *Pipeline) ResetAll() {
	for _, f := range p.Filters {
		f.Reset()
	}
}

// Next advances the cartesian product of parameters for this pipeline.
func (p *Pipeline) Next() bool {
	if len(p.Filters) == 0 {
		return false
	}
	
	// Try to advance from the last filter backwards
	for i := len(p.Filters) - 1; i >= 0; i-- {
		if p.Filters[i].Next() {
			return true // Successfully advanced this level, we keep previous levels as they are
		}
		
		// If this filter exhausted its options, we reset it and let the loop continue to advance the *previous* filter
		p.Filters[i].Reset()
		p.Filters[i].Next() // Put it at the first valid state
		
		// If i == 0, we exhausted all combinations for the entire pipeline
		if i == 0 {
			return false
		}
	}
	return false
}

// FilterFactory is a function that creates a fresh instance of a filter
type FilterFactory func() Filter

// GeneratePipelines creates all permutations of length 1 to maxLength using the provided factories.
// The resulting slice is ordered by pipeline length (Breadth-First Search order).
func GeneratePipelines(factories map[string]FilterFactory, maxLength int) []Pipeline {
	var results []Pipeline
	for length := 1; length <= maxLength; length++ {
		results = append(results, GeneratePipelinesOfLength(factories, length)...)
	}
	return results
}

// GeneratePipelinesOfLength creates all permutations of exactly `length` filters
// using the provided factories. Each filter appears at most once per pipeline.
func GeneratePipelinesOfLength(factories map[string]FilterFactory, length int) []Pipeline {
	var results []Pipeline
	var backtrack func(path []string)
	backtrack = func(path []string) {
		if len(path) == length {
			pipelineFilters := make([]Filter, len(path))
			for i, name := range path {
				pipelineFilters[i] = factories[name]()
			}
			results = append(results, Pipeline{Filters: pipelineFilters})
			return
		}
		for name := range factories {
			contains := false
			for _, pName := range path {
				if pName == name {
					contains = true
					break
				}
			}
			if !contains {
				newPath := make([]string, len(path))
				copy(newPath, path)
				backtrack(append(newPath, name))
			}
		}
	}
	backtrack([]string{})
	return results
}
