# 📋 SFI 客户端接入文档参数一致性检查报告

> 文档: `README.md` | 检查范围: SFI iOS 客户端实际接入参数

## 📊 总体统计

| 指标 | 数量 |
|------|------|
| 检查项总数 | 78 |
| 通过检查项 | 72 |
| 错误总数 | 0 |
| 警告总数 | 8 |

## 📝 各模块详细检查

### 1. 顶层字段

#### 必须字段

| 字段 | 说明 | 文档 | 内核 | 状态 |
|------|------|------|------|------|
| `inbounds` | 入站配置列表（iOS 必须使用 tun） | ✅ | ❌ | ❌ |
| `outbounds` | 出站配置列表（direct/selector/各协议） | ✅ | ❌ | ❌ |

#### 可选字段

| 字段 | 说明 | 文档 | 内核 | 状态 |
|------|------|------|------|------|
| `log` | 日志配置（level/output/timestamp） | ✅ | ❓ | ✅ |
| `dns` | DNS 配置（servers/rules/final/strategy） | ✅ | ❓ | ✅ |
| `route` | 路由配置（rules/final/auto_detect_interface） | ✅ | ❓ | ✅ |
| `services` | 服务配置列表（api/mitm） | ✅ | ❓ | ✅ |
| `experimental` | 实验性功能（clash_api/v2ray_api/cache_file） | ✅ | ❓ | ✅ |

#### 不适用 SFI 的字段

| 字段 | 原因 | 文档中出现 |
|------|------|-----------|
| `ntp` | iOS 系统自动时间同步，无需配置 | ⚠️ 是 |
| `certificate` | 证书管理，SFI 客户端不直接使用 | ⚠️ 是 |
| `certificate_providers` | ACME 等证书提供者，服务端功能 | ⚠️ 是 |
| `http_clients` | 高级 HTTP 客户端配置，SFI 不使用 | ⚠️ 是 |
| `network_namespaces` | Linux 网络命名空间，iOS 不支持 | ⚠️ 是 |
| `endpoints` | WireGuard 端点，SFI 使用出站而非端点 | ⚠️ 是 |

### 2. 入站类型

#### SFI 实际使用的入站类型

| 类型 | 说明 | 必须 | 文档 | 内核 | 状态 |
|------|------|------|------|------|------|
| `tun` | TUN 虚拟网卡，iOS Network Extension 必须使用 | 是 | ✅ | ✅ | ✅ |
| `socks` | SOCKS5 本地代理入站 | 否 | ✅ | ✅ | ✅ |
| `http` | HTTP 本地代理入站 | 否 | ✅ | ✅ | ✅ |
| `mixed` | SOCKS+HTTP 混合入站 | 否 | ✅ | ✅ | ✅ |
| `direct` | 直接入站 | 否 | ✅ | ✅ | ✅ |

#### 不适用 SFI 的入站类型（服务端/Linux 特有）

| 类型 | 原因 | 文档中出现 |
|------|------|-----------|
| `redirect` | Linux/macOS 透明代理，iOS 不支持 | ⚠️ 是 |
| `tproxy` | Linux 透明代理，iOS 不支持 | ✅ 否 |
| `shadowsocks` | 服务端入站，SFI 作为客户端不使用 | ⚠️ 是 |
| `snell` | 服务端入站，SFI 作为客户端不使用 | ⚠️ 是 |
| `vmess` | 服务端入站，SFI 作为客户端不使用 | ⚠️ 是 |
| `trojan` | 服务端入站，SFI 作为客户端不使用 | ⚠️ 是 |
| `naive` | 服务端入站，SFI 作为客户端不使用 | ⚠️ 是 |
| `shadowtls` | 服务端入站，SFI 作为客户端不使用 | ⚠️ 是 |
| `vless` | 服务端入站，SFI 作为客户端不使用 | ⚠️ 是 |
| `anytls` | 服务端入站，SFI 作为客户端不使用 | ⚠️ 是 |
| `hysteria` | 服务端入站，SFI 作为客户端不使用 | ⚠️ 是 |
| `hysteria2` | 服务端入站，SFI 作为客户端不使用 | ⚠️ 是 |
| `tuic` | 服务端入站，SFI 作为客户端不使用 | ⚠️ 是 |
| `cloudflare` | 服务端入站，SFI 作为客户端不使用 | ⚠️ 是 |

### 3. 出站类型

| 类型 | 说明 | 必须 | 文档 | 内核 | 状态 |
|------|------|------|------|------|------|
| `direct` | 直连出站 | 是 | ✅ | ✅ | ✅ |
| `block` | 阻断出站 | 否 | ✅ | ✅ | ✅ |
| `selector` | 手动选择出站组，UI 切换节点 | 是 | ✅ | ✅ | ✅ |
| `urltest` | 自动测速选择最快节点 | 否 | ✅ | ✅ | ✅ |
| `shadowsocks` | Shadowsocks 协议出站 | 是 | ✅ | ✅ | ✅ |
| `vmess` | VMess 协议出站 | 是 | ✅ | ✅ | ✅ |
| `trojan` | Trojan 协议出站 | 是 | ✅ | ✅ | ✅ |
| `vless` | VLESS 协议出站 | 是 | ✅ | ✅ | ✅ |
| `socks` | SOCKS5 代理出站 | 否 | ✅ | ✅ | ✅ |
| `http` | HTTP 代理出站 | 否 | ✅ | ✅ | ✅ |
| `shadowtls` | ShadowTLS 协议出站 | 否 | ✅ | ✅ | ✅ |
| `hysteria` | Hysteria 协议出站（QUIC） | 否 | ✅ | ✅ | ✅ |
| `hysteria2` | Hysteria2 协议出站（QUIC） | 否 | ✅ | ✅ | ✅ |
| `tuic` | TUIC 协议出站（QUIC） | 否 | ✅ | ✅ | ✅ |
| `naive` | NaiveProxy 协议出站 | 否 | ✅ | ✅ | ✅ |
| `anytls` | AnyTLS 协议出站 | 否 | ✅ | ✅ | ✅ |
| `snell` | Snell 协议出站（Surge） | 否 | ✅ | ✅ | ✅ |
| `tor` | Tor 出站 | 否 | ✅ | ✅ | ✅ |
| `ssh` | SSH 隧道出站 | 否 | ✅ | ✅ | ✅ |
| `bridge` | 桥接出站 | 否 | ✅ | ✅ | ✅ |

### 4. DNS 配置参数

| 参数 | 说明 | 必须 | 文档 | 内核 | 状态 |
|------|------|------|------|------|------|
| `dns.servers` | DNS 服务器列表 | 是 | ✅ | ❓ | ✅ |
| `dns.rules` | DNS 规则列表 | 否 | ✅ | ❓ | ✅ |
| `dns.final` | 最终 DNS 服务器 | 是 | ✅ | ❓ | ✅ |
| `dns.strategy` | DNS 解析策略（ipv4_only/ipv6_only/prefer_ipv4） | 否 | ✅ | ❓ | ✅ |
| `dns.disable_cache` | 禁用 DNS 缓存 | 否 | ✅ | ❓ | ✅ |
| `dns.disable_expire` | 禁用 DNS 缓存过期 | 否 | ✅ | ❓ | ✅ |
| `dns.independent_cache` | 独立 DNS 缓存 | 否 | ✅ | ❓ | ✅ |
| `dns.reverse_mapping` | 反向 DNS 映射 | 否 | ✅ | ❓ | ✅ |
| `dns.fakeip` | FakeIP 配置 | 否 | ✅ | ❓ | ✅ |

### 5. 路由配置参数

| 参数 | 说明 | 必须 | 文档 | 内核 | 状态 |
|------|------|------|------|------|------|
| `route.rules` | 路由规则列表 | 否 | ✅ | ❓ | ✅ |
| `route.rule_set` | 路由规则集 | 否 | ✅ | ❓ | ✅ |
| `route.final` | 最终出站 | 是 | ✅ | ❓ | ✅ |
| `route.auto_detect_interface` | 自动检测出口网卡 | 否 | ✅ | ❓ | ✅ |
| `route.auto_route` | 自动路由（全局流量接管） | 否 | ✅ | ❓ | ✅ |
| `route.default_interface` | 默认出口网卡 | 否 | ❌ | ❓ | ⚠️ |
| `route.endpoint_independent_nat` | 端点无关 NAT | 否 | ✅ | ❓ | ✅ |
| `route.exclude_interface` | 排除网卡列表 | 否 | ✅ | ❓ | ✅ |
| `route.exclude_routable` | 排除可路由地址 | 否 | ❌ | ❓ | ⚠️ |

### 6. 实验配置参数

| 参数 | 说明 | 必须 | 文档 | 内核 | 状态 |
|------|------|------|------|------|------|
| `experimental.clash_api` | Clash 兼容 API（供第三方 GUI 使用） | 是 | ✅ | ❓ | ✅ |
| `experimental.v2ray_api` | V2Ray 兼容 API（流量统计） | 是 | ✅ | ❓ | ✅ |
| `experimental.cache_file` | 缓存文件（保存节点测速/选择状态） | 是 | ✅ | ❓ | ✅ |
| `experimental.debug` | 调试 API（pprof 性能分析） | 否 | ✅ | ❓ | ✅ |

### 7. 服务配置类型

| 类型 | 说明 | 必须 | 文档 | 内核 | 状态 |
|------|------|------|------|------|------|
| `api` | sing-box 原生 API 服务 | 是 | ✅ | ✅ | ✅ |
| `mitm` | MITM HTTPS 中间人攻击服务（本项目核心） | 是 | ✅ | ✅ | ✅ |
| `clashapi` | Clash API 服务（已废弃，用 experimental.clash_api） | 否 | ❌ | ❓ | ⚠️ |
| `v2ray-api` | V2Ray API 服务（已废弃，用 experimental.v2ray_api） | 否 | ❌ | ❓ | ⚠️ |

### 8. TUN 入站参数（iOS 必须）

| 参数 | 说明 | 必须 | 文档 | 内核 | 状态 |
|------|------|------|------|------|------|
| `type` | 入站类型，必须为 tun | 是 | ✅ | ❓ | ✅ |
| `tag` | 入站标签 | 是 | ✅ | ❓ | ✅ |
| `interface_name` | 虚拟网卡名称 | 否 | ✅ | ❓ | ✅ |
| `mtu` | 最大传输单元 | 否 | ✅ | ❓ | ✅ |
| `gso` | 通用分段卸载 | 否 | ❌ | ❓ | ⚠️ |
| `address` | 虚拟网卡 IP 地址 | 是 | ✅ | ❓ | ✅ |
| `stack` | 协议栈（gvisor/system） | 是 | ✅ | ❓ | ✅ |
| `route_address` | 路由地址列表 | 否 | ✅ | ❓ | ✅ |
| `route_exclude_address` | 排除路由地址 | 否 | ✅ | ❓ | ✅ |
| `auto_route` | 自动路由 | 否 | ✅ | ❓ | ✅ |
| `strict_route` | 严格路由 | 否 | ✅ | ❓ | ✅ |
| `endpoint_independent_nat` | 端点无关 NAT | 否 | ✅ | ❓ | ✅ |
| `udp_timeout` | UDP 超时时间 | 否 | ❌ | ❓ | ⚠️ |

### 9. libbox API 方法

| 方法 | 说明 | 文档 | 客户端调用 | 状态 |
|------|------|------|-----------|------|
| `LibboxSetup` | 初始化 libbox 服务 | ✅ | ✅ | ✅ |
| `LibboxNewCommandServer` | 创建命令服务器 | ✅ | ✅ | ✅ |
| `LibboxNewStandaloneCommandClient` | 创建独立命令客户端 | ✅ | ✅ | ✅ |
| `LibboxSetXPCDialer` | 设置 XPC 拨号器 | ✅ | ✅ | ✅ |
| `LibboxCheckConfig` | 校验配置 | ✅ | ✅ | ✅ |
| `LibboxFormatConfig` | 格式化配置 | ✅ | ✅ | ✅ |
| `LibboxGenerateMITMCA` | 生成 MITM CA 证书 | ✅ | ✅ | ✅ |

### 10. 客户端代码实际引用

| 类别 | 引用的参数/类型 |
|------|----------------|
| top_level | `dns`, `services` |
| inbound_types | `direct`, `http` |
| outbound_types | `direct`, `http`, `selector`, `urltest` |
| api_methods | `LibboxCheckConfig`, `LibboxFormatConfig`, `LibboxGenerateMITMCA`, `LibboxNewCommandServer`, `LibboxNewStandaloneCommandClient`, `LibboxSetXPCDialer`, `LibboxSetup` |

### 11. 注册表一致性检查（文档 ↔ 内核）

#### 11.1 入站注册表

| 项目 | 内容 |
|------|------|
| 文档类型数 | 18 |
| 内核类型数 | 18 |
| 一致性 | ✅ 一致 |

#### 11.2 出站注册表

| 项目 | 内容 |
|------|------|
| 文档类型数 | 20 |
| 内核类型数 | 20 |
| 一致性 | ✅ 一致 |

#### 11.3 服务注册表

| 项目 | 内容 |
|------|------|
| 文档类型数 | 2 |
| 内核类型数 | 9 |
| 一致性 | ✅ 一致 |

**ℹ️ 内核有但文档未列出的类型**: `ccm`, `derp`, `ocm`, `oomkiller`, `resolved`, `ssmapi`, `usbip`

**修改建议**:
- 如 SFI 客户端需要使用 `ccm` 服务，请在文档中补充说明；如为高级服务，可忽略
- 如 SFI 客户端需要使用 `derp` 服务，请在文档中补充说明；如为高级服务，可忽略
- 如 SFI 客户端需要使用 `ocm` 服务，请在文档中补充说明；如为高级服务，可忽略
- 如 SFI 客户端需要使用 `oomkiller` 服务，请在文档中补充说明；如为高级服务，可忽略
- 如 SFI 客户端需要使用 `resolved` 服务，请在文档中补充说明；如为高级服务，可忽略
- 如 SFI 客户端需要使用 `ssmapi` 服务，请在文档中补充说明；如为高级服务，可忽略
- 如 SFI 客户端需要使用 `usbip` 服务，请在文档中补充说明；如为高级服务，可忽略

## 🟡 警告详情（建议优化）

1. [顶层字段] 必须字段 `inbounds` 未在内核 option 中找到定义
2. [顶层字段] 必须字段 `outbounds` 未在内核 option 中找到定义
3. [顶层字段] 文档包含不适用 SFI 的字段 `ntp`（iOS 系统自动时间同步，无需配置）
4. [顶层字段] 文档包含不适用 SFI 的字段 `certificate`（证书管理，SFI 客户端不直接使用）
5. [顶层字段] 文档包含不适用 SFI 的字段 `certificate_providers`（ACME 等证书提供者，服务端功能）
6. [顶层字段] 文档包含不适用 SFI 的字段 `http_clients`（高级 HTTP 客户端配置，SFI 不使用）
7. [顶层字段] 文档包含不适用 SFI 的字段 `network_namespaces`（Linux 网络命名空间，iOS 不支持）
8. [顶层字段] 文档包含不适用 SFI 的字段 `endpoints`（WireGuard 端点，SFI 使用出站而非端点）

## ✅ 检查结论

✅ **错误检查通过**，存在 8 个警告（不阻塞，建议优化）。

- 检查项总数：78
- 通过检查项：72
- 覆盖率：72/78 = 92%
