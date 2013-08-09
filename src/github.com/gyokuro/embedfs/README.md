
Command to run:

    go run main.go -destDir=. -generate=true static

This will generate .go files in the directory `static` which also has the resource files.
To run the example which embeds the resources in the `static` directory:

    go run example.go

Embed the fs.go source code itself in the executable:

    go run ../main.go -destDir=../resources -match="/fs\\.go$" -generate=true .

This is run from the pkg/ directory so that package names will not include pkg_
