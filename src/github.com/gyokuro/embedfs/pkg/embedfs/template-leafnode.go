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
        embedfs "{{.ImportRoot}}"
)

var {{.VarName}} = embedfs.EmbedFile{
	FileName:       "{{.BaseName}}",
	Original:   "{{.Original}}",
	Compressed: {{.IsCompressed}},
	ModificationTime: time.Unix({{.ModTimeUnix}},{{.ModTimeUnixNano}}),
        OriginalSize:     {{.SizeUncompressed}},
	Data:       {{.ContentAsString}},
}

func init() {
	DIR.AddFile(&{{.VarName}})
}
`

type leafModel struct {
	ImportRoot       string
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

	return t.Execute(w, leafModel{
		ImportRoot:       u.importRoot,
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
