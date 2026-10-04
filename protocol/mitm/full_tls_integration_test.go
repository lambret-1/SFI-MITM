package mitm

import (
	"bufio"
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/option"
	"golang.org/x/net/http2"
)

// ========== 完整 TLS 集成测试辅助函数 ==========

// 签发上游测试证书 使用测试 CA 为指定域名签发上游服务器证书（含 SAN）
func 签发上游测试证书(t *testing.T, ca证书 *x509.Certificate, ca私钥 any, 域名 string) tls.Certificate {
	t.Helper()
	// 生成 ECDSA P256 密钥对
	私钥, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("生成上游证书密钥失败: %v", err)
	}
	// 生成随机序列号
	序列号, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		t.Fatalf("生成序列号失败: %v", err)
	}
	// 构造证书模板（含 SAN）
	模板 := &x509.Certificate{
		SerialNumber: 序列号,
		Subject:      pkix.Name{CommonName: 域名},
		DNSNames:     []string{域名},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	// 用测试 CA 签发
	证书DER, err := x509.CreateCertificate(rand.Reader, 模板, ca证书, &私钥.PublicKey, ca私钥)
	if err != nil {
		t.Fatalf("签发上游证书失败: %v", err)
	}
	return tls.Certificate{
		Certificate: [][]byte{证书DER},
		PrivateKey:  私钥,
	}
}

// 创建测试证书池 创建包含测试 CA 的证书池
func 创建测试证书池(t *testing.T, ca证书 *x509.Certificate) *x509.CertPool {
	t.Helper()
	池 := x509.NewCertPool()
	池.AddCert(ca证书)
	return 池
}

// 创建完整TLS服务 创建配置完整的 MITM Service（含测试CA、根证书池、匹配器等）
func 创建完整TLS服务(t *testing.T) (*Service, *x509.Certificate, any) {
	t.Helper()
	证书PEM, 私钥PEM := 生成测试CA(t)
	ca证书, ca私钥 := 解析测试CA(t, 证书PEM, 私钥PEM)
	svc := &Service{
		ctx:     context.Background(),
		logger:  获取测试日志器(),
		options: option.MITMServiceOptions{Enabled: true, UpstreamTimeout: 10},
		根证书: &根证书实例{
			证书对: tls.Certificate{
				Certificate: [][]byte{ca证书.Raw},
				PrivateKey:  ca私钥,
			},
			证书实体: ca证书,
		},
		叶子缓存:     新证书缓存(默认缓存容量),
		日志缓冲区:    新日志环形缓冲区(100),
		匹配器:       新域名匹配器(option.MITMMatchOptions{Domain: []string{"example.com"}}),
		上游根证书池:  创建测试证书池(t, ca证书),
	}
	return svc, ca证书, ca私钥
}

// ========== 处理请求 完整流程测试 ==========

// Test处理请求_完整TLS转发 验证通过完整TLS连接的HTTP请求转发
func Test处理请求_完整TLS转发(t *testing.T) {
	svc, ca证书, ca私钥 := 创建完整TLS服务(t)
	上游证书 := 签发上游测试证书(t, ca证书, ca私钥, "example.com")

	// mock Router：建立上游 TLS 服务器，返回 HTTP 响应
	mock := &mock路由器{
		处理函数: func(conn net.Conn) {
			tls配置 := &tls.Config{
				Certificates: []tls.Certificate{上游证书},
				NextProtos:   []string{"http/1.1"},
			}
			tls服务端 := tls.Server(conn, tls配置)
			defer tls服务端.Close()
			if err := tls服务端.Handshake(); err != nil {
				return
			}
			// 读取请求
			读取器 := bufio.NewReader(tls服务端)
			req, err := http.ReadRequest(读取器)
			if err != nil {
				return
			}
			// 发送响应
			响应体 := "Hello from upstream: " + req.URL.Path
			tls服务端.Write([]byte("HTTP/1.1 200 OK\r\n" +
				"Content-Type: text/plain\r\n" +
				"Content-Length: " + itoa(len(响应体)) + "\r\n" +
				"Connection: close\r\n\r\n" + 响应体))
		},
	}

	处理器 := 新建HTTP1处理器(svc, mock, adapter.InboundContext{})

	req := &http.Request{
		Method: "GET",
		Host:   "example.com",
		URL:    &url.URL{Path: "/test", Host: "example.com"},
		Header: make(http.Header),
	}

	写入缓冲区 := &bytes.Buffer{}
	写入器 := bufio.NewWriter(写入缓冲区)
	读取器 := bufio.NewReader(strings.NewReader(""))

	err := 处理器.处理请求(context.Background(), req, 读取器, 写入器)
	if err != nil {
		t.Fatalf("期望完整转发成功，实际错误: %v", err)
	}
	响应文本 := 写入缓冲区.String()
	if !strings.Contains(响应文本, "200 OK") {
		t.Errorf("期望响应包含 200 OK，实际: %s", 响应文本)
	}
	if !strings.Contains(响应文本, "Hello from upstream: /test") {
		t.Errorf("期望响应包含上游响应体，实际: %s", 响应文本)
	}
}

// Test处理请求_上游响应读取失败 验证上游响应读取失败时返回错误
func Test处理请求_上游响应读取失败(t *testing.T) {
	svc, ca证书, ca私钥 := 创建完整TLS服务(t)
	上游证书 := 签发上游测试证书(t, ca证书, ca私钥, "example.com")

	// mock Router：建立上游 TLS 服务器，接收请求但不发送响应就关闭
	mock := &mock路由器{
		处理函数: func(conn net.Conn) {
			tls配置 := &tls.Config{
				Certificates: []tls.Certificate{上游证书},
				NextProtos:   []string{"http/1.1"},
			}
			tls服务端 := tls.Server(conn, tls配置)
			defer tls服务端.Close()
			if err := tls服务端.Handshake(); err != nil {
				return
			}
			// 读取请求但不发送响应
			缓冲区 := make([]byte, 4096)
			tls服务端.Read(缓冲区)
			// 不发送响应，直接关闭
		},
	}

	处理器 := 新建HTTP1处理器(svc, mock, adapter.InboundContext{})

	req := &http.Request{
		Method: "GET",
		Host:   "example.com",
		URL:    &url.URL{Path: "/", Host: "example.com"},
		Header: make(http.Header),
	}

	写入缓冲区 := &bytes.Buffer{}
	写入器 := bufio.NewWriter(写入缓冲区)
	读取器 := bufio.NewReader(strings.NewReader(""))

	err := 处理器.处理请求(context.Background(), req, 读取器, 写入器)
	if err == nil {
		t.Error("期望读取上游响应失败返回错误")
	}
}

// Test处理请求_写入上游请求失败 验证上游连接关闭时写入请求失败
func Test处理请求_写入上游请求失败(t *testing.T) {
	svc, ca证书, ca私钥 := 创建完整TLS服务(t)
	上游证书 := 签发上游测试证书(t, ca证书, ca私钥, "example.com")

	// mock Router：建立上游 TLS 服务器，握手成功后立即关闭
	mock := &mock路由器{
		处理函数: func(conn net.Conn) {
			tls配置 := &tls.Config{
				Certificates: []tls.Certificate{上游证书},
				NextProtos:   []string{"http/1.1"},
			}
			tls服务端 := tls.Server(conn, tls配置)
			if err := tls服务端.Handshake(); err != nil {
				return
			}
			// 握手成功后立即关闭，不读取请求
			tls服务端.Close()
		},
	}

	处理器 := 新建HTTP1处理器(svc, mock, adapter.InboundContext{})

	req := &http.Request{
		Method: "GET",
		Host:   "example.com",
		URL:    &url.URL{Path: "/", Host: "example.com"},
		Header: make(http.Header),
	}

	写入缓冲区 := &bytes.Buffer{}
	写入器 := bufio.NewWriter(写入缓冲区)
	读取器 := bufio.NewReader(strings.NewReader(""))

	err := 处理器.处理请求(context.Background(), req, 读取器, 写入器)
	if err == nil {
		t.Error("期望写入上游请求失败返回错误")
	}
}

// itoa 简单的整数转字符串（避免引入 strconv）
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	负数 := n < 0
	if 负数 {
		n = -n
	}
	var 数字 []byte
	for n > 0 {
		数字 = append([]byte{byte('0' + n%10)}, 数字...)
		n /= 10
	}
	if 负数 {
		数字 = append([]byte{'-'}, 数字...)
	}
	return string(数字)
}

// 确保 io 被引用
var _ io.Reader

// ========== 终止TLS 完整流程测试 ==========

// Test终止TLS_根证书未加载 验证根证书未加载时返回错误
func Test终止TLS_根证书未加载(t *testing.T) {
	svc := &Service{}
	客户端端, _ := net.Pipe()
	defer 客户端端.Close()
	_, err := svc.终止TLS(客户端端, "example.com", []string{"http/1.1"})
	if err == nil {
		t.Error("期望根证书未加载时返回错误")
	}
}

// Test终止TLS_完整握手 验证完整的TLS终止流程
func Test终止TLS_完整握手(t *testing.T) {
	svc, _, _ := 创建完整TLS服务(t)

	客户端端, 服务端端 := net.Pipe()
	defer 客户端端.Close()
	defer 服务端端.Close()

	// 服务端做 TLS 终止
	服务端完成 := make(chan *tls.Conn, 1)
	服务端错误 := make(chan error, 1)
	go func() {
		tls连接, err := svc.终止TLS(服务端端, "example.com", []string{"http/1.1"})
		if err != nil {
			服务端错误 <- err
			return
		}
		服务端完成 <- tls连接
	}()

	// 客户端做 TLS 握手（使用 InsecureSkipVerify，因为是测试证书）
	客户端tls配置 := &tls.Config{
		ServerName:         "example.com",
		InsecureSkipVerify: true,
		NextProtos:         []string{"http/1.1"},
	}
	客户端tls连接 := tls.Client(客户端端, 客户端tls配置)
	if err := 客户端tls连接.Handshake(); err != nil {
		t.Fatalf("客户端 TLS 握手失败: %v", err)
	}
	defer 客户端tls连接.Close()

	// 验证服务端握手完成
	select {
	case 服务端tls连接 := <-服务端完成:
		defer 服务端tls连接.Close()
		// 验证协商的 ALPN
		if 服务端tls连接.ConnectionState().NegotiatedProtocol != "http/1.1" {
			t.Errorf("期望协商 ALPN 为 http/1.1，实际 %s",
				服务端tls连接.ConnectionState().NegotiatedProtocol)
		}
		// 验证双向数据传输
		go func() {
			服务端tls连接.Write([]byte("server-data"))
		}()
		缓冲区 := make([]byte, 1024)
		n, err := 客户端tls连接.Read(缓冲区)
		if err != nil {
			t.Fatalf("读取服务端数据失败: %v", err)
		}
		if string(缓冲区[:n]) != "server-data" {
			t.Errorf("期望读取到 'server-data'，实际 '%s'", string(缓冲区[:n]))
		}
	case err := <-服务端错误:
		t.Fatalf("服务端 TLS 终止失败: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("超时：TLS 终止未完成")
	}
}

// Test终止TLS_握手失败 验证TLS握手失败时返回错误
func Test终止TLS_握手失败(t *testing.T) {
	svc, _, _ := 创建完整TLS服务(t)

	客户端端, 服务端端 := net.Pipe()
	defer 客户端端.Close()

	// 服务端做 TLS 终止
	服务端错误 := make(chan error, 1)
	go func() {
		_, err := svc.终止TLS(服务端端, "example.com", []string{"http/1.1"})
		服务端错误 <- err
	}()

	// 客户端不做 TLS 握手，直接发送非 TLS 数据
	客户端端.Write([]byte("not-a-tls-handshake"))
	客户端端.Close()

	select {
	case err := <-服务端错误:
		if err == nil {
			t.Error("期望握手失败返回错误")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("超时：TLS 握手失败未返回")
	}
}

// ========== 双向转发TLS 完整流程测试 ==========

// Test双向转发TLS_正常转发 验证双向数据转发
func Test双向转发TLS_正常转发(t *testing.T) {
	客户端A, 客户端B := net.Pipe()
	上游A, 上游B := net.Pipe()
	defer 客户端A.Close()
	defer 客户端B.Close()
	defer 上游A.Close()
	defer 上游B.Close()

	转发错误 := make(chan error, 1)
	go func() {
		转发错误 <- 双向转发TLS(客户端A, 上游A)
	}()

	// 客户端B -> 客户端A -> 上游A -> 上游B
	go func() {
		客户端B.Write([]byte("client-to-server"))
	}()

	缓冲区 := make([]byte, 1024)
	n, err := 上游B.Read(缓冲区)
	if err != nil {
		t.Fatalf("读取上游数据失败: %v", err)
	}
	if string(缓冲区[:n]) != "client-to-server" {
		t.Errorf("期望读取到 'client-to-server'，实际 '%s'", string(缓冲区[:n]))
	}

	// 上游B -> 上游A -> 客户端A -> 客户端B
	go func() {
		上游B.Write([]byte("server-to-client"))
	}()

	n, err = 客户端B.Read(缓冲区)
	if err != nil {
		t.Fatalf("读取客户端数据失败: %v", err)
	}
	if string(缓冲区[:n]) != "server-to-client" {
		t.Errorf("期望读取到 'server-to-client'，实际 '%s'", string(缓冲区[:n]))
	}

	// 关闭连接，验证转发返回
	客户端B.Close()
	上游B.Close()

	select {
	case <-转发错误:
		// 正常返回
	case <-time.After(5 * time.Second):
		t.Fatal("超时：双向转发未在连接关闭后返回")
	}
}

// ========== ServeHTTP 完整流程测试 ==========

// TestServeHTTP_完整HTTP2转发 验证通过完整TLS连接的HTTP/2请求转发
func TestServeHTTP_完整HTTP2转发(t *testing.T) {
	svc, ca证书, ca私钥 := 创建完整TLS服务(t)
	上游证书 := 签发上游测试证书(t, ca证书, ca私钥, "example.com")

	// mock Router：建立上游 TLS 服务器（h2），处理 HTTP/2 请求
	mock := &mock路由器{
		处理函数: func(conn net.Conn) {
			tls配置 := &tls.Config{
				Certificates: []tls.Certificate{上游证书},
				NextProtos:   []string{"h2"},
			}
			tls服务端 := tls.Server(conn, tls配置)
			defer tls服务端.Close()
			if err := tls服务端.Handshake(); err != nil {
				return
			}
			// 使用 http2.Server 处理 HTTP/2 请求
			h2服务器 := &http2.Server{}
			处理器 := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/plain")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("HTTP/2 upstream: " + r.URL.Path))
			})
			h2服务器.ServeConn(tls服务端, &http2.ServeConnOpts{Handler: 处理器})
		},
	}

	处理器 := &http2请求处理器{
		服务:   svc,
		路由器: mock,
		元数据: adapter.InboundContext{},
		上下文: context.Background(),
	}

	req := httptest.NewRequest("GET", "https://example.com/h2test", nil)
	响应记录器 := httptest.NewRecorder()
	处理器.ServeHTTP(响应记录器, req)

	if 响应记录器.Code != 200 {
		t.Errorf("期望状态码 200，实际 %d", 响应记录器.Code)
	}
	if !strings.Contains(响应记录器.Body.String(), "HTTP/2 upstream: /h2test") {
		t.Errorf("期望响应包含上游响应体，实际: %s", 响应记录器.Body.String())
	}
}

// ========== 处理WebSocket 完整流程测试（稳定版） ==========

// Test处理WebSocket_完整转发_稳定版 使用测试CA签发的上游TLS证书验证WebSocket完整流程
func Test处理WebSocket_完整转发_稳定版(t *testing.T) {
	svc, ca证书, ca私钥 := 创建完整TLS服务(t)
	上游证书 := 签发上游测试证书(t, ca证书, ca私钥, "example.com")

	// mock Router：在net.Pipe的上游服务端上做TLS服务端握手，处理WebSocket升级并回显
	mock := &mock路由器{
		处理函数: func(conn net.Conn) {
			tls配置 := &tls.Config{
				Certificates: []tls.Certificate{上游证书},
				NextProtos:   []string{"http/1.1"},
			}
			tls服务端 := tls.Server(conn, tls配置)
			if err := tls服务端.Handshake(); err != nil {
				conn.Close()
				return
			}
			defer tls服务端.Close()
			// 读取 WebSocket 升级请求
			读取器 := bufio.NewReader(tls服务端)
			_, err := http.ReadRequest(读取器)
			if err != nil {
				return
			}
			// 发送 101 Switching Protocols 响应
			tls服务端.Write([]byte("HTTP/1.1 101 Switching Protocols\r\n" +
				"Upgrade: websocket\r\n" +
				"Connection: Upgrade\r\n\r\n"))
			// WebSocket 双向回显（设置读取超时避免永久阻塞）
			tls服务端.SetReadDeadline(time.Now().Add(5 * time.Second))
			缓冲区 := make([]byte, 4096)
			for {
				n, err := tls服务端.Read(缓冲区)
				if err != nil {
					return
				}
				tls服务端.Write(缓冲区[:n])
			}
		},
	}

	// 创建真实的 TCP 监听器作为 MITM 服务端
	监听器, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("创建监听器失败: %v", err)
	}
	defer 监听器.Close()

	处理器 := 新建HTTP1处理器(svc, mock, adapter.InboundContext{})

	// 在 goroutine 中接受连接并处理 WebSocket
	处理完成 := make(chan error, 1)
	go func() {
		客户端连接, err := 监听器.Accept()
		if err != nil {
			处理完成 <- err
			return
		}
		defer 客户端连接.Close()
		req := &http.Request{
			Method: "GET",
			Host:   "example.com",
			URL:    &url.URL{Path: "/ws", Host: "example.com"},
			Header: make(http.Header),
		}
		req.Header.Set("Upgrade", "websocket")
		req.Header.Set("Connection", "Upgrade")
		处理完成 <- 处理器.处理WebSocket(context.Background(), 客户端连接, bufio.NewReader(客户端连接), req)
	}()

	// 客户端连接到 MITM 服务端
	客户端连接, err := net.Dial("tcp", 监听器.Addr().String())
	if err != nil {
		t.Fatalf("连接失败: %v", err)
	}
	defer 客户端连接.Close()

	// 读取 101 响应（使用bufio.Reader确保读取完整响应头）
	客户端读取器 := bufio.NewReader(客户端连接)
	客户端连接.SetReadDeadline(time.Now().Add(10 * time.Second))
	resp, err := http.ReadResponse(客户端读取器, nil)
	if err != nil {
		t.Fatalf("读取响应失败: %v", err)
	}
	if resp.StatusCode != http.StatusSwitchingProtocols {
		t.Errorf("期望 101 状态码，实际 %d", resp.StatusCode)
	}
	resp.Body.Close()

	// 发送 WebSocket 数据，验证回显
	测试数据 := []byte("test-ws-stable")
	客户端连接.Write(测试数据)

	响应缓冲区 := make([]byte, 4096)
	n, err := 客户端读取器.Read(响应缓冲区)
	if err != nil {
		t.Fatalf("读取回显失败: %v", err)
	}
	if string(响应缓冲区[:n]) != string(测试数据) {
		t.Errorf("期望回显 '%s'，实际 '%s'", 测试数据, string(响应缓冲区[:n]))
	}

	// 关闭客户端连接，让处理WebSocket返回
	客户端连接.Close()
	select {
	case <-处理完成:
		// 正常返回
	case <-time.After(10 * time.Second):
		t.Fatal("超时：处理WebSocket未在客户端关闭后返回")
	}
}

// ========== Intercept 域名匹配完整流程测试 ==========

// TestIntercept_域名匹配_完整TLS转发 验证Intercept域名匹配时终止TLS并转发HTTP请求
func TestIntercept_域名匹配_完整TLS转发(t *testing.T) {
	svc, ca证书, ca私钥 := 创建完整TLS服务(t)
	上游证书 := 签发上游测试证书(t, ca证书, ca私钥, "example.com")

	// mock Router：建立上游 TLS 服务器，处理 HTTP 请求后关闭连接
	mock := &mock路由器{
		处理函数: func(conn net.Conn) {
			tls配置 := &tls.Config{
				Certificates: []tls.Certificate{上游证书},
				NextProtos:   []string{"http/1.1"},
			}
			tls服务端 := tls.Server(conn, tls配置)
			defer tls服务端.Close()
			if err := tls服务端.Handshake(); err != nil {
				return
			}
			读取器 := bufio.NewReader(tls服务端)
			req, err := http.ReadRequest(读取器)
			if err != nil {
				return
			}
			resp := &http.Response{
				StatusCode: 200,
				Proto:      "HTTP/1.1",
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader("MITM upstream response")),
			}
			resp.Header.Set("Content-Length", "21")
			resp.Header.Set("Connection", "close")
			resp.Write(tls服务端)
			req.Body.Close()
			// 关闭上游连接，让双向转发返回
			tls服务端.CloseWrite()
		},
	}

	// 创建真实的 TCP 监听器作为 MITM 服务端
	监听器, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("创建监听器失败: %v", err)
	}
	defer 监听器.Close()

	// 在 goroutine 中接受连接并调用 Intercept
	go func() {
		客户端连接, err := 监听器.Accept()
		if err != nil {
			return
		}
		defer 客户端连接.Close()
		svc.Intercept(context.Background(), 客户端连接, adapter.InboundContext{}, mock, nil)
	}()

	// 客户端使用 TLS 连接到 MITM 服务端
	客户端配置 := &tls.Config{
		ServerName: "example.com",
		RootCAs:    创建测试证书池(t, ca证书),
		NextProtos: []string{"http/1.1"},
	}
	客户端连接, err := tls.Dial("tcp", 监听器.Addr().String(), 客户端配置)
	if err != nil {
		t.Fatalf("TLS 连接失败: %v", err)
	}
	defer 客户端连接.Close()

	// 发送 HTTP 请求
	请求 := "GET /test HTTP/1.1\r\nHost: example.com\r\nConnection: close\r\n\r\n"
	客户端连接.Write([]byte(请求))

	// 读取响应
	响应缓冲区 := make([]byte, 4096)
	n, err := 客户端连接.Read(响应缓冲区)
	if err != nil {
		t.Fatalf("读取响应失败: %v", err)
	}
	响应文本 := string(响应缓冲区[:n])
	if !strings.Contains(响应文本, "200 OK") {
		t.Errorf("期望 200 响应，实际: %s", 响应文本)
	}
	if !strings.Contains(响应文本, "MITM upstream response") {
		t.Errorf("期望响应包含上游响应体，实际: %s", 响应文本)
	}
	// 关闭客户端连接，让 Intercept 返回
	客户端连接.Close()
	time.Sleep(200 * time.Millisecond)

	// 打印日志缓冲区内容，用于调试
	日志条目 := svc.日志缓冲区.获取全部()
	t.Logf("MITM 日志条目数: %d", len(日志条目))
	for _, 条目 := range 日志条目 {
		t.Logf("  Level=%d Message=%s", 条目.Level, 条目.Message)
	}
}

