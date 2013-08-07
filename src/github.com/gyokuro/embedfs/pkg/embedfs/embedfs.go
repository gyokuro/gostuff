package embedfs

import (
	"bytes"
	"compress/zlib"
	"errors"
	"flag"
	"fmt"
	"go/parser"
	"go/printer"
	"go/token"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// pull this in for compilation
var _ *_file = (*_file)(nil)

var (
	maxUncompressedSize = flag.Int64("maxUncompressedK", 5, "Max in kilobytes uncompressed.")
	minCompressionRatio = flag.Float64("minCompressionRatio", 0.5, "Min compression ratio.")
	overwrite           = flag.Bool("overwrite", true, "Overwrite existing generated source.")
)

func NewTranslationUnit(packageName string, srcFile string, basename string, outDir string, byteSlice bool) *translationUnit {
	return &translationUnit{
		name:        strings.Replace(basename, ".", "_", -1),
		baseName:    basename,
		src:         srcFile,
		gofile:      filepath.Join(outDir, basename+".go"),
		packageName: packageName,
		newLine:     true,
		asByteSlice: byteSlice,
	}
}

func NewDirToc(destDirAbs string, importRoot string, dirName string, subDirNames []string) *dirToc {
	return &dirToc{
		importRoot:  importRoot,
		dirName:     dirName,
		subDirNames: subDirNames,
		outputPath:  filepath.Join(destDirAbs, dirName),
	}
}

type dirToc struct {
	importRoot  string
	dirName     string   // not a path -  base form
	subDirNames []string // children - base form, not full path
	outputPath  string
	gofile      string
}

func (d *dirToc) buildImports() map[string]string {
	result := make(map[string]string)

	for _, sub := range d.subDirNames {
		result[sub] = filepath.Join(d.importRoot, d.dirName, sub)
	}
	return result
}

func (d *dirToc) Translate() error {
	d.gofile = filepath.Join(d.outputPath, "generated-toc.go")
	_, err := os.Stat(d.gofile)
	if err == nil && !*overwrite {
		// file exits and is *after* the mod time of source -- do nothing
		log.Printf("Skipping %s", d.gofile)
		return nil
	}
	var goFile *os.File
	if err != nil {
		goFile, err = os.Create(d.gofile)
		if err != nil {
			log.Printf("Warning: cannot create file %s", d.gofile)
			return err
		}
	} else {
		goFile, err = os.OpenFile(d.gofile, os.O_RDWR|os.O_TRUNC, 0660)
		if err != nil {
			log.Printf("Warning: cannot open file %s", d.gofile)
			return err
		}
	}
	defer goFile.Close()
	err = d.writeDirToc(goFile)

	if err == nil {
		log.Printf("Generated toc --> %s\n", d.gofile)
	} else {
		log.Printf("FAIL to generate toc --> %s\n", d.gofile)
	}
	return nil
}

func (d *dirToc) Gofmt() error {
	gofile, err := os.Open(d.gofile)
	if err != nil {
		log.Printf("Cannot open %s to run gofmt: %s\n", d.gofile, err)
		return err
	}
	fileSet := token.NewFileSet()
	ast, err := parser.ParseFile(fileSet, "", gofile, parser.ParseComments)
	if err != nil {
		log.Fatal(err)
	}

	var formatted bytes.Buffer
	config := &printer.Config{
		Mode:     printer.TabIndent | printer.UseSpaces,
		Tabwidth: 8,
	}
	err = config.Fprint(&formatted, fileSet, ast)
	if err != nil {
		log.Printf("Gofmt failed on %s: %s\n", d.gofile, err)
		return err
	}

	if err := ioutil.WriteFile(d.gofile, formatted.Bytes(), 0644); err != nil {
		log.Printf("Cannot write %s after gofmt: %s\n", d.gofile, err)
		return err
	}

	log.Printf("Ran gofmt on %s\n", d.gofile)
	return nil
}

type translationUnit struct {
	name        string
	baseName    string
	src         string
	gofile      string
	packageName string
	compressed  bool
	data        []byte
	fileInfo    os.FileInfo
	asByteSlice bool
	writer      io.Writer
	written     int // in bytes
	newLine     bool
}

func (u *translationUnit) Write(p []byte) (n int, err error) {
	if len(p) == 0 {
		return
	}
	for n = range p {
		if u.written%16 == 0 && u.written > 0 {
			u.newLine = true
			if u.asByteSlice {
				u.writer.Write([]byte{'\n'})
			}
		}
		if u.asByteSlice {
			fmt.Fprintf(u.writer, "0x%02x,", p[n])
		} else {
			fmt.Fprintf(u.writer, "\\x%02x", p[n])
		}
		u.written++
	}
	if u.written == len(u.data) {
		if u.asByteSlice {
			u.writer.Write([]byte{'\n'})
		}
	}
	n++
	return
}

func (u *translationUnit) writeBinaryRepresentation() {

	if u.asByteSlice {
		fmt.Fprintf(u.writer, "[]byte{\n")
	} else {
		fmt.Fprintf(u.writer, "\"")
	}
	// write to output the binary data
	io.Copy(u, bytes.NewBuffer(u.data))

	if u.asByteSlice {
		fmt.Fprintf(u.writer, "}")
	} else {
		fmt.Fprintf(u.writer, "\"")
	}
	return
}

func (u *translationUnit) Translate() error {
	log.Println("Translating ", u.src)
	source, err := os.Stat(u.src)
	if err != nil {
		log.Fatalf("%s", err)
		return err
	}

	u.fileInfo = source

	zb, fileSize := compressFile(u.src)
	ratio := float64(len(zb)) / float64(fileSize)

	if fileSize < (*maxUncompressedSize<<10) || ratio > *minCompressionRatio {
		u.compressed = false
		u.data, err = ioutil.ReadFile(u.src)
		if err != nil {
			return err
		}
	} else {
		u.compressed = true
		u.data = zb
	}

	goStat, err := os.Stat(u.gofile)
	if err == nil && goStat.ModTime().After(source.ModTime()) && !*overwrite {
		// file exits and is *after* the mod time of source -- do nothing
		log.Printf("Skipping %s", u.gofile)
		return nil
	}

	var goFile *os.File
	if err != nil {
		goFile, err = os.Create(u.gofile)
		if err != nil {
			log.Printf("Warning: cannot create file %s", u.gofile)
			return err
		}
	} else {
		goFile, err = os.OpenFile(u.gofile, os.O_RDWR|os.O_TRUNC, 0660)
		if err != nil {
			log.Printf("Warning: cannot open file %s", u.gofile)
			return err
		}
	}
	defer goFile.Close()
	err = u.writeLeafNode(goFile)

	if err == nil {
		log.Printf("Generated %s --> %s\n", u.src, u.gofile)
	} else {
		log.Printf("FAIL to generate %s --> %s\n", u.src, u.gofile)
	}
	return err
}

func (u *translationUnit) Gofmt() error {
	gofile, err := os.Open(u.gofile)
	if err != nil {
		log.Printf("Cannot open %s to run gofmt: %s\n", u.gofile, err)
		return err
	}
	fileSet := token.NewFileSet()
	ast, err := parser.ParseFile(fileSet, "", gofile, parser.ParseComments)
	if err != nil {
		log.Fatal(err)
	}

	var formatted bytes.Buffer
	config := &printer.Config{
		Mode:     printer.TabIndent | printer.UseSpaces,
		Tabwidth: 8,
	}
	err = config.Fprint(&formatted, fileSet, ast)
	if err != nil {
		log.Printf("Gofmt failed on %s: %s\n", u.gofile, err)
		return err
	}

	if err := ioutil.WriteFile(u.gofile, formatted.Bytes(), 0644); err != nil {
		log.Printf("Cannot write %s after gofmt: %s\n", u.gofile, err)
		return err
	}

	log.Printf("Ran gofmt on %s\n", u.gofile)
	return nil
}

// Compress the file
func compressFile(fileName string) ([]byte, int64) {
	var compressed bytes.Buffer
	in, err := os.Open(fileName)
	if err != nil {
		log.Fatal(err)
	}
	defer in.Close()
	out := zlib.NewWriter(&compressed)
	n, err := io.Copy(out, in)
	if err != nil {
		log.Fatal(err)
	}
	out.Close()
	return compressed.Bytes(), n
}

var matcher, _ = regexp.Compile("^(src|pkg)/")

// Searches the GOPATH and checks if the given path (absolute path)
// is relative to a component in the GOPATH.  If so, return the path
// without the src or pkg so that the path is suitable for using as
// import statements
func CheckGoPath(path string) (string, error) {
	pathenv := os.Getenv("GOPATH")
	if pathenv == "" {
		return "", errors.New("not found")
	}
	for _, dir := range strings.Split(pathenv, ":") {
		if dir == "" {
			// Unix shell semantics: path element "" means "."
			dir = "."
		}
		if rel, err := filepath.Rel(dir, path); err == nil {
			if matcher.MatchString(rel) {
				return matcher.ReplaceAllString(rel, ""), err
			} else {
				return rel, err
			}
		}
	}
	return "", errors.New("not found")
}
