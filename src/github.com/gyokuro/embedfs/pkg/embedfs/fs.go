package embedfs

import (
	"bytes"
	"compress/zlib"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

/////////////////////////////////////
var DIR_NAME = "root"

var thisDir _filesystem = _filesystem{
	files: make(map[string]*_file),
	sync:  sync.Mutex{},
}

func register(basename string, ptr *_file) {
	ptr.isDir = false
	ptr.opener = Open
	thisDir.sync.Lock()
	thisDir.files[basename] = ptr
	thisDir.sync.Unlock()
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
	return &thisDir
}

func Dir(p string) http.FileSystem {
	rel, err := filepath.Rel(".", p)
	if err != nil {
		return &thisDir
	}

	subdir, exists := thisDir.files[rel]
	if p != "." && exists && subdir.dir != nil {
		if rel, err := filepath.Rel(subdir.name, p); err == nil {
			return subdir.dir(rel)
		}
	}
	return &thisDir
}

func Open(name string) (http.File, error) {
	name = filepath.Clean(name)
	if filepath.IsAbs(name) {
		var err error
		name, err = filepath.Rel("/", name)
		if err != nil {
			return &_handle{}, err
		}
	}
	baseDir := strings.Split(filepath.Dir(name), string(filepath.Separator))[0]
	ptr, exists := thisDir.files[name]
	if exists {
		h := &_handle{
			file: ptr,
			open: true,
		}
		if ptr.compressed {
			var err error
			h.inflater, err = zlib.NewReader(bytes.NewBuffer(h.file.data))
			if err != nil {
				return h, err
			}
		}
		return h, nil
	} else if d, exists := thisDir.files[baseDir]; exists && d.name != "." {
		if d.opener != nil {
			if p, err := filepath.Rel(d.name, name); err == nil {
				return d.opener(p)
			} else {
				return nil, err
			}
		} else {
			return nil, errors.New("no opener for " + name)
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
	dir        func(string) http.FileSystem
	opener     func(string) (http.File, error)
	readdir    func(int) ([]os.FileInfo, error)
}

type _handle struct {
	file     *_file
	offset   int64
	open     bool
	inflater io.ReadCloser
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

func (fs *_filesystem) Open(name string) (http.File, error) {
	return Open(name)
}

func (h *_handle) Close() error {
	if h.inflater != nil {
		return h.inflater.Close()
	}
	return nil
}

func (h *_handle) Stat() (os.FileInfo, error) {
	return h.file, nil
}

func (h *_handle) Readdir(count int) ([]os.FileInfo, error) {
	if h.file.isDir && h.file.name == "." {
		return Readdir(count)
	} else if h.file.readdir != nil {
		return h.file.readdir(count)
	}
	return nil, errors.New("not a directory")
}

func (h *_handle) Read(buff []byte) (int, error) {
	if h.inflater != nil {
		return h.inflater.Read(buff)
	} else {
		if h.offset >= int64(len(h.file.data)) {
			return 0, io.EOF
		}
		n := copy(buff, h.file.data[h.offset:])
		h.offset += int64(n)
		return n, nil
	}
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
