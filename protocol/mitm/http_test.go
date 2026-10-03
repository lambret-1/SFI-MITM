package mitm

import (
	"net/http/httptest"
	"testing"
)

// 新建测试HTTP1处理器 构造仅用于单元测试的 HTTP/1.1 处理器
//
// 提取域名 / 保持连接 方法仅依赖 *http.Request，不访问处理器内部字段，
// 因此可直接使用空结构体进行测试。
func 新建测试HTTP1处理器() *HTTP1处理器 {
	return &HTTP1处理器{}
}

// TestHTTP1处理器_提取域名_带端口 验证从带端口的 Host 头剥离端口得到纯域名
func TestHTTP1处理器_提取域名_带端口(t *testing.T) {
	h := 新建测试HTTP1处理器()
	req := httptest.NewRequest("GET", "https://api.example.com:443/path", nil)

	域名 := h.提取域名(req)
	if 域名 != "api.example.com" {
		t.Errorf("带端口 Host 应剥离为 api.example.com，实际: %s", 域名)
	}
}

// TestHTTP1处理器_提取域名_无端口 验证无端口的 Host 头原样返回域名
func TestHTTP1处理器_提取域名_无端口(t *testing.T) {
	h := 新建测试HTTP1处理器()
	req := httptest.NewRequest("GET", "https://api.example.com/path", nil)

	域名 := h.提取域名(req)
	if 域名 != "api.example.com" {
		t.Errorf("无端口 Host 应原样返回 api.example.com，实际: %s", 域名)
	}
}

// TestHTTP1处理器_提取域名_URLHost回退 验证 Host 头为空时回退到 URL.Host
func TestHTTP1处理器_提取域名_URLHost回退(t *testing.T) {
	h := 新建测试HTTP1处理器()
	req := httptest.NewRequest("GET", "https://backup.example.com/path", nil)
	req.Host = "" // 模拟反向代理场景：Host 头缺失，回退到 URL.Host

	域名 := h.提取域名(req)
	if 域名 != "backup.example.com" {
		t.Errorf("Host 为空时应回退到 URL.Host=backup.example.com，实际: %s", 域名)
	}
}

// TestHTTP1处理器_保持连接_HTTP11默认 验证 HTTP/1.1 默认保持连接
func TestHTTP1处理器_保持连接_HTTP11默认(t *testing.T) {
	h := 新建测试HTTP1处理器()
	// httptest.NewRequest 默认构造 HTTP/1.1 请求，且无 Connection 头
	req := httptest.NewRequest("GET", "https://api.example.com/path", nil)

	if !h.保持连接(req) {
		t.Error("HTTP/1.1 无 Connection 头时应默认保持连接")
	}
}

// TestHTTP1处理器_保持连接_ConnectionClose 验证 Connection: close 时不保持连接
func TestHTTP1处理器_保持连接_ConnectionClose(t *testing.T) {
	h := 新建测试HTTP1处理器()
	req := httptest.NewRequest("GET", "https://api.example.com/path", nil)
	req.Header.Set("Connection", "close")

	if h.保持连接(req) {
		t.Error("Connection: close 时应不保持连接")
	}
}

// TestHTTP1处理器_保持连接_HTTP10 验证 HTTP/1.0 请求默认不保持连接
func TestHTTP1处理器_保持连接_HTTP10(t *testing.T) {
	h := 新建测试HTTP1处理器()
	req := httptest.NewRequest("GET", "https://api.example.com/path", nil)
	// 降级为 HTTP/1.0：ProtoAtLeast(1,1) 为 false，默认关闭持久连接
	req.Proto = "HTTP/1.0"
	req.ProtoMajor = 1
	req.ProtoMinor = 0

	if h.保持连接(req) {
		t.Error("HTTP/1.0 请求应不保持连接")
	}
}
