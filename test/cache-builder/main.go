package main

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/pierrec/lz4/v4"
)

func collectFilesConcurrent(dir string) (map[string][]byte, error) {
	filesMap := make(map[string][]byte)
	var mu sync.Mutex
	var wg sync.WaitGroup
	fileCh := make(chan string)

	// Walk through the directory and send file paths to the channel
	go func() {
		i := 0
		defer close(fileCh)
		err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() {
				i++
				fileCh <- filepath.Base(path)
				if i%1000 == 0 {
					fmt.Println("found files:", i)
				}
			}
			return nil
		})
		if err != nil {
			fmt.Println("Error walking the path:", err)
		}
	}()

	// Read files concurrently
	for i := 0; i < 10; i++ { // Number of concurrent workers
		wg.Add(1)
		go func() {
			defer wg.Done()
			for filename := range fileCh {
				data, err := os.ReadFile(filepath.Join(dir, filename))
				if err != nil {
					fmt.Println("Error reading file:", err)
					continue
				}
				mu.Lock()
				filesMap[filename] = data
				mu.Unlock()
			}
		}()
	}

	wg.Wait()
	return filesMap, nil
}

func serializeAndCompressFiles(filesMap map[string][]byte, outputFile string) error {
	var buf bytes.Buffer
	encoder := gob.NewEncoder(&buf)
	err := encoder.Encode(filesMap)
	if err != nil {
		return err
	}

	// Compress the serialized data using LZ4
	var compressedBuf bytes.Buffer
	lz4Writer := lz4.NewWriter(&compressedBuf)
	_, err = lz4Writer.Write(buf.Bytes())
	if err != nil {
		return err
	}
	err = lz4Writer.Close()
	if err != nil {
		return err
	}

	return os.WriteFile(outputFile, compressedBuf.Bytes(), 0644)
}

func main() {
	dir := os.Args[1] // replace with your directory
	outputFile := "cache.gob.lz4"

	filesMap, err := collectFilesConcurrent(dir)
	if err != nil {
		fmt.Println("Error collecting files:", err)
		return
	}

	err = serializeAndCompressFiles(filesMap, outputFile)
	if err != nil {
		fmt.Println("Error serializing and compressing files:", err)
		return
	}

	fmt.Println("Files serialized and compressed successfully to", outputFile)
}
