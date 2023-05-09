package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/elmasy-com/columbus-sdk/db"
	"github.com/elmasy-com/columbus-sdk/fault"
	"github.com/elmasy-com/elnet/domain"
	"github.com/elmasy-com/slices"
	ct "github.com/google/certificate-transparency-go"
	"github.com/google/certificate-transparency-go/client"
	"github.com/google/certificate-transparency-go/jsonclient"
)

// Call the uptime hook every 60 sec in the background.
// Returns if hook is empty ("").
func UptimeWorker(hook string, ctx context.Context, wg *sync.WaitGroup) {

	defer wg.Done()

	if hook == "" {
		return
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
			fmt.Printf("Uptime hook stopped!\n")
			return
		}
	}
}

// SaveConfig saves the current settings every 60 second in the background.
func SaveConfigWorker(path string, ctx context.Context, wg *sync.WaitGroup) {

	defer wg.Done()

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:

			out, err := Conf.ToYaml()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to marshal config: %s", err)
				continue
			}

			err = os.WriteFile(path, out, 0644)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to save config: %s\n", err)
			}
		case <-ctx.Done():
			fmt.Printf("Background saver stopped!\n")
			return
		}
	}
}

// Insert Leaf Certificates into Columbus.
// The goroutine is stopped by closing the LeafENtryChan in main().
func InsertWorker(id int, wg *sync.WaitGroup) {

	defer wg.Done()

	for entry := range LeafEntryChan {

		rawEntry, err := ct.RawLogEntryFromLeaf(entry.Index, entry.Entry)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to convert #%d to raw log entry: %s\n", entry.Index, err)
			continue
		}

		e, err := rawEntry.ToLogEntry()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to convert #%d to log entry: %s\n", entry.Index, err)
			continue
		}

		domains := make([]string, 0)

		// Fetch domains from precert
		if !Conf.SkipPreCert && e.Precert != nil && e.Precert.TBSCertificate != nil {

			domains = slices.AppendUnique(domains, e.Precert.TBSCertificate.Subject.CommonName)

			for i := range e.Precert.TBSCertificate.DNSNames {
				domains = slices.AppendUnique(domains, e.Precert.TBSCertificate.DNSNames[i])
			}

			for i := range e.Precert.TBSCertificate.PermittedDNSDomains {
				domains = slices.AppendUnique(domains, e.Precert.TBSCertificate.PermittedDNSDomains[i])
			}
		}

		// Fetch domains from cert
		if e.X509Cert != nil {

			domains = slices.AppendUnique(domains, e.X509Cert.Subject.CommonName)

			for i := range e.X509Cert.DNSNames {
				domains = slices.AppendUnique(domains, e.X509Cert.DNSNames[i])
			}

			for i := range e.X509Cert.PermittedDNSDomains {
				domains = slices.AppendUnique(domains, e.X509Cert.PermittedDNSDomains[i])
			}
		}

		var d string

		// Write only unique and valid domains
		for i := range domains {
			if !domain.IsValid(domains[i]) {
				continue
			}

			d = domain.Clean(domains[i])

			if Conf.Verbose {
				fmt.Printf("Inserting %d %s ...\n", entry.Index, d)
			}

			if err := db.Insert(d); err != nil {

				// Failed insert is fatal error. Dont want to miss any domain.
				fmt.Fprintf(os.Stderr, "Failed to write %s: %s\n", domains[i], err)

				// d is probably a TLD
				if errors.Is(err, fault.ErrGetPartsFailed) {
					continue
				}

				Cancel()
			}
		}
	}
}

// FetchWorker is fetching certificates from log and send it into LeafEntryChan.
// The goroutine is stopped by closing the IndexRangeChan channel in main().
func FetchWorker(id int, wg *sync.WaitGroup) {

	defer wg.Done()

	logClient, err := client.New(
		Conf.LogURI,
		&http.Client{Timeout: 30 * time.Second},
		jsonclient.Options{UserAgent: "Columbus-Scanner"})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create log client: %s\n", err)
		Cancel()
		return
	}

	for r := range IndexRangeChan {

		// r.End will be the next r.Start, so skip it at the current iteration
		for r.Start < r.End {

			leafEntries, err := logClient.GetRawEntries(context.TODO(), int64(r.Start), int64(r.End))
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to get raw entries: %s\n", err)
				Cancel()
				break
			}

			if leafEntries == nil || leafEntries.Entries == nil {
				continue
			}

			for i := range leafEntries.Entries {

				LeafEntryChan <- LeafEntry{
					Index: int64(r.Start),
					Entry: &leafEntries.Entries[i],
				}
				r.Start++
			}
		}
	}
}
