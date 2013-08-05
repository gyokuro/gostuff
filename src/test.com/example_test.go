package main

import (
	"net/http"
	"regexp"
	"strings"
	"testing"
)

// embedded resources
import (
	filewatchDir "test.com/webapp/filewatch"
	cssDir "test.com/webapp/filewatch/css"
)

var listing = `
bootstrap-examples/.git/refs/remotes/origin
bootstrap-examples/.git/refs/remotes/origin/HEAD
bootstrap-examples/.git/refs/tags
bootstrap-examples/.gitignore
bootstrap-examples/assets
bootstrap-examples/assets/css
bootstrap-examples/assets/css/docs.css
bootstrap-examples/assets/ico
bootstrap-examples/assets/ico/apple-touch-icon-114-precomposed.png
bootstrap-examples/assets/ico/apple-touch-icon-144-precomposed.png
`

func TestFilewatchDir(t *testing.T) {

	t.Log("Test filewatch directory")

	fw := filewatchDir.Dir(".")
	t.Log("filewatch", fw)

	css := filewatchDir.Dir("css")
	t.Log("css", css)

	if fw == css {
		t.Error(". and css are not the same filesystems")
	}

	_css := filewatchDir.Dir("./css")
	t.Log("./css", _css)

	if css != _css {
		t.Error("css and ./css should return same filesystem object")
	}

	dir, _ := fw.Open(".")
	list, _ := dir.Readdir(-1)
	if len(list) != 2 {
		t.Error("Expecting two entries in filewatch/")
	}
	for _, l := range list {
		t.Log("\t", l.IsDir(), l)
		if l.IsDir() {
			lh, _ := fw.Open(l.Name())
			list2, _ := lh.Readdir(-1)
			for _, l2 := range list2 {
				t.Log("\t\t", l2)
			}
		}
	}

	var fsys http.FileSystem = filewatchDir.Mount()

	if fsys != filewatchDir.Dir(".") {
		t.Error("Mount() and Dir('.') should be the same")
	}

	dir, _ = fsys.Open(".")

	f, _ := fsys.Open("demo.html")
	stat, _ := f.Stat()
	t.Log("demo.html", f, stat.Name(), stat.Size(), stat.ModTime(), stat.IsDir())

	f, _ = fsys.Open("css/style.css")
	stat, _ = f.Stat()
	t.Log("style", f, stat.Name(), stat.Size(), stat.ModTime(), stat.IsDir())

	f, _ = fsys.Open("./css/style.css")
	stat, _ = f.Stat()
	t.Log("style", f, stat.Name(), stat.Size(), stat.ModTime(), stat.IsDir())

	f, _ = fsys.Open("./css/./style.css")
	stat, _ = f.Stat()
	t.Log("style", f, stat.Name(), stat.Size(), stat.ModTime(), stat.IsDir())

	_, err := f.Readdir(-1)
	if err == nil {
		t.Error("Shouldn't be able to readdir on file.")
	}

	_, err = fsys.Open("css/dontexist.css")
	if err == nil {
		t.Error("Expecting error opening non existent file.")
	}
	_, err = fsys.Open("./css/nodir/dontexist.css")
	if err == nil {
		t.Error("Expecting error opening non existent file.")
	}
}

func TestCssDir(t *testing.T) {
	t.Log("Testing css dir")

	var fsys http.FileSystem = cssDir.Mount() // Get the file system

	f, err := fsys.Open(".")
	stat, err := f.Stat()
	t.Log("css", f, err, stat.Name(), stat.Size(), stat.ModTime(), stat.IsDir())
	list, err := f.Readdir(-1)
	if err != nil {
		t.Error("Should not have error opening '.'")
	}
	if len(list) != 1 {
		t.Fatal("Expecting 1 memeber")
	}

	for _, l := range list {
		t.Log("--", l.Name(), l.Size(), l.ModTime(), l.IsDir())
	}

	f, err = fsys.Open("style.css")
	stat, err = f.Stat()
	if err != nil {
		t.Fail()
	}
	t.Log("style", f, err, stat.Name(), stat.Size(), stat.ModTime(), stat.IsDir())

	f, err = cssDir.Open("style.css")
	stat, err = f.Stat()
	if err != nil {
		t.Fail()
	}
	t.Log("style", f, err, stat.Name(), stat.Size(), stat.ModTime(), stat.IsDir())
}

func runregex(p string, t *testing.T) {
	paths := strings.Split(listing, "\n")
	for i, path := range paths {
		matched, err := regexp.MatchString(p, path)
		if err != nil {
			t.Error(p, " parse error")
		}
		if matched {
			t.Log(i, " path = ", path, matched, err)
		}
	}
}

func TestRegexp(t *testing.T) {
	runregex(".+\\.(js|css|html|png)$", t)
	runregex(".+(\\.git).*", t)
	runregex("^((?!git).)*$", t)
}
