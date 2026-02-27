package scanner

import (
	"fmt"
	"image"
	"path/filepath"

	"gocv.io/x/gocv"
	"gocv.io/x/gocv/contrib"
)

type WeChatQRScanner struct {
	detector *contrib.WeChatQRCode
}

func NewWeChatQRScanner(modelsDir string) (*WeChatQRScanner, error) {
	detectProto := filepath.Join(modelsDir, "detect.prototxt")
	detectModel := filepath.Join(modelsDir, "detect.caffemodel")
	srProto     := filepath.Join(modelsDir, "sr.prototxt")
	srModel     := filepath.Join(modelsDir, "sr.caffemodel")

	detector := contrib.NewWeChatQRCode(detectProto, detectModel, srProto, srModel)
	
	return &WeChatQRScanner{
		detector: detector,
	}, nil
}

func (s *WeChatQRScanner) Scan(imagePath string) ([]string, error) {
	img := gocv.IMRead(imagePath, gocv.IMReadColor)
	if img.Empty() {
		return nil, fmt.Errorf("falha ao ler a imagem: %s", imagePath)
	}
	defer img.Close()

	// Tentativa 1: Imagem Original
	results := s.detect(img)
	if len(results) > 0 {
		return results, nil
	}

	// Tentativa 2: Blur Simples
	blurImg := gocv.NewMat()
	defer blurImg.Close()
	gocv.GaussianBlur(img, &blurImg, image.Pt(5, 5), 0, 0, gocv.BorderDefault)
	results = s.detect(blurImg)
	if len(results) > 0 {
		return results, nil
	}

	// Tentativa 3: CLAHE
	grayImg := gocv.NewMat()
	defer grayImg.Close()
	gocv.CvtColor(img, &grayImg, gocv.ColorBGRToGray)

	clahe := gocv.NewCLAHEWithParams(2.0, image.Pt(8, 8))
	defer clahe.Close()
	claheImg := gocv.NewMat()
	defer claheImg.Close()
	clahe.Apply(grayImg, &claheImg)
	
	claheBgrImg := gocv.NewMat()
	defer claheBgrImg.Close()
	gocv.CvtColor(claheImg, &claheBgrImg, gocv.ColorGrayToBGR)

	results = s.detect(claheBgrImg)
	if len(results) > 0 {
		return results, nil
	}

	// Tentativa 4: Resize 1.5x
	scaledImg := gocv.NewMat()
	defer scaledImg.Close()
	gocv.Resize(img, &scaledImg, image.Pt(0, 0), 1.5, 1.5, gocv.InterpolationCubic)
	results = s.detect(scaledImg)
	if len(results) > 0 {
		return results, nil
	}

	return nil, nil
}

func (s *WeChatQRScanner) detect(img gocv.Mat) []string {
	if img.Empty() {
		return nil
	}
	// WeChatQRCode expects a BGR (3-channel) image.
	// Several pipeline filters (CLAHE, AdaptiveThreshold, BlackHat…) output
	// grayscale or binary Mats. Passing those directly to the C++ decoder
	// corrupts the malloc heap ("unsorted double linked list corrupted").
	// Always normalise to BGR before calling into OpenCV contrib.
	bgr := img
	needsClose := false
	if img.Channels() == 1 {
		bgr = gocv.NewMat()
		gocv.CvtColor(img, &bgr, gocv.ColorGrayToBGR)
		needsClose = true
	}
	if needsClose {
		defer bgr.Close()
	}

	var points []gocv.Mat
	results := s.detector.DetectAndDecode(bgr, &points)
	for _, p := range points {
		p.Close()
	}
	return results
}

// ScanRaw scans the image exactly once without fallback attempts.
// Useful for fuzzing and baselining.
func (s *WeChatQRScanner) ScanRaw(imagePath string) ([]string, error) {
	img := gocv.IMRead(imagePath, gocv.IMReadColor)
	if img.Empty() {
		return nil, fmt.Errorf("falha ao ler a imagem: %s", imagePath)
	}
	defer img.Close()

	results := s.detect(img)
	if len(results) > 0 {
		return results, nil
	}
	
	return nil, nil
}

// DetectRaw is exactly like detect but public for the fuzzer
func (s *WeChatQRScanner) DetectRaw(img gocv.Mat) []string {
	return s.detect(img)
}
