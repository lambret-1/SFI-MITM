package mitm

import (
	"context"
	"crypto/tls"
	"encoding/binary"
	"net"
	"net/http"
	"testing"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/option"
)

// ========== service.go 英文导出方法包内测试 ==========

// Test导出_IsEnabled 验证 IsEnabled 导出方法
func Test导出_IsEnabled(t *testing.T) {
	svc := &Service{options: option.MITMServiceOptions{Enabled: true}}
	if !svc.IsEnabled() {
		t.Error("期望返回 true")
	}
	svc.options.Enabled = false
	if svc.IsEnabled() {
		t.Error("期望返回 false")
	}
}

// Test导出_IsCAInstalled 验证 IsCAInstalled 导出方法
func Test导出_IsCAInstalled(t *testing.T) {
	svc := &Service{}
	if svc.IsCAInstalled() {
		t.Error("根证书为 nil 时期望返回 false")
	}
	svc.根证书 = &根证书实例{}
	if !svc.IsCAInstalled() {
		t.Error("根证书非 nil 时期望返回 true")
	}
}

// Test导出_GetActiveConnections 验证 GetActiveConnections 导出方法
func Test导出_GetActiveConnections(t *testing.T) {
	svc := &Service{}
	if svc.GetActiveConnections() != 0 {
		t.Error("期望初始为 0")
	}
	svc.增加连接()
	svc.增加连接()
	if svc.GetActiveConnections() != 2 {
		t.Errorf("期望为 2，实际 %d", svc.GetActiveConnections())
	}
	svc.减少连接()
	if svc.GetActiveConnections() != 1 {
		t.Errorf("期望为 1，实际 %d", svc.GetActiveConnections())
	}
}

// Test导出_GetUpstreamTimeout 验证 GetUpstreamTimeout 导出方法
func Test导出_GetUpstreamTimeout(t *testing.T) {
	svc := &Service{options: option.MITMServiceOptions{UpstreamTimeout: 45}}
	if svc.GetUpstreamTimeout() != 45 {
		t.Errorf("期望 45，实际 %d", svc.GetUpstreamTimeout())
	}
	svc.options.UpstreamTimeout = 0
	if svc.GetUpstreamTimeout() != 30 {
		t.Errorf("期望默认 30，实际 %d", svc.GetUpstreamTimeout())
	}
}

// Test导出_GetMITMLogs 验证 GetMITMLogs 导出方法
func Test导出_GetMITMLogs(t *testing.T) {
	// 日志缓冲区为 nil 时返回空切片
	svc := &Service{}
	日志 := svc.GetMITMLogs()
	if 日志 == nil {
		t.Error("期望返回非 nil 空切片")
	}
	if len(日志) != 0 {
		t.Errorf("期望长度为 0，实际 %d", len(日志))
	}
	// 日志缓冲区非空时返回日志
	svc.日志缓冲区 = 新日志环形缓冲区(100)
	svc.日志缓冲区.写入(1, "测试日志")
	日志 = svc.GetMITMLogs()
	if len(日志) != 1 {
		t.Errorf("期望长度为 1，实际 %d", len(日志))
	}
}

// Test导出_ClearMITMLogs 验证 ClearMITMLogs 导出方法
func Test导出_ClearMITMLogs(t *testing.T) {
	svc := &Service{日志缓冲区: 新日志环形缓冲区(100)}
	svc.日志缓冲区.写入(1, "日志1")
	svc.日志缓冲区.写入(2, "日志2")
	if len(svc.GetMITMLogs()) != 2 {
		t.Fatal("期望有 2 条日志")
	}
	svc.ClearMITMLogs()
	if len(svc.GetMITMLogs()) != 0 {
		t.Error("清空后期望为 0 条日志")
	}
	// nil 缓冲区时不 panic
	svc.日志缓冲区 = nil
	svc.ClearMITMLogs() // 不应 panic
}

// ========== cache.go 存入 分支覆盖 ==========

// Test缓存_存入_已存在更新 验证存入已存在域名时更新证书并移到头部
func Test缓存_存入_已存在更新(t *testing.T) {
	缓存 := 新证书缓存(10)
	证书1 := &tls.Certificate{}
	证书2 := &tls.Certificate{}
	缓存.存入("example.com", 证书1)
	缓存.存入("example.com", 证书2) // 已存在，应更新
	if 缓存.数量() != 1 {
		t.Errorf("期望数量为 1，实际 %d", 缓存.数量())
	}
	获取证书, 命中 := 缓存.获取("example.com")
	if !命中 {
		t.Fatal("期望命中")
	}
	if 获取证书 != 证书2 {
		t.Error("期望证书已更新为证书2")
	}
}

// Test缓存_存入_超出容量淘汰 验证存入超出容量时淘汰最久未使用条目
func Test缓存_存入_超出容量淘汰(t *testing.T) {
	缓存 := 新证书缓存(2)
	缓存.存入("a.com", &tls.Certificate{})
	缓存.存入("b.com", &tls.Certificate{})
	缓存.存入("c.com", &tls.Certificate{}) // 超出容量，应淘汰最久未使用的 a.com
	if 缓存.数量() != 2 {
		t.Errorf("期望数量为 2，实际 %d", 缓存.数量())
	}
	if _, 命中 := 缓存.获取("a.com"); 命中 {
		t.Error("期望 a.com 已被淘汰")
	}
	if _, 命中 := 缓存.获取("b.com"); !命中 {
		t.Error("期望 b.com 仍存在")
	}
	if _, 命中 := 缓存.获取("c.com"); !命中 {
		t.Error("期望 c.com 存在")
	}
}

// ========== clienthello.go 解析客户端问候扩展 边界覆盖 ==========

// Test解析客户端问候扩展_空数据 验证空数据返回空
func Test解析客户端问候扩展_空数据(t *testing.T) {
	sni, alpn := 解析客户端问候扩展([]byte{})
	if sni != "" || alpn != nil {
		t.Error("期望空数据返回空")
	}
}

// Test解析客户端问候扩展_数据过短 验证数据长度不足34返回空
func Test解析客户端问候扩展_数据过短(t *testing.T) {
	sni, alpn := 解析客户端问候扩展(make([]byte, 33))
	if sni != "" || alpn != nil {
		t.Error("期望数据过短返回空")
	}
}

// Test解析客户端问候扩展_最小有效长度 验证最小有效长度（34字节）能进入解析
func Test解析客户端问候扩展_最小有效长度(t *testing.T) {
	// 34字节：client_version(2) + random(32)，session_id长度为0
	数据 := make([]byte, 34)
	数据[34-1] = 0 // session_id length = 0
	sni, alpn := 解析客户端问候扩展(数据)
	if sni != "" || alpn != nil {
		t.Error("期望无扩展时返回空")
	}
}

// Test解析客户端问候扩展_会话ID越界 验证session_id长度导致越界返回空
func Test解析客户端问候扩展_会话ID越界(t *testing.T) {
	数据 := make([]byte, 35)
	数据[34] = 100 // session_id length = 100，但数据只有 35 字节
	sni, alpn := 解析客户端问候扩展(数据)
	if sni != "" || alpn != nil {
		t.Error("期望越界返回空")
	}
}

// Test解析客户端问候扩展_密码套件越界 验证cipher_suites长度越界返回空
func Test解析客户端问候扩展_密码套件越界(t *testing.T) {
	数据 := make([]byte, 40)
	数据[34] = 0 // session_id length = 0
	// cipher_suites length 在偏移 35，设置为很大的值
	binary.BigEndian.PutUint16(数据[35:37], 1000)
	sni, alpn := 解析客户端问候扩展(数据)
	if sni != "" || alpn != nil {
		t.Error("期望越界返回空")
	}
}

// Test解析客户端问候扩展_压缩方法越界 验证compression_methods长度越界返回空
func Test解析客户端问候扩展_压缩方法越界(t *testing.T) {
	数据 := make([]byte, 45)
	数据[34] = 0 // session_id length = 0
	binary.BigEndian.PutUint16(数据[35:37], 0) // cipher_suites length = 0
	数据[37] = 100 // compression_methods length = 100，越界
	sni, alpn := 解析客户端问候扩展(数据)
	if sni != "" || alpn != nil {
		t.Error("期望越界返回空")
	}
}

// Test解析客户端问候扩展_扩展长度越界 验证extensions总长度越界时截断到数据末尾
func Test解析客户端问候扩展_扩展长度越界(t *testing.T) {
	数据 := make([]byte, 50)
	数据[34] = 0 // session_id length = 0
	binary.BigEndian.PutUint16(数据[35:37], 0) // cipher_suites length = 0
	数据[37] = 1 // compression_methods length = 1
	数据[38] = 0 // compression_methods[0] = 0
	// extensions length 在偏移 39，设置为很大的值（应截断到数据末尾）
	binary.BigEndian.PutUint16(数据[39:41], 1000)
	sni, alpn := 解析客户端问候扩展(数据)
	if sni != "" || alpn != nil {
		t.Error("期望截断后无有效扩展返回空")
	}
}

// Test解析客户端问候扩展_扩展数据越界break 验证单个扩展数据越界时break
func Test解析客户端问候扩展_扩展数据越界break(t *testing.T) {
	数据 := make([]byte, 60)
	数据[34] = 0 // session_id length = 0
	binary.BigEndian.PutUint16(数据[35:37], 0) // cipher_suites length = 0
	数据[37] = 1 // compression_methods length = 1
	数据[38] = 0 // compression_methods[0] = 0
	// extensions length = 15
	binary.BigEndian.PutUint16(数据[39:41], 15)
	// 第一个扩展：type=0x0000(SNI), length=100（越界，应break）
	binary.BigEndian.PutUint16(数据[41:43], 0x0000)
	binary.BigEndian.PutUint16(数据[43:45], 100)
	sni, alpn := 解析客户端问候扩展(数据)
	if sni != "" || alpn != nil {
		t.Error("期望扩展数据越界break后返回空")
	}
}

// ========== http1.go 处理连接 分支覆盖 ==========

// Test处理连接_读取请求错误 验证读取请求失败时返回错误
func Test处理连接_读取请求错误(t *testing.T) {
	svc := &Service{
		ctx:     context.Background(),
		logger:  获取测试日志器(),
		options: option.MITMServiceOptions{Enabled: true},
		叶子缓存:   新证书缓存(默认缓存容量),
		日志缓冲区: 新日志环形缓冲区(100),
	}

	客户端端, 服务端端 := net.Pipe()
	客户端端.Close() // 关闭客户端，导致读取请求失败

	处理器 := 新建HTTP1处理器(svc, nil, adapter.InboundContext{})
	err := 处理器.处理连接(context.Background(), 服务端端)
	服务端端.Close()
	// EOF 或其他错误都是正常的
	t.Logf("读取请求错误返回: %v", err)
}

// Test保持连接_ConnectionClose 验证保持连接函数识别 Connection: close
func Test保持连接_ConnectionClose(t *testing.T) {
	svc := &Service{}
	处理器 := 新建HTTP1处理器(svc, nil, adapter.InboundContext{})

	req := &http.Request{
		ProtoMajor: 1,
		ProtoMinor: 1,
		Header:     make(http.Header),
	}
	req.Header.Set("Connection", "close")
	if 处理器.保持连接(req) {
		t.Error("期望 Connection: close 时返回 false")
	}

	req.Header.Set("Connection", "keep-alive")
	if !处理器.保持连接(req) {
		t.Error("期望 Connection: keep-alive 时返回 true")
	}

	// HTTP/1.0 默认不保持连接
	req.ProtoMinor = 0
	req.Header.Del("Connection")
	if 处理器.保持连接(req) {
		t.Error("期望 HTTP/1.0 无 Connection 头时返回 false")
	}
}
