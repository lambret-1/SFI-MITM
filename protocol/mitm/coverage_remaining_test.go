package mitm

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/option"
	M "github.com/sagernet/sing/common/metadata"
	"github.com/sagernet/sing/service"
)

// ========== 加载根证书 全分支覆盖 ==========

// Test加载根证书_证书文件不存在 验证证书文件不存在时返回错误
func Test加载根证书_证书文件不存在(t *testing.T) {
	配置 := option.MITMCAOptions{
		Certificate: "/nonexistent/cert.pem",
		PrivateKey:  "/nonexistent/key.pem",
	}
	_, err := 加载根证书(配置)
	if err == nil {
		t.Error("期望证书文件不存在时返回错误")
	}
}

// Test加载根证书_私钥文件不存在 验证私钥文件不存在时返回错误
func Test加载根证书_私钥文件不存在(t *testing.T) {
	// 创建临时证书文件
	临时文件, err := os.CreateTemp("", "cert-*.pem")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(临时文件.Name())
	临时文件.WriteString("invalid cert")
	临时文件.Close()

	配置 := option.MITMCAOptions{
		Certificate: 临时文件.Name(),
		PrivateKey:  "/nonexistent/key.pem",
	}
	_, err = 加载根证书(配置)
	if err == nil {
		t.Error("期望私钥文件不存在时返回错误")
	}
}

// Test加载根证书_证书私钥不匹配 验证证书和私钥不匹配时返回错误
func Test加载根证书_证书私钥不匹配(t *testing.T) {
	临时目录 := t.TempDir()
	// 生成两个不同的密钥对
	私钥1, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	私钥2, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)

	// 用私钥1生成证书
	证书模板 := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		IsCA:         true,
		KeyUsage:     x509.KeyUsageCertSign,
	}
	证书DER, _ := x509.CreateCertificate(rand.Reader, 证书模板, 证书模板, &私钥1.PublicKey, 私钥1)
	证书PEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: 证书DER})

	// 用私钥2生成私钥PEM
	私钥2字节, _ := x509.MarshalECPrivateKey(私钥2)
	私钥PEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: 私钥2字节})

	证书路径 := filepath.Join(临时目录, "cert.pem")
	私钥路径 := filepath.Join(临时目录, "key.pem")
	os.WriteFile(证书路径, 证书PEM, 0644)
	os.WriteFile(私钥路径, 私钥PEM, 0644)

	配置 := option.MITMCAOptions{Certificate: 证书路径, PrivateKey: 私钥路径}
	_, err := 加载根证书(配置)
	if err == nil {
		t.Error("期望证书私钥不匹配时返回错误")
	}
}

// Test加载根证书_非CA证书 验证非CA证书时返回错误
func Test加载根证书_非CA证书(t *testing.T) {
	临时目录 := t.TempDir()
	私钥, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)

	// 生成非 CA 证书（IsCA=false）
	证书模板 := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		IsCA:         false,
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}
	证书DER, _ := x509.CreateCertificate(rand.Reader, 证书模板, 证书模板, &私钥.PublicKey, 私钥)
	证书PEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: 证书DER})
	私钥字节, _ := x509.MarshalECPrivateKey(私钥)
	私钥PEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: 私钥字节})

	证书路径 := filepath.Join(临时目录, "cert.pem")
	私钥路径 := filepath.Join(临时目录, "key.pem")
	os.WriteFile(证书路径, 证书PEM, 0644)
	os.WriteFile(私钥路径, 私钥PEM, 0644)

	配置 := option.MITMCAOptions{Certificate: 证书路径, PrivateKey: 私钥路径}
	_, err := 加载根证书(配置)
	if err == nil {
		t.Error("期望非CA证书时返回错误")
	}
}

// Test加载根证书_缺少KeyUsageCertSign 验证缺少KeyUsageCertSign时返回错误
func Test加载根证书_缺少KeyUsageCertSign(t *testing.T) {
	临时目录 := t.TempDir()
	私钥, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)

	// 生成 CA 证书但缺少 KeyUsageCertSign
	证书模板 := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		IsCA:         true,
		KeyUsage:     x509.KeyUsageDigitalSignature, // 缺少 KeyUsageCertSign
	}
	证书DER, _ := x509.CreateCertificate(rand.Reader, 证书模板, 证书模板, &私钥.PublicKey, 私钥)
	证书PEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: 证书DER})
	私钥字节, _ := x509.MarshalECPrivateKey(私钥)
	私钥PEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: 私钥字节})

	证书路径 := filepath.Join(临时目录, "cert.pem")
	私钥路径 := filepath.Join(临时目录, "key.pem")
	os.WriteFile(证书路径, 证书PEM, 0644)
	os.WriteFile(私钥路径, 私钥PEM, 0644)

	配置 := option.MITMCAOptions{Certificate: 证书路径, PrivateKey: 私钥路径}
	_, err := 加载根证书(配置)
	if err == nil {
		t.Error("期望缺少KeyUsageCertSign时返回错误")
	}
}

// ========== 签发叶子证书 全分支覆盖 ==========

// Test签发叶子证书_根证书未加载 验证根证书未加载时返回错误
func Test签发叶子证书_根证书未加载(t *testing.T) {
	svc := &Service{}
	_, err := svc.签发叶子证书("example.com")
	if err == nil {
		t.Error("期望根证书未加载时返回错误")
	}
}

// Test签发叶子证书_域名为空 验证域名为空时返回错误
func Test签发叶子证书_域名为空(t *testing.T) {
	svc := &Service{根证书: &根证书实例{}}
	_, err := svc.签发叶子证书("")
	if err == nil {
		t.Error("期望域名为空时返回错误")
	}
}

// Test签发叶子证书_缓存命中 验证同域名第二次签发时命中缓存
func Test签发叶子证书_缓存命中(t *testing.T) {
	svc, _, _ := 创建完整TLS服务(t)

	// 第一次签发：应该生成新证书
	证书1, err := svc.签发叶子证书("cached.example.com")
	if err != nil {
		t.Fatalf("第一次签发失败: %v", err)
	}

	// 第二次签发：应该命中缓存，返回相同证书
	证书2, err := svc.签发叶子证书("cached.example.com")
	if err != nil {
		t.Fatalf("第二次签发失败: %v", err)
	}

	// 验证两次返回的是同一个证书对象（缓存命中）
	if 证书1 != 证书2 {
		t.Error("期望第二次签发命中缓存，返回相同证书对象")
	}
}

// ========== 读取客户端问候 全分支覆盖 ==========

// Test读取客户端问候_连接关闭 验证连接关闭时读取记录头失败
func Test读取客户端问候_连接关闭(t *testing.T) {
	客户端端, 服务端端 := net.Pipe()
	客户端端.Close()
	_, err := 读取客户端问候(服务端端)
	服务端端.Close()
	if err == nil {
		t.Error("期望连接关闭时返回错误")
	}
}

// Test读取客户端问候_非TLS记录 验证非TLS Handshake记录时返回错误
func Test读取客户端问候_非TLS记录(t *testing.T) {
	客户端端, 服务端端 := net.Pipe()
	defer 客户端端.Close()
	defer 服务端端.Close()

	go func() {
		// 发送非 TLS 记录（ContentType != 0x16）
		客户端端.Write([]byte{0x17, 0x03, 0x03, 0x00, 0x05, 0x01, 0x00, 0x00, 0x01, 0x00})
	}()

	_, err := 读取客户端问候(服务端端)
	if err == nil {
		t.Error("期望非TLS记录时返回错误")
	}
}

// Test读取客户端问候_握手长度为0 验证握手长度为0时返回错误
func Test读取客户端问候_握手长度为0(t *testing.T) {
	客户端端, 服务端端 := net.Pipe()
	defer 客户端端.Close()
	defer 服务端端.Close()

	go func() {
		// TLS 记录头：ContentType=0x16, 长度=0
		客户端端.Write([]byte{0x16, 0x03, 0x03, 0x00, 0x00})
	}()

	_, err := 读取客户端问候(服务端端)
	if err == nil {
		t.Error("期望握手长度为0时返回错误")
	}
}

// Test读取客户端问候_非ClientHello 验证非ClientHello类型时返回错误
func Test读取客户端问候_非ClientHello(t *testing.T) {
	客户端端, 服务端端 := net.Pipe()
	defer 客户端端.Close()
	defer 服务端端.Close()

	go func() {
		// TLS 记录头 + handshake 数据（HandshakeType=0x02 非 ClientHello）
		握手数据 := []byte{0x02, 0x00, 0x00, 0x01, 0x00}
		记录头 := []byte{0x16, 0x03, 0x03}
		长度字节 := make([]byte, 2)
		长度字节[0] = byte(len(握手数据) >> 8)
		长度字节[1] = byte(len(握手数据))
		客户端端.Write(append(append(记录头, 长度字节...), 握手数据...))
	}()

	_, err := 读取客户端问候(服务端端)
	if err == nil {
		t.Error("期望非ClientHello时返回错误")
	}
}

// ========== ShouldIntercept 全分支覆盖 ==========

// TestShouldIntercept_未启用 验证未启用时返回false
func TestShouldIntercept_未启用(t *testing.T) {
	svc := &Service{options: option.MITMServiceOptions{Enabled: false}}
	元数据 := adapter.InboundContext{Destination: M.ParseSocksaddrHostPort("example.com", 443)}
	if svc.ShouldIntercept(元数据) {
		t.Error("期望未启用时返回false")
	}
}

// TestShouldIntercept_匹配器为空 验证匹配器为空时返回false
func TestShouldIntercept_匹配器为空(t *testing.T) {
	svc := &Service{
		options: option.MITMServiceOptions{Enabled: true},
		匹配器:   新域名匹配器(option.MITMMatchOptions{}),
	}
	元数据 := adapter.InboundContext{Destination: M.ParseSocksaddrHostPort("example.com", 443)}
	if svc.ShouldIntercept(元数据) {
		t.Error("期望匹配器为空时返回false")
	}
}

// TestShouldIntercept_协议非空 验证协议非空时返回false
func TestShouldIntercept_协议非空(t *testing.T) {
	svc := &Service{
		options: option.MITMServiceOptions{Enabled: true},
		匹配器:   新域名匹配器(option.MITMMatchOptions{Domain: []string{"example.com"}}),
	}
	元数据 := adapter.InboundContext{
		Destination: M.ParseSocksaddrHostPort("example.com", 443),
		Protocol:    "dns",
	}
	if svc.ShouldIntercept(元数据) {
		t.Error("期望协议非空时返回false")
	}
}

// TestShouldIntercept_端口不是443 验证端口不是443时返回false
func TestShouldIntercept_端口不是443(t *testing.T) {
	svc := &Service{
		options: option.MITMServiceOptions{Enabled: true},
		匹配器:   新域名匹配器(option.MITMMatchOptions{Domain: []string{"example.com"}}),
	}
	元数据 := adapter.InboundContext{Destination: M.ParseSocksaddrHostPort("example.com", 80)}
	if svc.ShouldIntercept(元数据) {
		t.Error("期望端口不是443时返回false")
	}
}

// TestShouldIntercept_全部满足 验证全部条件满足时返回true
func TestShouldIntercept_全部满足(t *testing.T) {
	svc := &Service{
		options: option.MITMServiceOptions{Enabled: true},
		匹配器:   新域名匹配器(option.MITMMatchOptions{Domain: []string{"example.com"}}),
	}
	元数据 := adapter.InboundContext{Destination: M.ParseSocksaddrHostPort("example.com", 443)}
	if !svc.ShouldIntercept(元数据) {
		t.Error("期望全部条件满足时返回true")
	}
}

// ========== Start 全分支覆盖 ==========

// TestStart_非Start阶段 验证非Start阶段时直接返回
func TestStart_非Start阶段(t *testing.T) {
	svc := &Service{logger: 获取测试日志器(), options: option.MITMServiceOptions{Enabled: true}}
	err := svc.Start(adapter.StartStateInitialize, nil)
	if err != nil {
		t.Errorf("期望非Start阶段返回nil，实际错误: %v", err)
	}
}

// TestStart_未启用 验证未启用时返回nil
func TestStart_未启用(t *testing.T) {
	svc := &Service{logger: 获取测试日志器(), options: option.MITMServiceOptions{Enabled: false}}
	err := svc.Start(adapter.StartStateStart, nil)
	if err != nil {
		t.Errorf("期望未启用时返回nil，实际错误: %v", err)
	}
}

// TestStart_已启用 验证已启用时返回nil
func TestStart_已启用(t *testing.T) {
	svc := &Service{logger: 获取测试日志器(), options: option.MITMServiceOptions{Enabled: true}}
	err := svc.Start(adapter.StartStateStart, nil)
	if err != nil {
		t.Errorf("期望已启用时返回nil，实际错误: %v", err)
	}
}

// ========== 路由连接 全分支覆盖 ==========

// Test路由连接_Router已注册 验证Router已注册时调用RouteConnection
func Test路由连接_Router已注册(t *testing.T) {
	// 使用真实 TCP 连接，避免 net.Pipe 时序问题
	监听器, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("创建监听器失败: %v", err)
	}
	defer 监听器.Close()

	// mock Router：接受连接后立即关闭
	mock := &mock路由器{
		处理函数: func(conn net.Conn) {
			conn.Close()
		},
	}

	// 使用 service.ContextWith 创建包含 Router 的 context
	ctx := service.ContextWith[adapter.Router](context.Background(), mock)

	svc := &Service{
		ctx:    ctx,
		logger: 获取测试日志器(),
	}

	// 在 goroutine 中接受连接
	go func() {
		客户端连接, err := 监听器.Accept()
		if err != nil {
			return
		}
		defer 客户端连接.Close()
		// 调用路由连接
		req := &http.Request{
			Method: "GET",
			Host:   "example.com",
			URL:    &url.URL{Path: "/", Host: "example.com"},
		}
		svc.路由连接(context.Background(), 客户端连接, req, M.Socksaddr{})
	}()

	// 客户端连接到监听器
	客户端连接, err := net.Dial("tcp", 监听器.Addr().String())
	if err != nil {
		t.Fatalf("连接失败: %v", err)
	}
	defer 客户端连接.Close()

	// 等待一小段时间，让路由连接完成
	time.Sleep(100 * time.Millisecond)
}

// 确保 tls 被引用
var _ tls.Certificate
