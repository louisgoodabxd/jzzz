package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	PIDFile = "/data/adb/srun/daemon.pid"
)

// DaemonState holds the current state of the daemon
type DaemonState struct {
	mu            sync.RWMutex
	LoginState    string `json:"login_state"`    // online|idle|logging_in|connecting|error|reconnecting
	DaemonRunning bool   `json:"daemon_running"`
	PID           int    `json:"pid"`
	IP            string `json:"ip"`
	SSID          string `json:"ssid"`
	Gateway       string `json:"gateway"`
	Detail        string `json:"detail"`
	LastLogin     string `json:"last_login"`
	Username      string `json:"username"`
}

var (
	daemonState = &DaemonState{
		LoginState: "idle",
		PID:        os.Getpid(),
	}
	daemonRunning atomic.Bool
	stopDaemon    chan struct{}
)

func GetDaemonState() DaemonState {
	daemonState.mu.RLock()
	defer daemonState.mu.RUnlock()
	return DaemonState{
		LoginState:    daemonState.LoginState,
		DaemonRunning: daemonRunning.Load(),
		PID:           daemonState.PID,
		IP:            daemonState.IP,
		SSID:          daemonState.SSID,
		Gateway:       daemonState.Gateway,
		Detail:        daemonState.Detail,
		LastLogin:     daemonState.LastLogin,
		Username:      daemonState.Username,
	}
}

func setLoginState(state string) {
	daemonState.mu.Lock()
	defer daemonState.mu.Unlock()
	daemonState.LoginState = state
}

func setDaemonDetail(detail string) {
	daemonState.mu.Lock()
	defer daemonState.mu.Unlock()
	daemonState.Detail = detail
}

func setDaemonIP(ip string) {
	daemonState.mu.Lock()
	defer daemonState.mu.Unlock()
	daemonState.IP = ip
}

func setDaemonSSID(ssid string) {
	daemonState.mu.Lock()
	defer daemonState.mu.Unlock()
	daemonState.SSID = ssid
}

func setDaemonGateway(gw string) {
	daemonState.mu.Lock()
	defer daemonState.mu.Unlock()
	daemonState.Gateway = gw
}

func setLastLogin(t string) {
	daemonState.mu.Lock()
	defer daemonState.mu.Unlock()
	daemonState.LastLogin = t
}

// writePID writes the current PID to the PID file
func writePID() error {
	dir := strings.TrimSuffix(PIDFile, "/daemon.pid")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(PIDFile, []byte(fmt.Sprintf("%d", os.Getpid())), 0644)
}

// killOldDaemon kills any existing daemon process
func killOldDaemon() {
	data, err := os.ReadFile(PIDFile)
	if err != nil {
		return
	}
	pid := strings.TrimSpace(string(data))
	if pid == "" {
		return
	}
	LogInfo(fmt.Sprintf("发现旧进程 PID: %s，正在终止...", pid))
	cmd := exec.Command("kill", pid)
	cmd.Run()
	time.Sleep(500 * time.Millisecond)
}

// isWiFiConnected checks if WiFi is connected
func isWiFiConnected() bool {
	// Method 1: Check /sys/class/net/wlan0/operstate
	data, err := os.ReadFile("/sys/class/net/wlan0/operstate")
	if err == nil {
		state := strings.TrimSpace(string(data))
		if state == "up" {
			return true
		}
	}

	// Method 2: Use cmd wifi status
	cmd := exec.Command("cmd", "wifi", "status")
	output, err := cmd.Output()
	if err == nil {
		s := string(output)
		if strings.Contains(s, "Wi-Fi is enabled") || strings.Contains(s, "connected") {
			return true
		}
	}

	return false
}

// getCurrentSSID returns the current WiFi SSID
func getCurrentSSID() string {
	// Method: dumpsys wifi | grep mWifiInfo
	cmd := exec.Command("sh", "-c", "dumpsys wifi | grep mWifiInfo")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	s := string(output)
	// Parse SSID from line like: mWifiInfo: SSID: "imust", BSSID: ...
	if strings.Contains(s, "SSID:") {
		start := strings.Index(s, "SSID:")
		if start >= 0 {
			rest := s[start+5:]
			rest = strings.TrimSpace(rest)
			// Remove quotes
			rest = strings.TrimPrefix(rest, "\"")
			if idx := strings.Index(rest, "\""); idx >= 0 {
				return rest[:idx]
			}
			// Try without quotes
			if idx := strings.IndexAny(rest, ", \t\n"); idx >= 0 {
				return rest[:idx]
			}
			return rest
		}
	}

	return ""
}

// matchSSID checks if current SSID matches any in the config list
func matchSSID(current string, ssids string) bool {
	if current == "" {
		return false
	}
	for _, s := range strings.Split(ssids, ",") {
		s = strings.TrimSpace(s)
		if s != "" && strings.EqualFold(current, s) {
			return true
		}
	}
	return false
}

// StartDaemon starts the WiFi monitoring daemon
func StartDaemon(cfg *Config) {
	if daemonRunning.Load() {
		LogInfo("守护进程已在运行")
		return
	}

	killOldDaemon()
	if err := writePID(); err != nil {
		LogError(fmt.Sprintf("写入PID文件失败: %v", err))
	}

	daemonRunning.Store(true)
	stopDaemon = make(chan struct{})

	daemonState.mu.Lock()
	daemonState.DaemonRunning = true
	daemonState.PID = os.Getpid()
	daemonState.mu.Unlock()

	LogInfo(fmt.Sprintf("守护进程已启动 (PID: %d)", os.Getpid()))

	go func() {
		defer func() {
			daemonRunning.Store(false)
			daemonState.mu.Lock()
			daemonState.DaemonRunning = false
			daemonState.mu.Unlock()
			os.Remove(PIDFile)
			LogInfo("守护进程已停止")
		}()

		// Main loop with panic recovery
		for {
			func() {
				defer func() {
					if r := recover(); r != nil {
						LogError(fmt.Sprintf("守护进程 panic: %v，正在恢复...", r))
						time.Sleep(5 * time.Second)
					}
				}()

				daemonLoop(cfg)
			}()

			// Check if we should stop
			select {
			case <-stopDaemon:
				return
			default:
			}
		}
	}()
}

// StopDaemon stops the WiFi monitoring daemon
func StopDaemon() {
	if stopDaemon != nil {
		close(stopDaemon)
	}
	daemonRunning.Store(false)
}

// daemonLoop is one iteration of the daemon's main loop
func daemonLoop(cfg *Config) {
	c := cfg.Clone()

	// Check WiFi connection
	if !isWiFiConnected() {
		setLoginState("idle")
		setDaemonDetail("WiFi未连接")
		setDaemonSSID("")
		select {
		case <-stopDaemon:
			return
		case <-time.After(time.Duration(c.CHECK_INTERVAL) * time.Second):
			return
		}
	}

	// Get current SSID
	ssid := getCurrentSSID()
	setDaemonSSID(ssid)

	// Check if SSID matches
	if !matchSSID(ssid, c.SSIDS) {
		setLoginState("idle")
		setDaemonDetail(fmt.Sprintf("SSID '%s' 不在目标列表中", ssid))
		select {
		case <-stopDaemon:
			return
		case <-time.After(time.Duration(c.CHECK_INTERVAL) * time.Second):
			return
		}
	}

	// SSID matches, attempt login
	LogInfo(fmt.Sprintf("检测到目标WiFi: %s，开始登录...", ssid))
	setDaemonGateway(c.GATEWAY)
	setDaemonDetail("正在登录...")
	setLoginState("logging_in")

	retryCount := 0
	for retryCount < c.MAX_RETRY {
		select {
		case <-stopDaemon:
			return
		default:
		}

		result := doLogin(c)
		setDaemonIP(result.IP)

		if result.Result == 1 {
			setLoginState("online")
			setDaemonDetail(result.Message)
			setLastLogin(time.Now().Format("2006-01-02 15:04:05"))
			LogInfo(fmt.Sprintf("登录成功，等待 %d 秒后再次检查", c.SUCCESS_INTERVAL))

			select {
			case <-stopDaemon:
				return
			case <-time.After(time.Duration(c.SUCCESS_INTERVAL) * time.Second):
				return
			}
		}

		retryCount++
		if retryCount >= c.MAX_RETRY {
			setLoginState("error")
			setDaemonDetail(fmt.Sprintf("登录失败(已重试%d次): %s", retryCount, result.Error))
			LogError(fmt.Sprintf("登录失败，已重试 %d 次: %s", retryCount, result.Error))

			select {
			case <-stopDaemon:
				return
			case <-time.After(time.Duration(c.CHECK_INTERVAL) * time.Second):
				return
			}
		}

		LogWarn(fmt.Sprintf("登录失败，%d秒后重试(%d/%d): %s",
			c.CHECK_INTERVAL, retryCount, c.MAX_RETRY, result.Error))
		setLoginState("reconnecting")
		setDaemonDetail(fmt.Sprintf("重试中(%d/%d)...", retryCount, c.MAX_RETRY))

		select {
		case <-stopDaemon:
			return
		case <-time.After(time.Duration(c.CHECK_INTERVAL) * time.Second):
		}
	}
}

// TestLogin performs a single login attempt and returns the result
func TestLogin(cfg *Config) LoginResult {
	c := cfg.Clone()
	setLoginState("logging_in")
	setDaemonDetail("手动测试登录中...")
	result := doLogin(c)
	if result.Result == 1 {
		setLoginState("online")
		setDaemonIP(result.IP)
		setDaemonGateway(c.GATEWAY)
		setLastLogin(time.Now().Format("2006-01-02 15:04:05"))
	} else {
		setLoginState("error")
		setDaemonDetail(result.Error)
	}
	return result
}
