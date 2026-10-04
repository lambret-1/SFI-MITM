package mitm

import (
	"crypto/tls"
	"testing"

	"github.com/sagernet/sing-box/option"
)

// Test缓存_清空 验证证书缓存清空功能
func Test缓存_清空(t *testing.T) {
	缓存 := 新证书缓存(10)
	缓存.存入("example.com", &tls.Certificate{})
	缓存.存入("test.org", &tls.Certificate{})
	if 缓存.数量() != 2 {
		t.Fatalf("期望存入后数量为 2，实际 %d", 缓存.数量())
	}
	缓存.清空()
	if 缓存.数量() != 0 {
		t.Errorf("清空后期望数量为 0，实际 %d", 缓存.数量())
	}
	// 清空后再次获取应返回 nil
	if 证书, 命中 := 缓存.获取("example.com"); 命中 || 证书 != nil {
		t.Error("清空后期望获取不到证书")
	}
}

// Test匹配器_是否为空 验证域名匹配器空判断
func Test匹配器_是否为空(t *testing.T) {
	// 空匹配器
	空匹配器 := 新域名匹配器(option.MITMMatchOptions{})
	if !空匹配器.是否为空() {
		t.Error("期望空匹配器的是否为空返回 true")
	}
	// 非空匹配器（精确匹配）
	精确匹配器 := 新域名匹配器(option.MITMMatchOptions{Domain: []string{"example.com"}})
	if 精确匹配器.是否为空() {
		t.Error("期望含精确规则的匹配器的是否为空返回 false")
	}
	// 非空匹配器（后缀匹配）
	后缀匹配器 := 新域名匹配器(option.MITMMatchOptions{DomainSuffix: []string{"example.com"}})
	if 后缀匹配器.是否为空() {
		t.Error("期望含后缀规则的匹配器的是否为空返回 false")
	}
	// nil 匹配器
	var nil匹配器 *域名匹配器
	if !nil匹配器.是否为空() {
		t.Error("期望 nil 匹配器的是否为空返回 true")
	}
}

// Test协商ALPN_h2优先 验证 ALPN 协商优先选择 h2
func Test协商ALPN_h2优先(t *testing.T) {
	结果 := 协商ALPN([]string{"http/1.1", "h2"})
	if 结果 != "h2" {
		t.Errorf("期望协商结果为 h2，实际 %s", 结果)
	}
}

// Test协商ALPN_仅http1 验证仅支持 http/1.1 时协商结果
func Test协商ALPN_仅http1(t *testing.T) {
	结果 := 协商ALPN([]string{"http/1.1"})
	if 结果 != "http/1.1" {
		t.Errorf("期望协商结果为 http/1.1，实际 %s", 结果)
	}
}

// Test协商ALPN_空列表默认http1 验证空 ALPN 列表默认回退 http/1.1
func Test协商ALPN_空列表默认http1(t *testing.T) {
	结果 := 协商ALPN([]string{})
	if 结果 != "http/1.1" {
		t.Errorf("期望空列表默认回退 http/1.1，实际 %s", 结果)
	}
}

// Test协商ALPN_未知协议默认http1 验证未知协议默认回退 http/1.1
func Test协商ALPN_未知协议默认http1(t *testing.T) {
	结果 := 协商ALPN([]string{"unknown/proto", "h3"})
	if 结果 != "http/1.1" {
		t.Errorf("期望未知协议默认回退 http/1.1，实际 %s", 结果)
	}
}

// TestLastIndexByte_找到字符 验证 lastIndexByte 找到字符
func TestLastIndexByte_找到字符(t *testing.T) {
	结果 := lastIndexByte("example.com:443", ':')
	if 结果 != 11 {
		t.Errorf("期望返回 11，实际 %d", 结果)
	}
}

// TestLastIndexByte_最后一次出现 验证 lastIndexByte 返回最后一次出现位置
func TestLastIndexByte_最后一次出现(t *testing.T) {
	结果 := lastIndexByte("a:b:c:d", ':')
	if 结果 != 5 {
		t.Errorf("期望返回 5（最后一个冒号位置），实际 %d", 结果)
	}
}

// TestLastIndexByte_未找到 验证 lastIndexByte 未找到字符返回 -1
func TestLastIndexByte_未找到(t *testing.T) {
	结果 := lastIndexByte("example.com", ':')
	if 结果 != -1 {
		t.Errorf("期望未找到返回 -1，实际 %d", 结果)
	}
}

// TestLastIndexByte_空字符串 验证 lastIndexByte 空字符串返回 -1
func TestLastIndexByte_空字符串(t *testing.T) {
	结果 := lastIndexByte("", ':')
	if 结果 != -1 {
		t.Errorf("期望空字符串返回 -1，实际 %d", 结果)
	}
}
