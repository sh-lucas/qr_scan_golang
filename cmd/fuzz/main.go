package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/sh-lucas/qr_scan_golang/fuzz"
	"github.com/sh-lucas/qr_scan_golang/scanner"
	"gocv.io/x/gocv"
)

func main() {
	nfcesDir := flag.String("nfces", "nfces", "Directory covering the images to fuzz")
	modelsDir := flag.String("models", "models", "Directory containing the caffe models")
	outCSV := flag.String("out", "results.csv", "Output CSV file")
	step := flag.Int("step", 2, "Step granularity for filters")
	maxLength := flag.Int("max-length", 2, "Maximum filters in the pipeline")
	limit := flag.Int("limit", 0, "Limit the number of images to fuzz (0 for all)")
	workers := flag.Int("workers", 4, "Number of concurrent workers")
	flag.Parse()

	// Initializing writers...

	// Create Output file and writer
	outPath := *outCSV
	dir := filepath.Dir(outPath)
	ext := filepath.Ext(outPath)
	base := strings.TrimSuffix(filepath.Base(outPath), ext)
	
	// If base name is generic "results" or user asked for "result_", we follow the pattern
	timestamp := time.Now().Format("20060102_150405")
	finalPath := filepath.Join(dir, fmt.Sprintf("%s_%s%s", base, timestamp, ext))

	err := os.MkdirAll(dir, 0755)
	if err != nil {
		log.Fatalf("Erro ao criar diretorio para CSV: %v", err)
	}
	fOut, err := os.Create(finalPath)
	if err != nil {
		log.Fatalf("Erro ao criar CSV em %s: %v", finalPath, err)
	}
	defer fOut.Close()
	writer := csv.NewWriter(fOut)
	defer writer.Flush()
	
	fmt.Printf("Results will be saved to: %s\n", finalPath)
	writer.Write([]string{"image", "success", "qr_content", "filters", "configs", "duration_ms", "attempt_num"})

	// Setup factories
	factories := map[string]fuzz.FilterFactory{
		"Bilateral":         func() fuzz.Filter { return fuzz.NewBilateralFilter(*step) },
		"CLAHE":             func() fuzz.Filter { return fuzz.NewCLAHEFilter(*step) },
		"Resize":            func() fuzz.Filter { return fuzz.NewResizeFilter(*step) },
		"Sharpen":           func() fuzz.Filter { return fuzz.NewSharpenFilter(*step) },
		"Gamma":             func() fuzz.Filter { return fuzz.NewGammaCorrectionFilter(*step) },
		// "BlackHat":          func() fuzz.Filter { return fuzz.NewBlackHatFilter(*step) },
		"Dilation":          func() fuzz.Filter { return fuzz.NewDilationFilter(*step) },
		"Closing":           func() fuzz.Filter { return fuzz.NewClosingFilter(*step) },
		"AdaptiveThreshold": func() fuzz.Filter { return fuzz.NewAdaptiveThresholdFilter(*step) },
		// "EdgeContrast":      func() fuzz.Filter { return fuzz.NewEdgeContrastFilter(*step) },
	}

	pipelines := fuzz.GeneratePipelines(factories, *maxLength)
	fmt.Printf("Generated %d pipeline permutations.\n", len(pipelines))

	files, err := os.ReadDir(*nfcesDir)
	if err != nil {
		log.Fatalf("Erro ao ler diretório nfces: %v", err)
	}

	supportedExts := map[string]bool{
		".jpg":  true,
		".jpeg": true,
		".png":  true,
	}

	// Shuffle files
	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(files), func(i, j int) {
		files[i], files[j] = files[j], files[i]
	})

	var mu sync.Mutex
	imageChan := make(chan os.DirEntry, len(files))
	var wg sync.WaitGroup

	// Feed the channel
	processedCount := 0
	for _, f := range files {
		if f.IsDir() { continue }
		ext := strings.ToLower(filepath.Ext(f.Name()))
		if !supportedExts[ext] { continue }

		if *limit > 0 && processedCount >= *limit {
			break
		}
		processedCount++
		imageChan <- f
	}
	close(imageChan)

	fmt.Printf("Starting %d workers to process %d images...\n", *workers, processedCount)

	// Start workers
	for w := 0; w < *workers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			
			// Each worker needs its own isolated scanner instance because WeChatQRScanner might not be thread-safe
			localSc, err := scanner.NewWeChatQRScanner(*modelsDir)
			if err != nil {
				log.Printf("Worker %d falhou ao inicializar scanner: %v", workerID, err)
				return
			}
			
			// Each worker needs its own isolated pipeline instances
			localFactories := map[string]fuzz.FilterFactory{
				"Bilateral":         func() fuzz.Filter { return fuzz.NewBilateralFilter(*step) },
				"CLAHE":             func() fuzz.Filter { return fuzz.NewCLAHEFilter(*step) },
				"Resize":            func() fuzz.Filter { return fuzz.NewResizeFilter(*step) },
				"Sharpen":           func() fuzz.Filter { return fuzz.NewSharpenFilter(*step) },
				"Gamma":             func() fuzz.Filter { return fuzz.NewGammaCorrectionFilter(*step) },
				// "BlackHat":          func() fuzz.Filter { return fuzz.NewBlackHatFilter(*step) },
				"Dilation":          func() fuzz.Filter { return fuzz.NewDilationFilter(*step) },
				"Closing":           func() fuzz.Filter { return fuzz.NewClosingFilter(*step) },
				"AdaptiveThreshold": func() fuzz.Filter { return fuzz.NewAdaptiveThresholdFilter(*step) },
				// "EdgeContrast":      func() fuzz.Filter { return fuzz.NewEdgeContrastFilter(*step) },
			}
			localPipelines := fuzz.GeneratePipelines(localFactories, *maxLength)

			for f := range imageChan {
				path := filepath.Join(*nfcesDir, f.Name())
				fmt.Printf("[W%d] Fuzzing %s...\n", workerID, f.Name())

				img := gocv.IMRead(path, gocv.IMReadColor)
				if img.Empty() {
					fmt.Printf("  ⚠️ [W%d] Failed to read image %s\n", workerID, path)
					continue
				}

				attempt := 0
				foundBestLevel := -1 // Will hold the length of the pipeline that successfully read the QR code

				// Run all pipelines
				for _, pipe := range localPipelines {
					// If we found a QR code at a simpler depth (e.g. length 1), we don't proceed to test more complex pipelines (e.g. length 2 or 3)
					if foundBestLevel != -1 && len(pipe.Filters) > foundBestLevel {
						break
					}

					// initialize all combinations for this pipeline
					for _, filter := range pipe.Filters {
						filter.Reset()
						filter.Next()
					}

					// Iterate permutations
					hasMore := true
					for hasMore {
						attempt++
						if attempt%1000 == 0 {
							// Optional: comment this out to reduce terminal spam on multiple threads
							// fmt.Printf("    ... [W%d] attempt %d\n", workerID, attempt)
						}
						
						start := time.Now()
						dst, ok := pipe.Apply(img)
						var results []string
						if ok && !dst.Empty() {
							// Hack to use detector without wrapper parsing
							results = localSc.DetectRaw(dst) 
						}
						if !dst.Empty() {
							dst.Close()
						}
						dur := time.Since(start).Milliseconds()

						configs := pipe.Configs()
						names := pipe.Names()

						if len(results) > 0 {
							fmt.Printf("  ✅ [W%d] Found QR: %s (Pipeline: %s) after %d\n", workerID, results[0], names, attempt)
							
							mu.Lock()
							writer.Write([]string{
								f.Name(), "true", results[0], names, configs, fmt.Sprintf("%d", dur), fmt.Sprintf("%d", attempt),
							})
							writer.Flush()
							mu.Unlock()
							
							// Register that we found strings at this length so we skip deeper filters later
							foundBestLevel = len(pipe.Filters)
							break 
						}
						hasMore = pipe.Next()
					}
				}

				if foundBestLevel == -1 {
					fmt.Printf("  ❌ [W%d] Could not read any QR on %s after %d attempts.\n", workerID, f.Name(), attempt)
					mu.Lock()
					writer.Write([]string{
						f.Name(), "false", "", "N/A", "N/A", "0", fmt.Sprintf("%d", attempt),
					})
					writer.Flush()
					mu.Unlock()
				}
				
				img.Close()
			}
		}(w)
	}

	wg.Wait()
	fmt.Println("Fuzzing finished.")
}
