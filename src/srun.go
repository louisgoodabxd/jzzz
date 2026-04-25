package main

import (
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha1"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os/exec"
	"strings"
	"time"
)

// Custom Base64 alphabet used by SRUN
const srunBase64Alpha = "LVoJPiCN2R8G90yg+hmFHuacZ1OWMnrsSTXkYpUq/3dlbfKwv6xztjI7DeBE45QA"

var srunBase64Enc *base64.Encoding

func init() {
	srunBase64Enc = base64.NewEncoding(srunBase64Alpha).WithPadding(base64.StdPadding)
}

// getMD5 returns HMAC-MD5(password, token)
func getMD5(password, token string) string {
	h := hmac.New(md5.New, []byte(token))
	h.Write([]byte(password))
	return hex.EncodeToString(h.Sum(nil))
}

// getSHA1 returns SHA1(value)
func getSHA1(value string) string {
	h := sha1.New()
	h.Write([]byte(value))
	return hex.EncodeToString(h.Sum(nil))
}

// ordat returns the byte value at index idx, or 0 if out of range
func ordat(msg []byte, idx int) uint32 {
	if idx < len(msg) {
		return uint32(msg[idx])
	}
	return 0
}

// sencode converts a byte slice to uint32 slice (little-endian packing)
// If key is true, appends the length
func sencode(msg []byte, key bool) []uint32 {
	l := len(msg)
	var pwd []uint32
	for i := 0; i < l; i += 4 {
		pwd = append(pwd,
			ordat(msg, i)|
				ordat(msg, i+1)<<8|
				ordat(msg, i+2)<<16|
				ordat(msg, i+3)<<24)
	}
	if key {
		pwd = append(pwd, uint32(l))
	}
	return pwd
}

// lencode converts a uint32 slice back to bytes (little-endian unpacking)
// If key is true, uses msg[last] as the output length
func lencode(msg []uint32, key bool) []byte {
	l := len(msg)
	ll := (l - 1) << 2
	if key {
		m := int(msg[l-1])
		if m < ll-3 || m > ll {
			return nil
		}
		ll = m
	}
	result := make([]byte, 0, l*4)
	for i := 0; i < l; i++ {
		v := msg[i]
		result = append(result,
			byte(v&0xff),
			byte((v>>8)&0xff),
			byte((v>>16)&0xff),
			byte((v>>24)&0xff),
		)
	}
	if key {
		return result[:ll]
	}
	return result
}

// get_xencode implements the XXTEA-like encryption
func get_xencode(msg []byte, key []byte) []byte {
	if len(msg) == 0 {
		return nil
	}

	pwd := sencode(msg, true)
	pwdk := sencode(key, false)

	// Pad key to at least 4 elements
	for len(pwdk) < 4 {
		pwdk = append(pwdk, 0)
	}

	n := len(pwd) - 1
	if n < 1 {
		// Need at least 2 elements for XXTEA
		return lencode(pwd, false)
	}

	c := uint32(0x9E3779B9) // delta constant
	var d uint32 = 0
	z := pwd[n]
	y := pwd[0]
	var p int
	q := 6 + 52/(len(pwd))

	for q > 0 {
		d = (d + c) & 0xFFFFFFFF
		e := (d >> 2) & 3
		p = 0
		y = pwd[0]
		for p < n {
			y = pwd[p+1]
			m := (z>>5 ^ y<<2) + ((y>>3 ^ z<<4) ^ (d ^ y)) + (pwdk[(p&3)^int(e)] ^ z)
			pwd[p] = (pwd[p] + m) & 0xFFFFFFFF
			z = pwd[p]
			p++
		}
		y = pwd[0]
		m := (z>>5 ^ y<<2) + ((y>>3 ^ z<<4) ^ (d ^ y)) + (pwdk[(p&3)^int(e)] ^ z)
		pwd[n] = (pwd[n] + m) & 0xFFFFFFFF
		z = pwd[n]
		q--
	}

	return lencode(pwd, false)
}

// srunBase64Encode encodes bytes using the SRUN custom base64 alphabet
func srunBase64Encode(data []byte) string {
	return srunBase64Enc.EncodeToString(data)
}

// jsonResponse parses a JSONP response body
// Input: jQuery1234({"key":"value"})
// Output: map with the JSON data
func parseJSONP(body string) (map[string]interface{}, error) {
	// Find the JSON part between first ( and last )
	start := strings.Index(body, "(")
	end := strings.LastIndex(body, ")")
	if start < 0 || end < 0 || start >= end {
		return nil, fmt.Errorf("invalid JSONP response: %s", body)
	}
	jsonStr := body[start+1 : end]

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil, fmt.Errorf("JSON parse error: %v, body: %s", err, jsonStr)
	}
	return result, nil
}

// generateCallback generates a jQuery callback string
func generateCallback() string {
	return fmt.Sprintf("jQuery%d", time.Now().UnixMilli())
}

// resolveHost 通过多种方式解析域名，绕过 Android 上 Go 的 DNS 权限限制
// Shell 命令用系统 libc 解析器（有正确的 SELinux 上下文），可以解析校园内网域名
// DoH 用公共 DNS，无法解析校园域名，仅作兜底
func resolveHost(domain, fallbackIP string) string {
	// 方法1: getent hosts (Android 系统解析器，SELinux 上下文正确)
	if out, err := exec.Command("getent", "hosts", domain).Output(); err == nil {
		parts := strings.Fields(string(out))
		if len(parts) > 0 && strings.Contains(parts[0], ".") {
			LogInfo(fmt.Sprintf("DNS解析(getent): %s → %s", domain, parts[0]))
			return parts[0]
		}
	}

	// 方法2: nslookup
	if out, err := exec.Command("nslookup", domain).Output(); err == nil {
		lines := strings.Split(string(out), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "Address:") || strings.HasPrefix(line, "地址:") {
				ip := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(line, "Address:"), "地址:"))
				if strings.Contains(ip, "#") {
					continue
				}
				if strings.Contains(ip, ".") && !strings.HasPrefix(ip, "127.") {
					LogInfo(fmt.Sprintf("DNS解析(nslookup): %s → %s", domain, ip))
					return ip
				}
			}
		}
	}

	// 方法3: ping -c1 解析
	if out, err := exec.Command("ping", "-c1", "-W2", domain).Output(); err == nil {
		s := string(out)
		if idx := strings.Index(s, "("); idx >= 0 {
			end := strings.Index(s[idx:], ")")
			if end > 1 {
				ip := s[idx+1 : idx+end]
				if strings.Contains(ip, ".") {
					LogInfo(fmt.Sprintf("DNS解析(ping): %s → %s", domain, ip))
					return ip
				}
			}
		}
	}

	// 方法4: DoH (公共 DNS，可能无法解析校园内网域名，但外网域名可以)
	if ip := resolveDoH(domain); ip != "" {
		return ip
	}

	// 兜底: 使用配置的 IP
	LogWarn(fmt.Sprintf("DNS解析失败，使用配置IP: %s", fallbackIP))
	return fallbackIP
}

// resolveDoH 通过 DNS over HTTPS 解析域名（通过 IP 直连 DoH 服务器，不依赖系统 DNS）
func resolveDoH(domain string) string {
	// DoH 服务器列表（用 IP 直连，Host 头设为域名）
	dohServers := []struct {
		ip   string
		host string
		path string
	}{
		{"8.8.8.8", "dns.google", fmt.Sprintf("/resolve?name=%s&type=A", domain)},
		{"1.1.1.1", "cloudflare-dns.com", fmt.Sprintf("/dns-query?name=%s&type=A", domain)},
	}

	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	for _, doh := range dohServers {
		u := fmt.Sprintf("https://%s%s", doh.ip, doh.path)
		req, err := http.NewRequest("GET", u, nil)
		if err != nil {
			continue
		}
		req.Header.Set("Host", doh.host)
		req.Header.Set("Accept", "application/dns-json")

		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			continue
		}

		// 解析 DoH JSON 响应
		var result struct {
			Answer []struct {
				Data string `json:"data"`
			} `json:"Answer"`
		}
		if err := json.Unmarshal(body, &result); err != nil {
			continue
		}

		for _, ans := range result.Answer {
			if strings.Contains(ans.Data, ".") && !strings.HasPrefix(ans.Data, "127.") {
				LogInfo(fmt.Sprintf("DNS解析(DoH %s): %s → %s", doh.host, domain, ans.Data))
				return ans.Data
			}
		}
	}

	return ""
}

// resolvedIP 缓存解析结果，避免每次都 shell 调用
var resolvedIPCache string

func getResolvedIP(gateway, gatewayIP string) string {
	// 如果配置的是 IP 地址，直接返回
	if gatewayIP != "" && strings.Contains(gatewayIP, ".") && !strings.Contains(gatewayIP, " ") {
		return gatewayIP
	}
	// 如果有缓存，直接用
	if resolvedIPCache != "" {
		return resolvedIPCache
	}
	// 解析域名
	resolvedIPCache = resolveHost(gateway, gatewayIP)
	return resolvedIPCache
}

// srunHTTPClient returns an HTTP client that skips TLS verification
func srunHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
}

// doSRUNGet performs a GET request with automatic domain→IP fallback.
// Android 上 Go 二进制没有 DNS 权限，所以直接用 IP 连接，Host 头设为域名。
func doSRUNGet(gateway, gatewayIP, path string) (string, error) {
	client := srunHTTPClient()
	ua := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"

	// 优先用 IP（跳过 DNS），Host 头设为域名
	if gatewayIP != "" {
		u := fmt.Sprintf("https://%s/%s", gatewayIP, path)
		req, err := http.NewRequest("GET", u, nil)
		if err == nil {
			req.Header.Set("Host", gateway)
			req.Header.Set("User-Agent", ua)
			resp, err := client.Do(req)
			if err == nil {
				defer resp.Body.Close()
				data, err := io.ReadAll(resp.Body)
				if err == nil {
					return string(data), nil
				}
			}
		}
		// IP HTTPS 失败，尝试 IP HTTP
		u = fmt.Sprintf("http://%s/%s", gatewayIP, path)
		req, err = http.NewRequest("GET", u, nil)
		if err == nil {
			req.Header.Set("Host", gateway)
			req.Header.Set("User-Agent", ua)
			resp, err := client.Do(req)
			if err == nil {
				defer resp.Body.Close()
				data, err := io.ReadAll(resp.Body)
				if err == nil {
					return string(data), nil
				}
			}
		}
	}

	// 最后尝试域名直连
	u := fmt.Sprintf("https://%s/%s", gateway, path)
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Host", gateway)
	req.Header.Set("User-Agent", ua)
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("所有连接方式均失败 (IP: %s, 域名: %s): %v", gatewayIP, gateway, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// LoginResult holds the result of a login attempt
type LoginResult struct {
	Result  int    `json:"result"`
	IP      string `json:"ip"`
	Token   string `json:"token"`
	Gateway string `json:"gateway"`
	Message string `json:"message"`
	Error   string `json:"error"`
}

// doLogin performs the full SRUN login flow
func doLogin(cfg Config) LoginResult {
	result := LoginResult{
		Gateway: cfg.GATEWAY,
	}

	gateway := cfg.GATEWAY
	gatewayIP := getResolvedIP(gateway, cfg.GATEWAY_IP)
	username := cfg.USERNAME
	password := cfg.PASSWORD
	acID := cfg.AC_ID

	if username == "" || password == "" {
		result.Error = "用户名或密码为空"
		result.Message = "请先配置用户名和密码"
		return result
	}

	callback := generateCallback()
	ts := fmt.Sprintf("%d", time.Now().UnixMilli())

	// Step 1: Check current status / get IP
	LogInfo(fmt.Sprintf("正在检查在线状态... (网关: %s, IP: %s)", gateway, gatewayIP))

	radURL := fmt.Sprintf("cgi-bin/rad_user_info?callback=%s&_=%s", callback, ts)
	body, err := doSRUNGet(gateway, gatewayIP, radURL)
	if err != nil {
		result.Error = fmt.Sprintf("获取在线信息失败: %v", err)
		result.Message = "连接网关失败"
		LogError(result.Error)
		return result
	}

	var ip string
	var alreadyOnline bool

	if strings.Contains(body, "not_online_error") {
		// Not online, need to get IP from response
		data, err := parseJSONP(body)
		if err == nil {
			if v, ok := data["client_ip"]; ok {
				ip = fmt.Sprintf("%v", v)
			} else if v, ok := data["online_ip"]; ok {
				ip = fmt.Sprintf("%v", v)
			}
		}
		if ip == "" {
			// Try to get IP from the gateway directly
			ip = cfg.GATEWAY_IP
		}
		LogInfo(fmt.Sprintf("未在线，IP: %s", ip))
	} else {
		// Already online
		data, err := parseJSONP(body)
		if err == nil {
			if v, ok := data["client_ip"]; ok {
				ip = fmt.Sprintf("%v", v)
			} else if v, ok := data["online_ip"]; ok {
				ip = fmt.Sprintf("%v", v)
			}
			if v, ok := data["error"]; ok && fmt.Sprintf("%v", v) == "ok" {
				alreadyOnline = true
			}
		}
		if ip == "" {
			ip = cfg.GATEWAY_IP
		}
		if alreadyOnline {
			result.Result = 1
			result.IP = ip
			result.Message = "已经在线"
			LogInfo("已经在线，无需重新登录")
			return result
		}
		LogInfo(fmt.Sprintf("需要登录，IP: %s", ip))
	}

	// Step 2: Get challenge token
	callback = generateCallback()
	ts = fmt.Sprintf("%d", time.Now().UnixMilli())

	challengeURL := fmt.Sprintf("cgi-bin/get_challenge?callback=%s&username=%s&ip=%s&_=%s",
		callback, url.QueryEscape(username), url.QueryEscape(ip), ts)

	LogInfo("正在获取 challenge token...")
	body, err = doSRUNGet(gateway, gatewayIP, challengeURL)
	if err != nil {
		result.Error = fmt.Sprintf("获取challenge失败: %v", err)
		result.Message = "获取challenge失败"
		LogError(result.Error)
		return result
	}

	data, err := parseJSONP(body)
	if err != nil {
		result.Error = fmt.Sprintf("解析challenge响应失败: %v", err)
		result.Message = "解析challenge失败"
		LogError(result.Error)
		return result
	}

	tokenIface, ok := data["challenge"]
	if !ok {
		result.Error = "响应中没有challenge字段"
		result.Message = "challenge字段缺失"
		LogError(result.Error)
		return result
	}
	token := fmt.Sprintf("%v", tokenIface)
	result.Token = token
	LogInfo(fmt.Sprintf("获取到token: %s", token[:min(16, len(token))]+"..."))

	// Step 3: Build encrypted info
	// info = {"username":"...","password":"...","ip":"...","acid":"6","enc_ver":"srun_bx1"}
	acid := acID
	infoJSON := fmt.Sprintf(`{"username":"%s","password":"%s","ip":"%s","acid":"%s","enc_ver":"srun_bx1"}`,
		username, password, ip, acid)

	// i = "{SRBX1}" + custom_base64(xencode(info, token))
	encrypted := get_xencode([]byte(infoJSON), []byte(token))
	iValue := "{SRBX1}" + srunBase64Encode(encrypted)

	// hmd5 = HMAC-MD5(password, token)
	hmd5 := getMD5(password, token)

	// chkstr = token + username + token + hmd5 + token + acid + token + ip + token + "200" + token + "1" + token + i
	chkstr := token + username + token + hmd5 + token + acid + token + ip + token + "200" + token + "1" + token + iValue

	// chksum = SHA1(chkstr)
	chksum := getSHA1(chkstr)

	// Step 4: Login
	callback = generateCallback()
	ts = fmt.Sprintf("%d", time.Now().UnixMilli())

	loginURL := fmt.Sprintf(
		"cgi-bin/srun_portal?callback=%s&action=login&username=%s&password=%%7BMD5%%7D%s&ac_id=%s&ip=%s&chksum=%s&info=%s&n=200&type=1&os=windows+10&name=windows&double_stack=0&_=%s",
		callback,
		url.QueryEscape(username),
		url.QueryEscape(hmd5),
		url.QueryEscape(acid),
		url.QueryEscape(ip),
		url.QueryEscape(chksum),
		url.QueryEscape(iValue),
		ts,
	)

	LogInfo("正在登录...")
	body, err = doSRUNGet(gateway, gatewayIP, loginURL)
	if err != nil {
		result.Error = fmt.Sprintf("登录请求失败: %v", err)
		result.Message = "登录请求失败"
		LogError(result.Error)
		return result
	}

	data, err = parseJSONP(body)
	if err != nil {
		result.Error = fmt.Sprintf("解析登录响应失败: %v", err)
		result.Message = "解析登录响应失败"
		LogError(result.Error)
		return result
	}

	result.IP = ip
	if errMsg, ok := data["error"]; ok && fmt.Sprintf("%v", errMsg) == "ok" {
		result.Result = 1
		result.Message = "登录成功"
		LogInfo(fmt.Sprintf("登录成功! IP: %s", ip))
	} else {
		errorMsg := "未知错误"
		if msg, ok := data["error_msg"]; ok {
			errorMsg = fmt.Sprintf("%v", msg)
		} else if msg, ok := data["error"]; ok {
			errorMsg = fmt.Sprintf("%v", msg)
		}
		result.Error = errorMsg
		result.Message = fmt.Sprintf("登录失败: %s", errorMsg)
		LogError(fmt.Sprintf("登录失败: %s", errorMsg))
	}

	return result
}
