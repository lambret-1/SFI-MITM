package main

import (
	"testing"

	"github.com/sagernet/sing-box/adapter"
	M "github.com/sagernet/sing/common/metadata"
)

// TestRouter_服务创建后可拦截 验证服务创建后 443 端口连接可进入拦截流程
// 注意：ShouldIntercept 仅做端口级预过滤，域名匹配在 Intercept 中通过 SNI 完成
func TestRouter_服务创建后可拦截(t *testing.T) {
	服务 := 创建测试服务(t)
	元数据 := adapter.InboundContext{}
	元数据.Destination = M.ParseSocksaddrHostPort("example.com", 443)
	if !服务.ShouldIntercept(元数据) {
		t.Error("期望 example.com:443 进入拦截流程")
	}
}

// TestRouter_任意域名443端口均预过滤通过 验证 ShouldIntercept 不对域名做匹配
// 所有 443 端口连接均通过预过滤，实际域名匹配在读取 TLS ClientHello SNI 后进行
func TestRouter_任意域名443端口均预过滤通过(t *testing.T) {
	服务 := 创建测试服务(t)
	测试域名 := []string{"google.com", "example.com", "any.domain.org", "test.com"}
	for _, 域名 := range 测试域名 {
		元数据 := adapter.InboundContext{}
		元数据.Destination = M.ParseSocksaddrHostPort(域名, 443)
		if !服务.ShouldIntercept(元数据) {
			t.Errorf("期望 %s:443 通过预过滤（域名匹配在 SNI 阶段完成）", 域名)
		}
	}
}

// TestRouter_非443端口不拦截 验证非 443 端口不进入拦截流程
func TestRouter_非443端口不拦截(t *testing.T) {
	服务 := 创建测试服务(t)
	测试端口 := []uint16{80, 8080, 22, 4433, 0}
	for _, 端口 := range 测试端口 {
		元数据 := adapter.InboundContext{}
		元数据.Destination = M.ParseSocksaddrHostPort("example.com", 端口)
		if 服务.ShouldIntercept(元数据) {
			t.Errorf("期望 example.com:%d 不进入拦截流程", 端口)
		}
	}
}

// TestRouter_Protocol非空不拦截 验证 Protocol 字段非空时不拦截（DNS 等特殊协议）
func TestRouter_Protocol非空不拦截(t *testing.T) {
	服务 := 创建测试服务(t)
	元数据 := adapter.InboundContext{}
	元数据.Destination = M.ParseSocksaddrHostPort("example.com", 443)
	元数据.Protocol = "dns"
	if 服务.ShouldIntercept(元数据) {
		t.Error("期望 Protocol 非空时不拦截")
	}
}

// TestRouter_未启用服务不拦截 验证未启用服务不拦截任何流量
func TestRouter_未启用服务不拦截(t *testing.T) {
	服务 := 创建未启用服务(t)
	元数据 := adapter.InboundContext{}
	元数据.Destination = M.ParseSocksaddrHostPort("example.com", 443)
	if 服务.ShouldIntercept(元数据) {
		t.Error("未启用时期望不拦截任何流量")
	}
}

// TestRouter_活跃连接计数 验证活跃连接计数初始为零
func TestRouter_活跃连接计数(t *testing.T) {
	服务 := 创建测试服务(t)
	if 服务.GetActiveConnections() != 0 {
		t.Errorf("期望初始活跃连接数为 0，实际 %d", 服务.GetActiveConnections())
	}
}
