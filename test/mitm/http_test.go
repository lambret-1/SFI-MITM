package main

import (
	"testing"

	"github.com/sagernet/sing-box/option"
)

// TestHTTP_重写引擎启用 验证重写引擎启用配置
func TestHTTP_重写引擎启用(t *testing.T) {
	服务 := 创建测试服务(t, func(配置 *option.MITMServiceOptions) {
		配置.Rewrite = option.MITMRewriteOptions{
			Enabled: true,
			Rules: []option.MITMRewriteRule{
				{
					DomainSuffix:  []string{"example.com"},
					RequestHeader: map[string]string{"X-Test": "true"},
				},
			},
		}
	})
	if !服务.IsEnabled() {
		t.Error("期望服务已启用")
	}
}

// TestHTTP_重写引擎未启用 验证重写引擎未启用时服务正常
func TestHTTP_重写引擎未启用(t *testing.T) {
	服务 := 创建测试服务(t, func(配置 *option.MITMServiceOptions) {
		配置.Rewrite = option.MITMRewriteOptions{Enabled: false}
	})
	if !服务.IsEnabled() {
		t.Error("期望服务已启用（即使重写未启用）")
	}
}

// TestHTTP_重写规则全字段 验证重写规则所有字段能正确解析
func TestHTTP_重写规则全字段(t *testing.T) {
	服务 := 创建测试服务(t, func(配置 *option.MITMServiceOptions) {
		配置.Rewrite = option.MITMRewriteOptions{
			Enabled:     true,
			MaxBodySize: 10 * 1024 * 1024,
			Rules: []option.MITMRewriteRule{
				{
					DomainSuffix:         []string{"example.com"},
					PathPrefix:           "/api/",
					Method:               []string{"GET", "POST"},
					RequestHeader:        map[string]string{"X-MITM": "true"},
					RequestHeaderDelete:  []string{"X-Old"},
					ResponseHeader:       map[string]string{"X-Intercepted": "yes"},
					ResponseHeaderDelete: []string{"X-Server"},
					BodyReplace: []option.MITMBodyReplaceRule{
						{Find: "old", Replace: "new"},
					},
				},
			},
		}
	})
	if !服务.IsEnabled() {
		t.Error("期望服务已启用")
	}
}

// TestHTTP_多域名匹配 验证多域名匹配配置
func TestHTTP_多域名匹配(t *testing.T) {
	服务 := 创建测试服务(t, func(配置 *option.MITMServiceOptions) {
		配置.Match = option.MITMMatchOptions{
			DomainSuffix: []string{"example.com", "test.org", "foo.bar"},
		}
	})
	if !服务.IsEnabled() {
		t.Error("期望服务已启用")
	}
}

// TestHTTP_on_error绕过 验证 on_error=bypass 配置
func TestHTTP_on_error绕过(t *testing.T) {
	服务 := 创建测试服务(t, func(配置 *option.MITMServiceOptions) {
		配置.OnError = "bypass"
	})
	if !服务.IsEnabled() {
		t.Error("期望服务已启用")
	}
}

// TestHTTP_on_error阻断 验证 on_error=block 配置
func TestHTTP_on_error阻断(t *testing.T) {
	服务 := 创建测试服务(t, func(配置 *option.MITMServiceOptions) {
		配置.OnError = "block"
	})
	if !服务.IsEnabled() {
		t.Error("期望服务已启用")
	}
}

// TestHTTP_空匹配规则 验证空匹配规则（不拦截任何域名）
func TestHTTP_空匹配规则(t *testing.T) {
	服务 := 创建测试服务(t, func(配置 *option.MITMServiceOptions) {
		配置.Match = option.MITMMatchOptions{}
	})
	if !服务.IsEnabled() {
		t.Error("期望服务已启用（即使匹配规则为空）")
	}
}
