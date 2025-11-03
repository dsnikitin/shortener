package middleware

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
)

type compressWriter struct {
	rw  http.ResponseWriter
	gzw *gzip.Writer
}

func newCompressWriter(rw http.ResponseWriter) *compressWriter {
	return &compressWriter{
		rw:  rw,
		gzw: gzip.NewWriter(rw),
	}
}

func (w *compressWriter) Header() http.Header {
	return w.rw.Header()
}

func (w *compressWriter) Write(p []byte) (int, error) {
	return w.gzw.Write(p)
}

func (w *compressWriter) WriteHeader(statusCode int) {
	if statusCode < 300 {
		w.rw.Header().Set("Content-Encoding", "gzip")
	}
	w.rw.WriteHeader(statusCode)
}

func (w *compressWriter) Close() error {
	return w.gzw.Close()
}

type compressReader struct {
	rc  io.ReadCloser
	gzr *gzip.Reader
}

func newCompressReader(rc io.ReadCloser) (*compressReader, error) {
	gzr, err := gzip.NewReader(rc)
	if err != nil {
		return nil, err
	}

	return &compressReader{rc: rc, gzr: gzr}, nil
}

func (r *compressReader) Read(p []byte) (n int, err error) {
	return r.gzr.Read(p)
}

func (r *compressReader) Close() error {
	if err := r.rc.Close(); err != nil {
		return err
	}
	return r.gzr.Close()
}

func GzipCompress(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writer := w

		if strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			cw := newCompressWriter(w)
			defer cw.Close()

			writer = cw
		}

		if strings.Contains(r.Header.Get("Content-Encoding"), "gzip") {
			cr, err := newCompressReader(r.Body)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			r.Body = cr
			defer cr.Close()
		}

		h.ServeHTTP(writer, r)
	})
}
