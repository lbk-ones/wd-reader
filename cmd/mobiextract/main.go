package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"wd-reader/go/book/mobi"
)

func main() {
	inputPath := flag.String("i", "", "Input file or directory path")
	flag.Parse()

	if *inputPath == "" {
		fmt.Println("Mobi/AZW3 Text Extractor")
		fmt.Println("Usage: mobiextract -i <file_or_directory>")
		fmt.Println("Supported formats: .mobi, .azw, .azw3")
		flag.Usage()
		return
	}

	info, err := os.Stat(*inputPath)
	if err != nil {
		fmt.Printf("Error accessing path: %v\n", err)
		return
	}

	if info.IsDir() {
		processDirectory(*inputPath)
	} else {
		processFile(*inputPath)
	}
}

func processDirectory(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		fmt.Printf("Error reading directory: %v\n", err)
		return
	}

	count := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		ext := strings.ToLower(filepath.Ext(name))
		if ext == ".mobi" || ext == ".azw" || ext == ".azw3" {
			processFile(filepath.Join(dir, name))
			count++
		}
	}
	if count == 0 {
		fmt.Println("No supported files found in directory.")
	}
}

func processFile(path string) {
	fmt.Printf("Processing %s...\n", filepath.Base(path))
	reader, err := mobi.NewReader(path)
	if err != nil {
		fmt.Printf("  [FAILED] Parse error: %v\n", err)
		return
	}

	text, err := reader.ExtractText()
	if err != nil {
		fmt.Printf("  [FAILED] Extraction error: %v\n", err)
		return
	}

	if len(text) == 0 {
		fmt.Println("  [WARNING] No text extracted (encrypted or empty?)")
		return
	}

	ext := filepath.Ext(path)
	outPath := path[:len(path)-len(ext)] + ".txt"

	err = os.WriteFile(outPath, append([]byte{0xEF, 0xBB, 0xBF}, []byte(text)...), 0644)
	if err != nil {
		fmt.Printf("  [FAILED] Write error: %v\n", err)
		return
	}
	fmt.Printf("  [SUCCESS] Saved to %s (%d chars)\n", filepath.Base(outPath), len(text))
}
