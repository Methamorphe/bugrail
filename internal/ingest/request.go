package ingest

import (
	"compress/flate"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
)

func readRequestBody(r *http.Request) ([]byte, error) {
	var reader io.Reader = r.Body
	switch r.Header.Get("Content-Encoding") {
	case "gzip":
		gz, err := gzip.NewReader(r.Body)
		if err != nil {
			return nil, fmt.Errorf("init gzip reader: %w", err)
		}
		defer gz.Close()
		reader = gz
	case "deflate":
		rc := flate.NewReader(r.Body)
		defer rc.Close()
		reader = rc
	}
	body, err := io.ReadAll(io.LimitReader(reader, 2<<20))
	if err != nil {
		return nil, fmt.Errorf("read request body: %w", err)
	}
	return body, nil
}
