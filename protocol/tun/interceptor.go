package tun

import (
	"context"
	"net"

	"github.com/sagernet/sing-box/adapter"
	N "github.com/sagernet/sing/common/network"
)

// Interceptor TUN 连接拦截器接口
//
// 对应开发文档 Phase 6：TUN interception。
// TUN 层仅依赖此接口，不直接 import MITM 实现包，实现核心层与拦截层解耦。
// 由外部服务（如 MITM Service）实现并注册到 service 上下文，
// TUN 在处理 TCP 连接时通过 service.FromContext[Interceptor] 获取并调用。
//
// 数据流：
//
//	TUN → Interceptor.ShouldIntercept → Interceptor.Intercept → MITM 处理
type Interceptor interface {
	// ShouldIntercept 判断是否需要拦截该连接
	// 基于连接元数据（目标域名、端口、协议等）决定是否进入拦截流程
	ShouldIntercept(metadata adapter.InboundContext) bool

	// Intercept 拦截并处理连接
	// 当 ShouldIntercept 返回 true 时调用，接管连接的后续处理
	// 参数：
	//   ctx - 上下文，包含日志 ID 等
	//   conn - 客户端 TCP 连接
	//   metadata - 连接元数据（源/目标地址、入站标签等）
	//   router - 路由器实例，用于拦截后向上游转发
	//   onClose - 连接关闭回调
	Intercept(
		ctx context.Context,
		conn net.Conn,
		metadata adapter.InboundContext,
		router adapter.Router,
		onClose N.CloseHandlerFunc,
	)
}
