
Building

The embedfs command (`main.go`) itself depends on one of the source files (`pkg/embedfs/fs.go`) to be
packaged within the binary -- so that it can generate the filesystem api implementations.

The embedded filesystem that the program depends on is in the `resources` directory.
To embed the fs.go source code itself in the executable:

    cd pkg
    go run ../main.go -destDir=../resources -match="/fs\\.go$" -generate=true .

This will generate the go files to be compiled.  Then,

    cd .. # back to where main.go is
    go build -o embedfs main.go
