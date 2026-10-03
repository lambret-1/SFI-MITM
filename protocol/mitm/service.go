package mitm

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/sagernet/sing-box/adapter"
	boxService "github.com/sagernet/sing-box/adapter/service"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-box/protocol/mitm/rewrite"
	"github.com/sagernet/sing/service"
)

// RegisterService 将 MITM 服务注册到服务注册表
// 在 include/registry.go 的 ServiceRegistry() 中被调用
func RegisterService(registry *boxService.Registry) {
	boxService.Register[option.MITMServiceOptions](registry, C.TypeMITM, NewService)
}

// 编译期断言：Service 必须实现 adapter.Service 接口
var _ adapter.Service = (*Service)(nil)

// Service MITM 服务主体
// 负责根证书管理、域名匹配、动态证书签发、TLS 终止、HTTP 解密与重写，
// 并将解密后的流量交回 sing-box Router 进行分流。
//
// 生命周期：
//   - NewService：加载 CA、初始化各子模块
//   - Start：启动拦截逻辑（后续阶段接入 TUN 后激活）
type Service struct {
	boxService.Adapter

	// ctx 服务上下文，用于获取 Router 等全局依赖
	ctx context.Context
	// logger 带上下文的日志器
	logger log.ContextLogger
	// options MITM 服务配置
	options option.MITMServiceOptions

	// 根证书相关（ca.go）
	证书锁     sync.RWMutex
	根证书     *根证书实例
	叶子缓存   *证书缓存

	// 域名匹配器（matcher.go）
	匹配器 *域名匹配器

	// 重写引擎（rewrite/）
	重写引擎 *rewrite.Engine

	// 活跃连接数 当前正在进行 MITM 解密的连接数（原子操作）
	活跃连接数 atomic.Int32
}

// NewService 构造 MITM 服务
// 完成配置校验、根证书加载、各子模块初始化。
// 未启用时不加载证书，避免无意义的文件 IO。
func NewService(ctx context.Context, logger log.ContextLogger, tag string, options option.MITMServiceOptions) (adapter.Service, error) {
	svc := &Service{
		Adapter: boxService.NewAdapter(C.TypeMITM, tag),
		ctx:     ctx,
		logger:  logger,
		options: options,
		叶子缓存: 新证书缓存(默认缓存容量),
	}

	if !options.Enabled {
		return svc, nil
	}

	// 校验 CA 配置完整性
	if options.CA.Certificate == "" {
		return nil, 错误缺少证书路径
	}
	if options.CA.PrivateKey == "" {
		return nil, 错误缺少私钥路径
	}

	// 加载并验证根证书
	根证书, err := 加载根证书(options.CA)
	if err != nil {
		return nil, 包装错误(err, "加载根证书失败")
	}
	svc.根证书 = 根证书

	// 初始化域名匹配器
	svc.匹配器 = 新域名匹配器(options.Match)

	// 初始化重写引擎
	svc.重写引擎 = rewrite.NewEngine(options.Rewrite)

	// 将自身注册到上下文，供 TUN 拦截层获取
	service.MustRegister[*Service](ctx, svc)

	logger.Info("mitm: 服务初始化完成，根证书已加载")
	if options.Rewrite.Enabled {
		logger.Info("mitm: 重写引擎已启用，规则数: ", len(options.Rewrite.Rules))
	}
	return svc, nil
}

// Start 实现 adapter.Lifecycle 接口
// MITM 服务在第一阶段仅完成初始化，实际拦截逻辑在后续阶段接入 TUN 后启动。
func (s *Service) Start(stage adapter.StartStage, scope *adapter.Scope) error {
	if stage != adapter.StartStateStart {
		return nil
	}
	if !s.options.Enabled {
		s.logger.Info("mitm: 服务未启用，跳过启动")
		return nil
	}
	s.logger.Info("mitm: 服务已启动")
	return nil
}

// FromContext 从上下文中获取 MITM 服务实例
// 供 TUN 入站等模块调用，判断是否需要拦截流量。
func FromContext(ctx context.Context) *Service {
	return service.FromContext[*Service](ctx)
}

// 是否启用 返回 MITM 服务是否已启用
func (s *Service) 是否启用() bool {
	return s.options.Enabled
}

// 获取重写引擎 返回重写引擎实例
func (s *Service) 获取重写引擎() *rewrite.Engine {
	return s.重写引擎
}

// 是否失败时绕过 返回 MITM 拦截失败时是否回退到正常路由
//
// 参考 README 第 43 节：
//   - "bypass"（默认）：MITM 失败时回退到正常路由
//   - "block"：MITM 失败时关闭连接
func (s *Service) 是否失败时绕过() bool {
	return s.options.OnError != "block"
}

// 获取上游超时 返回上游连接超时（秒）
func (s *Service) 获取上游超时() int {
	if s.options.UpstreamTimeout <= 0 {
		return 30
	}
	return s.options.UpstreamTimeout
}

// CA已安装 返回根证书是否已成功加载
func (s *Service) CA已安装() bool {
	s.证书锁.RLock()
	defer s.证书锁.RUnlock()
	return s.根证书 != nil
}

// 获取活跃连接数 返回当前正在进行 MITM 解密的连接数
func (s *Service) 获取活跃连接数() int32 {
	return s.活跃连接数.Load()
}

// IsEnabled 导出方法：返回 MITM 服务是否已启用
// 供 daemon/libbox 等外部包调用（中文方法名首字符非 Unicode 大写字母，Go 视为未导出）
func (s *Service) IsEnabled() bool {
	return s.是否启用()
}

// IsCAInstalled 导出方法：返回根证书是否已成功加载
func (s *Service) IsCAInstalled() bool {
	return s.CA已安装()
}

// GetActiveConnections 导出方法：返回当前活跃 MITM 连接数
func (s *Service) GetActiveConnections() int32 {
	return s.获取活跃连接数()
}

// 增加连接 原子增加活跃连接计数，在拦截开始时调用
func (s *Service) 增加连接() {
	s.活跃连接数.Add(1)
}

// 减少连接 原子减少活跃连接计数，在拦截结束时调用
func (s *Service) 减少连接() {
	s.活跃连接数.Add(-1)
}
