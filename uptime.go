package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"
)

// Call the uptime hook every 60 sec in the background.
// Returns if hook is empty ("").
func UptimeWorker(hook string, ctx context.Context, wg *sync.WaitGroup) {

	defer wg.Done()

	if hook == "" {
		return
	}

	if Conf.Verbose {
		fmt.Printf("Uptime Worker started!\n")
		defer fmt.Printf("Uptime Worker stopped!\n")
	}

	httpClient := http.Client{Timeout: 30 * time.Second}

	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for {

		select {
		case <-ticker.C:

			_, err := httpClient.Get(hook)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to call UptimeHook: %s\n", err)
			}

		case <-ctx.Done():
			return
		}
	}
}
