package main

import (
	"fmt"
	"os"
	"sync"

	"gopkg.in/yaml.v3"
)

type Config struct {
	LogURI        string `yaml:"LogURI"`
	MongoURI      string `yaml:"MongoURI"`
	UptimeHook    string `yaml:"UptimeHook"`
	NumWorkers    int    `yaml:"NumWorkers"`
	BatchSize     int    `yaml:"BatchSize"`
	ParallelFetch int    `yaml:"ParallelFetch"`
	StartIndex    int    `yaml:"StartIndex"`
	BufferSize    int    `yaml:"BufferSize"`
	SkipPreCert   bool   `yaml:"SkipPreCert"`
	Verbose       bool   `yaml:"Verbose"`
	m             *sync.Mutex
}

var Conf *Config

func (c *Config) ToYaml() ([]byte, error) {

	c.m.Lock()
	defer c.m.Unlock()

	return yaml.Marshal(c)
}

// IncreaseIndex increase StartIndex by v.
func (c *Config) IncreaseIndex(v int) {

	c.m.Lock()
	c.StartIndex += v
	c.m.Unlock()
}

func (c *Config) GetIndex() int {

	c.m.Lock()
	defer c.m.Unlock()

	return c.StartIndex
}

func (c *Config) GetBatchSize() int {
	c.m.Lock()
	defer c.m.Unlock()

	return c.BatchSize
}

// ParseConfig parses the config file in path and set the global variable Conf.
func ParseConfig(path string) error {

	Conf = &Config{m: &sync.Mutex{}}

	out, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read %s: %s", path, err)
	}

	err = yaml.Unmarshal(out, Conf)
	if err != nil {
		return fmt.Errorf("failed to unmarshal: %s", err)
	}

	switch {
	case Conf.LogURI == "":
		return fmt.Errorf("LogURI is missing")
	case Conf.MongoURI == "":
		return fmt.Errorf("MongoURI is missing")
	}

	if Conf.NumWorkers < 0 {
		return fmt.Errorf("NumWorkers is negative")
	}
	if Conf.NumWorkers == 0 {
		Conf.NumWorkers = 2
	}

	if Conf.BatchSize < 0 {
		return fmt.Errorf("BatchSize is negative")
	}
	if Conf.BatchSize == 0 {
		Conf.BatchSize = 1000
	}

	if Conf.ParallelFetch < 0 {
		return fmt.Errorf("ParallelFetch is negative")
	}
	if Conf.ParallelFetch == 0 {
		Conf.ParallelFetch = 2
	}

	// removes the last batch*10, this is a very lazy method to ensure that every log is parsed.
	switch {
	case Conf.StartIndex > Conf.BatchSize*Conf.ParallelFetch*10:
		Conf.StartIndex -= Conf.BatchSize * Conf.ParallelFetch * 10
		fmt.Printf("Continue from index #%d\n", Conf.StartIndex)
	case Conf.StartIndex <= Conf.BatchSize*Conf.ParallelFetch*10:
		Conf.StartIndex = 0
	}

	if Conf.BufferSize < 0 {
		return fmt.Errorf("BufferSize is negative")
	}
	if Conf.BufferSize == 0 {
		Conf.BufferSize = 5000
	}

	return nil
}
