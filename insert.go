package main

import (
	"errors"
	"fmt"
	"os"
	"sync"

	"github.com/elmasy-com/columbus-sdk/db"
	"github.com/elmasy-com/columbus-sdk/fault"
	"github.com/elmasy-com/elnet/domain"
	"github.com/elmasy-com/go-ctstream"
	"github.com/elmasy-com/slices"
)

// Insert Leaf Certificates into Columbus.
// The goroutine is stopped by closing the LeafENtryChan in main().
func InsertWorker(ec <-chan *ctstream.Entry, wg *sync.WaitGroup) {

	defer wg.Done()

	if Conf.Verbose {
		fmt.Printf("Insert Worker started!\n")
		defer fmt.Printf("Insert Worker stopped!\n")
	}

	for e := range ec {

		domains := make([]string, 0)

		if domain.IsValid(e.Certificate.Subject.CommonName) {
			domains = slices.AppendUnique(domains, e.Certificate.Subject.CommonName)
		}

		for i := range e.Certificate.DNSNames {
			if domain.IsValid(e.Certificate.DNSNames[i]) {
				domains = slices.AppendUnique(domains, e.Certificate.DNSNames[i])
			}
		}

		for i := range e.Certificate.PermittedDNSDomains {
			if domain.IsValid(e.Certificate.PermittedDNSDomains[i]) {
				domains = slices.AppendUnique(domains, e.Certificate.PermittedDNSDomains[i])
			}
		}

		// Write only unique and valid domains
		for i := range domains {
			if !domain.IsValid(domains[i]) {
				continue
			}

			if err := db.Insert(domains[i]); err != nil {

				// Failed insert is fatal error. Dont want to miss any domain.
				fmt.Fprintf(os.Stderr, "Failed to write %s: %s\n", domains[i], err)

				// d is probably a TLD
				if errors.Is(err, fault.ErrGetPartsFailed) {
					continue
				}

				Cancel()
			}
		}

		if Index.Load() < int64(e.Index) {
			Index.Store(int64(e.Index))
		}
	}
}
