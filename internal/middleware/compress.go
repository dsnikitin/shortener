package middleware

import (
	"compress/gzip"
	"io"
	"net/http"
	"slices"
	"sync"
)

var writerPool = sync.Pool{
	New: func() any {
		return gzip.NewWriter(io.Discard)
	},
}

var readerPool = sync.Pool{
	New: func() any {
		return new(gzip.Reader)
	},
}

type compressWriter struct {
	http.ResponseWriter
	gzw *gzip.Writer
}

func newCompressWriter(rw http.ResponseWriter) *compressWriter {
	gzw := writerPool.Get().(*gzip.Writer)
	gzw.Reset(rw)

	return &compressWriter{
		ResponseWriter: rw,
		gzw:            gzw,
	}
}

func (cw *compressWriter) Write(p []byte) (int, error) {
	return cw.gzw.Write(p)
}

func (cw *compressWriter) Close() error {
	err := cw.gzw.Close()

	if err != nil {
		writerPool.Put(gzip.NewWriter(io.Discard))
	} else {
		writerPool.Put(cw.gzw)
	}

	return err
}

type compressReader struct {
	gzr *gzip.Reader
}

func newCompressReader(rc io.ReadCloser) (*compressReader, error) {
	gzr := readerPool.Get().(*gzip.Reader)

	if err := gzr.Reset(rc); err != nil {
		readerPool.Put(gzr)
		return nil, err
	}

	return &compressReader{gzr: gzr}, nil
}

func (cr *compressReader) Read(p []byte) (n int, err error) {
	return cr.gzr.Read(p)
}

func (cr *compressReader) Close() error {
	readerPool.Put(cr.gzr)
	return nil
}

func GzipCompress(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writer := w

		acceptEncoding := r.Header.Values("Accept-Encoding")
		if slices.Contains(acceptEncoding, "gzip") {
			cw := newCompressWriter(w)
			defer cw.Close()

			writer = cw
			writer.Header().Set("Content-Encoding", "gzip")
		}

		contentEncoding := r.Header.Values("Content-Encoding")
		if slices.Contains(contentEncoding, "gzip") {
			cr, err := newCompressReader(r.Body)
			if err != nil {
				http.Error(writer, err.Error(), http.StatusInternalServerError)
				return
			}

			r.Body = cr
			defer cr.Close()
		}

		h.ServeHTTP(writer, r)
	})
}
