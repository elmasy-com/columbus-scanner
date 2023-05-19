package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

var Index atomic.Int64

// IndexSaver saves the Index periodically in a goroutine.
func IndexSaver(ctx context.Context, wg *sync.WaitGroup) {

	defer wg.Done()

	if Conf.Verbose {
		fmt.Printf("Index Saver started!\n")
		defer fmt.Printf("Index Saver stopped!\n")
	}

	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for {

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:

			if Conf.Verbose {
				fmt.Printf("Saving index %d to %s\n", Index.Load(), Conf.IndexFile)
			}

			file, err := os.OpenFile(Conf.IndexFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0640)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to open %s: %s\n", Conf.IndexFile, err)
				continue
			}
			_, err = file.WriteString(strconv.Itoa(int(Index.Load())))
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to write to %s: %s\n", Conf.IndexFile, err)
			}
			file.Close()

		}
	}
}

// LoadIndex loads the global Index from a file.
func LoadIndex(path string) error {

	out, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			Index.Store(0)
			return nil
		}
		return fmt.Errorf("failed to read %s: %w", path, err)
	}

	i, err := strconv.Atoi(string(out))
	if err != nil {
		return fmt.Errorf("failed to convert %s to string: %w", out, err)
	}

	Index.Store(int64(i))

	return nil
}
