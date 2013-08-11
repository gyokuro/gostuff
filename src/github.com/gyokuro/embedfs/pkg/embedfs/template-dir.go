package embedfs

import (
	"io"
	"path/filepath"
	"text/template"
)

const dirTemplate = `
// AUTO-GENERATED TOC
// DO NOT EDIT!!!
package {{.PackageName}}

import (
	"net/http"
	"os"
        embedfs "{{.ImportRoot}}"
)

{{if len .Imports }}
import (

        {{range $alias, $import := .Imports}}
        {{$alias}} "{{$import}}"
        {{end}}
)
{{end}}

func init() {

	DIR.AddDir(DIR)

        {{if len .Imports }}
          {{range $alias, $import := .Imports}}
          DIR.AddDir({{$alias}}.DIR)
          {{end}}
        {{end}}

}

var DIR = embedfs.DirAlloc("{{$.DirBaseName}}")

func Dir(path string) http.FileSystem {
	if handle, err := DIR.Open(); err == nil {
		return handle
	}
	return nil
}

func Mount() http.FileSystem {
	return Dir(".")
}

func FileInfo() os.FileInfo {
	return DIR
}
`

type tocModel struct {
	ImportRoot  string
	DirName     string
	DirBaseName string
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
		DirBaseName: filepath.Base(d.dirName),
		PackageName: Sanitize(d.dirName),
		Imports:     d.buildImports(),
	})
}
