package mitm

import (
	"bufio"
	"context"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/option"
	"golang.org/x/net/http2"
)

// TestHTTP2_处理连接_连接关闭正常返回 验证处理连接在客户端关闭时正常返回
func TestHTTP2_处理连接_连接关闭正常返回(t *testing.T) {
	svc := &Service{
		ctx:     context.Background(),
		logger:  获取测试日志器(),
		options: option.MITMServiceOptions{Enabled: true},
		叶子缓存:   新证书缓存(默认缓存容量),
		日志缓冲区: 新日志环形缓冲区(100),
	}

	客户端端, 服务端端 := net.Pipe()
	defer 客户端端.Close()

	处理器 := 新建HTTP2处理器(svc, nil, adapter.InboundContext{})

	完成 := make(chan error, 1)
	go func() {
		完成 <- 处理器.处理连接(context.Background(), 服务端端)
	}()

	// 客户端发送 HTTP/2 连接前言后关闭
	客户端端.Write([]byte("PRI * HTTP/2.0\r\n\r\nSM\r\n\r\n"))
	time.Sleep(100 * time.Millisecond)
	客户端端.Close()

	select {
	case err := <-完成:
		// 连接关闭后 ServeConn 应返回（可能有错误，属正常）
		t.Logf("处理连接返回: %v", err)
	case <-time.After(5 * time.Second):
		服务端端.Close()
		t.Fatal("超时：处理连接未在连接关闭后返回")
	}
}

// TestHTTP2_处理连接_空连接立即关闭 验证空连接立即关闭时正常返回
func TestHTTP2_处理连接_空连接立即关闭(t *testing.T) {
	svc := &Service{
		ctx:     context.Background(),
		logger:  获取测试日志器(),
		options: option.MITMServiceOptions{Enabled: true},
		叶子缓存:   新证书缓存(默认缓存容量),
		日志缓冲区: 新日志环形缓冲区(100),
	}

	客户端端, 服务端端 := net.Pipe()
	客户端端.Close() // 立即关闭客户端

	处理器 := 新建HTTP2处理器(svc, nil, adapter.InboundContext{})
	err := 处理器.处理连接(context.Background(), 服务端端)
	服务端端.Close()
	// 空连接关闭时 ServeConn 应返回错误
	t.Logf("空连接处理返回: %v", err)
}

// TestHTTP2_处理连接_写入日志缓冲区 验证处理连接启动时写入日志缓冲区
func TestHTTP2_处理连接_写入日志缓冲区(t *testing.T) {
	svc := &Service{
		ctx:     context.Background(),
		logger:  获取测试日志器(),
		options: option.MITMServiceOptions{Enabled: true},
		叶子缓存:   新证书缓存(默认缓存容量),
		日志缓冲区: 新日志环形缓冲区(100),
	}

	客户端端, 服务端端 := net.Pipe()
	defer 客户端端.Close()
	defer 服务端端.Close()

	处理器 := 新建HTTP2处理器(svc, nil, adapter.InboundContext{})

	go func() {
		处理器.处理连接(context.Background(), 服务端端)
	}()

	// 等待处理连接启动
	time.Sleep(100 * time.Millisecond)

	// 验证日志缓冲区中有 HTTP 引擎启动日志
	日志列表 := svc.日志缓冲区.获取全部()
	if len(日志列表) == 0 {
		t.Error("期望日志缓冲区中有 HTTP 引擎启动日志")
	} else {
		找到 := false
		for _, 日志 := range 日志列表 {
			if 日志.Message != "" {
				找到 = true
				break
			}
		}
		if !找到 {
			t.Error("期望日志消息非空")
		}
	}
}

// TestHTTP1_处理WebSocket_缺少Host头 验证 WebSocket 处理缺少 Host 头时返回错误
func TestHTTP1_处理WebSocket_缺少Host头(t *testing.T) {
	svc := &Service{
		ctx:     context.Background(),
		logger:  获取测试日志器(),
		options: option.MITMServiceOptions{Enabled: true},
		叶子缓存:   新证书缓存(默认缓存容量),
		日志缓冲区: 新日志环形缓冲区(100),
	}

	客户端端, 服务端端 := net.Pipe()
	defer 客户端端.Close()
	defer 服务端端.Close()

	处理器 := 新建HTTP1处理器(svc, nil, adapter.InboundContext{})

	// 构造缺少 Host 头的 WebSocket Upgrade 请求
	req := &http.Request{
		Method: "GET",
		URL:    &url.URL{Path: "/ws"},
		Header: make(http.Header),
	}
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Connection", "Upgrade")

	go func() {
		处理器.处理WebSocket(context.Background(), 服务端端, newBufioReader(服务端端), req)
	}()

	// 读取错误响应
	缓冲区 := make([]byte, 1024)
	n, err := 客户端端.Read(缓冲区)
	if err != nil && err != io.EOF {
		t.Fatalf("读取响应失败: %v", err)
	}
	响应文本 := string(缓冲区[:n])
	if !contains(响应文本, "400") {
		t.Errorf("期望响应包含 400 状态码，实际: %s", 响应文本)
	}
	if !contains(响应文本, "缺少 Host 头") {
		t.Errorf("期望响应包含 '缺少 Host 头'，实际: %s", 响应文本)
	}
}

// TestHTTP1_处理WebSocket_上游连接失败 验证 WebSocket 上游连接失败时返回错误
func TestHTTP1_处理WebSocket_上游连接失败(t *testing.T) {
	svc := &Service{
		ctx:     context.Background(),
		logger:  获取测试日志器(),
		options: option.MITMServiceOptions{Enabled: true, UpstreamTimeout: 1},
		叶子缓存:   新证书缓存(默认缓存容量),
		日志缓冲区: 新日志环形缓冲区(100),
	}

	// mock Router：不处理连接，导致 TLS 握手超时
	mock := &mock路由器{
		处理函数: func(conn net.Conn) {
			// 不做任何事
		},
	}

	客户端端, 服务端端 := net.Pipe()
	defer 客户端端.Close()
	defer 服务端端.Close()

	处理器 := 新建HTTP1处理器(svc, mock, adapter.InboundContext{})

	req := &http.Request{
		Method: "GET",
		Host:   "example.com",
		URL:    &url.URL{Path: "/ws"},
		Header: make(http.Header),
	}
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Connection", "Upgrade")

	错误 := make(chan error, 1)
	go func() {
		错误 <- 处理器.处理WebSocket(context.Background(), 服务端端, newBufioReader(服务端端), req)
	}()

	select {
	case err := <-错误:
		if err == nil {
			t.Error("期望上游连接失败返回错误")
		}
		t.Logf("上游连接失败返回错误: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("超时：处理WebSocket 未在超时后返回")
	}
}

// TestHTTP1_处理WebSocket_转发握手请求失败 验证转发WebSocket握手请求失败时返回错误
func TestHTTP1_处理WebSocket_转发握手请求失败(t *testing.T) {
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
			// 握手成功后立即关闭，导致写入请求失败
			tls服务端.Close()
		},
	}

	客户端端, 服务端端 := net.Pipe()
	defer 客户端端.Close()
	defer 服务端端.Close()

	处理器 := 新建HTTP1处理器(svc, mock, adapter.InboundContext{})

	req := &http.Request{
		Method: "GET",
		Host:   "example.com",
		URL:    &url.URL{Path: "/ws", Host: "example.com"},
		Header: make(http.Header),
	}
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Connection", "Upgrade")

	错误 := make(chan error, 1)
	go func() {
		错误 <- 处理器.处理WebSocket(context.Background(), 服务端端, bufio.NewReader(服务端端), req)
	}()

	select {
	case err := <-错误:
		if err == nil {
			t.Error("期望转发握手请求失败返回错误")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("超时：处理WebSocket 未在转发失败后返回")
	}
}

// TestHTTP1_处理WebSocket_读取上游响应失败 验证读取上游WebSocket响应失败时返回错误
func TestHTTP1_处理WebSocket_读取上游响应失败(t *testing.T) {
	svc, ca证书, ca私钥 := 创建完整TLS服务(t)
	上游证书 := 签发上游测试证书(t, ca证书, ca私钥, "example.com")

	// mock Router：建立上游 TLS 服务器，握手成功后不发送响应就关闭
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
			// 读取请求但不发送响应，然后关闭
			读取器 := bufio.NewReader(tls服务端)
			tls服务端.SetReadDeadline(time.Now().Add(2 * time.Second))
			缓冲区 := make([]byte, 4096)
			读取器.Read(缓冲区)
			tls服务端.Close()
		},
	}

	客户端端, 服务端端 := net.Pipe()
	defer 客户端端.Close()
	defer 服务端端.Close()

	处理器 := 新建HTTP1处理器(svc, mock, adapter.InboundContext{})

	req := &http.Request{
		Method: "GET",
		Host:   "example.com",
		URL:    &url.URL{Path: "/ws", Host: "example.com"},
		Header: make(http.Header),
	}
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Connection", "Upgrade")

	错误 := make(chan error, 1)
	go func() {
		错误 <- 处理器.处理WebSocket(context.Background(), 服务端端, bufio.NewReader(服务端端), req)
	}()

	select {
	case err := <-错误:
		if err == nil {
			t.Error("期望读取上游响应失败返回错误")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("超时：处理WebSocket 未在读取响应失败后返回")
	}
}

// newBufioReader 创建 bufio.Reader（辅助函数）
func newBufioReader(r io.Reader) *bufio.Reader {
	return bufio.NewReader(r)
}

// TestHTTP1_处理连接_读取无效HTTP请求 验证读取无效HTTP请求时返回错误
func TestHTTP1_处理连接_读取无效HTTP请求(t *testing.T) {
	svc := &Service{
		ctx:     context.Background(),
		logger:  获取测试日志器(),
		options: option.MITMServiceOptions{Enabled: true},
		叶子缓存:   新证书缓存(默认缓存容量),
		日志缓冲区: 新日志环形缓冲区(100),
	}

	// 使用真实TCP连接（处理连接需要SetReadDeadline，net.Pipe不支持）
	监听器, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("创建监听器失败: %v", err)
	}
	defer 监听器.Close()

	处理器 := 新建HTTP1处理器(svc, nil, adapter.InboundContext{})

	错误 := make(chan error, 1)
	go func() {
		客户端连接, err := 监听器.Accept()
		if err != nil {
			return
		}
		defer 客户端连接.Close()
		错误 <- 处理器.处理连接(context.Background(), 客户端连接)
	}()

	客户端连接, err := net.Dial("tcp", 监听器.Addr().String())
	if err != nil {
		t.Fatalf("连接失败: %v", err)
	}
	defer 客户端连接.Close()

	// 发送无效的 HTTP 数据（不是以 HTTP 方法开头）
	客户端连接.Write([]byte("INVALID HTTP DATA\r\n\r\n"))

	select {
	case err := <-错误:
		if err == nil {
			t.Error("期望读取无效HTTP请求返回错误")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("超时：处理连接未在读取无效请求后返回")
	}
}

// TestHTTP1_处理连接_保持连接 验证保持连接时处理多个请求
func TestHTTP1_处理连接_保持连接(t *testing.T) {
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
			读取器 := bufio.NewReader(tls服务端)
			// 处理多个请求
			for i := 0; i < 2; i++ {
				req, err := http.ReadRequest(读取器)
				if err != nil {
					return
				}
				响应体 := "Response " + itoa(i)
				tls服务端.Write([]byte("HTTP/1.1 200 OK\r\n" +
					"Content-Type: text/plain\r\n" +
					"Content-Length: " + itoa(len(响应体)) + "\r\n" +
					"Connection: keep-alive\r\n\r\n" + 响应体))
				req.Body.Close()
			}
		},
	}

	// 使用真实TCP连接（处理连接需要SetReadDeadline）
	监听器, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("创建监听器失败: %v", err)
	}
	defer 监听器.Close()

	处理器 := 新建HTTP1处理器(svc, mock, adapter.InboundContext{})

	完成 := make(chan struct{}, 1)
	go func() {
		客户端连接, err := 监听器.Accept()
		if err != nil {
			return
		}
		defer 客户端连接.Close()
		处理器.处理连接(context.Background(), 客户端连接)
		完成 <- struct{}{}
	}()

	客户端连接, err := net.Dial("tcp", 监听器.Addr().String())
	if err != nil {
		t.Fatalf("连接失败: %v", err)
	}
	defer 客户端连接.Close()

	// 发送第一个请求（keep-alive）
	客户端连接.Write([]byte("GET /1 HTTP/1.1\r\nHost: example.com\r\nConnection: keep-alive\r\n\r\n"))
	// 读取第一个响应
	客户端读取器 := bufio.NewReader(客户端连接)
	客户端连接.SetReadDeadline(time.Now().Add(10 * time.Second))
	resp1, err := http.ReadResponse(客户端读取器, nil)
	if err != nil {
		t.Fatalf("读取第一个响应失败: %v", err)
	}
	if resp1.StatusCode != 200 {
		t.Errorf("期望第一个响应 200，实际 %d", resp1.StatusCode)
	}
	resp1.Body.Close()

	// 发送第二个请求（close）
	客户端连接.Write([]byte("GET /2 HTTP/1.1\r\nHost: example.com\r\nConnection: close\r\n\r\n"))
	// 读取第二个响应
	resp2, err := http.ReadResponse(客户端读取器, nil)
	if err != nil {
		t.Fatalf("读取第二个响应失败: %v", err)
	}
	if resp2.StatusCode != 200 {
		t.Errorf("期望第二个响应 200，实际 %d", resp2.StatusCode)
	}
	resp2.Body.Close()

	// 等待处理连接返回
	select {
	case <-完成:
		// 正常返回
	case <-time.After(10 * time.Second):
		t.Fatal("超时：处理连接未在第二个请求后返回")
	}
}

// 确保引用
var _ = tls.VersionTLS12
var _ http2.Server
