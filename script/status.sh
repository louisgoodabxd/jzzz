#!/system/bin/sh
# SRun Auto Login - 模块状态查询

STATUS_FILE="/data/adb/srun/status"
PID_FILE="/data/adb/srun/daemon.pid"
WEB_PID_FILE="/data/adb/srun/web.pid"
CONFIG="/data/adb/srun/config.conf"

check_daemon() {
    if [ -f "$PID_FILE" ]; then
        local pid=$(cat "$PID_FILE" 2>/dev/null)
        [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null && return 0
    fi
    return 1
}

check_web() {
    if [ -f "$WEB_PID_FILE" ]; then
        local pid=$(cat "$WEB_PID_FILE" 2>/dev/null)
        [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null && return 0
    fi
    return 1
}

echo "╔══════════════════════════════════════╗"
echo "║  SRun Auto Login - IMUST             ║"
echo "║  校园网深澜自动登录                   ║"
echo "╠══════════════════════════════════════╣"

if check_daemon; then
    echo "║  守护进程: 运行中 ✓                  ║"
else
    echo "║  守护进程: 未运行 ✗                  ║"
fi

if check_web; then
    echo "║  WebUI:    运行中 ✓                  ║"
    echo "║  地址: http://localhost:20080        ║"
else
    echo "║  WebUI:    未运行 ✗                  ║"
fi

if [ -f "$STATUS_FILE" ]; then
    IFS='|' read -r _ login_state detail < "$STATUS_FILE"
    case "$login_state" in
        online)     echo "║  状态: 已联网 ✓                      ║" ;;
        logging_in) echo "║  状态: 登录中...                     ║" ;;
        connecting) echo "║  状态: 连接中...                     ║" ;;
        error)      echo "║  状态: 错误 ✗                        ║" ;;
        *)          echo "║  状态: 空闲                          ║" ;;
    esac
    [ -n "$detail" ] && echo "║  $detail"
fi

if [ -f "$CONFIG" ]; then
    . "$CONFIG"
    echo "╠══════════════════════════════════════╣"
    echo "║  账号: ${USERNAME:-未配置}"
    echo "║  网关: ${GATEWAY:-gw.imust.edu.cn}"
    echo "║  监听: ${SSIDS:-imust,THUNDER}"
fi

echo "╚══════════════════════════════════════╝"
