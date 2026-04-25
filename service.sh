#!/system/bin/sh
# SRun Auto Login - IMUST
# Magisk 服务脚本 (开机启动)

MODDIR="${0%/*}"

# 等待系统启动完成
while [ "$(getprop sys.boot_completed)" != "1" ]; do
    sleep 1
done

sleep 5

# 启动主程序 (内置守护进程 + WebUI 服务器)
nohup "$MODDIR/srun-login" >> /data/adb/srun/srun.log 2>&1 &

echo "service_started" > /data/adb/srun/module_status

# 同时清理旧的 shell 守护进程 (如果存在)
OLD_PID=$(cat /data/adb/srun/daemon.pid 2>/dev/null)
if [ -n "$OLD_PID" ]; then
    # 检查是不是旧的 shell 进程
    if ! grep -q "srun-login" /proc/$OLD_PID/cmdline 2>/dev/null; then
        kill "$OLD_PID" 2>/dev/null
        rm -f /data/adb/srun/daemon.pid
    fi
fi
