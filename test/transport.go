package test

import (
	"archive/zip"
	"bytes"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type TestTransport struct {
	Transport   http.RoundTripper
	CacheDir    string
	zipCacheMap map[string]*zip.File
	zipCacheMtx sync.Mutex
}

func removeFirstPart(path, target string) string {
	// Split the path into segments based on "/".
	segments := strings.Split(path, "/")

	// Check if the first segment matches the target string.
	if len(segments) > 0 && segments[0] == target {
		// Remove the first segment.
		segments = segments[1:]
	}

	// Join the segments back into a path.
	return strings.Join(segments, "/")
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

func NewTestTransport(transport http.RoundTripper, cacheDir, zipPath string) (*TestTransport, error) {
	var err error
	zipReader, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, err
	}

	result := &TestTransport{
		Transport:   transport,
		CacheDir:    cacheDir,
		zipCacheMap: make(map[string]*zip.File),
		zipCacheMtx: sync.Mutex{},
	}

	for _, f := range zipReader.File {
		if f.FileInfo().IsDir() {
			continue
		}

		result.zipCacheMap[removeFirstPart(f.Name, getFilenameWithoutExt(zipPath))] = f
	}

	return result, nil
}

func (t *TestTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Method != "GET" {
		return t.Transport.RoundTrip(req) // Only cache GET requests
	}

	filename := t.cacheFilename(req.URL.Path)

	// t.zipCacheMtx.Lock()
	// defer t.zipCacheMtx.Unlock()
	if f, ok := t.zipCacheMap[filename]; ok {
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		defer rc.Close()

		data, err := io.ReadAll(rc)
		if err != nil {
			return nil, err
		}
		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(bytes.NewReader(data)),
			Header:     make(http.Header),
		}, nil
	}

	if data, err := os.ReadFile(t.CacheDir + "/" + filename); err == nil {
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

	// Save the response body to the cache
	os.MkdirAll(t.CacheDir, 0755)
	os.WriteFile(filename, body, 0644)

	// Reconstruct the response body before returning
	resp.Body = io.NopCloser(bytes.NewBuffer(body))
	return resp, nil
}

func (t *TestTransport) cacheFilename(urlPath string) string {
	// Remove leading slashes and replace remaining slashes with underscores
	safePath := strings.TrimLeft(urlPath, "/")
	return strings.ReplaceAll(safePath, "/", "_")
}
