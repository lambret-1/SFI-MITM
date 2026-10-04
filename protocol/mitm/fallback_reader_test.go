package mitm

import (
	"bytes"
	"net"
	"testing"
	"time"
)

// Test回退读取器_先返回已读数据 验证回退读取器先返回已读取的数据
func Test回退读取器_先返回已读数据(t *testing.T) {
	客户端, 服务端 := net.Pipe()
	defer 客户端.Close()
	defer 服务端.Close()

	已读数据 := []byte("GET / HTTP/1.1\r\nHost: example.com\r\n\r\n")
	读取器 := 新建回退读取器(服务端, 已读数据)

	// 先读取已读数据
	缓冲区 := make([]byte, 1024)
	n, err := 读取器.Read(缓冲区)
	if err != nil {
		t.Fatalf("读取已读数据失败: %v", err)
	}
	if !bytes.Equal(缓冲区[:n], 已读数据) {
		t.Errorf("期望返回已读数据，实际 %s", 缓冲区[:n])
	}
}

// Test回退读取器_已读数据读完后从底层读取 验证已读数据读完后从底层连接读取
func Test回退读取器_已读数据读完后从底层读取(t *testing.T) {
	客户端, 服务端 := net.Pipe()
	defer 客户端.Close()
	defer 服务端.Close()

	已读数据 := []byte("PRE-READ")
	读取器 := 新建回退读取器(服务端, 已读数据)

	// 先读完已读数据
	缓冲区1 := make([]byte, len(已读数据))
	_, _ = 读取器.Read(缓冲区1)

	// 在 goroutine 中向客户端写入数据
	go func() {
		客户端.Write([]byte("FROM-BACKEND"))
	}()

	// 从底层连接读取
	缓冲区2 := make([]byte, 1024)
	n, err := 读取器.Read(缓冲区2)
	if err != nil {
		t.Fatalf("从底层连接读取失败: %v", err)
	}
	if string(缓冲区2[:n]) != "FROM-BACKEND" {
		t.Errorf("期望从底层读取 FROM-BACKEND，实际 %s", 缓冲区2[:n])
	}
}

// Test回退读取器_Write委托底层 验证 Write 委托给底层连接
func Test回退读取器_Write委托底层(t *testing.T) {
	客户端, 服务端 := net.Pipe()
	defer 客户端.Close()
	defer 服务端.Close()

	读取器 := 新建回退读取器(服务端, nil)

	go func() {
		读取器.Write([]byte("WRITE-TEST"))
	}()

	缓冲区 := make([]byte, 1024)
	n, err := 客户端.Read(缓冲区)
	if err != nil {
		t.Fatalf("客户端读取失败: %v", err)
	}
	if string(缓冲区[:n]) != "WRITE-TEST" {
		t.Errorf("期望客户端收到 WRITE-TEST，实际 %s", 缓冲区[:n])
	}
}

// Test回退读取器_Close关闭底层 验证 Close 关闭底层连接
func Test回退读取器_Close关闭底层(t *testing.T) {
	客户端, 服务端 := net.Pipe()
	defer 客户端.Close()

	读取器 := 新建回退读取器(服务端, nil)
	if err := 读取器.Close(); err != nil {
		t.Fatalf("关闭失败: %v", err)
	}
	// 关闭后写入应失败
	_, err := 服务端.Write([]byte("test"))
	if err == nil {
		t.Error("期望关闭后写入失败")
	}
}

// Test回退读取器_LocalAddrRemoteAddr 验证 LocalAddr 和 RemoteAddr 委托底层
func Test回退读取器_LocalAddrRemoteAddr(t *testing.T) {
	客户端, 服务端 := net.Pipe()
	defer 客户端.Close()
	defer 服务端.Close()

	读取器 := 新建回退读取器(服务端, nil)
	if 读取器.LocalAddr() == nil {
		t.Error("期望 LocalAddr 非 nil")
	}
	if 读取器.RemoteAddr() == nil {
		t.Error("期望 RemoteAddr 非 nil")
	}
}

// Test回退读取器_SetDeadline 验证 SetDeadline 委托底层
func Test回退读取器_SetDeadline(t *testing.T) {
	客户端, 服务端 := net.Pipe()
	defer 客户端.Close()
	defer 服务端.Close()

	读取器 := 新建回退读取器(服务端, nil)
	截止时间 := time.Now().Add(5 * time.Second)
	if err := 读取器.SetDeadline(截止时间); err != nil {
		t.Errorf("SetDeadline 失败: %v", err)
	}
	if err := 读取器.SetReadDeadline(截止时间); err != nil {
		t.Errorf("SetReadDeadline 失败: %v", err)
	}
	if err := 读取器.SetWriteDeadline(截止时间); err != nil {
		t.Errorf("SetWriteDeadline 失败: %v", err)
	}
}

// Test回退读取器_空已读数据 验证已读数据为空时直接从底层读取
func Test回退读取器_空已读数据(t *testing.T) {
	客户端, 服务端 := net.Pipe()
	defer 客户端.Close()
	defer 服务端.Close()

	读取器 := 新建回退读取器(服务端, []byte{})

	go func() {
		客户端.Write([]byte("DIRECT"))
	}()

	缓冲区 := make([]byte, 1024)
	n, err := 读取器.Read(缓冲区)
	if err != nil {
		t.Fatalf("读取失败: %v", err)
	}
	if string(缓冲区[:n]) != "DIRECT" {
		t.Errorf("期望直接从底层读取 DIRECT，实际 %s", 缓冲区[:n])
	}
}

// Test回退读取器_编译期断言 验证回退读取器实现 net.Conn 接口
func Test回退读取器_编译期断言(t *testing.T) {
	var _ net.Conn = (*回退读取器)(nil)
}
