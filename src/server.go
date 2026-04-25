package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

const (
	ListenAddr = "0.0.0.0:20080"
)

// StatusResponse matches the required JSON format exactly
type StatusResponse struct {
	LoginState       string `json:"login_state"`
	DaemonRunning    bool   `json:"daemon_running"`
	PID              int    `json:"pid"`
	IP               string `json:"ip"`
	SSID             string `json:"ssid"`
	Gateway          string `json:"gateway"`
	Detail           string `json:"detail"`
	LastLogin        string `json:"last_login"`
	USERNAME         string `json:"USERNAME"`
	PASSWORD         string `json:"PASSWORD"`
	GATEWAY          string `json:"GATEWAY"`
	GATEWAY_IP       string `json:"GATEWAY_IP"`
	AC_ID            string `json:"AC_ID"`
	SSIDS            string `json:"SSIDS"`
	MAX_RETRY        int    `json:"MAX_RETRY"`
	CHECK_INTERVAL   int    `json:"CHECK_INTERVAL"`
	SUCCESS_INTERVAL int    `json:"SUCCESS_INTERVAL"`
}

// LogsResponse is the response for /api/logs
type LogsResponse struct {
	Logs []string `json:"logs"`
}

// TestLoginResponse is the response for /api/test-login
type TestLoginResponse struct {
	Result  int    `json:"result"`
	IP      string `json:"ip"`
	Token   string `json:"token"`
	Gateway string `json:"gateway"`
	Message string `json:"message"`
	Error   string `json:"error"`
}

func setCORSHeaders(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
}

func jsonResponse(w http.ResponseWriter, data interface{}) {
	setCORSHeaders(w)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(data)
}

func handleRoot(w http.ResponseWriter, r *http.Request, cfg *Config) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	// Try to serve webroot/index.html
	data, err := os.ReadFile("/data/adb/srun/webroot/index.html")
	if err != nil {
		// Fallback: try relative path
		data, err = os.ReadFile("webroot/index.html")
		if err != nil {
			setCORSHeaders(w)
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprint(w, `<!DOCTYPE html>
<html><head><meta charset="utf-8"><title>SRun Auto Login</title></head>
<body>
<h1>SRun Auto Login</h1>
<p>模块正在运行。</p>
<p><a href="/api/status">查看状态</a> | <a href="/api/logs">查看日志</a></p>
</body></html>`)
			return
		}
	}
	setCORSHeaders(w)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(data)
}

func handleStatus(w http.ResponseWriter, r *http.Request, cfg *Config) {
	state := GetDaemonState()
	c := cfg.Clone()

	resp := StatusResponse{
		LoginState:       state.LoginState,
		DaemonRunning:    state.DaemonRunning,
		PID:              state.PID,
		IP:               state.IP,
		SSID:             state.SSID,
		Gateway:          state.Gateway,
		Detail:           state.Detail,
		LastLogin:        state.LastLogin,
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

	jsonResponse(w, resp)
}

func handleConfig(w http.ResponseWriter, r *http.Request, cfg *Config) {
	if r.Method == http.MethodOptions {
		setCORSHeaders(w)
		w.WriteHeader(http.StatusOK)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "读取请求失败", http.StatusBadRequest)
		return
	}

	var newCfg Config
	if err := json.Unmarshal(body, &newCfg); err != nil {
		http.Error(w, "JSON解析失败", http.StatusBadRequest)
		return
	}

	cfg.UpdateFromJSON(&newCfg)
	if err := cfg.Save(); err != nil {
		setCORSHeaders(w)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": fmt.Sprintf("保存配置失败: %v", err)})
		return
	}

	LogInfo("配置已更新")
	jsonResponse(w, map[string]string{"status": "ok", "message": "配置已保存"})
}

func handleLogs(w http.ResponseWriter, r *http.Request) {
	lines := GetLogLines()
	if lines == nil {
		lines = []string{}
	}
	jsonResponse(w, LogsResponse{Logs: lines})
}

func handleLogsClear(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		setCORSHeaders(w)
		w.WriteHeader(http.StatusOK)
		return
	}
	ClearLogs()
	LogInfo("日志已清空")
	jsonResponse(w, map[string]string{"status": "ok", "message": "日志已清空"})
}

func handleTestLogin(w http.ResponseWriter, r *http.Request, cfg *Config) {
	if r.Method == http.MethodOptions {
		setCORSHeaders(w)
		w.WriteHeader(http.StatusOK)
		return
	}

	result := TestLogin(cfg)

	resp := TestLoginResponse{
		Result:  result.Result,
		IP:      result.IP,
		Token:   result.Token,
		Gateway: result.Gateway,
		Message: result.Message,
		Error:   result.Error,
	}

	jsonResponse(w, resp)
}

var (
	server *http.Server
)

// StartServer starts the HTTP API server
func StartServer(cfg *Config) {
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		handleRoot(w, r, cfg)
	})

	mux.HandleFunc("/api/status", func(w http.ResponseWriter, r *http.Request) {
		setCORSHeaders(w)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		handleStatus(w, r, cfg)
	})

	mux.HandleFunc("/api/config", func(w http.ResponseWriter, r *http.Request) {
		handleConfig(w, r, cfg)
	})

	mux.HandleFunc("/api/logs", func(w http.ResponseWriter, r *http.Request) {
		setCORSHeaders(w)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		handleLogs(w, r)
	})

	mux.HandleFunc("/api/logs/clear", func(w http.ResponseWriter, r *http.Request) {
		handleLogsClear(w, r)
	})

	mux.HandleFunc("/api/test-login", func(w http.ResponseWriter, r *http.Request) {
		handleTestLogin(w, r, cfg)
	})

	server = &http.Server{
		Addr:         ListenAddr,
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	LogInfo(fmt.Sprintf("HTTP服务器启动在 %s", ListenAddr))

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			LogError(fmt.Sprintf("HTTP服务器错误: %v", err))
		}
	}()
}
