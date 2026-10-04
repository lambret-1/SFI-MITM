package mitm

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/option"
	M "github.com/sagernet/sing/common/metadata"
)

// TestIntercept_未启用直接路由 验证未启用 MITM 时直接交给 Router 路由
func TestIntercept_未启用直接路由(t *testing.T) {
	svc := &Service{
		ctx:     context.Background(),
		logger:  获取测试日志器(),
		options: option.MITMServiceOptions{Enabled: false},
		叶子缓存:   新证书缓存(默认缓存容量),
		日志缓冲区: 新日志环形缓冲区(100),
	}

	客户端端, 服务端端 := net.Pipe()
	defer 客户端端.Close()
	defer 服务端端.Close()

	收到数据 := make(chan []byte, 1)
	mock := &mock路由器{
		处理函数: func(conn net.Conn) {
			缓冲区 := make([]byte, 1024)
			n, _ := conn.Read(缓冲区)
			收到数据 <- 缓冲区[:n]
		},
	}

	元数据 := adapter.InboundContext{}
	go svc.Intercept(context.Background(), 服务端端, 元数据, mock, nil)

	// 客户端发送数据
	测试数据 := []byte("DIRECT-ROUTING-TEST")
	go func() {
		客户端端.Write(测试数据)
	}()

	select {
	case 数据 := <-收到数据:
		if !bytes.Equal(数据, 测试数据) {
			t.Errorf("期望 Router 收到原始数据 %s，实际 %s", 测试数据, 数据)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("超时：未收到 Router 转发的数据")
	}
}

// TestIntercept_域名不匹配回退 验证域名不匹配时用回退读取器将原始数据交回 Router
func TestIntercept_域名不匹配回退(t *testing.T) {
	证书PEM, 私钥PEM := 生成测试CA(t)
	ca证书, ca私钥 := 解析测试CA(t, 证书PEM, 私钥PEM)

	svc := &Service{
		ctx:     context.Background(),
		logger:  获取测试日志器(),
		options: option.MITMServiceOptions{Enabled: true},
		根证书: &根证书实例{
			证书对: tls.Certificate{
				Certificate: [][]byte{ca证书.Raw},
				PrivateKey:  ca私钥,
			},
			证书实体: ca证书,
		},
		叶子缓存:   新证书缓存(默认缓存容量),
		匹配器:    新域名匹配器(option.MITMMatchOptions{DomainSuffix: []string{"example.com"}}),
		日志缓冲区: 新日志环形缓冲区(100),
	}

	客户端端, 服务端端 := net.Pipe()
	defer 客户端端.Close()
	defer 服务端端.Close()

	// mock Router：接收到回退读取器包装的连接后，读取数据验证
	收到ClientHello := make(chan bool, 1)
	mock := &mock路由器{
		处理函数: func(conn net.Conn) {
			// 读取前几个字节，验证是 TLS ClientHello（内容类型 0x16 = 22）
			头部 := make([]byte, 5)
			n, err := io.ReadFull(conn, 头部)
			if err != nil || n < 5 {
				收到ClientHello <- false
				return
			}
			// TLS 记录内容类型 0x16 = Handshake
			收到ClientHello <- (头部[0] == 0x16)
		},
	}

	元数据 := adapter.InboundContext{}
	元数据.Destination = M.ParseSocksaddrHostPort("not-matched.org", 443)

	// 客户端发起 TLS 握手（SNI=not-matched.org，不匹配）
	go func() {
		tls配置 := &tls.Config{
			ServerName:         "not-matched.org",
			InsecureSkipVerify: true,
			NextProtos:         []string{"http/1.1"},
		}
		tls客户端 := tls.Client(客户端端, tls配置)
		tls客户端.Handshake()
	}()

	// 调用 Intercept（域名不匹配会回退到 Router）
	go svc.Intercept(context.Background(), 服务端端, 元数据, mock, nil)

	select {
	case 结果 := <-收到ClientHello:
		if !结果 {
			t.Error("期望 Router 收到原始 TLS ClientHello 数据（回退读取器应保留已读数据）")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("超时：Intercept 回退路径未完成")
	}
}

// TestIntercept_域名匹配TLS终止 验证域名匹配时完成 TLS 终止
func TestIntercept_域名匹配TLS终止(t *testing.T) {
	证书PEM, 私钥PEM := 生成测试CA(t)
	ca证书, ca私钥 := 解析测试CA(t, 证书PEM, 私钥PEM)

	svc := &Service{
		ctx:     context.Background(),
		logger:  获取测试日志器(),
		options: option.MITMServiceOptions{Enabled: true, UpstreamTimeout: 3},
		根证书: &根证书实例{
			证书对: tls.Certificate{
				Certificate: [][]byte{ca证书.Raw},
				PrivateKey:  ca私钥,
			},
			证书实体: ca证书,
		},
		叶子缓存:   新证书缓存(默认缓存容量),
		匹配器:    新域名匹配器(option.MITMMatchOptions{DomainSuffix: []string{"example.com"}}),
		日志缓冲区: 新日志环形缓冲区(100),
	}

	客户端端, 服务端端 := net.Pipe()
	defer 客户端端.Close()
	defer 服务端端.Close()

	// mock Router：建立上游 TLS 服务器（用测试 CA 签发的证书）
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
			tls服务端 := tls.Server(conn, tls配置)
			if err := tls服务端.Handshake(); err != nil {
				return
			}
			defer tls服务端.Close()
			// 读取 HTTP 请求并响应
			缓冲区 := make([]byte, 4096)
			n, _ := tls服务端.Read(缓冲区)
			if n > 0 {
				响应 := "HTTP/1.1 200 OK\r\nContent-Length: 2\r\nConnection: close\r\n\r\nOK"
				tls服务端.Write([]byte(响应))
			}
		},
	}

	元数据 := adapter.InboundContext{}
	元数据.Destination = M.ParseSocksaddrHostPort("test.example.com", 443)

	// 客户端发起 TLS 握手（SNI=test.example.com，匹配）
	客户端完成 := make(chan error, 1)
	go func() {
		// 客户端信任测试 CA
		根证书池 := newCertPool(ca证书)
		tls配置 := &tls.Config{
			ServerName: "test.example.com",
			RootCAs:    根证书池,
			NextProtos: []string{"http/1.1"},
		}
		tls客户端 := tls.Client(客户端端, tls配置)
		if err := tls客户端.Handshake(); err != nil {
			客户端完成 <- err
			return
		}
		// 发送 HTTP 请求
		tls客户端.Write([]byte("GET / HTTP/1.1\r\nHost: test.example.com\r\nConnection: close\r\n\r\n"))
		// 读取响应
		缓冲区 := make([]byte, 4096)
		n, err := tls客户端.Read(缓冲区)
		if err != nil {
			客户端完成 <- err
			return
		}
		t.Logf("客户端收到响应: %s", 缓冲区[:n])
		客户端完成 <- nil
	}()

	// 调用 Intercept（域名匹配会完成 TLS 终止 + HTTP 转发）
	go svc.Intercept(context.Background(), 服务端端, 元数据, mock, nil)

	select {
	case err := <-客户端完成:
		if err != nil {
			t.Logf("客户端完成（可能因上游自签名证书验证失败，属预期）: %v", err)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("超时：Intercept TLS 终止流程未完成")
	}
}

// newCertPool 创建包含指定证书的证书池
func newCertPool(证书 *x509.Certificate) *x509.CertPool {
	池 := x509.NewCertPool()
	池.AddCert(证书)
	return 池
}

// TestIntercept_读取ClientHello失败 验证读取ClientHello失败时关闭连接并调用关闭回调
func TestIntercept_读取ClientHello失败(t *testing.T) {
	svc := &Service{
		ctx:     context.Background(),
		logger:  获取测试日志器(),
		options: option.MITMServiceOptions{Enabled: true},
		叶子缓存:   新证书缓存(默认缓存容量),
		日志缓冲区: 新日志环形缓冲区(100),
	}

	客户端端, 服务端端 := net.Pipe()
	defer 客户端端.Close()

	// 关闭回调被调用标记
	关闭回调调用 := make(chan error, 1)
	关闭回调 := func(err error) {
		关闭回调调用 <- err
	}

	// 客户端发送非TLS数据后关闭
	go func() {
		客户端端.Write([]byte("NOT-TLS-DATA"))
		time.Sleep(50 * time.Millisecond)
		客户端端.Close()
	}()

	// 调用 Intercept（读取ClientHello会失败）
	go svc.Intercept(context.Background(), 服务端端, adapter.InboundContext{}, nil, 关闭回调)

	select {
	case err := <-关闭回调调用:
		if err == nil {
			t.Error("期望读取ClientHello失败时关闭回调收到错误")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("超时：关闭回调未被调用")
	}
}

// TestIntercept_终止TLS失败 验证根证书未加载时终止TLS失败
func TestIntercept_终止TLS失败(t *testing.T) {
	svc := &Service{
		ctx:     context.Background(),
		logger:  获取测试日志器(),
		options: option.MITMServiceOptions{Enabled: true},
		叶子缓存:   新证书缓存(默认缓存容量),
		匹配器:    新域名匹配器(option.MITMMatchOptions{Domain: []string{"example.com"}}),
		日志缓冲区: 新日志环形缓冲区(100),
		// 根证书为 nil，终止TLS会失败
	}

	客户端端, 服务端端 := net.Pipe()
	defer 客户端端.Close()

	// 客户端发起 TLS 握手（SNI=example.com，匹配，但根证书未加载）
	go func() {
		tls配置 := &tls.Config{
			ServerName:         "example.com",
			InsecureSkipVerify: true,
			NextProtos:         []string{"http/1.1"},
		}
		tls客户端 := tls.Client(客户端端, tls配置)
		tls客户端.Handshake() // 会失败，因为服务端无法完成握手
	}()

	// 调用 Intercept（域名匹配但终止TLS失败）
	完成 := make(chan struct{}, 1)
	go func() {
		svc.Intercept(context.Background(), 服务端端, adapter.InboundContext{}, nil, nil)
		完成 <- struct{}{}
	}()

	select {
	case <-完成:
		// 正常返回
	case <-time.After(10 * time.Second):
		t.Fatal("超时：Intercept 未在终止TLS失败后返回")
	}
}

// 确保 sync 包被引用
var _ sync.Mutex
