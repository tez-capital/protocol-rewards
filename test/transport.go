package test

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var (
	cacheMtx sync.RWMutex = sync.RWMutex{}
)

type TestTransport struct {
	Transport     http.RoundTripper
	CacheDir      string
	inMemoryCache map[string][]byte
	pathPrefix    string
}

func getFilenameWithoutExt(path string) string {
	// Extract the base filename from the path.
	filename := filepath.Base(path)

	// Find the last dot in the filename.
	dotIndex := strings.LastIndex(filename, ".")

	// If there's a dot, and it's not the first character, trim everything after the dot.
	if dotIndex > 0 {
		filename = filename[:dotIndex]
	}

	return filename
}

func NewTestTransport(transport http.RoundTripper, cacheDir, cacheArchivePath string) (*TestTransport, error) {
	result := &TestTransport{
		Transport:  transport,
		CacheDir:   cacheDir,
		pathPrefix: getFilenameWithoutExt(cacheArchivePath),
	}

	if cacheArchivePath != "" && cacheArchivePath != ".gob.lz4" {
		slog.Info("loading cache archive", "path", cacheArchivePath)
		inMemoryCache, err := DecompressAndDeserializeCache(cacheArchivePath)
		if err != nil {
			slog.Warn("failed to load cache archive", "error", err.Error())
		}
		slog.Info("loaded cache archive", "count", len(inMemoryCache))
		result.inMemoryCache = inMemoryCache
	}

	return result, nil
}

func (t *TestTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Method != "GET" {
		return t.Transport.RoundTrip(req) // Only cache GET requests
	}

	path := req.URL.Path
	if strings.Contains("api.tzkt.io", req.URL.Host) { // we need full path for tzkt
		path = path + req.URL.RawQuery
	}
	path = strings.TrimPrefix(path, "/mainnet")

	filename := t.cacheFilename(path)
	filename = strings.TrimPrefix(filename, t.pathPrefix)

	cacheMtx.RLock()
	defer cacheMtx.RUnlock()
	if data, ok := t.inMemoryCache[filename]; ok {
		var tmp json.RawMessage
		if err := json.Unmarshal(data, &tmp); err == nil {
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewReader(data)),
				Header:     make(http.Header),
			}, nil
		}
	}
	cachedFileName := t.CacheDir + "/" + filename

	if data, err := os.ReadFile(cachedFileName); err == nil {
		// Cache hit, return the response from the cache
		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(bytes.NewReader(data)),
			Header:     make(http.Header),
		}, nil
	}

	// Cache miss, make the actual request
	resp, err := t.Transport.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	resp.Body.Close() // close the original body

	var tmp json.RawMessage
	if err := json.Unmarshal(body, &tmp); err == nil {
		// Save the response body to the cache
		os.MkdirAll(t.CacheDir, 0755)
		os.WriteFile(cachedFileName, body, 0644)
	}

	// Reconstruct the response body before returning
	resp.Body = io.NopCloser(bytes.NewBuffer(body))
	return resp, nil
}

func (t *TestTransport) cacheFilename(urlPath string) string {
	// Remove leading slashes and replace remaining slashes with underscores
	safePath := strings.TrimLeft(urlPath, "/")
	return strings.ReplaceAll(safePath, "/", "_")
}
