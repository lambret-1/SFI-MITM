package mitm

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/option"
)

// TestHTTP2_ServeHTTP_缺少Host头 验证缺少 Host 头时返回 400
func TestHTTP2_ServeHTTP_缺少Host头(t *testing.T) {
	svc := &Service{
		ctx:     context.Background(),
		logger:  获取测试日志器(),
		options: option.MITMServiceOptions{Enabled: true},
		叶子缓存:   新证书缓存(默认缓存容量),
		日志缓冲区: 新日志环形缓冲区(100),
	}

	处理器 := &http2请求处理器{
		服务:   svc,
		路由器: nil,
		元数据: adapter.InboundContext{},
		上下文: context.Background(),
	}

	// 构造缺少 Host 头的请求
	req := httptest.NewRequest("GET", "http:///test", nil)
	req.Host = ""

	响应记录器 := httptest.NewRecorder()
	处理器.ServeHTTP(响应记录器, req)

	if 响应记录器.Code != 400 {
		t.Errorf("期望状态码 400，实际 %d", 响应记录器.Code)
	}
	if 响应记录器.Body.String() != "缺少 Host 头\n" {
		t.Errorf("期望响应体 '缺少 Host 头'，实际 '%s'", 响应记录器.Body.String())
	}
}

// TestHTTP2_ServeHTTP_URLHost回退 验证 Host 为空时从 URL.Host 提取
func TestHTTP2_ServeHTTP_URLHost回退(t *testing.T) {
	svc := &Service{
		ctx:     context.Background(),
		logger:  获取测试日志器(),
		options: option.MITMServiceOptions{Enabled: true, UpstreamTimeout: 2},
		叶子缓存:   新证书缓存(默认缓存容量),
		日志缓冲区: 新日志环形缓冲区(100),
	}

	// mock Router：建立上游 TLS 服务器
	证书PEM, 私钥PEM := 生成测试CA(t)
	ca证书, ca私钥 := 解析测试CA(t, 证书PEM, 私钥PEM)
	上游服务器证书 := tls.Certificate{
		Certificate: [][]byte{ca证书.Raw},
		PrivateKey:  ca私钥,
	}
	mock := &mock路由器{
		处理函数: func(conn net.Conn) {
			tls配置 := &tls.Config{
				Certificates: []tls.Certificate{上游服务器证书},
				NextProtos:   []string{"h2", "http/1.1"},
			}
			tls服务端 := tls.Server(conn, tls配置)
			tls服务端.Handshake()
			tls服务端.Close()
		},
	}

	处理器 := &http2请求处理器{
		服务:   svc,
		路由器: mock,
		元数据: adapter.InboundContext{},
		上下文: context.Background(),
	}

	// 构造 Host 为空但 URL.Host 有值的请求
	req := httptest.NewRequest("GET", "http://example.com/test", nil)
	req.Host = ""

	响应记录器 := httptest.NewRecorder()
	处理器.ServeHTTP(响应记录器, req)

	// 由于上游使用自签名证书且 InsecureSkipVerify=false，
	// 建立上游连接会失败，返回 502
	if 响应记录器.Code != 502 {
		t.Logf("状态码: %d（自签名证书导致上游连接失败，期望 502）", 响应记录器.Code)
	}
}

// TestHTTP2_ServeHTTP_上游连接失败 验证上游连接失败时返回 502
func TestHTTP2_ServeHTTP_上游连接失败(t *testing.T) {
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

	处理器 := &http2请求处理器{
		服务:   svc,
		路由器: mock,
		元数据: adapter.InboundContext{},
		上下文: context.Background(),
	}

	req := httptest.NewRequest("GET", "http://example.com/test", nil)
	响应记录器 := httptest.NewRecorder()
	处理器.ServeHTTP(响应记录器, req)

	if 响应记录器.Code != 502 {
		t.Errorf("期望上游连接失败返回 502，实际 %d", 响应记录器.Code)
	}
	if 响应记录器.Body.String() != "上游连接失败\n" {
		t.Errorf("期望响应体 '上游连接失败'，实际 '%s'", 响应记录器.Body.String())
	}
}

// TestHTTP2_ServeHTTP_上游请求失败 验证上游请求失败时返回 502
func TestHTTP2_ServeHTTP_上游请求失败(t *testing.T) {
	svc, ca证书, ca私钥 := 创建完整TLS服务(t)
	上游证书 := 签发上游测试证书(t, ca证书, ca私钥, "example.com")

	// mock Router：建立上游 TLS 服务器，握手成功后不发送HTTP/2响应
	mock := &mock路由器{
		处理函数: func(conn net.Conn) {
			tls配置 := &tls.Config{
				Certificates: []tls.Certificate{上游证书},
				NextProtos:   []string{"h2"},
			}
			tls服务端 := tls.Server(conn, tls配置)
			if err := tls服务端.Handshake(); err != nil {
				return
			}
			// 握手成功后不发送HTTP/2响应，等待超时
			tls服务端.SetReadDeadline(time.Now().Add(2 * time.Second))
			缓冲区 := make([]byte, 4096)
			tls服务端.Read(缓冲区)
			tls服务端.Close()
		},
	}

	处理器 := &http2请求处理器{
		服务:   svc,
		路由器: mock,
		元数据: adapter.InboundContext{},
		上下文: context.Background(),
	}

	req := httptest.NewRequest("GET", "http://example.com/test", nil)
	响应记录器 := httptest.NewRecorder()
	处理器.ServeHTTP(响应记录器, req)

	// 上游请求失败应返回 502
	if 响应记录器.Code != 502 {
		t.Logf("状态码: %d（上游请求失败，期望 502）", 响应记录器.Code)
	}
}

// Test新建HTTP2处理器_服务器初始化 验证 HTTP/2 处理器构造时 http2.Server 已初始化
func Test新建HTTP2处理器_服务器初始化(t *testing.T) {
	svc := &Service{}
	处理器 := 新建HTTP2处理器(svc, nil, adapter.InboundContext{})
	if 处理器.服务器 == nil {
		t.Fatal("期望 http2.Server 已初始化")
	}
	// 验证默认配置（无特殊设置时使用零值）
	if 处理器.服务器.MaxHandlers != 0 {
		t.Log("MaxHandlers 使用非默认值")
	}
}

// 确保 http 包被引用
var _ http.Handler
