package fuzz

import (
	"fmt"
	"image"
	"math"

	"gocv.io/x/gocv"
)

// BilateralFilter: A non-linear, edge-preserving, and noise-reducing smoothing filter.
type BilateralFilter struct {
	step      int
	dMin      int
	dMax      int
	sigmaMin  float64
	sigmaMax  float64
	currD     int
	currSigma float64
	started   bool
}

func NewBilateralFilter(step int) *BilateralFilter {
	if step < 1 {
		step = 1
	}
	return &BilateralFilter{
		step:      step,
		dMin:      3,
		dMax:      7,
		sigmaMin:  10.0,
		sigmaMax:  50.0,
		currD:     3,
		currSigma: 10.0,
	}
}
func (f *BilateralFilter) Name() string { return "Bilateral" }
func (f *BilateralFilter) Reset() {
	f.started = false
	f.currD = f.dMin
	f.currSigma = f.sigmaMin
}
func (f *BilateralFilter) Total() int {
	dSteps := ((f.dMax - f.dMin) / (2 * f.step)) + 1
	sSteps := int((f.sigmaMax-f.sigmaMin)/(15.0*float64(f.step))) + 1
	return dSteps * sSteps
}
func (f *BilateralFilter) Config() FilterConfig {
	return FilterConfig{
		Name: f.Name(),
		Params: map[string]string{
			"d":     fmt.Sprintf("%d", f.currD),
			"sigma": fmt.Sprintf("%.1f", f.currSigma),
		},
	}
}
func (f *BilateralFilter) Next() bool {
	if !f.started {
		f.started = true
		return true
	}
	f.currSigma += 15.0 * float64(f.step)
	if f.currSigma > f.sigmaMax {
		f.currSigma = f.sigmaMin
		f.currD += 2 * f.step
		if f.currD > f.dMax {
			return false
		}
	}
	return true
}
func (f *BilateralFilter) Apply(src gocv.Mat) gocv.Mat {
	dst := gocv.NewMat()
	// Bilateral filter requires 8-bit image, usually 1 or 3 channels
	gocv.BilateralFilter(src, &dst, f.currD, f.currSigma, f.currSigma)
	return dst
}

// GammaCorrectionFilter: Adjusts the brightness of the image using a power-law relationship.
type GammaCorrectionFilter struct {
	step     int
	gammaMin float64
	gammaMax float64
	currGamma float64
	started  bool
}

func NewGammaCorrectionFilter(step int) *GammaCorrectionFilter {
	if step < 1 { step = 1 }
	return &GammaCorrectionFilter{
		step:      step,
		gammaMin:  0.6,
		gammaMax:  1.8,
		currGamma: 0.6,
	}
}
func (f *GammaCorrectionFilter) Name() string { return "Gamma" }
func (f *GammaCorrectionFilter) Reset() {
	f.started = false
	f.currGamma = f.gammaMin
}
func (f *GammaCorrectionFilter) Total() int {
	return int((f.gammaMax-f.gammaMin)/(0.3*float64(f.step))) + 1
}
func (f *GammaCorrectionFilter) Config() FilterConfig {
	return FilterConfig{
		Name: f.Name(),
		Params: map[string]string{
			"gamma": fmt.Sprintf("%.2f", f.currGamma),
		},
	}
}
func (f *GammaCorrectionFilter) Next() bool {
	if !f.started {
		f.started = true
		return true
	}
	f.currGamma += 0.4 * float64(f.step)
	return f.currGamma <= f.gammaMax
}
func (f *GammaCorrectionFilter) Apply(src gocv.Mat) gocv.Mat {
	dst := gocv.NewMat()
	
	// Create lookup table
	lut := gocv.NewMatWithSize(1, 256, gocv.MatTypeCV8U)
	defer lut.Close()
	
	invGamma := 1.0 / f.currGamma
	for i := 0; i < 256; i++ {
		v := float64(i) / 255.0
		res := math.Pow(v, invGamma) * 255.0
		if res > 255 {
			res = 255
		}
		lut.SetUCharAt(0, i, uint8(res))
	}

	gocv.LUT(src, lut, &dst)
	return dst
}

// CLAHEFilter
type CLAHEFilter struct {
	step      int
	clipMin   float64
	clipMax   float64
	tileMin   int
	tileMax   int
	currClip  float64
	currTile  int
	started   bool
}

func NewCLAHEFilter(step int) *CLAHEFilter {
	if step < 1 { step = 1 }
	return &CLAHEFilter{
		step:      step,
		clipMin:   1.0,
		clipMax:   3.5,
		tileMin:   4,
		tileMax:   12,
		currClip:  1.0,
		currTile:  4,
	}
}
func (f *CLAHEFilter) Name() string { return "CLAHE" }
func (f *CLAHEFilter) Reset() {
	f.started = false
	f.currClip = f.clipMin
	f.currTile = f.tileMin
}
func (f *CLAHEFilter) Total() int {
	cSteps := int((f.clipMax-f.clipMin)/(f.clipStep())) + 1
	tSteps := ((f.tileMax - f.tileMin) / f.tileStep()) + 1
	if cSteps < 1 { cSteps = 1 }
	if tSteps < 1 { tSteps = 1 }
	return cSteps * tSteps
}
func (f *CLAHEFilter) clipStep() float64 { return 0.5 * float64(f.step) }
func (f *CLAHEFilter) tileStep() int     { return 4 * f.step }

func (f *CLAHEFilter) Config() FilterConfig {
	return FilterConfig{
		Name: f.Name(),
		Params: map[string]string{
			"clipLimit": fmt.Sprintf("%.1f", f.currClip),
			"tileSize":  fmt.Sprintf("%dx%d", f.currTile, f.currTile),
		},
	}
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
		if f.currClip > f.clipMax {
			return false
		}
	}
	return true
}
func (f *CLAHEFilter) Apply(src gocv.Mat) gocv.Mat {
	dst := gocv.NewMat()
	// CLAHE usually needs 1-channel (grayscale or LAB L-channel)
	// We'll apply it on gray
	gray := src
	needsClose := false
	if src.Channels() == 3 {
		gray = gocv.NewMat()
		gocv.CvtColor(src, &gray, gocv.ColorBGRToGray)
		needsClose = true
	}
	
	clahe := gocv.NewCLAHEWithParams(f.currClip, image.Pt(f.currTile, f.currTile))
	defer clahe.Close()
	clahe.Apply(gray, &dst)
	
	if needsClose {
		gray.Close()
	}
	return dst
}

// ResizeFilter
type ResizeFilter struct {
	step      int
	scaleMin  float64
	scaleMax  float64
	currScale float64
	started   bool
}

func NewResizeFilter(step int) *ResizeFilter {
	if step < 1 { step = 1 }
	return &ResizeFilter{
		step:      step,
		scaleMin:  0.5,
		scaleMax:  2.0,
		currScale: 0.5,
	}
}
func (f *ResizeFilter) Name() string { return "Resize" }
func (f *ResizeFilter) Reset() {
	f.started = false
	f.currScale = f.scaleMin
}
func (f *ResizeFilter) scaleStep() float64 { return 0.25 * float64(f.step) }
func (f *ResizeFilter) Total() int {
	return int((f.scaleMax-f.scaleMin)/f.scaleStep()) + 1
}
func (f *ResizeFilter) Config() FilterConfig {
	return FilterConfig{
		Name: f.Name(),
		Params: map[string]string{
			"scale": fmt.Sprintf("%.2f", f.currScale),
		},
	}
}
func (f *ResizeFilter) Next() bool {
	if !f.started {
		f.started = true
		// Skip exactly 1.0 (no resize)
		if f.currScale == 1.0 { f.currScale += f.scaleStep() }
		return f.currScale <= f.scaleMax
	}
	f.currScale += f.scaleStep()
	if f.currScale == 1.0 { f.currScale += f.scaleStep() } // skip identity
	return f.currScale <= f.scaleMax
}
func (f *ResizeFilter) Apply(src gocv.Mat) gocv.Mat {
	dst := gocv.NewMat()
	gocv.Resize(src, &dst, image.Pt(0, 0), f.currScale, f.currScale, gocv.InterpolationCubic)
	// Some images get too large to process further and cause OOM.
	// Cap dimensions if reasonably large? 
	// For now let's just resize.
	return dst
}

// SharpenFilter
type SharpenFilter struct {
	step    int
	alphaMin float64
	alphaMax float64
	currAlpha float64
	started  bool
}

func NewSharpenFilter(step int) *SharpenFilter {
	if step < 1 { step = 1 }
	return &SharpenFilter{
		step:      step,
		alphaMin:  1.0,
		alphaMax:  2.0,
		currAlpha: 1.0,
	}
}
func (f *SharpenFilter) Name() string { return "Sharpen" }
func (f *SharpenFilter) Reset() {
	f.started = false
	f.currAlpha = f.alphaMin
}
func (f *SharpenFilter) stepSize() float64 { return 0.5 * float64(f.step) }
func (f *SharpenFilter) Total() int {
	return int((f.alphaMax-f.alphaMin)/f.stepSize()) + 1
}
func (f *SharpenFilter) Config() FilterConfig {
	return FilterConfig{
		Name: f.Name(),
		Params: map[string]string{
			"alpha": fmt.Sprintf("%.2f", f.currAlpha),
		},
	}
}
func (f *SharpenFilter) Next() bool {
	if !f.started {
		f.started = true
		return true
	}
	f.currAlpha += f.stepSize()
	return f.currAlpha <= f.alphaMax
}
func (f *SharpenFilter) Apply(src gocv.Mat) gocv.Mat {
	dst := gocv.NewMat()
	// Unsharp mask: dst = src*(1+alpha) - blur(src)*alpha
	// AddWeighted allows: src1*alpha + src2*beta + gamma
	// src*1.5 - blur*0.5 -> overall sharpen

	blurred := gocv.NewMat()
	defer blurred.Close()
	gocv.GaussianBlur(src, &blurred, image.Pt(0, 0), 2.0, 2.0, gocv.BorderDefault)
	
	// AddWeighted(src1, alpha, src2, beta, gamma, &dst)
	gocv.AddWeighted(src, 1.0 + f.currAlpha, blurred, -f.currAlpha, 0.0, &dst)
	
	return dst
}

// AdaptiveThresholdFilter
type AdaptiveThresholdFilter struct {
	step      int
	blockMin  int
	blockMax  int
	cMin      float32
	cMax      float32

	currBlock int
	currC     float32
	currMethod gocv.AdaptiveThresholdType
	started   bool
}

func NewAdaptiveThresholdFilter(step int) *AdaptiveThresholdFilter {
	if step < 1 { step = 1 }
	return &AdaptiveThresholdFilter{
		step:      step,
		blockMin:  3,
		blockMax:  21,
		cMin:      2.0,
		cMax:      10.0,
		currBlock: 3,
		currC:     2.0,
		currMethod: gocv.AdaptiveThresholdMean,
	}
}
func (f *AdaptiveThresholdFilter) Name() string { return "AdaptiveThreshold" }
func (f *AdaptiveThresholdFilter) Reset() {
	f.started = false
	f.currBlock = f.blockMin
	f.currC = f.cMin
	f.currMethod = gocv.AdaptiveThresholdMean
}
func (f *AdaptiveThresholdFilter) blockStep() int { return 2 * f.step }
func (f *AdaptiveThresholdFilter) cStep() float32 { return 2.0 * float32(f.step) }
func (f *AdaptiveThresholdFilter) Total() int {
	bSteps := ((f.blockMax - f.blockMin) / f.blockStep()) + 1
	cSteps := int((f.cMax-f.cMin)/f.cStep()) + 1
	if bSteps < 1 { bSteps = 1 }
	if cSteps < 1 { cSteps = 1 }
	return bSteps * cSteps * 2 // 2 methods: Mean and Gaussian
}
func (f *AdaptiveThresholdFilter) Config() FilterConfig {
	methodStr := "Mean"
	if f.currMethod == gocv.AdaptiveThresholdGaussian {
		methodStr = "Gaussian"
	}
	return FilterConfig{
		Name: f.Name(),
		Params: map[string]string{
			"blockSize": fmt.Sprintf("%d", f.currBlock),
			"C":         fmt.Sprintf("%.1f", f.currC),
			"method":    methodStr,
		},
	}
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
			if f.currMethod == gocv.AdaptiveThresholdMean {
				f.currMethod = gocv.AdaptiveThresholdGaussian
			} else {
				return false
			}
		}
	}
	return true
}
func (f *AdaptiveThresholdFilter) Apply(src gocv.Mat) gocv.Mat {
	dst := gocv.NewMat()
	// Must be 1-channel
	gray := src
	needsClose := false
	if src.Channels() == 3 {
		gray = gocv.NewMat()
		gocv.CvtColor(src, &gray, gocv.ColorBGRToGray)
		needsClose = true
	}
	
	gocv.AdaptiveThreshold(gray, &dst, 255.0, f.currMethod, gocv.ThresholdBinary, f.currBlock, f.currC)
	
	if needsClose {
		gray.Close()
	}
	return dst
}

// EdgeContrastFilter (Canny Edge Detection + blending back to original for contrast)
type EdgeContrastFilter struct {
	step      int
	lowMin    float32
	lowMax    float32
	highMin   float32
	highMax   float32
	alphaMin  float64
	alphaMax  float64

	currLow   float32
	currHigh  float32
	currAlpha float64
	started   bool
}

func NewEdgeContrastFilter(step int) *EdgeContrastFilter {
	if step < 1 { step = 1 }
	return &EdgeContrastFilter{
		step:      step,
		lowMin:    50.0,
		lowMax:    150.0,
		highMin:   100.0,
		highMax:   300.0,
		alphaMin:  0.1,
		alphaMax:  0.3,
		currLow:   50.0,
		currHigh:  100.0,
		currAlpha: 0.1,
	}
}
func (f *EdgeContrastFilter) Name() string { return "EdgeContrast" }
func (f *EdgeContrastFilter) Reset() {
	f.started = false
	f.currLow = f.lowMin
	f.currHigh = f.highMin
	f.currAlpha = f.alphaMin
}
func (f *EdgeContrastFilter) threshStep() float32 { return 25.0 * float32(f.step) }
func (f *EdgeContrastFilter) alphaStep() float64  { return 0.2 * float64(f.step) }
func (f *EdgeContrastFilter) Total() int {
	lSteps := int((f.lowMax-f.lowMin)/f.threshStep()) + 1
	hSteps := int((f.highMax-f.highMin)/f.threshStep()) + 1
	aSteps := int((f.alphaMax-f.alphaMin)/f.alphaStep()) + 1
	return lSteps * hSteps * aSteps
}
func (f *EdgeContrastFilter) Config() FilterConfig {
	return FilterConfig{
		Name: f.Name(),
		Params: map[string]string{
			"cannyLow":   fmt.Sprintf("%.0f", f.currLow),
			"cannyHigh":  fmt.Sprintf("%.0f", f.currHigh),
			"alpha":      fmt.Sprintf("%.2f", f.currAlpha),
		},
	}
}
func (f *EdgeContrastFilter) Next() bool {
	if !f.started {
		f.started = true
		return true
	}
	f.currAlpha += f.alphaStep()
	if f.currAlpha > f.alphaMax {
		f.currAlpha = f.alphaMin
		f.currHigh += f.threshStep()
		if f.currHigh > f.highMax {
			f.currHigh = f.highMin
			f.currLow += f.threshStep()
			if f.currLow > f.lowMax {
				return false
			}
		}
	}
	return true
}
func (f *EdgeContrastFilter) Apply(src gocv.Mat) gocv.Mat {
	dst := gocv.NewMat()
	
	edges := gocv.NewMat()
	defer edges.Close()
	
	gray := src
	needsClose := false
	if src.Channels() == 3 {
		gray = gocv.NewMat()
		gocv.CvtColor(src, &gray, gocv.ColorBGRToGray)
		needsClose = true
	}
	
	gocv.Canny(gray, &edges, f.currLow, f.currHigh)
	
	// Canny returns 1-channel, if src is 3-channel we need to convert edges to BGR to merge
	edgesColored := edges
	if src.Channels() == 3 {
		edgesColored = gocv.NewMat()
		defer edgesColored.Close()
		gocv.CvtColor(edges, &edgesColored, gocv.ColorGrayToBGR)
	}

	// Blend edges back onto original
	// original * 1.0 + edges * f.currAlpha
	gocv.AddWeighted(src, 1.0, edgesColored, f.currAlpha, 0.0, &dst)
	
	if needsClose {
		gray.Close()
	}
	
	return dst
}
// BlackHatFilter: Enhances dark details against a light background.
// Useful for extracting black QR codes on bright/white backgrounds.
type BlackHatFilter struct {
	step     int
	sizeMin  int
	sizeMax  int
	currSize int
	started  bool
}

func NewBlackHatFilter(step int) *BlackHatFilter {
	if step < 1 { step = 1 }
	return &BlackHatFilter{
		step:     step,
		sizeMin:  3,
		sizeMax:  21,
		currSize: 3,
	}
}
func (f *BlackHatFilter) Name() string { return "BlackHat" }
func (f *BlackHatFilter) Reset() {
	f.started = false
	f.currSize = f.sizeMin
}
func (f *BlackHatFilter) Total() int {
	return ((f.sizeMax - f.sizeMin) / (2 * f.step)) + 1
}
func (f *BlackHatFilter) Config() FilterConfig {
	return FilterConfig{
		Name: f.Name(),
		Params: map[string]string{
			"kernelSize": fmt.Sprintf("%d", f.currSize),
		},
	}
}
func (f *BlackHatFilter) Next() bool {
	if !f.started {
		f.started = true
		return true
	}
	f.currSize += 2 * f.step
	return f.currSize <= f.sizeMax
}
func (f *BlackHatFilter) Apply(src gocv.Mat) gocv.Mat {
	dst := gocv.NewMat()
	// Must be 1-channel for better effect, but can work on 3. Prefer gray.
	gray := src
	needsClose := false
	if src.Channels() == 3 {
		gray = gocv.NewMat()
		gocv.CvtColor(src, &gray, gocv.ColorBGRToGray)
		needsClose = true
	}

	kernel := gocv.GetStructuringElement(gocv.MorphRect, image.Pt(f.currSize, f.currSize))
	defer kernel.Close()

	gocv.MorphologyEx(gray, &dst, gocv.MorphBlackhat, kernel)
	
	if needsClose {
		gray.Close()
	}
	return dst
}

// DilationFilter: Expands the bright areas of an image.
type DilationFilter struct {
	step     int
	sizeMin  int
	sizeMax  int
	currSize int
	started  bool
}

func NewDilationFilter(step int) *DilationFilter {
	if step < 1 { step = 1 }
	return &DilationFilter{
		step:     step,
		sizeMin:  3,
		sizeMax:  5,
		currSize: 3,
	}
}
func (f *DilationFilter) Name() string { return "Dilation" }
func (f *DilationFilter) Reset() {
	f.started = false
	f.currSize = f.sizeMin
}
func (f *DilationFilter) Total() int {
	return ((f.sizeMax - f.sizeMin) / (2 * f.step)) + 1
}
func (f *DilationFilter) Config() FilterConfig {
	return FilterConfig{
		Name: f.Name(),
		Params: map[string]string{
			"kernelSize": fmt.Sprintf("%d", f.currSize),
		},
	}
}
func (f *DilationFilter) Next() bool {
	if !f.started {
		f.started = true
		return true
	}
	f.currSize += 2 * f.step
	return f.currSize <= f.sizeMax
}
func (f *DilationFilter) Apply(src gocv.Mat) gocv.Mat {
	dst := gocv.NewMat()
	kernel := gocv.GetStructuringElement(gocv.MorphRect, image.Pt(f.currSize, f.currSize))
	defer kernel.Close()
	gocv.Dilate(src, &dst, kernel)
	return dst
}

// ClosingFilter: Dilation followed by Erosion. Useful for closing small holes inside foreground objects.
type ClosingFilter struct {
	step     int
	sizeMin  int
	sizeMax  int
	currSize int
	started  bool
}

func NewClosingFilter(step int) *ClosingFilter {
	if step < 1 { step = 1 }
	return &ClosingFilter{
		step:     step,
		sizeMin:  3,
		sizeMax:  9,
		currSize: 3,
	}
}
func (f *ClosingFilter) Name() string { return "Closing" }
func (f *ClosingFilter) Reset() {
	f.started = false
	f.currSize = f.sizeMin
}
func (f *ClosingFilter) Total() int {
	return ((f.sizeMax - f.sizeMin) / (2 * f.step)) + 1
}
func (f *ClosingFilter) Config() FilterConfig {
	return FilterConfig{
		Name: f.Name(),
		Params: map[string]string{
			"kernelSize": fmt.Sprintf("%d", f.currSize),
		},
	}
}
func (f *ClosingFilter) Next() bool {
	if !f.started {
		f.started = true
		return true
	}
	f.currSize += 2 * f.step
	return f.currSize <= f.sizeMax
}
func (f *ClosingFilter) Apply(src gocv.Mat) gocv.Mat {
	dst := gocv.NewMat()
	kernel := gocv.GetStructuringElement(gocv.MorphRect, image.Pt(f.currSize, f.currSize))
	defer kernel.Close()
	gocv.MorphologyEx(src, &dst, gocv.MorphClose, kernel)
	return dst
}
