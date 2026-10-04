package main

import (
	"testing"
)

// TestTLS_服务启用状态 验证 TLS 相关服务启用状态
func TestTLS_服务启用状态(t *testing.T) {
	服务 := 创建测试服务(t)
	if !服务.IsEnabled() {
		t.Error("期望服务已启用")
	}
}

// TestTLS_未启用状态 验证未启用时服务状态
func TestTLS_未启用状态(t *testing.T) {
	服务 := 创建未启用服务(t)
	if 服务.IsEnabled() {
		t.Error("期望服务未启用")
	}
}

// TestTLS_CA证书加载 验证 TLS 终止所需的 CA 证书已加载
func TestTLS_CA证书加载(t *testing.T) {
	服务 := 创建测试服务(t)
	if !服务.IsCAInstalled() {
		t.Error("TLS 终止需要 CA 证书，期望已加载")
	}
}

// TestTLS_未启用时不加载CA 验证未启用时不加载 CA 证书（TLS 终止不激活）
func TestTLS_未启用时不加载CA(t *testing.T) {
	服务 := 创建未启用服务(t)
	if 服务.IsCAInstalled() {
		t.Error("未启用时期望 CA 未加载，TLS 终止不激活")
	}
}

// TestTLS_日志缓冲区初始化 验证服务创建后日志缓冲区已初始化
func TestTLS_日志缓冲区初始化(t *testing.T) {
	服务 := 创建测试服务(t)
	日志列表 := 服务.GetMITMLogs()
	if 日志列表 == nil {
		t.Fatal("期望日志列表非 nil")
	}
	// 初始状态下日志应为空
	if len(日志列表) != 0 {
		t.Errorf("期望初始状态下日志为空，实际 %d 条", len(日志列表))
	}
}

// TestTLS_清空日志 验证清空日志功能
func TestTLS_清空日志(t *testing.T) {
	服务 := 创建测试服务(t)
	服务.ClearMITMLogs()
	日志列表 := 服务.GetMITMLogs()
	if len(日志列表) != 0 {
		t.Errorf("清空后期望日志为空，实际 %d 条", len(日志列表))
	}
}
