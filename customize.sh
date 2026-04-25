#!/system/bin/sh
# SRun Auto Login - IMUST
# Magisk Module Installation Script

SKIPUNZIP=1

ui_print "╔══════════════════════════════════════╗"
ui_print "║  SRun Auto Login - IMUST             ║"
ui_print "║  校园网深澜自动登录模块                ║"
ui_print "╚══════════════════════════════════════╝"
ui_print ""

# Extract module files
ui_print "- 解压模块文件..."
unzip -o "$ZIPFILE" -x 'META-INF/*' -d "$MODPATH" >&2

# Set permissions
ui_print "- 设置权限..."
set_perm_recursive "$MODPATH" 0 0 0755 0644
set_perm "$MODPATH/service.sh" 0 0 0755
set_perm "$MODPATH/srun-login" 0 0 0755
set_perm "$MODPATH/script/status.sh" 0 0 0755
set_perm "$MODPATH/script/uninstall.sh" 0 0 0755

# 部署配置文件
CONFIG_DIR="/data/adb/srun"
mkdir -p "$CONFIG_DIR"
if [ ! -f "$CONFIG_DIR/config.conf" ]; then
    ui_print "- 部置配置文件..."
    cp "$MODPATH/config/config.conf" "$CONFIG_DIR/config.conf"
    ui_print "  ✓ 配置已部署"
else
    ui_print "- 检测到已有配置文件，跳过"
fi

ui_print ""
ui_print "╔══════════════════════════════════════╗"
ui_print "║  安装完成！                           ║"
ui_print "║                                      ║"
ui_print "║  ✓ 账号已配置                        ║"
ui_print "║  📱 重启手机即可生效                  ║"
ui_print "║                                      ║"
ui_print "║  🌐 WebUI: Suki Ultra 中点 <> 按钮   ║"
ui_print "╚══════════════════════════════════════╝"
