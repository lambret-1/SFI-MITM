package main

import (
	"crypto/x509"
	"encoding/pem"
	"os"
	"testing"

	"github.com/sagernet/sing-box/option"
)

// TestCA_加载合法CA 验证合法 CA 证书能正确加载
func TestCA_加载合法CA(t *testing.T) {
	服务 := 创建测试服务(t)
	if !服务.IsCAInstalled() {
		t.Error("期望 CA 证书已成功加载")
	}
}

// TestCA_未启用时不加载CA 验证未启用 MITM 时不加载 CA 证书
func TestCA_未启用时不加载CA(t *testing.T) {
	服务 := 创建未启用服务(t)
	if 服务.IsCAInstalled() {
		t.Error("未启用时期望 CA 未加载")
	}
}

// TestCA_非CA证书被拒绝 验证非 CA 证书被拒绝
func TestCA_非CA证书被拒绝(t *testing.T) {
	临时目录 := t.TempDir()
	非CA证书路径 := 临时目录 + "/non-ca.pem"
	非CA私钥路径 := 临时目录 + "/non-ca.key"
	生成非CA证书(t, 非CA证书路径, 非CA私钥路径)

	配置 := option.MITMServiceOptions{
		Enabled: true,
		CA: option.MITMCAOptions{
			Certificate: 非CA证书路径,
			PrivateKey:  非CA私钥路径,
		},
	}
	ctx := 创建测试上下文(t)
	logger := 创建测试日志器(t)
	_, err := 创建服务从配置(t, ctx, logger, 配置)
	if err == nil {
		t.Error("期望非 CA 证书被拒绝")
	}
}

// TestCA_证书文件不存在 验证证书文件不存在时返回错误
func TestCA_证书文件不存在(t *testing.T) {
	配置 := option.MITMServiceOptions{
		Enabled: true,
		CA: option.MITMCAOptions{
			Certificate: "/nonexistent/ca.pem",
			PrivateKey:  "/nonexistent/ca.key",
		},
	}
	ctx := 创建测试上下文(t)
	logger := 创建测试日志器(t)
	_, err := 创建服务从配置(t, ctx, logger, 配置)
	if err == nil {
		t.Error("期望证书文件不存在时返回错误")
	}
}

// TestCA_证书是有效的CA 验证生成的测试证书确实是有效的 CA 证书
func TestCA_证书是有效的CA(t *testing.T) {
	证书路径, _ := 生成测试CA(t)
	证书PEM, err := os.ReadFile(证书路径)
	if err != nil {
		t.Fatalf("读取证书文件失败: %v", err)
	}
	块, _ := pem.Decode(证书PEM)
	if 块 == nil {
		t.Fatal("PEM 解码失败")
	}
	证书, err := x509.ParseCertificate(块.Bytes)
	if err != nil {
		t.Fatalf("解析证书失败: %v", err)
	}
	if !证书.IsCA {
		t.Error("期望证书是 CA 证书")
	}
	if 证书.KeyUsage&x509.KeyUsageCertSign == 0 {
		t.Error("期望证书具有 keyCertSign 密钥用途")
	}
}

// 生成非CA证书 生成非 CA 证书用于负向测试
func 生成非CA证书(t *testing.T, 证书路径 string, 私钥路径 string) {
	t.Helper()
	// 使用 openssl 生成非 CA 证书
	执行命令(t, "openssl", "req", "-x509", "-newkey", "rsa:2048",
		"-keyout", 私钥路径, "-out", 证书路径,
		"-days", "365", "-nodes",
		"-subj", "/CN=Not a CA",
		"-addext", "basicConstraints=critical,CA:FALSE",
		"-addext", "keyUsage=critical,digitalSignature")
}
