package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-box/protocol/mitm"
	"github.com/sagernet/sing/service"
)

// 生成测试CA 生成测试用的自签名 CA 根证书与私钥，返回文件路径
// 测试完成后自动清理临时文件
func 生成测试CA(t *testing.T) (证书路径 string, 私钥路径 string) {
	t.Helper()
	临时目录 := t.TempDir()
	证书路径 = filepath.Join(临时目录, "ca.pem")
	私钥路径 = filepath.Join(临时目录, "ca.key")

	私钥, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("生成 ECDSA 私钥失败: %v", err)
	}
	序列号, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		t.Fatalf("生成序列号失败: %v", err)
	}
	模板 := x509.Certificate{
		SerialNumber:          序列号,
		Subject:               pkix.Name{CommonName: "MITM Test CA", Organization: []string{"Test"}},
		NotBefore:             time.Now().Add(-1 * time.Hour),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLenZero:        true,
	}
	证书DER, err := x509.CreateCertificate(rand.Reader, &模板, &模板, &私钥.PublicKey, 私钥)
	if err != nil {
		t.Fatalf("生成自签名证书失败: %v", err)
	}
	证书文件, err := os.Create(证书路径)
	if err != nil {
		t.Fatalf("创建证书文件失败: %v", err)
	}
	defer 证书文件.Close()
	if err := pem.Encode(证书文件, &pem.Block{Type: "CERTIFICATE", Bytes: 证书DER}); err != nil {
		t.Fatalf("写入证书 PEM 失败: %v", err)
	}
	私钥DER, err := x509.MarshalECPrivateKey(私钥)
	if err != nil {
		t.Fatalf("编码私钥失败: %v", err)
	}
	私钥文件, err := os.OpenFile(私钥路径, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		t.Fatalf("创建私钥文件失败: %v", err)
	}
	defer 私钥文件.Close()
	if err := pem.Encode(私钥文件, &pem.Block{Type: "EC PRIVATE KEY", Bytes: 私钥DER}); err != nil {
		t.Fatalf("写入私钥 PEM 失败: %v", err)
	}
	return
}

// 创建测试服务 创建 MITM 服务实例，用于测试
// 自动生成测试 CA，构造默认配置，调用 NewService 创建服务
func 创建测试服务(t *testing.T, 配置选项 ...func(*option.MITMServiceOptions)) *mitm.Service {
	t.Helper()
	证书路径, 私钥路径 := 生成测试CA(t)
	配置 := option.MITMServiceOptions{
		Enabled: true,
		CA: option.MITMCAOptions{
			Certificate: 证书路径,
			PrivateKey:  私钥路径,
		},
		Match: option.MITMMatchOptions{
			DomainSuffix: []string{"example.com"},
		},
	}
	for _, 选项 := range 配置选项 {
		选项(&配置)
	}
	ctx := service.ContextWithDefaultRegistry(context.Background())
	logger := log.NewNOPFactory().Logger()
	服务接口, err := mitm.NewService(ctx, logger, "mitm-test", 配置)
	if err != nil {
		t.Fatalf("创建 MITM 服务失败: %v", err)
	}
	服务, ok := 服务接口.(*mitm.Service)
	if !ok {
		t.Fatalf("服务类型断言失败，期望 *mitm.Service")
	}
	return 服务
}

// 创建未启用服务 创建未启用 MITM 的服务实例
func 创建未启用服务(t *testing.T) *mitm.Service {
	t.Helper()
	配置 := option.MITMServiceOptions{Enabled: false}
	ctx := service.ContextWithDefaultRegistry(context.Background())
	logger := log.NewNOPFactory().Logger()
	服务接口, err := mitm.NewService(ctx, logger, "mitm-disabled", 配置)
	if err != nil {
		t.Fatalf("创建未启用 MITM 服务失败: %v", err)
	}
	return 服务接口.(*mitm.Service)
}

// 创建测试上下文 创建带 service 注册表的测试上下文
func 创建测试上下文(t *testing.T) context.Context {
	t.Helper()
	return service.ContextWithDefaultRegistry(context.Background())
}

// 创建测试日志器 创建测试用日志器
func 创建测试日志器(t *testing.T) log.ContextLogger {
	t.Helper()
	return log.NewNOPFactory().Logger()
}

// 创建服务从配置 从配置选项创建 MITM 服务，返回服务接口和错误
func 创建服务从配置(t *testing.T, ctx context.Context, logger log.ContextLogger, 配置 option.MITMServiceOptions) (adapter.Service, error) {
	t.Helper()
	return mitm.NewService(ctx, logger, "mitm-test", 配置)
}

// 执行命令 执行外部命令，失败时终止测试
func 执行命令(t *testing.T, 名称 string, 参数 ...string) {
	t.Helper()
	cmd := exec.Command(名称, 参数...)
	输出, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("执行命令 %s %v 失败: %v\n输出: %s", 名称, 参数, err, 输出)
	}
}
