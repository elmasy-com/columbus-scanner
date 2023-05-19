package main

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	LogName       string `yaml:"LogName"`
	MongoURI      string `yaml:"MongoURI"`
	UptimeHook    string `yaml:"UptimeHook"`
	InsertWorkers int    `yaml:"InsertWorkers"`
	FetcherWorker int    `yaml:"FetcherWorker"`
	IndexFile     string `yaml:"IndexFile"`
	SkipPreCert   bool   `yaml:"SkipPreCert"`
	Verbose       bool   `yaml:"Verbose"`
}

var Conf *Config

// ParseConfig parses the config file in path and set the global variable Conf.
func ParseConfig(path string) error {

	Conf = &Config{}

	out, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read %s: %s", path, err)
	}

	err = yaml.Unmarshal(out, Conf)
	if err != nil {
		return fmt.Errorf("failed to unmarshal: %s", err)
	}

	switch {
	case Conf.LogName == "":
		return fmt.Errorf("LogName is missing")
	case Conf.MongoURI == "":
		return fmt.Errorf("MongoURI is missing")
	}

	if Conf.InsertWorkers < 0 {
		return fmt.Errorf("NumWorkers is negative")
	}
	if Conf.InsertWorkers == 0 {
		Conf.InsertWorkers = 2
	}

	if Conf.FetcherWorker < 0 {
		return fmt.Errorf("ParallelFetch is negative")
	}
	if Conf.FetcherWorker == 0 {
		Conf.FetcherWorker = 2
	}

	if Conf.IndexFile == "" {
		return fmt.Errorf("IndexFile is empty")
	}

	return nil
}
