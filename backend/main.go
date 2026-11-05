package main

import (
	"log"
	"net/http"
	"os"
	"time"
)

// The first day the mocked rate history covers.
var historyStart = time.Date(2016, time.January, 1, 0, 0, 0, 0, time.UTC)

func main() {
	addr := ":8080"
	if port := os.Getenv("PORT"); port != "" {
		addr = ":" + port
	}

	// The history ends on the day the server starts; a restart picks up any days since.
	srv := &server{store: newRateStore(historyStart, today())}

	log.Printf("listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, srv.routes()))
}
