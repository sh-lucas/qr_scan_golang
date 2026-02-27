// dryrun simulates the fuzzing pipeline without any images or OpenCV.
// Prints every (pipeline, param-combo) the real fuzzer would test.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"strings"
)

// ── types ─────────────────────────────────────────────────────────────────

type FilterConfig struct {
Name   string            `json:"name"`
Params map[string]string `json:"params"`
}

type Filter interface {
Name() string
Reset()
Next() bool
Config() FilterConfig
Total() int
}

type FilterFactory func() Filter

// ── Pipeline ──────────────────────────────────────────────────────────────

type Pipeline struct{ Filters []Filter }

func (p *Pipeline) Names() string {
ns := make([]string, len(p.Filters))
for i, f := range p.Filters {
ns[i] = f.Name()
}
return strings.Join(ns, "|")
}

func (p *Pipeline) Configs() string {
cs := make([]FilterConfig, len(p.Filters))
for i, f := range p.Filters {
cs[i] = f.Config()
}
b, _ := json.Marshal(cs)
return string(b)
}

func (p *Pipeline) ResetAll() {
for _, f := range p.Filters {
f.Reset()
}
}

// Next mirrors fuzz.Pipeline.Next exactly.
func (p *Pipeline) Next() bool {
for i := len(p.Filters) - 1; i >= 0; i-- {
if p.Filters[i].Next() {
return true
}
p.Filters[i].Reset()
p.Filters[i].Next()
if i == 0 {
return false
}
}
return false
}

// ── Filter implementations ────────────────────────────────────────────────

type BilateralFilter struct {
step                       int
dMin, dMax                 int
sigmaMin, sigmaMax         float64
currD                      int
currSigma                  float64
started                    bool
}

func NewBilateralFilter(step int) *BilateralFilter {
return &BilateralFilter{step: step, dMin: 3, dMax: 7, sigmaMin: 10, sigmaMax: 50, currD: 3, currSigma: 10}
}
func (f *BilateralFilter) Name() string { return "Bilateral" }
func (f *BilateralFilter) Reset() {
f.started = false
f.currD, f.currSigma = f.dMin, f.sigmaMin
}
func (f *BilateralFilter) Total() int {
return (((f.dMax-f.dMin)/(2*f.step))+1) * (int((f.sigmaMax-f.sigmaMin)/(15*float64(f.step)))+1)
}
func (f *BilateralFilter) Config() FilterConfig {
return FilterConfig{Name: f.Name(), Params: map[string]string{
"d": fmt.Sprintf("%d", f.currD), "sigma": fmt.Sprintf("%.1f", f.currSigma),
}}
}
func (f *BilateralFilter) Next() bool {
if !f.started {
f.started = true
return true
}
f.currSigma += 15 * float64(f.step)
if f.currSigma > f.sigmaMax {
f.currSigma = f.sigmaMin
f.currD += 2 * f.step
return f.currD <= f.dMax
}
return true
}

type GammaFilter struct {
step                   int
gammaMin, gammaMax     float64
currGamma              float64
started                bool
}

func NewGammaFilter(step int) *GammaFilter {
return &GammaFilter{step: step, gammaMin: 0.6, gammaMax: 1.8, currGamma: 0.6}
}
func (f *GammaFilter) Name() string { return "Gamma" }
func (f *GammaFilter) Reset()       { f.started = false; f.currGamma = f.gammaMin }
func (f *GammaFilter) Total() int   { return int((f.gammaMax-f.gammaMin)/(0.3*float64(f.step))) + 1 }
func (f *GammaFilter) Config() FilterConfig {
return FilterConfig{Name: f.Name(), Params: map[string]string{"gamma": fmt.Sprintf("%.2f", f.currGamma)}}
}
func (f *GammaFilter) Next() bool {
if !f.started {
f.started = true
return true
}
f.currGamma += 0.4 * float64(f.step)
return f.currGamma <= f.gammaMax
}

type CLAHEFilter struct {
step                   int
clipMin, clipMax       float64
tileMin, tileMax       int
currClip               float64
currTile               int
started                bool
}

func NewCLAHEFilter(step int) *CLAHEFilter {
return &CLAHEFilter{step: step, clipMin: 1, clipMax: 3.5, tileMin: 4, tileMax: 12, currClip: 1, currTile: 4}
}
func (f *CLAHEFilter) Name() string      { return "CLAHE" }
func (f *CLAHEFilter) clipStep() float64 { return 0.5 * float64(f.step) }
func (f *CLAHEFilter) tileStep() int     { return 4 * f.step }
func (f *CLAHEFilter) Reset() {
f.started = false
f.currClip, f.currTile = f.clipMin, f.tileMin
}
func (f *CLAHEFilter) Total() int {
c := int((f.clipMax-f.clipMin)/f.clipStep()) + 1
t := ((f.tileMax - f.tileMin) / f.tileStep()) + 1
if c < 1 { c = 1 }
if t < 1 { t = 1 }
return c * t
}
func (f *CLAHEFilter) Config() FilterConfig {
return FilterConfig{Name: f.Name(), Params: map[string]string{
"clipLimit": fmt.Sprintf("%.1f", f.currClip),
"tileSize":  fmt.Sprintf("%dx%d", f.currTile, f.currTile),
}}
}
func (f *CLAHEFilter) Next() bool {
if !f.started {
f.started = true
return true
}
f.currTile += f.tileStep()
if f.currTile > f.tileMax {
f.currTile = f.tileMin
f.currClip += f.clipStep()
return f.currClip <= f.clipMax
}
return true
}

type ResizeFilter struct {
step                   int
scaleMin, scaleMax     float64
currScale              float64
started                bool
}

func NewResizeFilter(step int) *ResizeFilter {
return &ResizeFilter{step: step, scaleMin: 0.5, scaleMax: 2.0, currScale: 0.5}
}
func (f *ResizeFilter) Name() string       { return "Resize" }
func (f *ResizeFilter) scaleStep() float64 { return 0.25 * float64(f.step) }
func (f *ResizeFilter) Reset()             { f.started = false; f.currScale = f.scaleMin }
func (f *ResizeFilter) Total() int         { return int((f.scaleMax-f.scaleMin)/f.scaleStep()) + 1 }
func (f *ResizeFilter) Config() FilterConfig {
return FilterConfig{Name: f.Name(), Params: map[string]string{"scale": fmt.Sprintf("%.2f", f.currScale)}}
}
func (f *ResizeFilter) Next() bool {
if !f.started {
f.started = true
if f.currScale == 1.0 {
f.currScale += f.scaleStep()
}
return f.currScale <= f.scaleMax
}
f.currScale += f.scaleStep()
if f.currScale == 1.0 {
f.currScale += f.scaleStep()
}
return f.currScale <= f.scaleMax
}

type SharpenFilter struct {
step                   int
alphaMin, alphaMax     float64
currAlpha              float64
started                bool
}

func NewSharpenFilter(step int) *SharpenFilter {
return &SharpenFilter{step: step, alphaMin: 1.0, alphaMax: 2.0, currAlpha: 1.0}
}
func (f *SharpenFilter) Name() string      { return "Sharpen" }
func (f *SharpenFilter) stepSize() float64 { return 0.5 * float64(f.step) }
func (f *SharpenFilter) Reset()            { f.started = false; f.currAlpha = f.alphaMin }
func (f *SharpenFilter) Total() int        { return int((f.alphaMax-f.alphaMin)/f.stepSize()) + 1 }
func (f *SharpenFilter) Config() FilterConfig {
return FilterConfig{Name: f.Name(), Params: map[string]string{"alpha": fmt.Sprintf("%.2f", f.currAlpha)}}
}
func (f *SharpenFilter) Next() bool {
if !f.started {
f.started = true
return true
}
f.currAlpha += f.stepSize()
return f.currAlpha <= f.alphaMax
}

type DilationFilter struct {
step                   int
sizeMin, sizeMax       int
currSize               int
started                bool
}

func NewDilationFilter(step int) *DilationFilter {
return &DilationFilter{step: step, sizeMin: 3, sizeMax: 5, currSize: 3}
}
func (f *DilationFilter) Name() string { return "Dilation" }
func (f *DilationFilter) Reset()       { f.started = false; f.currSize = f.sizeMin }
func (f *DilationFilter) Total() int   { return ((f.sizeMax-f.sizeMin)/(2*f.step)) + 1 }
func (f *DilationFilter) Config() FilterConfig {
return FilterConfig{Name: f.Name(), Params: map[string]string{"kernelSize": fmt.Sprintf("%d", f.currSize)}}
}
func (f *DilationFilter) Next() bool {
if !f.started {
f.started = true
return true
}
f.currSize += 2 * f.step
return f.currSize <= f.sizeMax
}

type ClosingFilter struct {
step                   int
sizeMin, sizeMax       int
currSize               int
started                bool
}

func NewClosingFilter(step int) *ClosingFilter {
return &ClosingFilter{step: step, sizeMin: 3, sizeMax: 9, currSize: 3}
}
func (f *ClosingFilter) Name() string { return "Closing" }
func (f *ClosingFilter) Reset()       { f.started = false; f.currSize = f.sizeMin }
func (f *ClosingFilter) Total() int   { return ((f.sizeMax-f.sizeMin)/(2*f.step)) + 1 }
func (f *ClosingFilter) Config() FilterConfig {
return FilterConfig{Name: f.Name(), Params: map[string]string{"kernelSize": fmt.Sprintf("%d", f.currSize)}}
}
func (f *ClosingFilter) Next() bool {
if !f.started {
f.started = true
return true
}
f.currSize += 2 * f.step
return f.currSize <= f.sizeMax
}

type AdaptiveThresholdFilter struct {
step               int
blockMin, blockMax int
cMin, cMax         float32
currBlock          int
currC              float32
currMethod         int // 0=Mean 1=Gaussian
started            bool
}

func NewAdaptiveThresholdFilter(step int) *AdaptiveThresholdFilter {
return &AdaptiveThresholdFilter{step: step, blockMin: 3, blockMax: 21, cMin: 2, cMax: 10, currBlock: 3, currC: 2}
}
func (f *AdaptiveThresholdFilter) Name() string   { return "AdaptiveThreshold" }
func (f *AdaptiveThresholdFilter) blockStep() int { return 2 * f.step }
func (f *AdaptiveThresholdFilter) cStep() float32 { return 2 * float32(f.step) }
func (f *AdaptiveThresholdFilter) Reset() {
f.started = false
f.currBlock, f.currC, f.currMethod = f.blockMin, f.cMin, 0
}
func (f *AdaptiveThresholdFilter) Total() int {
b := ((f.blockMax-f.blockMin)/f.blockStep()) + 1
c := int((f.cMax-f.cMin)/f.cStep()) + 1
if b < 1 { b = 1 }
if c < 1 { c = 1 }
return b * c * 2
}
func (f *AdaptiveThresholdFilter) Config() FilterConfig {
m := "Mean"
if f.currMethod == 1 { m = "Gaussian" }
return FilterConfig{Name: f.Name(), Params: map[string]string{
"blockSize": fmt.Sprintf("%d", f.currBlock),
"C":         fmt.Sprintf("%.1f", f.currC),
"method":    m,
}}
}
func (f *AdaptiveThresholdFilter) Next() bool {
if !f.started {
f.started = true
return true
}
f.currC += f.cStep()
if f.currC > f.cMax {
f.currC = f.cMin
f.currBlock += f.blockStep()
if f.currBlock > f.blockMax {
f.currBlock = f.blockMin
if f.currMethod == 0 {
f.currMethod = 1
} else {
return false
}
}
}
return true
}

// ── helpers ───────────────────────────────────────────────────────────────

func buildFactories(step int) map[string]FilterFactory {
return map[string]FilterFactory{
"Bilateral":         func() Filter { return NewBilateralFilter(step) },
"CLAHE":             func() Filter { return NewCLAHEFilter(step) },
"Resize":            func() Filter { return NewResizeFilter(step) },
"Sharpen":           func() Filter { return NewSharpenFilter(step) },
"Gamma":             func() Filter { return NewGammaFilter(step) },
"Dilation":          func() Filter { return NewDilationFilter(step) },
"Closing":           func() Filter { return NewClosingFilter(step) },
"AdaptiveThreshold": func() Filter { return NewAdaptiveThresholdFilter(step) },
}
}

func generatePipelinesOfLength(factories map[string]FilterFactory, length int) []Pipeline {
var results []Pipeline
var bt func(path []string)
bt = func(path []string) {
if len(path) == length {
filters := make([]Filter, len(path))
for i, n := range path {
filters[i] = factories[n]()
}
results = append(results, Pipeline{Filters: filters})
return
}
for name := range factories {
found := false
for _, p := range path {
if p == name {
found = true
break
}
}
if !found {
cp := make([]string, len(path))
copy(cp, path)
bt(append(cp, name))
}
}
}
bt([]string{})
return results
}

func comboCount(p Pipeline) int {
n := 1
for _, f := range p.Filters {
n *= f.Total()
}
return n
}

func fmtInt(n int) string {
switch {
case n >= 1_000_000:
return fmt.Sprintf("%d,%03d,%03d", n/1_000_000, (n/1000)%1000, n%1000)
case n >= 1_000:
return fmt.Sprintf("%d,%03d", n/1000, n%1000)
default:
return fmt.Sprintf("%d", n)
}
}

// ── main ──────────────────────────────────────────────────────────────────

func main() {
step      := flag.Int("step", 2, "Step granularity (mirrors -step in the fuzzer)")
maxLength := flag.Int("max-length", 2, "Max pipeline length (mirrors -max-length)")
verbose   := flag.Bool("verbose", false, "Print every individual param combination")
flag.Parse()

factories := buildFactories(*step)
grandTotal := 0

for length := 1; length <= *maxLength; length++ {
pipelines := generatePipelinesOfLength(factories, length)
phaseTotal := 0

fmt.Printf("\n=== PHASE %d  (%d pipeline permutations) ===\n", length, len(pipelines))

for _, pipe := range pipelines {
combos := comboCount(pipe)
phaseTotal += combos
fmt.Printf("  %-45s  %s combos\n", pipe.Names(), fmtInt(combos))

if *verbose {
pipe.ResetAll()
i := 0
for pipe.Next() {
i++
fmt.Printf("    [%4d] %s\n", i, pipe.Configs())
}
}
}

fmt.Printf("--- Phase %d total: %s combinations\n", length, fmtInt(phaseTotal))
grandTotal += phaseTotal
}

fmt.Printf("\nGrand total: %s combinations\n", fmtInt(grandTotal))

// Per-filter breakdown
fmt.Printf("\n--- Per-filter variant count (step=%d) ---\n", *step)
for _, name := range []string{"Bilateral", "CLAHE", "Resize", "Sharpen", "Gamma", "Dilation", "Closing", "AdaptiveThreshold"} {
fmt.Printf("  %-20s %s variants\n", name, fmtInt(factories[name]().Total()))
}

// Max resize scale warning
r := NewResizeFilter(*step)
r.Reset()
maxScale := 0.0
for r.Next() {
var s float64
fmt.Sscanf(r.Config().Params["scale"], "%f", &s)
maxScale = math.Max(maxScale, s)
}
fmt.Printf("\nMax Resize scale: %.2fx  →  1920px source becomes %dpx\n", maxScale, int(1920*maxScale))
}
