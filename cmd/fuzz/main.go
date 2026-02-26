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

	// Note: Factories are now created per-phase in the BFS loop below

	// Note: Pipelines are now generated per-phase in the BFS loop below

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
	// Initializing writers...

	fmt.Printf("Starting %d workers to process up to %d images (limit set to %d)...\n", *workers, len(files), *limit)

	// Phase-by-Phase Execution (BFS across images)
	solvedImages := sync.Map{}

	for length := 1; length <= *maxLength; length++ {
		fmt.Printf("\n--- 🚀 STARTING PHASE %d: Pipelines of length %d ---\n", length, length)

		// Filter factories for this phase
		localFactories := map[string]fuzz.FilterFactory{
			"Bilateral":         func() fuzz.Filter { return fuzz.NewBilateralFilter(*step) },
			"CLAHE":             func() fuzz.Filter { return fuzz.NewCLAHEFilter(*step) },
			"Resize":            func() fuzz.Filter { return fuzz.NewResizeFilter(*step) },
			"Sharpen":           func() fuzz.Filter { return fuzz.NewSharpenFilter(*step) },
			"Gamma":             func() fuzz.Filter { return fuzz.NewGammaCorrectionFilter(*step) },
			"Dilation":          func() fuzz.Filter { return fuzz.NewDilationFilter(*step) },
			"Closing":           func() fuzz.Filter { return fuzz.NewClosingFilter(*step) },
			"AdaptiveThreshold": func() fuzz.Filter { return fuzz.NewAdaptiveThresholdFilter(*step) },
		}
		phasePipelines := fuzz.GeneratePipelines(localFactories, length)
		// Only keep pipelines of EXACTLY the current length
		var currentLevelPipelines []fuzz.Pipeline
		for _, p := range phasePipelines {
			if len(p.Filters) == length {
				currentLevelPipelines = append(currentLevelPipelines, p)
			}
		}
		fmt.Printf("Phase %d has %d pipeline permutations.\n", length, len(currentLevelPipelines))

		// Prepare images for this phase (only unsolved ones)
		var imagesToProcess []os.DirEntry
		for _, f := range files {
			if f.IsDir() { continue }
			ext := strings.ToLower(filepath.Ext(f.Name()))
			if !supportedExts[ext] { continue }
			
			if _, solved := solvedImages.Load(f.Name()); solved {
				continue
			}
			imagesToProcess = append(imagesToProcess, f)
		}

		if len(imagesToProcess) == 0 {
			fmt.Printf("All images solved! Skipping remaining phases.\n")
			break
		}

		if *limit > 0 && len(imagesToProcess) > *limit {
			imagesToProcess = imagesToProcess[:*limit]
		}

		imageChan := make(chan os.DirEntry, len(imagesToProcess))
		for _, img := range imagesToProcess {
			imageChan <- img
		}
		close(imageChan)

		fmt.Printf("Processing %d unsolved images in Phase %d with %d workers...\n", len(imagesToProcess), length, *workers)

		var wg sync.WaitGroup
		for w := 0; w < *workers; w++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()
				
				localSc, err := scanner.NewWeChatQRScanner(*modelsDir)
				if err != nil {
					log.Printf("Worker %d falhou ao inicializar scanner: %v", workerID, err)
					return
				}

				for f := range imageChan {
					// Double check if solved by another worker while in queue
					if _, solved := solvedImages.Load(f.Name()); solved {
						continue
					}

					path := filepath.Join(*nfcesDir, f.Name())
					img := gocv.IMRead(path, gocv.IMReadColor)
					if img.Empty() {
						continue
					}

					found := false
					for _, pipe := range currentLevelPipelines {
						pipe.ResetAll()
						for pipe.Next() {
							start := time.Now()
							dst, ok := pipe.Apply(img)
							var results []string
							if ok && !dst.Empty() {
								results = localSc.DetectRaw(dst)
							}
							if !dst.Empty() {
								dst.Close()
							}

							if len(results) > 0 {
								dur := time.Since(start).Milliseconds()
								configs := pipe.Configs()
								names := pipe.Names()
								
								fmt.Printf("  ✅ [W%d][Phase %d] Found QR: %s (Pipeline: %s)\n", workerID, length, results[0], names)
								
								mu.Lock()
								writer.Write([]string{
									f.Name(), "true", results[0], names, configs, fmt.Sprintf("%d", dur), "N/A",
								})
								writer.Flush()
								mu.Unlock()
								
								solvedImages.Store(f.Name(), true)
								found = true
								break
							}
						}
						if found { break }
					}
					img.Close()
				}
			}(w)
		}
		wg.Wait()
		fmt.Printf("--- PHASE %d FINISHED ---\n", length)

		// Final check to log unsolved images for this phase if it was the last one
		if length == *maxLength {
			for _, f := range imagesToProcess {
				if _, solved := solvedImages.Load(f.Name()); !solved {
					mu.Lock()
					writer.Write([]string{f.Name(), "false", "", "N/A", "N/A", "0", "N/A"})
					writer.Flush()
					mu.Unlock()
				}
			}
		}
	}
	fmt.Println("\nFuzzing finished.")
}
