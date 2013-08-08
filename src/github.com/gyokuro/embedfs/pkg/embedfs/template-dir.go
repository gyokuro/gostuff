package embedfs

import (
	"io"
	"text/template"
)

const dirTemplate = `
// AUTO-GENERATED TOC
// DO NOT EDIT!!!
package {{.PackageName}}

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

import (
	"log"
)

{{if len .Imports }}
import (
        {{range $alias, $import := .Imports}}
        {{$alias}} "{{$import}}"
        {{end}}
)
{{end}}

func init() {

	register(".", &_file{
		name:    ".",
		info:    FileInfo,
		opener:  Open,
		readdir: Readdir,
	})

       {{if len .Imports }}
        {{range $alias, $import := .Imports}}
	register("{{$alias}}",  &_file{
		name:    "{{$alias}}",
		info:    {{$alias}}.FileInfo,
		opener:  {{$alias}}.Open,
		readdir: {{$alias}}.Readdir,
	})
        {{end}}
       {{end}}
}

var _ = log.Flags()

/// ** Filesystem implementation

var __thisDir = _dir{
	name:    ".",
	modTime: time.Now(),
	files:   make(map[string]*_file),
}

func register(basename string, ptr *_file) {
	__thisDir.sync.Lock()
	__thisDir.files[basename] = ptr
	__thisDir.sync.Unlock()
}

func Mount() *_dirHandle {
	return Dir(".")
}

func FileInfo() os.FileInfo {
	return &__thisDir
}

func Dir(path string) *_dirHandle {
	if path == "." {
		files := make([]os.FileInfo, 0)
		for _, file := range __thisDir.files {
			files = append(files, file)
		}
		return &_dirHandle{
			static: &__thisDir,
			files:  files,
		}
	}

	// search for subdirectories
	if filepath.IsAbs(path) {
		path, _ = filepath.Rel("/", path) // make it relative
	}
	subdir := strings.Split(path, string(filepath.Separator))[0]
	if d, exists := __thisDir.files[subdir]; exists {
		return &_dirHandle{
			info:    d.info,
			opener:  d.opener,
			readdir: d.readdir,
		}
	}

	return &_dirHandle{}
}

func Open(path string) (http.File, error) {
	return Mount().Open(path)
}

func Readdir(count int) ([]os.FileInfo, error) {
	return Mount().Readdir(count)
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
var _ http.FileSystem = (*_dirHandle)(nil)
var _ http.File = (*_fileHandle)(nil)
var _ os.FileInfo = (*_dir)(nil)
var _ os.FileInfo = (*_file)(nil)

////////////////////////////////////////////////////////////////////////
// DIRECTORY

type _dir struct {
	name    string
	modTime time.Time
	files   map[string]*_file
	sync    sync.Mutex
}

func (d *_dir) Name() string {
	return d.name
}
func (d *_dir) Size() int64 {
	return 0
}
func (d *_dir) Mode() os.FileMode {
	return 0444 | os.ModeDir
}
func (d *_dir) ModTime() time.Time {
	return d.modTime
}
func (d *_dir) IsDir() bool {
	return true
}
func (d *_dir) Sys() interface{} {
	return nil
}

type _dirHandle struct {
	static *_dir
	offset int
	files  []os.FileInfo // for implementing Readdir

	info    func() os.FileInfo
	opener  func(string) (http.File, error)
	readdir func(int) ([]os.FileInfo, error)
}

func (d *_dirHandle) Open(name string) (http.File, error) {
	name = filepath.Clean(name)
	if filepath.IsAbs(name) {
		var err error
		name, err = filepath.Rel("/", name)
		if err != nil {
			return &_fileHandle{}, err
		}
	}
	baseDir := strings.Split(filepath.Dir(name), string(filepath.Separator))[0]
	ptr, exists := d.static.files[name]
	if exists {
		if ptr.opener != nil {

			if ptr.name == "." {
				files := make([]os.FileInfo, 0)
				for _, file := range __thisDir.files {
					files = append(files, file)
				}
				return &_dirHandle{
					static: &__thisDir,
					files:  files,
					info:   FileInfo,
				}, nil
			} else {
				if p, err := filepath.Rel(ptr.name, name); err == nil {
					return ptr.opener(p)
				} else {
					return nil, err
				}
			}
		} else {
			h := &_fileHandle{
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
		}
	} else if d, exists := d.static.files[baseDir]; exists && d.opener != nil && d.name != "." {
		if p, err := filepath.Rel(d.name, name); err == nil {
			return d.opener(p)
		} else {
			return nil, err
		}
	}
	return nil, errors.New("not found: " + name)
}

func (d *_dirHandle) Readdir(count int) ([]os.FileInfo, error) {
	if d.readdir != nil {
		// delegate to subdirectory readdir
		return d.readdir(count)
	}

	if count <= 0 {
		return d.files, nil
	}
	if d.offset >= len(d.files) {
		return []os.FileInfo{}, io.EOF
	}

	if d.offset+count > len(d.files) {
		count = len(d.files) - d.offset
	}
	result := d.files[d.offset : d.offset+count]
	d.offset += count

	var err error
	if d.offset > len(d.files) {
		err = io.EOF
	}
	return result, err
}

func (d *_dirHandle) Close() error {
	return nil
}
func (d *_dirHandle) Read(p []byte) (int, error) {
	return 0, errors.New("not file")
}
func (d *_dirHandle) Seek(int64, int) (int64, error) {
	return 0, os.ErrInvalid
}
func (d *_dirHandle) Stat() (os.FileInfo, error) {
	return d.info(), nil
}

////////////////////////////////////////////////////////////////////////
// REGULAR FILE

type _file struct {
	name       string
	original   string
	compressed bool
	data       []byte
	size       int64
	modTime    time.Time

	info    func() os.FileInfo
	opener  func(string) (http.File, error)
	readdir func(int) ([]os.FileInfo, error)
}

type _fileHandle struct {
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
	return false
}

func (f *_file) Sys() interface{} {
	return nil
}

func (h *_fileHandle) Close() error {
	if h.inflater != nil {
		return h.inflater.Close()
	}
	return nil
}

func (h *_fileHandle) Stat() (os.FileInfo, error) {
	return h.file, nil
}

func (h *_fileHandle) Readdir(count int) ([]os.FileInfo, error) {
	return nil, errors.New("not a directory")
}

func (h *_fileHandle) Read(buff []byte) (int, error) {
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

func (h *_fileHandle) Seek(offset int64, whence int) (int64, error) {
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
`

type tocModel struct {
	DirName     string
	PackageName string
	Imports     map[string]string // map[alias]import
}

func (d *dirToc) writeDirToc(w io.Writer) error {
	t, err := template.New("dir-toc").Parse(dirTemplate)
	if err != nil {
		panic(err)
	}

	return t.Execute(w, tocModel{
		DirName:     d.dirName,
		PackageName: Sanitize(d.dirName),
		Imports:     d.buildImports(),
	})
}
