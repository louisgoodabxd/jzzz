package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
)

const (
	ConfigPath     = "/data/adb/srun/config.conf"
	DefaultGateway = "gw.imust.edu.cn"
	DefaultGatewayIP = "10.16.42.48"
	DefaultACID    = "6"
	DefaultSSIDs   = "imust,THUNDER"
	DefaultMaxRetry = 3
	DefaultCheckInterval = 5
	DefaultSuccessInterval = 30
)

type Config struct {
	mu              sync.RWMutex
	USERNAME        string `json:"USERNAME"`
	PASSWORD        string `json:"PASSWORD"`
	GATEWAY         string `json:"GATEWAY"`
	GATEWAY_IP      string `json:"GATEWAY_IP"`
	AC_ID           string `json:"AC_ID"`
	SSIDS           string `json:"SSIDS"`
	MAX_RETRY       int    `json:"MAX_RETRY"`
	CHECK_INTERVAL  int    `json:"CHECK_INTERVAL"`
	SUCCESS_INTERVAL int   `json:"SUCCESS_INTERVAL"`
}

func NewConfig() *Config {
	return &Config{
		USERNAME:         "",
		PASSWORD:         "",
		GATEWAY:          DefaultGateway,
		GATEWAY_IP:       DefaultGatewayIP,
		AC_ID:            DefaultACID,
		SSIDS:            DefaultSSIDs,
		MAX_RETRY:        DefaultMaxRetry,
		CHECK_INTERVAL:   DefaultCheckInterval,
		SUCCESS_INTERVAL: DefaultSuccessInterval,
	}
}

func (c *Config) Load() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	f, err := os.Open(ConfigPath)
	if err != nil {
		if os.IsNotExist(err) {
			return c.saveLocked()
		}
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])

		switch key {
		case "USERNAME":
			c.USERNAME = trimQuotes(val)
		case "PASSWORD":
			c.PASSWORD = trimQuotes(val)
		case "GATEWAY":
			c.GATEWAY = trimQuotes(val)
		case "GATEWAY_IP":
			c.GATEWAY_IP = trimQuotes(val)
		case "AC_ID":
			c.AC_ID = trimQuotes(val)
		case "SSIDS":
			c.SSIDS = trimQuotes(val)
		case "MAX_RETRY":
			c.MAX_RETRY = parseInt(val, DefaultMaxRetry)
		case "CHECK_INTERVAL":
			c.CHECK_INTERVAL = parseInt(val, DefaultCheckInterval)
		case "SUCCESS_INTERVAL":
			c.SUCCESS_INTERVAL = parseInt(val, DefaultSuccessInterval)
		}
	}
	return scanner.Err()
}

func (c *Config) Save() error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.saveLocked()
}

func (c *Config) saveLocked() error {
	dir := strings.TrimSuffix(ConfigPath, "/config.conf")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	f, err := os.Create(ConfigPath)
	if err != nil {
		return err
	}
	defer f.Close()

	w := bufio.NewWriter(f)
	fmt.Fprintf(w, "USERNAME=%q\n", c.USERNAME)
	fmt.Fprintf(w, "PASSWORD=%q\n", c.PASSWORD)
	fmt.Fprintf(w, "GATEWAY=%q\n", c.GATEWAY)
	fmt.Fprintf(w, "GATEWAY_IP=%q\n", c.GATEWAY_IP)
	fmt.Fprintf(w, "AC_ID=%q\n", c.AC_ID)
	fmt.Fprintf(w, "SSIDS=%q\n", c.SSIDS)
	fmt.Fprintf(w, "MAX_RETRY=%d\n", c.MAX_RETRY)
	fmt.Fprintf(w, "CHECK_INTERVAL=%d\n", c.CHECK_INTERVAL)
	fmt.Fprintf(w, "SUCCESS_INTERVAL=%d\n", c.SUCCESS_INTERVAL)
	return w.Flush()
}

func (c *Config) UpdateFromJSON(other *Config) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if other.USERNAME != "" {
		c.USERNAME = other.USERNAME
	}
	if other.PASSWORD != "" {
		c.PASSWORD = other.PASSWORD
	}
	if other.GATEWAY != "" {
		c.GATEWAY = other.GATEWAY
	}
	if other.GATEWAY_IP != "" {
		c.GATEWAY_IP = other.GATEWAY_IP
	}
	if other.AC_ID != "" {
		c.AC_ID = other.AC_ID
	}
	if other.SSIDS != "" {
		c.SSIDS = other.SSIDS
	}
	if other.MAX_RETRY > 0 {
		c.MAX_RETRY = other.MAX_RETRY
	}
	if other.CHECK_INTERVAL > 0 {
		c.CHECK_INTERVAL = other.CHECK_INTERVAL
	}
	if other.SUCCESS_INTERVAL > 0 {
		c.SUCCESS_INTERVAL = other.SUCCESS_INTERVAL
	}
}

func (c *Config) Clone() Config {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return Config{
		USERNAME:         c.USERNAME,
		PASSWORD:         c.PASSWORD,
		GATEWAY:          c.GATEWAY,
		GATEWAY_IP:       c.GATEWAY_IP,
		AC_ID:            c.AC_ID,
		SSIDS:            c.SSIDS,
		MAX_RETRY:        c.MAX_RETRY,
		CHECK_INTERVAL:   c.CHECK_INTERVAL,
		SUCCESS_INTERVAL: c.SUCCESS_INTERVAL,
	}
}

func trimQuotes(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

func parseInt(s string, def int) int {
	s = trimQuotes(s)
	v, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return v
}
