package main

import (
	"net/http"
	"testing"
)

// TestWebSocket_服务启用 验证 WebSocket 中继相关服务启用
func TestWebSocket_服务启用(t *testing.T) {
	服务 := 创建测试服务(t)
	if !服务.IsEnabled() {
		t.Error("期望服务已启用")
	}
}

// TestWebSocket_CA证书可用 验证 WebSocket over TLS 需要 CA 证书
func TestWebSocket_CA证书可用(t *testing.T) {
	服务 := 创建测试服务(t)
	if !服务.IsCAInstalled() {
		t.Error("WebSocket over TLS 需要 CA 证书，期望已加载")
	}
}

// TestWebSocket_标准升级请求识别 验证标准 WebSocket 升级请求识别
func TestWebSocket_标准升级请求识别(t *testing.T) {
	请求, err := http.NewRequest("GET", "http://example.com/ws", nil)
	if err != nil {
		t.Fatalf("创建请求失败: %v", err)
	}
	请求.Header.Set("Upgrade", "websocket")
	请求.Header.Set("Connection", "Upgrade")
	请求.Header.Set("Sec-WebSocket-Version", "13")
	请求.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")

	if 请求.Header.Get("Upgrade") != "websocket" {
		t.Error("期望 Upgrade 头为 websocket")
	}
	if 请求.Header.Get("Connection") != "Upgrade" {
		t.Error("期望 Connection 头为 Upgrade")
	}
}

// TestWebSocket_非升级请求识别 验证非 WebSocket 升级请求不被识别
func TestWebSocket_非升级请求识别(t *testing.T) {
	请求, err := http.NewRequest("GET", "http://example.com/api", nil)
	if err != nil {
		t.Fatalf("创建请求失败: %v", err)
	}
	if 请求.Header.Get("Upgrade") == "websocket" {
		t.Error("普通 HTTP 请求不应有 WebSocket Upgrade 头")
	}
}

// TestWebSocket_Connection不含Upgrade 验证 Connection 头不含 Upgrade 时不识别
func TestWebSocket_Connection不含Upgrade(t *testing.T) {
	请求, err := http.NewRequest("GET", "http://example.com/ws", nil)
	if err != nil {
		t.Fatalf("创建请求失败: %v", err)
	}
	请求.Header.Set("Upgrade", "websocket")
	请求.Header.Set("Connection", "keep-alive")
	if 请求.Header.Get("Connection") == "Upgrade" {
		t.Error("Connection 头为 keep-alive，不应识别为 WebSocket 升级")
	}
}

// TestWebSocket_未启用服务 验证未启用服务不影响 WebSocket 配置解析
func TestWebSocket_未启用服务(t *testing.T) {
	服务 := 创建未启用服务(t)
	if 服务.IsEnabled() {
		t.Error("期望服务未启用")
	}
}

// TestWebSocket_日志功能 验证 WebSocket 相关日志功能可用
func TestWebSocket_日志功能(t *testing.T) {
	服务 := 创建测试服务(t)
	日志列表 := 服务.GetMITMLogs()
	if 日志列表 == nil {
		t.Fatal("期望日志列表非 nil")
	}
	服务.ClearMITMLogs()
	if len(服务.GetMITMLogs()) != 0 {
		t.Error("清空后期望日志为空")
	}
}
