# SRun Auto Login v2.0.3

IMUST 校园网深澜 SRUN 自动登录 Magisk 模块。

## 功能

- ✅ 自动检测 WiFi (imust/THUNDER)
- ✅ SRUN 协议自动登录 (HMAC-MD5 + XXTEA + 自定义 Base64 + SHA1)
- ✅ WebUI 管理界面 (内置 HTTP 服务器 port 20080)
- ✅ 通知栏状态显示
- ✅ 掉线自动重连
- ✅ Panic 恢复，永不崩溃
- ✅ 在线修改配置
- ✅ 运行日志查看
- ✅ 手动测试登录

## 安装

1. 下载 `SRunAutoLogin-v2.0.3.zip`
2. 通过 Magisk / KernelSU 刷入
3. 重启手机
4. WebUI 配置账号密码：打开 Suki Ultra 中的 `<>` 按钮，或浏览器访问 `http://localhost:20080`

## 项目结构

```
├── srun-login              # Go 编译 ARM64 二进制
├── service.sh              # Magisk 开机启动脚本
├── customize.sh            # 安装脚本
├── module.prop
├── config/config.conf      # 配置文件
├── webroot/index.html      # WebUI (深色主题)
├── script/
│   ├── status.sh           # 状态查询
│   └── uninstall.sh        # 卸载清理
└── src/                    # Go 源码
    ├── main.go             # 入口
    ├── srun.go             # SRUN 协议 + DNS 解析
    ├── daemon.go           # WiFi 检测守护进程
    ├── server.go           # HTTP API 服务器
    ├── config.go           # 配置管理
    ├── logger.go           # 日志系统
    └── go.mod
```

## 编译

```bash
# 安装 Go
curl -sL -o /tmp/go.tar.gz "https://mirrors.aliyun.com/golang/go1.22.4.linux-amd64.tar.gz"
cd /tmp && tar xzf go.tar.gz
export PATH=/tmp/go/bin:$PATH

# 编译 ARM64 Android 二进制
cd src
GOOS=android GOARCH=arm64 CGO_ENABLED=0 go build -ldflags="-s -w" -o ../srun-login

# 打包 Magisk 模块
cd ..
python3 -c "
import zipfile, os
with zipfile.ZipFile('SRunAutoLogin.zip', 'w', zipfile.ZIP_DEFLATED) as zf:
    for root, dirs, files in os.walk('.'):
        if 'src' in root or '.git' in root: continue
        for f in files:
            path = os.path.join(root, f)
            zf.write(path, path[2:])
"
```

## API 接口

| 端点 | 方法 | 说明 |
|------|------|------|
| `/` | GET | WebUI 页面 |
| `/api/status` | GET | 状态 JSON |
| `/api/config` | POST | 保存配置 |
| `/api/logs` | GET | 运行日志 |
| `/api/logs/clear` | POST | 清空日志 |
| `/api/test-login` | POST | 手动测试登录 |

## SRUN 协议

登录流程：

1. `GET /cgi-bin/rad_user_info` → 获取客户端 IP
2. `GET /cgi-bin/get_challenge` → 获取 Token
3. 加密：HMAC-MD5(password, token) + XXTEA(info, token) + 自定义 Base64 + SHA1
4. `GET /cgi-bin/srun_portal?action=login` → 登录

参数：ac_id=6, n=200, type=1, enc=srun_bx1

自定义 Base64 ALPHA: `LVoJPiCN2R8G90yg+hmFHuacZ1OWMnrsSTXkYpUq/3dlbfKwv6xztjI7DeBE45QA`

## 开发记录

### v2.0.3 (2026-04-25)
- 客户端 IP 改为从网卡获取 (getLocalIP/getLocalIPFromWlan)
- 不再依赖 rad_user_info 响应中的 client_ip
- 不再 fallback 到网关 IP

### v2.0.2 (2026-04-25)
- DNS 解析改为 shell 命令优先 (getent/nslookup/ping)
- DoH 降为兜底 (公共 DNS 无法解析校园域名)
- GATEWAY_IP 改为 198.18.0.85

### v2.0.1 (2026-04-25)
- 修复 Android DNS 权限问题
- doSRUNGet 改为 IP 直连 + Host 头

### v2.0.0 (2026-04-25)
- 从零重写 Go 源码
- 对标 Python 版 SRunPy-GUI-IMUST 的 SRUN 协议
- WebUI 全部重写
- 毛玻璃视觉设计

## 踩坑记录

| 问题 | 原因 | 解决 |
|------|------|------|
| DNS 解析 `operation not permitted` | Go 的 DNS 解析器被 Android SELinux 拦截 | 用 shell 命令 (getent/nslookup) 解析 |
| 校园域名公共 DNS 解析不了 | `gw.imust.edu.cn` 是内网域名 | shell 命令优先，DoH 兜底 |
| `0x9E3779B9` 编译溢出 | 超出 int32 范围 | 用 `uint32(0x9E3779B9)` |
| WebUI `ksu.exec()` 不工作 | Suki Ultra 不注入 KernelSU API | 改用内置 HTTP 服务器 |
| challenge 返回 HTML 重定向 | 客户端 IP 错误 (用了网关 IP) | 从网卡获取真实 IP |
| `p` 变量作用域问题 | for 循环内定义，外层引用不到 | 提前声明 `var p int` |
| 类型不匹配 XOR | `int ^ uint32` | 转换为 `uint32(p&3)^e` |

## 参考

- 协议来源: [SRunPy-GUI-IMUST](https://github.com/louisgoodabxd/SRunPy-GUI-IMUST)
- Magisk 模块参考: [jzzz](https://github.com/louisgoodabxd/jzzz)
- 原始协议: [iskoldt-X/SRUN-authenticator](https://github.com/iskoldt-X/SRUN-authenticator)
