package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/elmasy-com/columbus-sdk/db"
	"github.com/g0rbe/slitu"
	ct "github.com/google/certificate-transparency-go"
	"github.com/google/certificate-transparency-go/client"
	"github.com/google/certificate-transparency-go/jsonclient"
)

type LeafEntry struct {
	Index int64
	Entry *ct.LeafEntry
}

type IndexRange struct {
	Start int
	End   int
}

var (
	Version        string
	Commit         string
	configPath     = flag.String("config", "", "Path to the config file")
	version        = flag.Bool("version", false, "Print current version")
	LeafEntryChan  chan LeafEntry
	IndexRangeChan chan IndexRange
	Cancel         context.CancelFunc
)

func GetTreeSize() (int, error) {

	logClient, err := client.New(
		Conf.LogURI,
		&http.Client{Timeout: 30 * time.Second},
		jsonclient.Options{UserAgent: "Columbus-Scanner"})
	if err != nil {
		return 0, fmt.Errorf("failed to create log client: %w", err)
	}

	sth, err := logClient.GetSTH(context.TODO())
	if err != nil {
		return 0, fmt.Errorf("failed to get STH: %w", err)
	}

	return int(sth.TreeSize), nil
}

func main() {

	flag.Parse()

	if *version {
		fmt.Printf("Version: %s\n", Version)
		fmt.Printf("Git Commit: %s\n", Commit)
		os.Exit(0)
	}
	if *configPath == "" {
		fmt.Fprintf(os.Stderr, "-config is missing!\n")
		os.Exit(1)
	}

	err := ParseConfig(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to parse config: %s\n", err)
		os.Exit(1)
	}

	err = db.Connect(Conf.MongoURI)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to connect to MongoDB: %s\n", err)
		os.Exit(1)
	}
	defer db.Disconnect()

	IndexRangeChan = make(chan IndexRange)
	LeafEntryChan = make(chan LeafEntry, Conf.BufferSize)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	Cancel = cancel
	wg := sync.WaitGroup{}
	fetchWg := sync.WaitGroup{}

	wg.Add(2)
	go UptimeWorker(Conf.UptimeHook, ctx, &wg)
	go SaveConfigWorker(*configPath, ctx, &wg)

	for i := 0; i < Conf.NumWorkers; i++ {
		wg.Add(1)
		go InsertWorker(i, &wg)
	}
	for i := 0; i < Conf.ParallelFetch; i++ {
		fetchWg.Add(1)
		go FetchWorker(i, &fetchWg)
	}

infiniteLoop:
	for {

		size, err := GetTreeSize()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to get TreeSize: %s\n", err)
			cancel()
			break
		}

		if Conf.Verbose {
			fmt.Printf("Log size: %d\n", size)
		}

		for Conf.GetIndex() < size {

			select {
			case <-ctx.Done():
				break infiniteLoop
			default:

				batch := 0

				// Do not query more entry than size.
				if size-Conf.GetIndex() < Conf.GetBatchSize() {
					batch = size - Conf.GetIndex()
				} else {
					batch = Conf.GetBatchSize()
				}

				ir := IndexRange{Start: Conf.GetIndex()}
				ir.End = ir.Start + batch

				// BLOCKED HERE
				IndexRangeChan <- ir

				// Update Index after sent to the workers
				Conf.IncreaseIndex(batch)
			}
		}

		select {
		case <-ctx.Done():
			break infiniteLoop
		default:
			slitu.Sleep(ctx, 60*time.Second)
		}
	}

	// Close FetchWorkers first
	fmt.Printf("Closing fetchers...\n")
	close(IndexRangeChan)
	fetchWg.Wait()

	fmt.Printf("Closing insert workers...\n")
	close(LeafEntryChan)

	fmt.Printf("Waiting to close...\n")
	wg.Wait()
	fmt.Printf("Closed!\n")
}
