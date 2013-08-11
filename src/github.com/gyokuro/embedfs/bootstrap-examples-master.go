package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"strconv"
)

import (
	bootstrap_examples "github.com/gyokuro/embedfs/examples/bootstrap-examples-master"
)

// FLAGS
var port = flag.Int("p", 7777, "Port number")

// Current working directory...  for default directory to monitor
var currentWorkingDir, _ = os.Getwd()

func main() {

	flag.Parse()

	done := make(chan bool)

	log.Println("Setting up resources")
	fs := bootstrap_examples.Dir(".")
	log.Println("Resources ready.")

	http.Handle("/", http.FileServer(fs))
	httpListen := ":" + strconv.Itoa(*port)

	go func() {
		if err := http.ListenAndServe(httpListen, nil); err != nil {
			panic(err)
		}
	}()

	log.Println("Started server @", httpListen)

	<-done
}
