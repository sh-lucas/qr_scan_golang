package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/sh-lucas/qr_scan_golang/scanner"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Uso: go run main.go <diretorio_com_imagens>")
		fmt.Println("Ou via Docker: make docker-run")
		os.Exit(1)
	}

	imgDir := os.Args[1]

	sc, err := scanner.NewWeChatQRScanner("models")
	if err != nil {
		log.Fatalf("Erro ao inicializar scanner: %v", err)
	}

	files, err := os.ReadDir(imgDir)
	if err != nil {
		log.Fatalf("Erro ao ler diretório: %v", err)
	}

	for _, f := range files {
		if f.IsDir() {
			continue
		}
		
		path := filepath.Join(imgDir, f.Name())
		fmt.Printf("Analisando %s...\n", f.Name())
		
		results, err := sc.Scan(path)
		if err != nil {
			fmt.Printf("  Erro: %v\n", err)
			continue
		}
		
		if len(results) > 0 {
			for i, res := range results {
				fmt.Printf("  ✅ [%d] %s\n", i+1, res)
			}
		} else {
			fmt.Printf("  ❌ Nenhum QR Code encontrado.\n")
		}
	}
}
