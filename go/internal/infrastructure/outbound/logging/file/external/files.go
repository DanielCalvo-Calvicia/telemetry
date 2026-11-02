// Package external holds the only I/O of the file destination: every call to the file system. Nothing here keeps a file
// open: each function opens, works and closes.
package external

import (
	"errors"
	"io/fs"
	"os"
)

// =============================================================================
// FILES
// =============================================================================

const (
	// directory_mode is the mode of a directory that the library creates: the owner only.
	directory_mode = 0o700
	// file_mode is the mode of a file that the library creates: the owner only.
	file_mode = 0o600
)

// ErrShortWrite is the cause of a write that took fewer bytes than it was given without saying why.
var ErrShortWrite = errors.New("the file took fewer bytes than the line has")

// EnsureDirectory creates the directory and its parents, readable by the owner only, when they do not exist.
func EnsureDirectory(path string) error {
	return os.MkdirAll(path, directory_mode)
}

// FileSize returns the size of the file and whether it exists. A file that does not exist is not an error.
func FileSize(path string) (size int64, exists bool, size_error error) {
	info, stat_error := os.Stat(path)
	if stat_error != nil {
		if errors.Is(stat_error, fs.ErrNotExist) {
			return 0, false, nil
		}
		return 0, false, stat_error
	}
	return info.Size(), true, nil
}

// ListNames returns the names of the entries of the directory.
func ListNames(directory string) ([]string, error) {
	entries, read_error := os.ReadDir(directory)
	if read_error != nil {
		return nil, read_error
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names, nil
}

// AppendLine appends the whole line to the file with one write call, creating the file readable by the owner only, and
// closes it. A short write is an error.
func AppendLine(path string, line []byte) (append_error error) {
	file, open_error := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, file_mode)
	if open_error != nil {
		return open_error
	}
	defer func() {
		if close_error := file.Close(); close_error != nil && append_error == nil {
			append_error = close_error
		}
	}()
	written, write_error := file.Write(line)
	if write_error != nil {
		return write_error
	}
	if written < len(line) {
		return ErrShortWrite
	}
	return nil
}

// TouchAppend opens the file for append, creating it readable by the owner only, and closes it without writing: it
// proves that a line can be appended.
func TouchAppend(path string) error {
	file, open_error := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, file_mode)
	if open_error != nil {
		return open_error
	}
	return file.Close()
}

// Rename moves the file from one name to another.
func Rename(from string, to string) error {
	return os.Rename(from, to)
}

// Remove deletes the file.
func Remove(path string) error {
	return os.Remove(path)
}
