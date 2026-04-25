# SRun Auto Login 开发全记录

## 项目背景

用户是 IMUST（内蒙古科技大学）学生，需要一个 Magisk 模块在手机连接校园网 WiFi 时自动登录深澜 SRUN 认证系统。

参考项目：
- Python 桌面端: https://github.com/louisgoodabxd/SRunPy-GUI-IMUST
- Magisk 模块骨架: https://github.com/louisgoodabxd/jzzz

## 时间线 (2026-04-25)

### 15:42 - 16:36 初版开发 (v1.0.0)
- 尝试纯 Shell 实现 → awk 做 XXTEA 各种溢出问题
- 尝试 WebUI → KernelSU API 在 Suki Ultra 上不工作
- 转向 Go 编译 → 单一二进制搞定一切
- 修了 panic 恢复、旧脚本引用等问题
- **源码丢失**，只有编译好的二进制

### 16:40 - 16:46 重写开始 (v2.0.0)
- 从 GitHub 拉取 jzzz 仓库，分析现有二进制 (strings)
- 分析 Python 版 SRunPy-GUI-IMUST 源码，理解完整 SRUN 协议
- 用子代理从零编写 Go 源码 (7 个文件)
- 修复编译错误: XXTEA 常量溢出、变量作用域、类型不匹配
- WebUI 全部重写，修复 HTML 结构损坏、按钮无功能等问题

### 16:50 - 16:58 WebUI 美化 + 源码保护
- 毛玻璃视觉设计 (backdrop-filter)
- 推送到 GitHub 保护源码
- **安全提醒**: 用户在聊天中暴露了 PAT，建议撤销

### 17:01 - 17:06 DNS 权限修复
- 错误: `dial tcp: lookup gw.imust.edu.cn on [::1]:53: operation not permitted`
- 原因: Go 的 DNS 解析器被 Android SELinux 拦截
- 方案 1: IP 直连 + Host 头 (跳过 DNS)
- 方案 2: shell 命令解析 DNS (getent/nslookup/ping)
- 方案 3: DNS over HTTPS (Google/Cloudflare)
- **关键教训**: 校园域名是内网域名，公共 DNS 解析不了

### 17:17 - 17:25 DNS 解析策略定型
- shell 命令优先 (getent → nslookup → ping)
- DoH 降为兜底 (只能解析外网域名)
- shell 命令用系统 libc 解析器，有正确 SELinux 上下文
- GATEWAY_IP 改为 198.18.0.85 (用户要求)

### 17:25 - 17:32 客户端 IP 修复
- 错误: challenge 请求返回 HTML 重定向
- 原因: 客户端 IP 用了网关 IP (198.18.0.85)，网关不认
- 解决: 从网卡获取本机 IP (getLocalIP / getLocalIPFromWlan)
- 与 Python 版 ip_utils.py 的 UDP socket 方式一致

## 技术要点

### SRUN 协议加密流程
```
info = {"username":"...","password":"...","ip":"...","acid":"6","enc_ver":"srun_bx1"}
i    = "{SRBX1}" + custom_base64(xencode(info, token))
hmd5 = hmac_md5(password, token)
chksum = sha1(token + username + token + hmd5 + token + ac_id + token + ip + token + "200" + token + "1" + token + i)
```

### XXTEA 常量
- `0x9E3779B9` 超出 int32 范围
- Go 中用 `uint32(0x9E3779B9)`
- 所有位运算用 uint32，避免类型不匹配

### Android 网络权限
- Go 的纯 Go DNS 解析器被 SELinux 拦截
- shell 命令 (getent/nslookup) 用 libc 解析器，有正确 SELinux 上下文
- 校园网关域名是内网域名，只能在校园网内解析
- 用 IP 直连 + Host 头可以绕过 DNS

### WebUI 架构
- Go 二进制内置 HTTP 服务器 (port 20080)
- 前端直接 fetch `/api/*`，不依赖任何外部 API
- Suki Ultra 的 `<>` 按钮加载 `webroot/index.html`
- 不需要 KernelSU WebUI API (Suki Ultra 不支持)

## 文件清单

```
SRunAutoLogin/
├── srun-login              # 编译产物 (不提交到 git)
├── service.sh              # Magisk 开机启动
├── customize.sh            # 安装脚本
├── module.prop             # 模块信息
├── config/config.conf      # 预置配置
├── webroot/index.html      # WebUI
├── script/
│   ├── status.sh
│   └── uninstall.sh
├── src/                    # Go 源码
│   ├── main.go
│   ├── srun.go             # 最复杂的文件
│   ├── daemon.go
│   ├── server.go
│   ├── config.go
│   ├── logger.go
│   └── go.mod
├── README.md               # 项目说明
└── DEVLOG.md               # 本文档
```

## 编译命令

```bash
export PATH=/tmp/go/bin:$PATH
cd src
GOOS=android GOARCH=arm64 CGO_ENABLED=0 go build -ldflags="-s -w" -o ../srun-login
```

## 待改进

- [ ] 添加注销 (logout) 功能
- [ ] 支持多账号/多网关
- [ ] 添加 ac_id 自动检测 (update_acid)
- [ ] 日志文件自动轮转 (按日期)
- [ ] WebUI 密码保护 (防止他人修改配置)
- [ ] 通知栏实时状态 (Android Notification)
