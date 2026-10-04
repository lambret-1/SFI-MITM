package main

import (
	"testing"

	"github.com/sagernet/sing-box/adapter"
	M "github.com/sagernet/sing/common/metadata"
)

// Test证书_服务创建后CA可用 验证服务创建后 CA 证书已加载
func Test证书_服务创建后CA可用(t *testing.T) {
	服务 := 创建测试服务(t)
	if !服务.IsCAInstalled() {
		t.Error("期望服务创建后 CA 证书可用")
	}
}

// Test证书_未启用服务无CA 验证未启用服务不加载 CA
func Test证书_未启用服务无CA(t *testing.T) {
	服务 := 创建未启用服务(t)
	if 服务.IsCAInstalled() {
		t.Error("未启用时期望 CA 不可用")
	}
}

// Test证书_活跃连接初始为零 验证服务创建后活跃连接数为零
func Test证书_活跃连接初始为零(t *testing.T) {
	服务 := 创建测试服务(t)
	if 服务.GetActiveConnections() != 0 {
		t.Errorf("期望初始活跃连接数为 0，实际 %d", 服务.GetActiveConnections())
	}
}

// Test证书_ShouldIntercept端口预过滤 验证 ShouldIntercept 仅做 443 端口预过滤
// 域名匹配在 Intercept 中通过读取 TLS ClientHello SNI 完成，不在此阶段判断
func Test证书_ShouldIntercept端口预过滤(t *testing.T) {
	服务 := 创建测试服务(t)
	测试用例 := []struct {
		域名     string
		端口     uint16
		期望拦截 bool
	}{
		{"example.com", 443, true},
		{"api.example.com", 443, true},
		{"google.com", 443, true}, // 预过滤阶段不判断域名
		{"test.org", 80, false},
		{"example.com", 8080, false},
	}
	for _, 用例 := range 测试用例 {
		元数据 := adapter.InboundContext{}
		元数据.Destination = M.ParseSocksaddrHostPort(用例.域名, 用例.端口)
		结果 := 服务.ShouldIntercept(元数据)
		if 结果 != 用例.期望拦截 {
			t.Errorf("域名 %s 端口 %d: 期望拦截=%v，实际=%v", 用例.域名, 用例.端口, 用例.期望拦截, 结果)
		}
	}
}

// Test证书_未启用服务不拦截 验证未启用服务不拦截任何连接
func Test证书_未启用服务不拦截(t *testing.T) {
	服务 := 创建未启用服务(t)
	元数据 := adapter.InboundContext{}
	元数据.Destination = M.ParseSocksaddrHostPort("example.com", 443)
	if 服务.ShouldIntercept(元数据) {
		t.Error("未启用时期望不拦截任何连接")
	}
}

// Test证书_非443端口不拦截 验证非 443 端口不拦截
func Test证书_非443端口不拦截(t *testing.T) {
	服务 := 创建测试服务(t)
	元数据 := adapter.InboundContext{}
	元数据.Destination = M.ParseSocksaddrHostPort("example.com", 80)
	if 服务.ShouldIntercept(元数据) {
		t.Error("期望非 443 端口不拦截")
	}
}
