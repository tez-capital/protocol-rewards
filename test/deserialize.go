package test

import (
	"bytes"
	"encoding/gob"
	"os"

	"github.com/pierrec/lz4/v4"
)

func DecompressAndDeserializeCache(inputFile string) (map[string][]byte, error) {
	data, err := os.ReadFile(inputFile)
	if err != nil {
		return nil, err
	}

	// Decompress the data using LZ4
	lz4Reader := lz4.NewReader(bytes.NewReader(data))
	var decompressedBuf bytes.Buffer
	_, err = decompressedBuf.ReadFrom(lz4Reader)
	if err != nil {
		return nil, err
	}

	// Deserialize the data
	decoder := gob.NewDecoder(&decompressedBuf)
	var filesMap map[string][]byte
	err = decoder.Decode(&filesMap)
	return filesMap, err
}
