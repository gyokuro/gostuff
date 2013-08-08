package main

import (
	"flag"
	"fmt"
	generator "github.com/gyokuro/embedfs/pkg/embedfs"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	destDir        = flag.String("destDir", ".", "Destination directory.")
	createDestDir  = flag.Bool("createDestDir", true, "Creation destination directory if not exists.")
	matchPattern   = flag.String("match", ".+\\.(js|css|html|png)$", "Regex to match target files.")
	excludePattern = flag.String("exclude", ".+(\\.git).*", "Regex to exclude target files.")
	byteSlice      = flag.Bool("byteSlice", true, "Represent binary data as byte slice.")
	gofmt          = flag.Bool("gofmt", true, "Run gofmt on generated source.")
	generate       = flag.Bool("generate", false, "True to really write actual files.")
)

func main() {
	flag.Parse()

	pwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	log.Println("Current working directory: ", pwd)

	d, err := os.Stat(*destDir)
	if err != nil && *createDestDir {
		err = os.MkdirAll(*destDir, 0777)
		if err != nil {
			fmt.Println("Cannot create destDir: ", *destDir)
			panic(err)
		}
	} else if !d.IsDir() {
		fmt.Println("destDir ", *destDir, " is not a directory.")
		panic(err)
	}

	destDirAbs, err := filepath.Abs(*destDir)
	if err != nil {
		fmt.Println("Not valid directory -- Cannot derive absolute path from destDir: ", *destDir)
		panic(err)
	}

	// Get the import root for the packages that will be generated.
	importRoot, err := generator.CheckGoPath(destDirAbs)
	if err != nil {
		fmt.Println("destDir ", *destDir, " not reachable in $GOPATH")
		panic(err)
	}
	log.Println("Import root: ", importRoot)

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

	var match, exclude *regexp.Regexp = nil, nil

	if len(*matchPattern) > 0 {
		match, err = regexp.Compile(*matchPattern)
		if err != nil {
			panic(err)
		}
	}
	if len(*excludePattern) > 0 {
		exclude, err = regexp.Compile(*excludePattern)
		if err != nil {
			panic(err)
		}
	}

	// Get all the target files -- keyed by the directory
	files, err := getAllFiles(dir)

	filesByDirectory := make(map[string][]string)
	for _, file := range files {

		selected := true

		if match != nil && !match.MatchString(file) {
			selected = false
		}

		if selected && exclude != nil && exclude.MatchString(file) {
			selected = false
		}

		if selected {
			log.Printf("Selected: %s/%s\n", filepath.Dir(file), filepath.Base(file))
			dir := filepath.Dir(file)
			base := filepath.Base(file)
			if _, exists := filesByDirectory[dir]; exists {
				filesByDirectory[dir] = append(filesByDirectory[dir], base)
			} else {
				filesByDirectory[dir] = []string{base}
			}
		}
	}

	// 1. Create directories for all the keys in filesByDirectory
	// 2. Generate the go file and place them in the directory
	for dir, files := range filesByDirectory {
		outDir := filepath.Join(*destDir, dir)
		err = os.MkdirAll(outDir, 0777)
		if err != nil {
			log.Fatalf("Cannot create directory %s: %s", outDir, err)
		}
		packageName := strings.Replace(dir, string(os.PathSeparator), "_", -1)
		packageName = strings.Replace(packageName, ".", "_", -1)
		for _, file := range files {
			srcFile := filepath.Join(dir, file)
			u := generator.NewTranslationUnit(packageName, srcFile, file, outDir, *byteSlice)
			if *generate {
				err = u.Translate()
				if err != nil {
					panic(err)
				}

				if *gofmt && u.Gofmt() != nil {
					panic(err)
				}
			} else {
				log.Printf("Translation Unit: %s", u)
			}
		}
	}

	// 3. Look at the directory hierachy and generate toc entries for each directory
	dirSeen := make(map[string]bool)
	dirHierarchy := make(map[string][]string)
	for directory, _ := range filesByDirectory {

		p := directory
		if _, exists := dirHierarchy[p]; !exists {
			dirHierarchy[p] = []string{}
		}
		for {
			parent := filepath.Dir(p)
			child := filepath.Base(p)

			if parent == "." {
				break
			}

			if _, seen := dirSeen[p]; !seen {
				if list, exists := dirHierarchy[parent]; exists {
					dirHierarchy[parent] = append(list, child)
				} else {
					dirHierarchy[parent] = []string{child}
				}
				dirSeen[p] = true
			}
			p = parent
		}
	}

	for directory, children := range dirHierarchy {
		toc := generator.NewDirToc(destDirAbs, importRoot, directory, children)
		if *generate {
			err = toc.Translate()
			if err != nil {
				panic(err)
			}
			if *gofmt && toc.Gofmt() != nil {
				panic(err)
			}
		} else {
			log.Printf("TOC: %s", toc)
		}
	}
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
