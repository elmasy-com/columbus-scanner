package main

import (
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	LogURI        string `yaml:"LogURI"`
	APIKey        string `yaml:"APIKey"`
	PrintOK       bool   `yaml:"PrintOK"`
	UptimeHook    string `yaml:"UptimeHook"`
	NumWorkers    int    `yaml:"NumWorkers"`
	BatchSize     int    `yaml:"BatchSize"`
	ParallelFetch int    `yaml:"ParallelFetch"`
	StartIndex    int64  `yaml:"StartIndex"`
	Server        string `yaml:"Server"`
	BufferSize    int    `yaml:"BufferSize"`
	SkipPreCert   bool   `yaml:"SkipPreCert"`
	m             *sync.Mutex
}

var IndexChan chan int64

func callUptimeHook(hook string) {

	if hook == "" {
		return
	}

	for {

		time.Sleep(60 * time.Second)

		_, err := http.Get(hook)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to call UptimeHook: %s\n", err)
		}

	}
}

func saveConfig(path string, c *Config) {

	IndexChan = make(chan int64, c.BatchSize)

	fmt.Printf("Background saver started!\n")

	timer := time.NewTicker(60 * time.Second)

	for {

		select {
		case <-timer.C:
			out, err := yaml.Marshal(&c)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to save config: %s", err)
				continue
			}

			err = os.WriteFile(path, out, 0644)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to save config: %s\n", err)
			}
		case i := <-IndexChan:
			if i > c.StartIndex {
				c.StartIndex = i
			}
		}
	}
}

func ParseConfig(path string) (Config, error) {

	c := Config{}

	c.m = &sync.Mutex{}

	out, err := os.ReadFile(path)
	if err != nil {
		return c, fmt.Errorf("failed to read %s: %s", path, err)
	}

	err = yaml.Unmarshal(out, &c)
	if err != nil {
		return c, fmt.Errorf("failed to unmarshal: %s", err)
	}

	switch {
	case c.LogURI == "":
		return c, fmt.Errorf("LogURI is missing")
	case c.APIKey == "":
		return c, fmt.Errorf("APIKey is missing")
	}

	if c.NumWorkers == 0 {
		c.NumWorkers = 2
	}
	if c.BatchSize == 0 {
		c.BatchSize = 1000
	}
	if c.ParallelFetch == 0 {
		c.ParallelFetch = 1
	}

	// removes the last batch*10, this is a very lazy method to ensure that every log is parsed.
	switch {
	case c.StartIndex > int64(c.BatchSize)*10:
		c.StartIndex -= int64(c.BatchSize) * 10
		fmt.Printf("Continue from index #%d\n", c.StartIndex)
	case c.StartIndex < int64(c.BatchSize):
		c.StartIndex = 0
	}

	if c.Server == "" {
		c.Server = "https://columbus.elmasy.com"
	}

	if c.BufferSize < 0 {
		return c, fmt.Errorf("BufferSize is negative")
	}
	if c.BufferSize == 0 {
		c.BufferSize = 5000
	}

	return c, nil
}
