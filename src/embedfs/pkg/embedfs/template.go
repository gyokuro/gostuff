package embedfs

import (
	"bytes"
	"io"
	"strconv"
	"text/template"
)

const leafTemplate = `
// AUTO-GENERATED FROM {{.Original}}
// DO NOT EDIT!!!
package {{.PackageName}}

import (
	"time"
)

var {{.VarName}} = _file{
	name:       "{{.BaseName}}",
	original:   "{{.Original}}",
	compressed: {{.IsCompressed}},
	modTime:    time.Unix({{.ModTimeUnix}},{{.ModTimeUnixNano}}),
        size:       {{.SizeUncompressed}},
	data:       {{.ContentAsString}},
}

func init() {
	register("{{.BaseName}}", &{{.VarName}})
}
`

type model struct {
	PackageName      string
	BaseName         string
	Original         string
	VarName          string
	IsCompressed     string
	SizeUncompressed int64
	ContentAsString  string
	ModTimeUnix      int64
	ModTimeUnixNano  int64
}

func (u *translationUnit) writeLeafNode(w io.Writer) error {
	t, err := template.New("leafnode").Parse(leafTemplate)
	if err != nil {
		return err
	}

	buff := bytes.NewBufferString("")
	u.writer = buff
	u.writeBinaryRepresentation()

	return t.Execute(w, model{
		PackageName:      u.packageName,
		BaseName:         u.baseName,
		Original:         u.src,
		VarName:          u.name,
		IsCompressed:     strconv.FormatBool(u.compressed),
		SizeUncompressed: u.fileInfo.Size(),
		ContentAsString:  buff.String(),
		ModTimeUnix:      u.fileInfo.ModTime().Unix(),
		ModTimeUnixNano:  u.fileInfo.ModTime().UnixNano(),
	})
}
