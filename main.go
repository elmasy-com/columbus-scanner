package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/elmasy-com/columbus-sdk/db"
	"github.com/elmasy-com/go-ctstream"
	"github.com/g0rbe/slitu"
	ct "github.com/google/certificate-transparency-go"
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

	fmt.Printf("Reading config file %s...\n", *configPath)

	err := ParseConfig(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to parse config: %s\n", err)
		os.Exit(1)
	}

	fmt.Printf("Loading index from %s...\n", Conf.IndexFile)

	err = LoadIndex(Conf.IndexFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load index: %s\n", err)
		os.Exit(1)
	}

	fmt.Printf("Continue from index %d\n", Index.Load())

	fmt.Printf("Connecting to MongoDB...\n")
	err = db.Connect(Conf.MongoURI)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to connect to MongoDB: %s\n", err)
		os.Exit(1)
	}
	defer db.Disconnect()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	Cancel = cancel
	wg := new(sync.WaitGroup)

	wg.Add(1)
	go UptimeWorker(Conf.UptimeHook, ctx, wg)

	wg.Add(1)
	go IndexSaver(ctx, wg)

infiniteLoop:
	for {

		select {
		case <-ctx.Done():
			break infiniteLoop
		default:

			scanner, err := ctstream.NewScanner(ctx, ctstream.FindByName(Conf.LogName), int(Index.Load()), Conf.FetcherWorker, Conf.SkipPreCert)
			if err != nil {
				if errors.Is(err, ctstream.ErrNothingToDo) {
					fmt.Printf("Nothing to do...\n")
					slitu.Sleep(ctx, 60*time.Second)
					continue infiniteLoop
				}
				fmt.Fprintf(os.Stderr, "Failed to create scanner: %s\n", err)
				Cancel()
				break infiniteLoop
			}

			fmt.Printf("%s progress: %d/%d(%.2f%%)\n", Conf.LogName, Index.Load(), scanner.End, float64(Index.Load())/float64(scanner.End)*100)

			for i := 0; i < Conf.InsertWorkers; i++ {
				wg.Add(1)
				go InsertWorker(scanner.EntryChan, wg)
			}

			for err := range scanner.ErrChan {
				fmt.Fprintf(os.Stderr, "Scanner error: %s\n", err)
				if !strings.Contains(err.Error(), "NonFatalErrors") {
					Cancel()
					break infiniteLoop
				}
			}
		}

		slitu.Sleep(ctx, 60*time.Second)
	}

	fmt.Printf("Waiting to close...\n")
	wg.Wait()
	fmt.Printf("Closed!\n")
}
