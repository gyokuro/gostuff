package main

import (
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var thisDir _filesystem = _filesystem{
	files: make(map[string]*_file),
}

var openDelegates = make(map[string]_openDelegate)

type _openDelegate struct {
	dirname string
	opener  func(string) (http.File, error)
}

// http.FileSystem
// type FileSystem interface {
// 	Open(name string) (File, error)
// }
//
// http.File
// type File interface {
// 	Close() error
// 	Stat() (os.FileInfo, error)
// 	Readdir(count int) ([]os.FileInfo, error)
// 	Read([]byte) (int, error)
// 	Seek(offset int64, whence int) (int64, error)
// }
//
// os.FileInfo
// type FileInfo interface {
// 	Name() string       // base name of the file
// 	Size() int64        // length in bytes for regular files; system-dependent for others
// 	Mode() FileMode     // file mode bits
// 	ModTime() time.Time // modification time
// 	IsDir() bool        // abbreviation for Mode().IsDir()
// 	Sys() interface{}   // underlying data source (can return nil)
// }

// Ensures proper implementation of interfaces
var _ http.FileSystem = (*_filesystem)(nil)
var _ http.File = (*_handle)(nil)
var _ os.FileInfo = (*_file)(nil)

func Mount() http.FileSystem {
	return &_filesystem{}
}

type _filesystem struct {
	files map[string]*_file
	sync  sync.Mutex
}

type _file struct {
	name       string
	original   string
	compressed bool
	data       []byte
	size       int64
	modTime    time.Time
	isDir      bool
	opener     func(string) (http.File, error)
	readdir    func(int) ([]os.FileInfo, error)
}

type _handle struct {
	file   *_file
	offset int64
	open   bool
}

func register(basename string, ptr *_file) {
	ptr.isDir = false
	ptr.opener = Open
	thisDir.files[basename] = ptr
}

func Open(name string) (http.File, error) {
	ptr, exists := thisDir.files[name]
	if exists {
		return &_handle{
			file: ptr,
			open: true,
		}, nil
	} else if d, exists := openDelegates[name]; exists {
		p, err := filepath.Rel(d.dirname, name)
		if err == nil {
			return d.opener(p)
		}
	}
	return nil, errors.New("not found: " + name)
}

func Readdir(count int) ([]os.FileInfo, error) {
	files := make([]os.FileInfo, 0)
	for k, file := range thisDir.files {
		if k != "." {
			files = append(files, file)
		}
	}
	return files, nil
}

func (fs *_filesystem) Open(name string) (http.File, error) {
	return Open(name)
}

func (h *_handle) Close() error {
	return nil
}

func (h *_handle) Stat() (os.FileInfo, error) {
	return h.file, nil
}

func (h *_handle) Readdir(count int) ([]os.FileInfo, error) {
	if h.file.isDir && h.file.name == DIR_NAME {
		return Readdir(count)
	} else if h.file.readdir != nil {
		return h.file.readdir(count)
	}
	return nil, errors.New("not a directory")
}

func (h *_handle) Read(buff []byte) (int, error) {
	if h.offset >= int64(len(h.file.data)) {
		return 0, io.EOF
	}
	n := copy(buff, h.file.data[h.offset:])
	h.offset += int64(n)
	return n, nil
}

func (h *_handle) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case os.SEEK_SET:
		h.offset = offset
	case os.SEEK_CUR:
		h.offset += offset
	case os.SEEK_END:
		h.offset = h.file.size + offset
	default:
		return 0, os.ErrInvalid
	}
	if h.offset < 0 {
		h.offset = 0
	}
	return h.offset, nil
}

func (f *_file) Name() string {
	return f.name
}

func (f *_file) Size() int64 {
	return f.size
}

func (f *_file) Mode() os.FileMode {
	return 0444
}

func (f *_file) ModTime() time.Time {
	return f.modTime
}

func (f *_file) IsDir() bool {
	return f.isDir
}

func (f *_file) Sys() interface{} {
	return nil
}
