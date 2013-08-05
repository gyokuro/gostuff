package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"strconv"
)

// FLAGS
var port = flag.Int("p", 7777, "Port number")

// Current working directory...  for default directory to monitor
var currentWorkingDir, _ = os.Getwd()

func main() {

	flag.Parse()

	// Signal for shutdown
	done := make(chan bool)

	http.Handle("/", http.FileServer(http.Dir(".")))
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
