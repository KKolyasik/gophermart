package gzipEncoding

import (
	"compress/gzip"
	"io"
)

// GZIPCompressor кодирует и декодирует HTTP-тело в формате gzip.
type GZIPCompressor struct {
	level int
}

// NewGZIPCompressor создает gzip-компрессор с заданным уровнем сжатия.
func NewGZIPCompressor(level int) *GZIPCompressor {
	return &GZIPCompressor{
		level: level,
	}
}

// Encoding возвращает имя поддерживаемого кодирования.
func (g *GZIPCompressor) Encoding() string {
	return "gzip"
}

// NewWriter создает gzip-писатель поверх целевого writer.
func (g *GZIPCompressor) NewWriter(w io.Writer) (io.WriteCloser, error) {
	return gzip.NewWriterLevel(w, g.level)
}

// NewReader создает gzip-ридер поверх входного reader.
func (g *GZIPCompressor) NewReader(r io.Reader) (io.ReadCloser, error) {
	return gzip.NewReader(r)
}
