package middleware

import (
	"compress/gzip"
	"io"
	"net/http"
	"slices"
	"sync"

	"github.com/dsnikitin/shortener/internal/logger"
)

const (
	gzipThreshold  = 512 // не сжимать ответы меньше 512 Б
	initialBufSize = 512 // стартовый размер буфера под ответ
	maxBufferCap   = 32 * 1024
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

var bufWriterPool = sync.Pool{
	New: func() any {
		return &bufferWriter{
			buf: make([]byte, 0, initialBufSize),
		}
	},
}

// compressWriter обертка для ResponseWriter с поддержкой gzip сжатия.
type compressWriter struct {
	http.ResponseWriter
	gzw *gzip.Writer
}

// newCompressWriter создает новый compressWriter.
func newCompressWriter(rw http.ResponseWriter) *compressWriter {
	gzw := writerPool.Get().(*gzip.Writer)
	gzw.Reset(rw)

	return &compressWriter{
		ResponseWriter: rw,
		gzw:            gzw,
	}
}

// Write записывает данные со сжатием gzip.
func (cw *compressWriter) Write(p []byte) (int, error) {
	return cw.gzw.Write(p)
}

// Close закрывает gzip writer и возвращает его в пул.
func (cw *compressWriter) Close() error {
	err := cw.gzw.Close()
	if err == nil {
		writerPool.Put(cw.gzw)
	}

	return err
}

// compressReader обертка для чтения сжатых gzip данных.
type compressReader struct {
	gzr *gzip.Reader
}

// newCompressReader создает новый compressReader.
func newCompressReader(rc io.ReadCloser) (*compressReader, error) {
	gzr := readerPool.Get().(*gzip.Reader)

	if err := gzr.Reset(rc); err != nil {
		readerPool.Put(gzr)
		return nil, err
	}

	return &compressReader{gzr: gzr}, nil
}

// Read читает распакованные данные.
func (cr *compressReader) Read(p []byte) (n int, err error) {
	return cr.gzr.Read(p)
}

// Close закрывает gzip reader и возвращает его в пул.
func (cr *compressReader) Close() error {
	err := cr.gzr.Close()
	if err == nil {
		readerPool.Put(cr.gzr)
	}

	return err
}

// bufferWriter буферизует ответ для проверки размера перед сжатием.
type bufferWriter struct {
	http.ResponseWriter
	buf  []byte
	code int
}

// getBufferWriter получает bufferWriter из пула.
func getBufferWriter(w http.ResponseWriter) *bufferWriter {
	bw := bufWriterPool.Get().(*bufferWriter)
	bw.ResponseWriter = w
	return bw
}

// putBufferWriter возвращает bufferWriter в пул.
func putBufferWriter(bw *bufferWriter) {
	bw.buf = bw.buf[:0]
	if cap(bw.buf) > maxBufferCap {
		bw.buf = make([]byte, 0, initialBufSize)
	}
	bw.code = 0
	bw.ResponseWriter = nil
	bufWriterPool.Put(bw)
}

// Header возвращает заголовки ответа.
func (bw *bufferWriter) Header() http.Header {
	return bw.ResponseWriter.Header()
}

// WriteHeader сохраняет код статуса ответа.
func (bw *bufferWriter) WriteHeader(statusCode int) {
	if bw.code == 0 {
		bw.code = statusCode
	}
}

// Write буферизует данные ответа.
func (bw *bufferWriter) Write(p []byte) (int, error) {
	if bw.code == 0 {
		bw.WriteHeader(http.StatusOK)
	}

	bw.buf = append(bw.buf, p...)
	n := len(p)
	return n, nil
}

// GzipCompress middleware для сжатия gzip запросов и ответов.
func GzipCompress(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		contentEncoding := r.Header.Values("Content-Encoding")
		if slices.Contains(contentEncoding, "gzip") {
			cr, err := newCompressReader(r.Body)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			r.Body = cr
			defer cr.Close()
		}

		if !slices.Contains(r.Header.Values("Accept-Encoding"), "gzip") {
			h.ServeHTTP(w, r)
			return
		}

		bw := getBufferWriter(w)
		defer putBufferWriter(bw)

		h.ServeHTTP(bw, r)

		statusCode := bw.code
		if statusCode == 0 {
			statusCode = http.StatusOK
		}

		if len(bw.buf) < gzipThreshold {
			write(bw.ResponseWriter, bw.buf, statusCode)
			return
		}

		// клиент поддерживает сжатие и ответ большой
		cw := newCompressWriter(w)
		defer cw.Close()

		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Del("Content-Length")
		write(cw, bw.buf, statusCode)
	})
}

// write записывает данные в ResponseWriter с указанным кодом статуса.
func write(w http.ResponseWriter, data []byte, statusCode int) {
	w.WriteHeader(statusCode)
	if n, err := w.Write(data); err != nil {
		logger.Log.Errorw("Failed to write response", "error", err, "written", n)
	}
}
