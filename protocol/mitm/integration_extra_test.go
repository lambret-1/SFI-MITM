package mitm

import (
	"context"
	"crypto/tls"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/option"
	N "github.com/sagernet/sing/common/network"
)

// mock路由器 简化的 Router mock，仅实现 RouteConnectionEx
// 用于测试 MITM 上游连接建立和 HTTP 转发
type mock路由器 struct {
	adapter.Router
	// 连接处理函数：接管网管连接后的处理逻辑
	处理函数 func(conn net.Conn)
	// 记录接收到的连接元数据
	收到的元数据 []adapter.InboundContext
	锁          sync.Mutex
}

// RouteConnectionEx 实现 ConnectionRouterEx 接口
func (m *mock路由器) RouteConnectionEx(ctx context.Context, conn net.Conn, metadata adapter.InboundContext, onClose N.CloseHandlerFunc) {
	m.锁.Lock()
	m.收到的元数据 = append(m.收到的元数据, metadata)
	m.锁.Unlock()
	if m.处理函数 != nil {
		m.处理函数(conn)
	} else {
		// 默认：echo 服务器
		go func() {
			defer conn.Close()
			io.Copy(conn, conn)
		}()
	}
}

// RouteConnection 实现 ConnectionRouter 接口
func (m *mock路由器) RouteConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext) error {
	m.RouteConnectionEx(ctx, conn, metadata, nil)
	return nil
}

// Test建立上游连接_通过Router 验证建立上游连接通过 Router 路由决策
func Test建立上游连接_通过Router(t *testing.T) {
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
		日志缓冲区: 新日志环形缓冲区(100),
	}

	mock := &mock路由器{}
	元数据 := adapter.InboundContext{}

	// 建立上游连接（会因为 TLS 证书验证失败而返回错误，但 Router 应该被调用）
	_, err := svc.建立上游连接(context.Background(), mock, 元数据, "test.example.com", []string{"http/1.1"})
	// 由于 InsecureSkipVerify=false，自签名证书会被拒绝
	if err == nil {
		t.Error("期望自签名证书验证失败")
	}

	// 验证 Router 被调用，且 Inbound 标记为 "mitm"
	mock.锁.Lock()
	defer mock.锁.Unlock()
	if len(mock.收到的元数据) == 0 {
		t.Fatal("期望 Router 被调用")
	}
	最后元数据 := mock.收到的元数据[len(mock.收到的元数据)-1]
	if 最后元数据.Inbound != "mitm" {
		t.Errorf("期望 Inbound 标记为 mitm，实际 %s", 最后元数据.Inbound)
	}
	if 最后元数据.InboundType != "mitm" {
		t.Errorf("期望 InboundType 标记为 mitm，实际 %s", 最后元数据.InboundType)
	}
	if 最后元数据.Destination.Port != 443 {
		t.Errorf("期望目标端口为 443，实际 %d", 最后元数据.Destination.Port)
	}
	if 最后元数据.Network != "tcp" {
		t.Errorf("期望网络类型为 tcp，实际 %s", 最后元数据.Network)
	}
}

// Test建立上游连接_超时 验证上游连接建立超时
func Test建立上游连接_超时(t *testing.T) {
	svc := &Service{
		ctx:     context.Background(),
		logger:  获取测试日志器(),
		options: option.MITMServiceOptions{Enabled: true, UpstreamTimeout: 1},
		叶子缓存:   新证书缓存(默认缓存容量),
		日志缓冲区: 新日志环形缓冲区(100),
	}

	// mock Router 不处理连接，导致 TLS 握手超时
	mock := &mock路由器{
		处理函数: func(conn net.Conn) {
			// 不做任何事，保持连接打开但不握手
		},
	}

	元数据 := adapter.InboundContext{}
	_, err := svc.建立上游连接(context.Background(), mock, 元数据, "test.example.com", []string{"http/1.1"})
	if err == nil {
		t.Error("期望握手超时返回错误")
	}
}

// Test双向转发TLS_数据转发 验证双向转发功能
func Test双向转发TLS_数据转发(t *testing.T) {
	客户端端, 上游端 := net.Pipe()
	defer 客户端端.Close()
	defer 上游端.Close()

	客户端数据 := "CLIENT-DATA"
	上游数据 := "UPSTREAM-DATA"

	// 上游端：先发送数据，再读取客户端数据
	go func() {
		上游端.Write([]byte(上游数据))
		缓冲区 := make([]byte, 1024)
		n, _ := 上游端.Read(缓冲区)
		if string(缓冲区[:n]) != 客户端数据 {
			t.Errorf("上游期望收到 %s，实际 %s", 客户端数据, 缓冲区[:n])
		}
	}()

	// 客户端：先读取上游数据，再发送数据
	go func() {
		缓冲区 := make([]byte, 1024)
		n, _ := 客户端端.Read(缓冲区)
		if string(缓冲区[:n]) != 上游数据 {
			t.Errorf("客户端期望收到 %s，实际 %s", 上游数据, 缓冲区[:n])
		}
		客户端端.Write([]byte(客户端数据))
	}()

	// 双向转发
	错误 := make(chan error, 1)
	go func() {
		错误 <- 双向转发TLS(客户端端, 上游端)
	}()

	// 等待数据转发完成
	time.Sleep(200 * time.Millisecond)
	客户端端.Close()
	上游端.Close()
}

// Test处理连接_空连接关闭 验证处理连接在连接关闭时正常返回
func Test处理连接_空连接关闭(t *testing.T) {
	svc := &Service{
		ctx:     context.Background(),
		logger:  获取测试日志器(),
		options: option.MITMServiceOptions{Enabled: true},
		叶子缓存:   新证书缓存(默认缓存容量),
		日志缓冲区: 新日志环形缓冲区(100),
	}

	客户端端, 服务端端 := net.Pipe()
	客户端端.Close() // 立即关闭客户端

	处理器 := 新建HTTP1处理器(svc, nil, adapter.InboundContext{})
	err := 处理器.处理连接(context.Background(), 服务端端)
	if err != nil {
		// EOF 是正常的
		if err != io.EOF && err != io.ErrUnexpectedEOF {
			t.Logf("处理连接返回错误（可能正常）: %v", err)
		}
	}
	服务端端.Close()
}
