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

        embedfs "{{.ImportRoot}}"

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

var __thisDir = embedfs.EmbedDir{
	name:    ".",
	modTime: time.Now(),
	files:   make(map[string]*_file),
}

func Mount() http.FileSystem {
	return embedfs.Mount(&__thisDir)
}

func FileInfo() os.FileInfo {
	return &__thisDir
}

func Dir(path string) http.FileSystem {
        return embedfs.Dir(path, &__thisDir)
}

func Open(path string) (http.File, error) {
	return Mount().Open(path)
}

func Readdir(count int) ([]os.FileInfo, error) {
	return Mount().Readdir(count)
}
`

type tocModel struct {
	ImportRoot  string
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
		ImportRoot:  d.importRoot,
		DirName:     d.dirName,
		PackageName: Sanitize(d.dirName),
		Imports:     d.buildImports(),
	})
}
