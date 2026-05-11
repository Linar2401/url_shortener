package middleware

import (
	"compress/gzip"
	"net/http"
	"strings"

	"go.uber.org/zap"
)

type compressWriter struct {
	w           http.ResponseWriter
	zw          *gzip.Writer
	shouldGzip  bool
	wroteHeader bool
}

func newCompressWriter(w http.ResponseWriter) *compressWriter {
	return &compressWriter{
		w:  w,
		zw: gzip.NewWriter(w),
	}
}

func (c *compressWriter) Header() http.Header {
	return c.w.Header()
}

func (c *compressWriter) Write(p []byte) (int, error) {
	if !c.wroteHeader {
		c.WriteHeader(http.StatusOK)
	}
	if c.shouldGzip {
		return c.zw.Write(p)
	}
	return c.w.Write(p)
}

func (c *compressWriter) WriteHeader(statusCode int) {
	if c.wroteHeader {
		c.w.WriteHeader(statusCode)
		return
	}
	c.wroteHeader = true

	contentType := c.Header().Get("Content-Type")
	if strings.Contains(contentType, "application/json") || strings.Contains(contentType, "text/html") {
		c.shouldGzip = true
		c.w.Header().Set("Content-Encoding", "gzip")
	}

	c.w.WriteHeader(statusCode)
}

func (c *compressWriter) Close() error {
	if c.shouldGzip {
		return c.zw.Close()
	}
	return nil
}

func GzipMiddleware(log *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			contentEncoding := r.Header.Get("Content-Encoding")
			sendsGzip := strings.Contains(contentEncoding, "gzip")
			if sendsGzip {
				cr, err := gzip.NewReader(r.Body)
				if err != nil {
					log.Error("failed to create gzip reader", zap.Error(err))
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				r.Body = cr
				defer func(cr *gzip.Reader) {
					if err := cr.Close(); err != nil {
						log.Error("failed to close gzip reader", zap.Error(err))
					}
				}(cr)
			}

			acceptEncoding := r.Header.Get("Accept-Encoding")
			supportsGzip := strings.Contains(acceptEncoding, "gzip")

			if supportsGzip {
				cw := newCompressWriter(w)
				defer func(cw *compressWriter) {
					if err := cw.Close(); err != nil {
						log.Error("failed to close gzip writer", zap.Error(err))
					}
				}(cw)

				next.ServeHTTP(cw, r)
			} else {
				next.ServeHTTP(w, r)
			}
		})
	}
}
