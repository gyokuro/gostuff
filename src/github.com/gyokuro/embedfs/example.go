package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"strconv"
)

import (
	static "github.com/gyokuro/embedfs/static"
)

// FLAGS
var port = flag.Int("p", 7777, "Port number")

// Current working directory...  for default directory to monitor
var currentWorkingDir, _ = os.Getwd()

func main() {

	flag.Parse()

	// Signal for shutdown
	done := make(chan bool)

	// http://localhost:7777/site/js/bootstrap.min.js

	// The following are equivalent, if run from working directory containing 'static'
	//
	//http.Handle("/", http.FileServer(http.Dir("static")))
	http.Handle("/", http.FileServer(static.Dir(".")))

	httpListen := ":" + strconv.Itoa(*port)
	go func() {
		err := http.ListenAndServe(httpListen, nil)
		if err != nil {
			panic(err)
		}
	}()
	log.Printf("Started in fileserver. Listening on %s\n", httpListen)

	<-done // This just blocks until a bool is sent on the channel
}
