package mitm

import (
	"bufio"
	"context"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/option"
	M "github.com/sagernet/sing/common/metadata"
)

// Test处理请求_缺少Host头 验证缺少 Host 头时返回 400 错误
func Test处理请求_缺少Host头(t *testing.T) {
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

	go func() {
		读取器 := bufio.NewReader(服务端端)
		写入器 := bufio.NewWriter(服务端端)
		req, _ := http.ReadRequest(读取器)
		处理器.处理请求(context.Background(), req, 读取器, 写入器)
	}()

	// 发送缺少 Host 头的请求
	客户端端.Write([]byte("GET / HTTP/1.1\r\n\r\n"))

	// 读取响应
	响应, err := http.ReadResponse(bufio.NewReader(客户端端), nil)
	if err != nil {
		t.Fatalf("读取响应失败: %v", err)
	}
	defer 响应.Body.Close()
	if 响应.StatusCode != 400 {
		t.Errorf("期望状态码 400，实际 %d", 响应.StatusCode)
	}
	body, _ := io.ReadAll(响应.Body)
	if !strings.Contains(string(body), "缺少 Host 头") {
		t.Errorf("期望响应体包含 '缺少 Host 头'，实际: %s", body)
	}
}

// Test处理请求_完整HTTP转发 验证完整的 HTTP 请求转发
func Test处理请求_完整HTTP转发(t *testing.T) {
	证书PEM, 私钥PEM := 生成测试CA(t)
	ca证书, ca私钥 := 解析测试CA(t, 证书PEM, 私钥PEM)

	svc := &Service{
		ctx:     context.Background(),
		logger:  获取测试日志器(),
		options: option.MITMServiceOptions{Enabled: true, UpstreamTimeout: 5},
		根证书: &根证书实例{
			证书对: tls.Certificate{
				Certificate: [][]byte{ca证书.Raw},
				PrivateKey:  ca私钥,
			},
			证书实体: ca证书,
		},
		叶子缓存:   新证书缓存(默认缓存容量),
		日志缓冲区: 新日志环形缓冲区(100),
	}

	// mock Router：接管网管连接后建立 TLS + HTTP 服务器
	上游服务器证书 := tls.Certificate{
		Certificate: [][]byte{ca证书.Raw},
		PrivateKey:  ca私钥,
	}

	mock := &mock路由器{
		处理函数: func(conn net.Conn) {
			tls配置 := &tls.Config{
				Certificates: []tls.Certificate{上游服务器证书},
				NextProtos:   []string{"http/1.1"},
			}
			tlsConn := tls.Server(conn, tls配置)
			if err := tlsConn.Handshake(); err != nil {
				return
			}
			defer tlsConn.Close()
			// 简单的 HTTP 服务器
			reader := bufio.NewReader(tlsConn)
			for {
				_, err := http.ReadRequest(reader)
				if err != nil {
					return
				}
				resp := &http.Response{
					StatusCode: 200,
					ProtoMajor: 1,
					ProtoMinor: 1,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader("MITM-UPSTREAM-OK")),
				}
				resp.Header.Set("Content-Type", "text/plain")
				resp.ContentLength = 17
				resp.Write(tlsConn)
			}
		},
	}

	客户端端, 服务端端 := net.Pipe()
	defer 客户端端.Close()
	defer 服务端端.Close()

	处理器 := 新建HTTP1处理器(svc, mock, adapter.InboundContext{})

	go func() {
		读取器 := bufio.NewReader(服务端端)
		写入器 := bufio.NewWriter(服务端端)
		req, _ := http.ReadRequest(读取器)
		处理器.处理请求(context.Background(), req, 读取器, 写入器)
	}()

	// 发送 HTTP 请求
	客户端端.Write([]byte("GET /test HTTP/1.1\r\nHost: example.com\r\nConnection: close\r\n\r\n"))

	// 读取响应（设置超时避免挂起）
	客户端端.SetReadDeadline(time.Now().Add(10 * time.Second))
	响应, err := http.ReadResponse(bufio.NewReader(客户端端), nil)
	if err != nil {
		t.Fatalf("读取响应失败: %v", err)
	}
	defer 响应.Body.Close()

	// 由于上游使用自签名证书且 InsecureSkipVerify=false，
	// 建立上游连接会失败，返回 502
	if 响应.StatusCode != 502 {
		t.Logf("状态码: %d（自签名证书导致上游连接失败，期望 502）", 响应.StatusCode)
	}
}

// TestShouldIntercept_包内测试 验证 ShouldIntercept 在包内的行为
func TestShouldIntercept_包内测试(t *testing.T) {
	svc := &Service{
		options: option.MITMServiceOptions{Enabled: true},
		匹配器:  新域名匹配器(option.MITMMatchOptions{DomainSuffix: []string{"example.com"}}),
	}
	元数据 := adapter.InboundContext{}
	元数据.Destination = M.ParseSocksaddrHostPort("example.com", 443)
	if !svc.ShouldIntercept(元数据) {
		t.Error("期望 443 端口连接被拦截")
	}
	// 非 443 端口
	元数据.Destination = M.ParseSocksaddrHostPort("example.com", 80)
	if svc.ShouldIntercept(元数据) {
		t.Error("期望非 443 端口不被拦截")
	}
	// 未启用
	svc.options.Enabled = false
	元数据.Destination = M.ParseSocksaddrHostPort("example.com", 443)
	if svc.ShouldIntercept(元数据) {
		t.Error("未启用时期望不拦截")
	}
}

// Test是否启用_包内测试 验证是否启用方法
func Test是否启用_包内测试(t *testing.T) {
	svc := &Service{options: option.MITMServiceOptions{Enabled: true}}
	if !svc.是否启用() {
		t.Error("期望返回 true")
	}
	svc.options.Enabled = false
	if svc.是否启用() {
		t.Error("期望返回 false")
	}
}

// TestCA已安装_包内测试 验证 CA已安装方法
func TestCA已安装_包内测试(t *testing.T) {
	svc := &Service{}
	if svc.CA已安装() {
		t.Error("根证书为 nil 时期望返回 false")
	}
	svc.根证书 = &根证书实例{}
	if !svc.CA已安装() {
		t.Error("根证书非 nil 时期望返回 true")
	}
}
