// AUTO-GENERATED TOC
// DO NOT EDIT!!!
package embedfs

import (
	embedfs "github.com/gyokuro/embedfs/resources"
	"net/http"
	"os"
)

func init() {

	DIR.AddDir(DIR)

}

var DIR = embedfs.DirAlloc("embedfs")

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
