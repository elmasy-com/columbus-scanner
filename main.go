package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	sdk "github.com/elmasy-com/columbus-sdk"
	"github.com/elmasy-com/columbus-sdk/fault"
	"github.com/elmasy-com/elnet/domain"
	"github.com/elmasy-com/slices"
	"github.com/g0rbe/certificate-transparency-go/scanner"
	ct "github.com/google/certificate-transparency-go"
	"github.com/google/certificate-transparency-go/client"
	"github.com/google/certificate-transparency-go/jsonclient"
	"github.com/google/trillian/client/backoff"
)

var (
	Version string
	Commit  string
	config  = flag.String("config", "", "Path to the config file")
	version = flag.Bool("version", false, "Print current version")
	printOk bool
)

func skipPreCert(entry *ct.RawLogEntry) {
}

func insertCert(entry *ct.RawLogEntry) {

	e, err := entry.ToLogEntry()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed convert %d to logentry: %s\n", entry.Index, err)
		return
	}

	// Use this with slices.Contain() to filter duplicated domains (prefilter).
	domains := make([]string, 0)

	// Fetch domains from cert and send it to writer through domainChan
	if e.X509Cert != nil {

		if !slices.Contain(domains, e.X509Cert.Subject.CommonName) {
			domains = append(domains, e.X509Cert.Subject.CommonName)
		}

		for i := range e.X509Cert.DNSNames {
			if !slices.Contain(domains, e.X509Cert.DNSNames[i]) {
				domains = append(domains, e.X509Cert.DNSNames[i])
			}
		}

		for i := range e.X509Cert.PermittedDNSDomains {
			if !slices.Contain(domains, e.X509Cert.PermittedDNSDomains[i]) {
				domains = append(domains, e.X509Cert.PermittedDNSDomains[i])
			}
		}
	}

	// Write only unique and valid domains
	for i := range domains {
		if !domain.IsValid(domains[i]) {
			continue
		}
		if err := sdk.Insert(domains[i]); err != nil {
			if errors.Is(err, fault.ErrInvalidDomain) ||
				errors.Is(err, fault.ErrPublicSuffix) ||
				// Check string for backward compatibility
				strings.Contains(err.Error(), "cannot derive eTLD+1 for domain") {
				continue
			}

			// Failed write is fatal error. Dont want to miss any domain.
			fmt.Fprintf(os.Stderr, "Failed to write %s: %s\n", domains[i], err)
			os.Exit(1)

		} else if printOk {
			fmt.Printf("Domain (#%d) successfully inserted: %s\n", e.Index, domains[i])
		}
	}

	// The index of failed converted entry is not sent.
	IndexChan <- e.Index
}

func main() {

	flag.Parse()

	if *version {
		fmt.Printf("Version: %s\n", Version)
		fmt.Printf("Git Commit: %s\n", Commit)
		os.Exit(0)
	}
	if *config == "" {
		fmt.Fprintf(os.Stderr, "config is missing!\n")
		os.Exit(1)
	}

	c, err := ParseConfig(*config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to parse config: %s\n", err)
		os.Exit(1)
	}

	sdk.SetURI(c.Server)

	go callUptimeHook(c.UptimeHook)
	go saveConfig(*config, &c)
	printOk = c.PrintOK

	if err := sdk.GetDefaultUser(c.APIKey); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get Columbus user: %s\n", err)
		os.Exit(1)
	}

	logClient, err := client.New(c.LogURI, &http.Client{
		Timeout: 60 * time.Second,
		Transport: &http.Transport{
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 60 * time.Second,
			MaxIdleConnsPerHost:   200,
			DisableKeepAlives:     false,
			MaxIdleConns:          200,
			IdleConnTimeout:       90 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}, jsonclient.Options{UserAgent: "ct-go-scanlog/1.0"})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create log client: %s\n", err)
		os.Exit(1)
	}

	opts := scanner.ScannerOptions{
		FetcherOptions: scanner.FetcherOptions{
			BatchSize:     c.BatchSize,
			ParallelFetch: c.ParallelFetch,
			StartIndex:    c.StartIndex,
			//EndIndex:      *endIndex, // Always use getSTH.
			Continuous: true,
			LogBackoff: &backoff.Backoff{
				Min:    1 * time.Second,
				Max:    3600 * time.Second,
				Factor: 2,
				Jitter: true,
			},
		},
		Matcher:    scanner.MatchAll{},
		NumWorkers: c.NumWorkers,
		BufferSize: c.BufferSize,
	}

	s := scanner.NewScanner(logClient, opts)

	err = s.Scan(context.Background(), insertCert, skipPreCert)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Scanner failed: %s\n", err)
		os.Exit(1)
	}

}
