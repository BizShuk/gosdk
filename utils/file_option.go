package utils

import "io"

type FileOptions struct {
	backup     bool
	backupKeep int // 0 = keep every backup
	create     bool
	writer     *io.Writer
}

// FileOptionFunc defines configuration builder for CreateFile.
type FileOptionFunc func(*FileOptions)

// WithBackup moves an existing file to <path>.bak before writing; an older
// .bak is kept as <path>.bak.<date> (see BACKUP_DATE_LAYOUT).
func WithBackup() FileOptionFunc {
	return func(o *FileOptions) {
		o.backup = true
	}
}

// WithBackupKeep is WithBackup that keeps at most n backups in total: <path>.bak
// plus the newest n-1 <path>.bak.<date>; older dated backups are deleted.
// n <= 0 keeps every backup, same as WithBackup.
func WithBackupKeep(n int) FileOptionFunc {
	return func(o *FileOptions) {
		o.backup = true
		o.backupKeep = max(n, 0)
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
