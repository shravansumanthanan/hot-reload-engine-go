package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	crashMode := flag.Bool("crash-mode", false, "Crash immediately to test loop protection")
	flag.Parse()

	if *crashMode {
		log.Println("CRASH MODE: Simulating rapid failure")
		time.Sleep(100 * time.Millisecond) // Small delay before crash
		os.Exit(1)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintln(w, "<html><body><h1>I am testing this!</h1></body></html>")
	})

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		/* #nosec G706 */
		log.Printf("Starting server on port %s...", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("ListenAndServe error: %v", err)
		}
	}()

	// Simulate a process that ignores SIGTERM slightly but then exits
	// Or gracefully shuts down.
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	sig := <-sigs
	log.Printf("Received signal: %v, gracefully shutting down...", sig)

	// Simulate some cleanup work
	time.Sleep(200 * time.Millisecond)

	_ = server.Close()
	log.Println("Server exiting")
}
