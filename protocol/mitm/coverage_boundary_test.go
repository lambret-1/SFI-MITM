package mitm

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/binary"
	"errors"
	"net"
	"net/http"
	"testing"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/option"
)

// ========== 新证书缓存 边界覆盖 ==========

// Test新证书缓存_容量为0使用默认 验证容量为0时使用默认容量
func Test新证书缓存_容量为0使用默认(t *testing.T) {
	缓存 := 新证书缓存(0)
	缓存.存入("example.com", &tls.Certificate{})
	if _, 命中 := 缓存.获取("example.com"); !命中 {
		t.Error("容量为0时使用默认容量，期望命中")
	}
}

// Test新证书缓存_容量为负数使用默认 验证容量为负数时使用默认容量
func Test新证书缓存_容量为负数使用默认(t *testing.T) {
	缓存 := 新证书缓存(-1)
	缓存.存入("example.com", &tls.Certificate{})
	if _, 命中 := 缓存.获取("example.com"); !命中 {
		t.Error("容量为负数时使用默认容量，期望命中")
	}
}

// ========== 新日志环形缓冲区 边界覆盖 ==========

// Test新日志环形缓冲区_容量为0使用默认 验证容量为0时使用默认容量
func Test新日志环形缓冲区_容量为0使用默认(t *testing.T) {
	缓冲区 := 新日志环形缓冲区(0)
	缓冲区.写入(1, "测试")
	日志列表 := 缓冲区.获取全部()
	if len(日志列表) != 1 {
		t.Errorf("容量为0时使用默认容量，期望 1 条，实际 %d 条", len(日志列表))
	}
}

// Test新日志环形缓冲区_容量为负数使用默认 验证容量为负数时使用默认容量
func Test新日志环形缓冲区_容量为负数使用默认(t *testing.T) {
	缓冲区 := 新日志环形缓冲区(-1)
	缓冲区.写入(1, "测试")
	日志列表 := 缓冲区.获取全部()
	if len(日志列表) != 1 {
		t.Errorf("容量为负数时使用默认容量，期望 1 条，实际 %d 条", len(日志列表))
	}
}

// ========== 解析SNI扩展 边界覆盖 ==========

// Test解析SNI扩展_空数据 验证空数据返回空
func Test解析SNI扩展_空数据(t *testing.T) {
	if 解析SNI扩展([]byte{}) != "" {
		t.Error("期望空数据返回空")
	}
}

// Test解析SNI扩展_数据过短 验证数据长度不足返回空
func Test解析SNI扩展_数据过短(t *testing.T) {
	if 解析SNI扩展(make([]byte, 2)) != "" {
		t.Error("期望数据过短返回空")
	}
}

// Test解析SNI扩展_列表长度越界 验证server_name列表长度越界返回空
func Test解析SNI扩展_列表长度越界(t *testing.T) {
	数据 := make([]byte, 5)
	binary.BigEndian.PutUint16(数据[0:2], 100) // 列表长度 = 100，越界
	if 解析SNI扩展(数据) != "" {
		t.Error("期望列表长度越界返回空")
	}
}

// Test解析SNI扩展_直接解析第一个名称 验证不检查name_type直接解析第一个名称
func Test解析SNI扩展_直接解析第一个名称(t *testing.T) {
	域名 := "example.com"
	域名长度 := len(域名)
	// 列表长度 = 3(类型+长度) + 域名长度
	列表长度 := 3 + 域名长度
	数据 := make([]byte, 2+列表长度)
	binary.BigEndian.PutUint16(数据[0:2], uint16(列表长度))
	数据[2] = 0x01 // 类型 = 非 host_name（但函数不检查，直接解析）
	binary.BigEndian.PutUint16(数据[3:5], uint16(域名长度))
	copy(数据[5:], 域名)
	if 解析SNI扩展(数据) != 域名 {
		t.Errorf("期望直接解析为 %q，实际 %q", 域名, 解析SNI扩展(数据))
	}
}

// Test解析SNI扩展_正常解析 验证正常SNI扩展解析
func Test解析SNI扩展_正常解析(t *testing.T) {
	域名 := "example.com"
	域名长度 := len(域名)
	// 列表长度 = 3(类型+长度) + 域名长度
	列表长度 := 3 + 域名长度
	数据 := make([]byte, 2+列表长度)
	binary.BigEndian.PutUint16(数据[0:2], uint16(列表长度))
	数据[2] = 0x00 // 类型 = host_name
	binary.BigEndian.PutUint16(数据[3:5], uint16(域名长度))
	copy(数据[5:], 域名)
	if 解析SNI扩展(数据) != 域名 {
		t.Errorf("期望解析为 %q，实际 %q", 域名, 解析SNI扩展(数据))
	}
}

// ========== 解析ALPN扩展 边界覆盖 ==========

// Test解析ALPN扩展_空数据 验证空数据返回nil
func Test解析ALPN扩展_空数据(t *testing.T) {
	if 解析ALPN扩展([]byte{}) != nil {
		t.Error("期望空数据返回nil")
	}
}

// Test解析ALPN扩展_数据过短 验证数据长度不足返回nil
func Test解析ALPN扩展_数据过短(t *testing.T) {
	if 解析ALPN扩展(make([]byte, 1)) != nil {
		t.Error("期望数据过短返回nil")
	}
}

// Test解析ALPN扩展_总长度越界截断 验证ALPN总长度越界时截断到数据末尾
func Test解析ALPN扩展_总长度越界截断(t *testing.T) {
	// 总长度 = 100（越界），但实际只有 5 字节数据
	// 函数会截断到数据末尾，然后尝试解析（可能因数据不足而break）
	数据 := []byte{0x00, 0x64, 0x02, 'h', '2'}
	结果 := 解析ALPN扩展(数据)
	// 截断后结束位置 = len(数据) = 5
	// 偏移 = 2，协议长度 = 数据[2] = 2，偏移+协议长度 = 4 <= 5，应解析成功
	if len(结果) != 1 || 结果[0] != "h2" {
		t.Errorf("期望截断后解析出 ['h2']，实际 %v", 结果)
	}
}

// Test解析ALPN扩展_协议长度越界 验证单个协议长度越界返回nil
func Test解析ALPN扩展_协议长度越界(t *testing.T) {
	// 总长度 = 5
	数据 := []byte{0x00, 0x05, 0x03, 'h', '2'}
	// 协议长度 = 3，但只有 2 字节数据，越界
	if 解析ALPN扩展(数据) != nil {
		t.Error("期望协议长度越界返回nil")
	}
}

// Test解析ALPN扩展_正常解析 验证正常ALPN扩展解析
func Test解析ALPN扩展_正常解析(t *testing.T) {
	协议列表 := []string{"h2", "http/1.1"}
	总长度 := 0
	for _, 协议 := range 协议列表 {
		总长度 += 1 + len(协议)
	}
	数据 := make([]byte, 2+总长度)
	binary.BigEndian.PutUint16(数据[0:2], uint16(总长度))
	偏移 := 2
	for _, 协议 := range 协议列表 {
		数据[偏移] = byte(len(协议))
		偏移++
		copy(数据[偏移:], 协议)
		偏移 += len(协议)
	}
	结果 := 解析ALPN扩展(数据)
	if len(结果) != len(协议列表) {
		t.Fatalf("期望 %d 个协议，实际 %d", len(协议列表), len(结果))
	}
	for i, 协议 := range 协议列表 {
		if 结果[i] != 协议 {
			t.Errorf("期望第 %d 个协议为 %q，实际 %q", i, 协议, 结果[i])
		}
	}
}

// ========== 发送错误响应 写入错误覆盖 ==========

// 错误写入器 模拟写入失败的 Writer
type 错误写入器 struct {
	错误 error
}

func (w *错误写入器) Write(p []byte) (n int, err error) {
	return 0, w.错误
}

// Test发送错误响应_写入失败 验证写入失败时返回错误
func Test发送错误响应_写入失败(t *testing.T) {
	svc := &Service{}
	处理器 := 新建HTTP1处理器(svc, nil, adapter.InboundContext{})
	底层写入器 := &错误写入器{错误: errors.New("模拟写入失败")}
	写入器 := bufio.NewWriter(底层写入器)

	err := 处理器.发送错误响应(写入器, http.StatusBadRequest, "测试")
	if err == nil {
		t.Error("期望写入失败返回错误")
	}
}

// ========== 匹配 分支覆盖 ==========

// Test匹配_空匹配器 验证空匹配器不匹配任何域名
func Test匹配_空匹配器(t *testing.T) {
	matcher := 新域名匹配器(option.MITMMatchOptions{})
	if matcher.匹配("example.com") {
		t.Error("期望空匹配器不匹配任何域名")
	}
}

// Test匹配_nil匹配器 验证nil匹配器不匹配任何域名
func Test匹配_nil匹配器(t *testing.T) {
	var matcher *域名匹配器
	if matcher.匹配("example.com") {
		t.Error("期望nil匹配器不匹配任何域名")
	}
}

// Test匹配_精确匹配 验证精确域名匹配
func Test匹配_精确匹配(t *testing.T) {
	matcher := 新域名匹配器(option.MITMMatchOptions{
		Domain: []string{"example.com", "test.org"},
	})
	if !matcher.匹配("example.com") {
		t.Error("期望精确匹配 example.com")
	}
	if !matcher.匹配("test.org") {
		t.Error("期望精确匹配 test.org")
	}
	if matcher.匹配("other.com") {
		t.Error("期望不匹配 other.com")
	}
}

// Test匹配_后缀匹配 验证后缀域名匹配
func Test匹配_后缀匹配(t *testing.T) {
	matcher := 新域名匹配器(option.MITMMatchOptions{
		DomainSuffix: []string{"example.com"},
	})
	if !matcher.匹配("www.example.com") {
		t.Error("期望后缀匹配 www.example.com")
	}
	if !matcher.匹配("sub.example.com") {
		t.Error("期望后缀匹配 sub.example.com")
	}
	if !matcher.匹配("example.com") {
		t.Error("期望后缀匹配根域名 example.com")
	}
	if matcher.匹配("notexample.com") {
		t.Error("期望不匹配 notexample.com")
	}
}

// Test匹配_是否为空 验证匹配器是否为空
func Test匹配_是否为空(t *testing.T) {
	空匹配器 := 新域名匹配器(option.MITMMatchOptions{})
	if !空匹配器.是否为空() {
		t.Error("期望空匹配器是否为空返回 true")
	}

	非空匹配器 := 新域名匹配器(option.MITMMatchOptions{
		Domain: []string{"example.com"},
	})
	if 非空匹配器.是否为空() {
		t.Error("期望非空匹配器是否为空返回 false")
	}
}

// ========== 建立上游TLSWithContext 握手失败覆盖 ==========

// Test建立上游TLSWithContext_握手失败 验证TLS握手失败返回错误
func Test建立上游TLSWithContext_握手失败(t *testing.T) {
	客户端端, 服务端端 := net.Pipe()
	defer 客户端端.Close()

	// 服务端立即关闭，导致握手失败
	服务端端.Close()

	ctx := context.Background()
	_, err := 建立上游TLSWithContext(ctx, 客户端端, "example.com", []string{"http/1.1"})
	if err == nil {
		t.Error("期望握手失败返回错误")
	}
}

// 确保 bytes 被引用
var _ bytes.Buffer

// 确保 adapter 被引用
var _ adapter.InboundContext

// 确保 option 被引用
var _ option.MITMServiceOptions
