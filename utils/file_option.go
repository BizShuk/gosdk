package utils

type createOpts struct {
	backup bool
	create bool
}

// FileOption defines configuration builder for CreateFile.
type FileOption func(*createOpts)

// WithBackup enables recursive backup of existing files.
func WithBackup() FileOption {
	return func(o *createOpts) {
		o.backup = true
	}
}

// WithCreate enables file creation if it doesn't exist.
func WithCreate() FileOption {
	return func(o *createOpts) {
		o.create = true
	}
}
