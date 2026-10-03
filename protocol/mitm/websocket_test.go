package mitm

import (
	"context"
	"net/http/httptest"
	"testing"
)

// Test是否WebSocketUpgrade_标准升级请求 验证标准 WebSocket 升级头能被识别
func Test是否WebSocketUpgrade_标准升级请求(t *testing.T) {
	req := httptest.NewRequest("GET", "https://api.example.com/ws", nil)
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")

	if !是否WebSocketUpgrade(req) {
		t.Error("带 Connection: Upgrade 与 Upgrade: websocket 的请求应被识别为 WebSocket 升级")
	}
}

// Test是否WebSocketUpgrade_非升级请求 验证普通 GET 请求不被误判
func Test是否WebSocketUpgrade_非升级请求(t *testing.T) {
	req := httptest.NewRequest("GET", "https://api.example.com/path", nil)

	if 是否WebSocketUpgrade(req) {
		t.Error("普通 GET 请求不应被识别为 WebSocket 升级")
	}
}

// Test是否WebSocketUpgrade_Connection不含Upgrade 验证 Connection 头缺少 upgrade 时不识别
func Test是否WebSocketUpgrade_Connection不含Upgrade(t *testing.T) {
	req := httptest.NewRequest("GET", "https://api.example.com/ws", nil)
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Upgrade", "websocket")

	if 是否WebSocketUpgrade(req) {
		t.Error("Connection 为 keep-alive 时不应被识别为 WebSocket 升级")
	}
}

// Test是否WebSocketUpgrade_Upgrade非websocket 验证 Upgrade 为其他协议时不识别
func Test是否WebSocketUpgrade_Upgrade非websocket(t *testing.T) {
	req := httptest.NewRequest("GET", "https://api.example.com/path", nil)
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "h2c")

	if 是否WebSocketUpgrade(req) {
		t.Error("Upgrade 为 h2c 时不应被识别为 WebSocket 升级")
	}
}

// Test是否WebSocketUpgrade_nil请求 验证传入 nil 不 panic 且返回 false
func Test是否WebSocketUpgrade_nil请求(t *testing.T) {
	if 是否WebSocketUpgrade(nil) {
		t.Error("nil 请求应返回 false")
	}
}

// Test新建WebSocket处理器 验证构造的处理器服务字段已正确注入
func Test新建WebSocket处理器(t *testing.T) {
	svc := &Service{
		ctx:    context.Background(),
		logger: 获取测试日志器(),
	}

	处理器 := 新建WebSocket处理器(svc)
	if 处理器 == nil {
		t.Fatal("新建WebSocket处理器不应返回 nil")
	}
	if 处理器.服务 == nil {
		t.Error("处理器的服务字段不应为 nil")
	}
	if 处理器.服务 != svc {
		t.Error("处理器的服务字段应与传入的服务实例一致")
	}
}
