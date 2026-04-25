package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	// Initialize logger
	if err := InitLogger(); err != nil {
		fmt.Fprintf(os.Stderr, "初始化日志失败: %v\n", err)
		os.Exit(1)
	}

	LogInfo("========================================")
	LogInfo("SRun Auto Login 启动中...")
	LogInfo(fmt.Sprintf("PID: %d", os.Getpid()))

	// Load config
	cfg := NewConfig()
	if err := cfg.Load(); err != nil {
		LogWarn(fmt.Sprintf("加载配置失败，使用默认配置: %v", err))
	} else {
		LogInfo("配置加载成功")
		c := cfg.Clone()
		LogInfo(fmt.Sprintf("  网关: %s", c.GATEWAY))
		LogInfo(fmt.Sprintf("  用户: %s", c.USERNAME))
		LogInfo(fmt.Sprintf("  SSID: %s", c.SSIDS))
	}

	// Start HTTP server
	StartServer(cfg)

	// Start daemon
	StartDaemon(cfg)

	LogInfo("所有服务已启动")
	LogInfo("========================================")

	// Wait for signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	sig := <-sigCh
	LogInfo(fmt.Sprintf("收到信号 %v，正在关闭...", sig))

	StopDaemon()
	LogInfo("SRun Auto Login 已退出")
}
