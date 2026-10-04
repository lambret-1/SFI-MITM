package main

import (
	"testing"

	"github.com/sagernet/sing-box/option"
)

// Test配置解析_完整配置 验证完整 MITM 配置能正确解析并创建服务
func Test配置解析_完整配置(t *testing.T) {
	服务 := 创建测试服务(t, func(配置 *option.MITMServiceOptions) {
		配置.Match = option.MITMMatchOptions{
			Domain:       []string{"api.example.com"},
			DomainSuffix: []string{"example.com", "test.org"},
		}
		配置.OnError = "bypass"
		配置.UpstreamTimeout = 60
		配置.Rewrite = option.MITMRewriteOptions{
			Enabled:      true,
			MaxBodySize:  5 * 1024 * 1024,
			Rules: []option.MITMRewriteRule{
				{
					DomainSuffix:   []string{"example.com"},
					PathPrefix:     "/api/",
					RequestHeader:  map[string]string{"X-MITM": "true"},
					ResponseHeader: map[string]string{"X-Intercepted": "yes"},
				},
			},
		}
	})
	if !服务.IsEnabled() {
		t.Error("期望服务已启用")
	}
	if !服务.IsCAInstalled() {
		t.Error("期望 CA 证书已加载")
	}
}

// Test配置解析_未启用 验证未启用 MITM 的配置能正确解析
func Test配置解析_未启用(t *testing.T) {
	服务 := 创建未启用服务(t)
	if 服务.IsEnabled() {
		t.Error("期望服务未启用")
	}
	if 服务.IsCAInstalled() {
		t.Error("未启用时期望 CA 未加载")
	}
}

// Test配置解析_缺少证书路径 验证缺少证书路径时返回错误
func Test配置解析_缺少证书路径(t *testing.T) {
	_, 私钥路径 := 生成测试CA(t)
	配置 := option.MITMServiceOptions{
		Enabled: true,
		CA: option.MITMCAOptions{
			PrivateKey: 私钥路径,
		},
	}
	ctx := 创建测试上下文(t)
	logger := 创建测试日志器(t)
	_, err := 创建服务从配置(t, ctx, logger, 配置)
	if err == nil {
		t.Error("期望缺少证书路径时返回错误")
	}
}

// Test配置解析_缺少私钥路径 验证缺少私钥路径时返回错误
func Test配置解析_缺少私钥路径(t *testing.T) {
	证书路径, _ := 生成测试CA(t)
	配置 := option.MITMServiceOptions{
		Enabled: true,
		CA: option.MITMCAOptions{
			Certificate: 证书路径,
		},
	}
	ctx := 创建测试上下文(t)
	logger := 创建测试日志器(t)
	_, err := 创建服务从配置(t, ctx, logger, 配置)
	if err == nil {
		t.Error("期望缺少私钥路径时返回错误")
	}
}

// Test配置解析_on_error枚举 验证 on_error 字段的合法值
func Test配置解析_on_error枚举(t *testing.T) {
	合法值 := []string{"bypass", "block", "close"}
	for _, 值 := range 合法值 {
		创建测试服务(t, func(配置 *option.MITMServiceOptions) {
			配置.OnError = 值
		})
	}
}

// Test配置解析_upstreamTimeout 验证上游超时配置
func Test配置解析_upstreamTimeout(t *testing.T) {
	服务 := 创建测试服务(t, func(配置 *option.MITMServiceOptions) {
		配置.UpstreamTimeout = 45
	})
	if 服务.GetUpstreamTimeout() != 45 {
		t.Errorf("期望上游超时 45 秒，实际 %d", 服务.GetUpstreamTimeout())
	}
}

// Test配置解析_默认上游超时 验证未设置上游超时时使用默认值
func Test配置解析_默认上游超时(t *testing.T) {
	服务 := 创建测试服务(t)
	if 服务.GetUpstreamTimeout() != 30 {
		t.Errorf("期望默认上游超时 30 秒，实际 %d", 服务.GetUpstreamTimeout())
	}
}
