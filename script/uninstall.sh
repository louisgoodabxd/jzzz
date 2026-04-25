#!/system/bin/sh
# SRun Auto Login - 卸载脚本

# 停止主程序
PID_FILE="/data/adb/srun/daemon.pid"
if [ -f "$PID_FILE" ]; then
    pid=$(cat "$PID_FILE" 2>/dev/null)
    [ -n "$pid" ] && kill "$pid" 2>/dev/null
    rm -f "$PID_FILE"
fi

# 也杀掉可能残留的进程
pkill -f "srun-login" 2>/dev/null

# 清除通知
cmd notification cancel "srun_auto" 2>/dev/null

# 清理状态
rm -f /data/adb/srun/module_status
rm -f /data/adb/srun/status

# 注意: 保留配置文件
# 如需完全清理: rm -rf /data/adb/srun/
