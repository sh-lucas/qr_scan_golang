package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/sh-lucas/qr_scan_golang/scanner"
)

func main() {
	nfcesDir := "nfces"
	if len(os.Args) >= 2 {
		nfcesDir = os.Args[1]
	}

	sc, err := scanner.NewWeChatQRScanner("models")
	if err != nil {
		log.Fatalf("Erro ao inicializar scanner: %v", err)
	}

	files, err := os.ReadDir(nfcesDir)
	if err != nil {
		log.Fatalf("Erro ao ler diretório: %v", err)
	}

	supportedExts := map[string]bool{
		".jpg":  true,
		".jpeg": true,
		".png":  true,
	}

	total := 0
	deleted := 0
	skipped := 0

	for _, f := range files {
		if f.IsDir() {
			continue
		}

		ext := strings.ToLower(filepath.Ext(f.Name()))
		if !supportedExts[ext] {
			skipped++
			continue
		}

		total++
		path := filepath.Join(nfcesDir, f.Name())

		results, err := sc.ScanRaw(path)
		if err != nil {
			fmt.Printf("  ⚠️  Erro: %s — %v\n", f.Name(), err)
			continue
		}

		if len(results) > 0 {
			fmt.Printf("  🗑️  %s — QR encontrado sem filtros → removendo\n", f.Name())
			if err := os.Remove(path); err != nil {
				fmt.Printf("  ❌ Erro ao remover %s: %v\n", f.Name(), err)
			} else {
				deleted++
			}
		} else {
			fmt.Printf("  ⏩ %s — sem QR (mantido)\n", f.Name())
		}
	}

	fmt.Println()
	fmt.Println("=== Resumo Baseline ===")
	fmt.Printf("Total imagens suportadas: %d\n", total)
	fmt.Printf("Removidas (QR sem filtro): %d\n", deleted)
	fmt.Printf("Mantidas (difíceis):       %d\n", total-deleted)
	fmt.Printf("Ignoradas (HEIC/outros):   %d\n", skipped)
}
