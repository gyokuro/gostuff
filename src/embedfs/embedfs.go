package main

import (
	"bytes"
	"compress/zlib"
	"flag"
	"fmt"
	"go/parser"
	"go/printer"
	"go/token"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

const (
	maxUncompressed = 50 << 10 // 50KB
	// Threshold ratio for compression.
	// Files which don't compress at least as well are kept uncompressed.
	zRatio = 0.5
)

var (
	destDir             = flag.String("dest", ".", "Destination directory.")
	pattern             = flag.String("pattern", ".+\\.(js|css|html)$", "Regex to match target files.")
	maxUncompressedSize = flag.Int64("maxUncompressedK", 5, "Max in kilobytes uncompressed.")
	minCompressionRatio = flag.Float64("minCompressionRatio", 0.5, "Min compression ratio.")
	byteSlice           = flag.Bool("byteSlice", true, "Represent binary data as byte slice.")
	overwrite           = flag.Bool("overwrite", true, "Overwrite existing generated source.")
	gofmt               = flag.Bool("gofmt", true, "Run gofmt on generated source.")
	test                = flag.Bool("test", true, "Test run. No writing actual files.")
)

func main() {
	flag.Parse()

	pwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	fmt.Println("Current working directory: ", pwd)

	dir := "."
	switch flag.NArg() {
	case 0:
	case 1:
		dir = flag.Arg(0)
		dirStat, err := os.Lstat(dir)
		switch {
		case err != nil:
			log.Fatalf("%s does not exist.", dir)
		case !dirStat.IsDir():
			log.Fatalf("%s is not a directory.", dir)
		}
	default:
		executable, err := exec.LookPath(os.Args[0])
		if err == nil {
			fmt.Fprintf(os.Stderr, "usage: %s [<dir>]\n", executable)
		} else {
			fmt.Fprintf(os.Stderr, "usage: resourcefs [<dir>]\n")
		}
		os.Exit(2)
	}

	match, err := regexp.Compile(*pattern)
	if err != nil {
		panic(err)
	}

	// Get all the target files -- keyed by the directory
	fs := make(map[string][]string)
	files, err := getAllFiles(dir)
	for _, file := range files {
		if match.MatchString(file) {
			fmt.Printf("%s:  %s -- %s\n", file, filepath.Dir(file), filepath.Base(file))

			dir := filepath.Dir(file)
			base := filepath.Base(file)
			if _, exists := fs[dir]; exists {
				fs[dir] = append(fs[dir], file)
			} else {
				fs[dir] = []string{base}
			}
		}
	}

	// 1. Create directories for all the keys in fs
	// 2. Generate the go file and place them in the directory
	// 3. For each directory, generate a toc.go to mimic the file system -- including nested folders

	for dir, files := range fs {
		outDir := filepath.Join(*destDir, dir)
		err = os.MkdirAll(outDir, 0777)
		if err != nil {
			log.Fatalf("Cannot create directory %s: %s", outDir, err)
		}
		packageName := strings.Replace(dir, string(os.PathSeparator), "_", -1)
		packageName = strings.Replace(packageName, ".", "_", -1)
		for _, file := range files {
			goFile := file + ".go"
			srcFile := filepath.Join(dir, file)

			u := &translationUnit{
				name:        strings.Replace(file, ".", "_", -1),
				src:         srcFile,
				gofile:      filepath.Join(outDir, goFile),
				packageName: packageName,
				newLine:     true,
				asByteSlice: *byteSlice,
			}

			if !*test {
				err = u.translate()
				if err != nil {
					panic(err)
				}

				if *gofmt && u.gofmt() != nil {
					panic(err)
				}
			} else {
				log.Printf("Translation Unit: %s", u)
			}
		}
	}
}

type translationUnit struct {
	name        string
	src         string
	gofile      string
	packageName string
	compressed  bool
	data        []byte
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
		fmt.Fprintf(u.writer, "var %s = []byte{\n", u.name)
	} else {
		fmt.Fprintf(u.writer, "const %s = \"", u.name)
	}
	// write to output the binary data
	io.Copy(u, bytes.NewBuffer(u.data))

	if u.asByteSlice {
		fmt.Fprintf(u.writer, "}\n")
	} else {
		fmt.Fprintf(u.writer, "\"\n")
	}
	return
}

func (u *translationUnit) translate() error {
	source, err := os.Stat(u.src)
	if err != nil {
		log.Fatalf("%s", err)
		return err
	}
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
	u.writer = goFile
	fmt.Fprintf(u.writer, "// %s\n", u.gofile)
	fmt.Fprintf(u.writer, "// GENERATED FROM %s\n", u.src)
	fmt.Fprintf(u.writer, "// DO NOT EDIT!!!\n")
	fmt.Fprintf(u.writer, "package %s\n", u.packageName)
	fmt.Fprintf(u.writer, "\n\n")
	fmt.Fprintf(u.writer, "const %s_compressed = %s\n", u.name, strconv.FormatBool(u.compressed))
	u.writeBinaryRepresentation()

	log.Printf("Generated %s --> %s\n", u.src, u.gofile)

	return err
}

func (u *translationUnit) gofmt() error {
	gofile, err := os.Open(u.gofile)
	if err != nil {
		log.Printf("Cannot open %s to run gofmt: %s\n", u.gofile, err)
		return err
	}
	fset := token.NewFileSet()
	ast, err := parser.ParseFile(fset, "", gofile, parser.ParseComments)
	if err != nil {
		log.Fatal(err)
	}

	var formatted bytes.Buffer
	config := &printer.Config{
		Mode:     printer.TabIndent | printer.UseSpaces,
		Tabwidth: 8,
	}
	err = config.Fprint(&formatted, fset, ast)
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

func concat(a []string, b []string) []string {
	c := make([]string, len(a)+len(b))
	copy(c, a)
	copy(c[len(a):], b)
	return c
}

// Returns a list of files under the given directory
func getAllFiles(path string) ([]string, error) {
	var result = make([]string, 0)
	stat, err := os.Lstat(path)
	if err != nil {
		log.Printf("Error stat %s: %s", path, err)
		return result, err
	}

	switch {
	case stat.Mode().IsRegular():
		result = append(result, filepath.Clean(path))
	case stat.Mode().IsDir():
		// List the directory contents
		files, err := ioutil.ReadDir(path)
		if err != nil {
			log.Printf("Error readdir %s: %s", path, err)
			return result, err
		}
		for _, file := range files {
			children, err := getAllFiles(filepath.Join(path, file.Name()))
			if err != nil {
				return result, err
			}
			result = concat(result, children)
		}
	}
	return result, err
}
