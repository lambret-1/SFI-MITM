package libbox

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"time"

	E "github.com/sagernet/sing/common/exceptions"
)

// MITMStatus MITM 服务运行状态
//
// 对应开发文档 Phase 13 libbox export API。
// 字段类型均为 gomobile bind 支持的类型（bool/int32），
// 可直接暴露给 Swift/Objective-C 调用。
type MITMStatus struct {
	// Enabled MITM 服务是否已启用
	Enabled bool
	// CAInstalled 根证书是否已成功加载
	CAInstalled bool
	// ActiveConnections 当前正在进行 MITM 解密的活跃连接数
	ActiveConnections int32
}

// GetMITMStatus 获取 MITM 服务运行状态
//
// 从当前运行的 sing-box 实例中查询 MITM 服务状态。
// 服务未启动或未配置 MITM 时返回零值（Enabled=false）。
//
// 此方法运行在 Network Extension 进程中，由 Swift 端直接调用，
// 不需要通过 gRPC 跨进程通信。
func (s *CommandServer) GetMITMStatus() *MITMStatus {
	instance := s.Instance()
	if instance == nil {
		return &MITMStatus{}
	}
	状态 := instance.GetMITMStatus()
	return &MITMStatus{
		Enabled:           状态.Enabled,
		CAInstalled:       状态.CAInstalled,
		ActiveConnections: 状态.ActiveConnections,
	}
}

// GenerateMITMCA 生成 MITM 根证书与私钥
//
// 使用 ECDSA P-256 曲线生成自签名 CA 根证书，
// 证书有效期 10 年，具备 CA:TRUE 基本约束和 keyCertSign 密钥用途。
//
// 参数：
//   - certificatePath: 证书输出文件路径（PEM 格式）
//   - privateKeyPath: 私钥输出文件路径（PEM 格式）
//
// 对应开发文档 Phase 13 的 GenerateMITMCA API，
// 供 SFI 设置页面"生成 CA"按钮调用。
func GenerateMITMCA(certificatePath string, privateKeyPath string) error {
	if certificatePath == "" {
		return E.New("mitm: 证书输出路径不能为空")
	}
	if privateKeyPath == "" {
		return E.New("mitm: 私钥输出路径不能为空")
	}

	// 生成 ECDSA P-256 私钥
	私钥, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return E.Cause(err, "mitm: 生成 ECDSA 私钥失败")
	}

	// 生成随机序列号
	序列号, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return E.Cause(err, "mitm: 生成证书序列号失败")
	}

	// 构造 CA 证书模板
	模板 := x509.Certificate{
		SerialNumber: 序列号,
		Subject: pkix.Name{
			CommonName:   "SFI MITM Root CA",
			Organization: []string{"SFI"},
		},
		NotBefore:             time.Now().Add(-1 * time.Hour),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLenZero:        true,
	}

	// 自签名生成证书 DER
	证书DER, err := x509.CreateCertificate(rand.Reader, &模板, &模板, &私钥.PublicKey, 私钥)
	if err != nil {
		return E.Cause(err, "mitm: 生成自签名证书失败")
	}

	// 确保输出目录存在
	证书目录 := filepath.Dir(certificatePath)
	if 证书目录 != "." && 证书目录 != "" {
		if err := os.MkdirAll(证书目录, 0o755); err != nil {
			return E.Cause(err, "mitm: 创建证书目录失败: ", 证书目录)
		}
	}
	私钥目录 := filepath.Dir(privateKeyPath)
	if 私钥目录 != "." && 私钥目录 != "" {
		if err := os.MkdirAll(私钥目录, 0o755); err != nil {
			return E.Cause(err, "mitm: 创建私钥目录失败: ", 私钥目录)
		}
	}

	// 写入证书 PEM
	证书文件, err := os.OpenFile(certificatePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return E.Cause(err, "mitm: 创建证书文件失败: ", certificatePath)
	}
	defer 证书文件.Close()
	if err := pem.Encode(证书文件, &pem.Block{Type: "CERTIFICATE", Bytes: 证书DER}); err != nil {
		return E.Cause(err, "mitm: 写入证书 PEM 失败")
	}

	// 编码私钥 DER
	私钥DER, err := x509.MarshalECPrivateKey(私钥)
	if err != nil {
		return E.Cause(err, "mitm: 编码私钥失败")
	}

	// 写入私钥 PEM（权限 0o600，仅所有者可读写）
	私钥文件, err := os.OpenFile(privateKeyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return E.Cause(err, "mitm: 创建私钥文件失败: ", privateKeyPath)
	}
	defer 私钥文件.Close()
	if err := pem.Encode(私钥文件, &pem.Block{Type: "EC PRIVATE KEY", Bytes: 私钥DER}); err != nil {
		return E.Cause(err, "mitm: 写入私钥 PEM 失败")
	}

	return nil
}
