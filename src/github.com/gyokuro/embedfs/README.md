
# Building

The embedfs command (`main.go`) itself depends on one of the source files (`pkg/embedfs/fs.go`) to be
packaged within the binary -- so that it can generate the filesystem api implementations.

The embedded filesystem that the program depends on is in the `resources` directory.
To embed the fs.go source code itself in the executable:

    cd pkg
    go run ../main.go -destDir=../resources -match="/fs\\.go$" -generate=true .

This will generate the go files to be compiled.  Then,

    cd .. # back to where main.go is
    go build -o embedfs main.go


# Running the Twitter Bootstrap Example

The Twitter Bootstrap example site is included in the `examples` folder, along with a simple server
`bootstrap-examples-master.go`.  To embed the entire site into a single server, do

    cd examples
    unzip bootstrap-examples-master.zip
    ../embedfs -generate=true bootstrap-examples-master
    go run ../bootstrap-examples-master.go

To prove that the entire site is embedded into a single executable, just build the example

    go build -o bootstrap-examples-master ../bootstrap-examples-master.go

Then run the resulting executable, `bootstrap-examples-master`:

    ./bootstrap-examples-master

and open the browser at [localhost](http://localhost:7777)
