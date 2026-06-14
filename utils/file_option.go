package utils

import "io"

type FileOptions struct {
	backup bool
	create bool
	writer *io.Writer
}

// FileOptionFunc defines configuration builder for CreateFile.
type FileOptionFunc func(*FileOptions)

// WithBackup enables recursive backup of existing files.
func WithBackup() FileOptionFunc {
	return func(o *FileOptions) {
		o.backup = true
	}
}

// WithCreate enables file creation if it doesn't exist.
func WithCreate() FileOptionFunc {
	return func(o *FileOptions) {
		o.create = true
	}
}

// WithWriter allows retrieving the opened writer.
func WithReturnWriter(w *io.Writer) FileOptionFunc {
	return func(o *FileOptions) {
		o.writer = w
	}
}
