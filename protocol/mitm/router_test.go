package mitm

import (
	"context"
	"net/http"
	"net/netip"
	"strings"
	"testing"

	C "github.com/sagernet/sing-box/constant"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

// Test构造入站上下文_字段验证 验证根据 HTTPS 请求构造的入站上下文字段正确
func Test构造入站上下文_字段验证(t *testing.T) {
	req, err := http.NewRequest("GET", "https://api.example.com/path", nil)
	if err != nil {
		t.Fatalf("构造请求失败: %v", err)
	}

	源地址 := M.Socksaddr{
		Addr: netip.MustParseAddr("127.0.0.1"),
		Port: 12345,
	}

	入站上下文 := 构造入站上下文(req, 源地址)

	// 校验流量标记
	if 入站上下文.Inbound != "mitm" {
		t.Errorf("Inbound 应为 mitm，实际: %s", 入站上下文.Inbound)
	}
	if 入站上下文.InboundType != C.TypeMITM {
		t.Errorf("InboundType 应为 %s，实际: %s", C.TypeMITM, 入站上下文.InboundType)
	}
	if 入站上下文.Network != N.NetworkTCP {
		t.Errorf("Network 应为 %s，实际: %s", N.NetworkTCP, 入站上下文.Network)
	}

	// 校验源地址
	if 入站上下文.Source != 源地址 {
		t.Errorf("Source 应为 %v，实际: %v", 源地址, 入站上下文.Source)
	}

	// 校验目的地址（域名 + 固定 443 端口）
	if 入站上下文.Destination.Fqdn != "api.example.com" {
		t.Errorf("Destination.Fqdn 应为 api.example.com，实际: %s", 入站上下文.Destination.Fqdn)
	}
	if 入站上下文.Destination.Port != 443 {
		t.Errorf("Destination.Port 应为 443，实际: %d", 入站上下文.Destination.Port)
	}
}

// Test构造入站上下文_带端口Host 验证 URL Host 带端口时目的端口仍固定为 443
func Test构造入站上下文_带端口Host(t *testing.T) {
	req, err := http.NewRequest("GET", "https://api.example.com:8443/path", nil)
	if err != nil {
		t.Fatalf("构造请求失败: %v", err)
	}

	源地址 := M.Socksaddr{
		Addr: netip.MustParseAddr("127.0.0.1"),
		Port: 12345,
	}

	入站上下文 := 构造入站上下文(req, 源地址)

	// URL.Hostname() 会剥离端口，域名应正确提取
	if 入站上下文.Destination.Fqdn != "api.example.com" {
		t.Errorf("Destination.Fqdn 应为 api.example.com，实际: %s", 入站上下文.Destination.Fqdn)
	}
	// MITM 固定使用 443 端口，忽略 URL 中的 8443
	if 入站上下文.Destination.Port != 443 {
		t.Errorf("MITM 目的端口应固定为 443，实际: %d", 入站上下文.Destination.Port)
	}
}

// Test获取Router_未注册 验证上下文中未注册 Router 时返回 nil
func Test获取Router_未注册(t *testing.T) {
	svc := &Service{
		ctx:    context.Background(),
		logger: 获取测试日志器(),
	}

	if router := svc.获取Router(); router != nil {
		t.Error("上下文中未注册 Router 时应返回 nil")
	}
}

// Test路由连接_Router未注册 验证 Router 未注册时路由连接返回包含提示的错误
func Test路由连接_Router未注册(t *testing.T) {
	svc := &Service{
		ctx:    context.Background(),
		logger: 获取测试日志器(),
	}

	req, err := http.NewRequest("GET", "https://api.example.com/path", nil)
	if err != nil {
		t.Fatalf("构造请求失败: %v", err)
	}
	源地址 := M.Socksaddr{
		Addr: netip.MustParseAddr("127.0.0.1"),
		Port: 12345,
	}

	// Router 未注册时应在解引用连接前就返回错误，故连接可传 nil
	err = svc.路由连接(context.Background(), nil, req, 源地址)
	if err == nil {
		t.Fatal("Router 未注册时应返回错误")
	}
	if !strings.Contains(err.Error(), "Router 未在上下文中注册") {
		t.Errorf("错误信息应包含 'Router 未在上下文中注册'，实际: %v", err)
	}
}
