# 《SFI 接入 sing-box MITM 完整指南》

> 本文档详细说明如何将 **sing-box MITM Service** 接入 **SFI（sing-box for iOS）客户端体系**，涵盖 Core 层修改、libbox API 导出、SFI 工程修改、Network Extension 集成、Xcode 配置、Build 流程以及完整的 API 清单。
>
> 整体架构：
>
> ```
> SFI UI
>    |
>    |
> Network Extension
>    |
>    |
> Libbox.framework
>    |
>    |
> sing-box Core
>    |
>    +-- TUN
>    +-- Router
>    +-- Outbound
>    +-- MITM Service
> ```
>
> SFI 属于 sing-box Apple 客户端体系，负责配置管理、TUN/VPN Extension 和 libbox 集成。

---

# 《SFI 接入 sing-box 内核完整指南》

> 本文档详细说明 **SFI（sing-box for iOS）客户端体系** 如何通过 **Libbox.framework** 接入 **sing-box 内核**，涵盖整体架构、Libbox 核心 API、配置管理、服务生命周期、TUN 接入、PlatformInterface 接口、CommandServerHandler 接口、MITM 专项接入、Xcode 配置、Build 流程以及完整的 API 清单。
>
> 整体架构：
>
> ```
> SFI UI（SwiftUI）
>    |
>    |  配置管理 / 状态查询 / 日志展示
>    ▼
> CommandClient（gRPC 客户端）
>    |
>    |  XPC / 本地端口通信
>    ▼
> Network Extension（PacketTunnelProvider）
>    |
>    |  LibboxSetup / LibboxNewCommandServer / startOrReloadService
>    ▼
> Libbox.framework（gomobile bind 桥接）
>    |
>    |  Go 函数 / 接口跨语言调用
>    ▼
> sing-box Core（Go）
>    |
>    +-- TUN Inbound（虚拟网卡）
>    +-- DNS（域名解析）
>    +-- Router（路由规则）
>    +-- Outbound（代理出站：vmess/vless/trojan/shadowsocks...）
>    +-- Services（服务：api/clashapi/mitm/...）
>    +-- MITM Service（HTTPS 中间人攻击）
>    +-- Rewrite Engine（HTTP 重写）
>    +-- Certificate Authority（动态证书签发）
>    +-- TLS Termination（TLS 终止）
>    +-- HTTP/1.1 + HTTP/2 + WebSocket 引擎
>    +-- Service Registry（服务注册表）
>    +-- TUN Interceptor（TUN 连接拦截器接口）
>    +-- PlatformInterface（平台接口回调）
>    +-- CommandServer（gRPC 命令服务器）
>    +-- Log Ring Buffer（日志环形缓冲区）
>    +-- OOM Killer（内存溢出保护）
>    +-- Power Report（功耗报告）
>    +-- Clash API（外部控制 API）
>    +-- V2Ray API（统计 API）
> ```
>
> SFI 属于 sing-box Apple 客户端体系，负责配置管理、TUN/VPN Extension 和 libbox 集成。本文档同时覆盖通用内核接入和 MITM 专项接入两部分内容。

---

# 一、整体架构与内核接入流程

## 1.1 三层架构

SFI 接入 sing-box 内核采用三层架构：

| 层级 | 技术 | 职责 | 运行进程 |
|------|------|------|----------|
| **UI 层** | SwiftUI + CommandClient | 配置编辑、状态展示、日志查看、连接管理 | 主 App 进程 |
| **Extension 层** | Network Extension + Libbox | VPN 隧道管理、TUN 接口提供、内核生命周期管理 | Network Extension 进程 |
| **Core 层** | sing-box Go 内核 | 协议解析、路由转发、TLS 终止、MITM 解密、出站代理 | Network Extension 进程（内嵌） |

## 1.2 内核启动完整流程

```
用户点击「连接」按钮
    ↓
SFI UI 调用 NEVPNManager.startVPNTunnel()
    ↓
系统启动 Network Extension（PacketTunnelProvider）
    ↓
PacketTunnelProvider.startTunnel(options:) 被调用
    ↓
1. 解析启动选项（configContent 等）
2. 持久化启动选项到快照文件
3. 创建 LibboxSetupOptions（BasePath/WorkingPath/TempPath...）
4. 调用 LibboxSetup(options, &error) 初始化 libbox
5. 创建 ExtensionPlatformInterface（实现 PlatformInterface 接口）
6. 调用 LibboxNewCommandServer(handler, platformInterface, &error)
7. 调用 commandServer.start() 启动 gRPC 命令服务器
8. 调用 commandServer.startOrReloadService(configContent, options) 启动 sing-box 服务
    ↓
sing-box 内核启动完成
    ↓
TUN 接口创建 → 路由规则加载 → 出站连接建立 → 流量开始转发
```

## 1.3 UI 与 Extension 通信

主 App 与 Network Extension 通过 gRPC 通信：

```
SFI UI（CommandClient）
    ↓  gRPC 调用（通过 XPC 或本地端口）
CommandServer（运行在 Extension 中）
    ↓  调用 sing-box 内核 API
sing-box Core
```

**CommandClient 主要功能：**
- 查询服务状态（`status`）
- 查询连接列表（`connections`）
- 查询/切换出站组（`group`）
- 查询日志（`logs`）
- 切换模式（`mode`）
- 网络质量测试（`network_quality`）
- STUN 测试（`stun`）
- Tailscale 管理（`tailscale`）
- USB/IP 共享（`usbip`）
- OpenConnect/OpenVPN 认证（`openconnect`/`openvpn`）

---

# 二、Libbox 核心 API

Libbox 是 sing-box 内核的跨语言桥接层，通过 `gomobile bind` 将 Go 代码编译为 `Libbox.xcframework`，供 Swift/Objective-C 直接调用。

## 2.1 初始化 API

### LibboxSetup — 初始化 libbox 服务

**Go 侧定义**（`experimental/libbox/setup.go`）：
```go
func Setup(options *SetupOptions) error
```

**Swift 调用**：
```swift
let options = LibboxSetupOptions()
options.basePath = basePath
options.workingPath = workingPath
options.tempPath = tempPath
options.appVersion = appVersion
options.appMarketingVersion = appMarketingVersion
options.logMaxLines = 1000
options.debug = false

var setupError: NSError?
LibboxSetup(options, &setupError)
if let setupError {
    throw setupError
}
```

### SetupOptions — 初始化选项结构体

| 字段 | 类型 | 说明 |
|------|------|------|
| `basePath` | `String` | 基础路径（配置文件、数据库存储目录） |
| `workingPath` | `String` | 工作路径（运行时文件、缓存目录） |
| `tempPath` | `String` | 临时路径（临时文件目录） |
| `fixAndroidStack` | `Bool` | 修复 Android 栈（iOS 不使用） |
| `commandServerListenPort` | `Int32` | CommandServer 监听端口（0=自动分配） |
| `commandServerSecret` | `String` | CommandServer 认证密钥 |
| `logMaxLines` | `Int` | 日志环形缓冲区最大行数 |
| `debug` | `Bool` | 是否启用调试模式 |
| `crashReportSource` | `String` | 崩溃报告来源标识 |
| `appVersion` | `String` | App 版本号（内部版本） |
| `appMarketingVersion` | `String` | App 营销版本号（对外显示） |
| `oomKillerEnabled` | `Bool` | 是否启用 OOM Killer |
| `oomKillerDisabled` | `Bool` | 是否禁用 OOM Killer |
| `oomMemoryLimit` | `Int64` | OOM 内存限制（字节） |
| `powerReportEnabled` | `Bool` | 是否启用功耗报告 |
| `platformMetadata` | `String` | 平台元数据（JSON 格式） |

### 其他初始化相关函数

| 函数 | 签名 | 说明 |
|------|------|------|
| `LibboxSetLocale` | `(localeID: String) -> Error?` | 设置本地化语言 |
| `LibboxVersion` | `() -> String` | 获取 sing-box 版本号 |
| `LibboxGoVersion` | `() -> String` | 获取 Go 运行时版本 |
| `LibboxReloadSetupOptions` | `(options: SetupOptions) -> Void` | 重新加载初始化选项 |

## 2.2 CommandServer — 命令服务器

CommandServer 是运行在 Network Extension 中的 gRPC 服务器，负责接收主 App 的命令调用并转发给 sing-box 内核。

### LibboxNewCommandServer — 创建命令服务器

**Go 侧定义**（`experimental/libbox/command_server.go`）：
```go
func NewCommandServer(handler CommandServerHandler, platformInterface PlatformInterface) (*CommandServer, error)
```

**Swift 调用**：
```swift
let platformInterface = ExtensionPlatformInterface(self)
var error: NSError?
commandServer = LibboxNewCommandServer(platformInterface, platformInterface, &error)
if let error {
    throw error
}
try commandServer!.start()
```

### CommandServer 主要方法

| 方法 | 签名 | 说明 |
|------|------|------|
| `start` | `() throws -> Void` | 启动 gRPC 服务器 |
| `close` | `() -> Void` | 关闭 gRPC 服务器 |
| `startOrReloadService` | `(configContent: String, options: OverrideOptions) throws -> Void` | 启动或重载 sing-box 服务 |
| `closeService` | `() throws -> Void` | 关闭 sing-box 服务 |
| `writeMessage` | `(level: Int32, message: String) -> Void` | 写入日志消息 |
| `setError` | `(message: String) -> Void` | 设置错误状态 |
| `needWIFIState` | `() -> Bool` | 是否需要 WiFi 状态 |
| `needFindProcess` | `() -> Bool` | 是否需要查找进程 |
| `pause` | `() -> Void` | 暂停服务（低电量模式） |
| `wake` | `() -> Void` | 唤醒服务 |
| `wakeNow` | `() -> Void` | 立即唤醒服务 |
| `recordScreenState` | `(on: Bool) -> Void` | 记录屏幕状态 |
| `recordLockState` | `(locked: Bool) -> Void` | 记录锁屏状态 |
| `resetNetwork` | `() -> Void` | 重置网络 |
| `updateWIFIState` | `() -> Void` | 更新 WiFi 状态 |
| `getMITMStatus` | `() -> MITMStatus?` | 查询 MITM 运行状态（MITM 专项） |
| `getMITMLogs` | `() -> MITMLogIterator?` | 获取 MITM 日志（MITM 专项） |
| `clearMITMLogs` | `() -> Void` | 清空 MITM 日志（MITM 专项） |

### OverrideOptions — 服务重载选项

```go
type OverrideOptions struct {
    // 保留字段，用于未来扩展
}
```

## 2.3 CommandClient — 命令客户端

CommandClient 是运行在主 App 中的 gRPC 客户端，用于向 Network Extension 中的 CommandServer 发送命令。

### LibboxNewStandaloneCommandClient — 创建独立命令客户端

**Go 侧定义**（`experimental/libbox/command_client.go`）：
```go
func NewStandaloneCommandClient() *CommandClient
```

**Swift 调用**：
```swift
let commandClient = LibboxNewStandaloneCommandClient()
try commandClient?.connect()
```

### CommandClient 主要方法

| 方法 | 签名 | 说明 |
|------|------|------|
| `connect` | `() throws -> Void` | 连接到 CommandServer |
| `connectWithFD` | `(fd: Int32) throws -> Void` | 通过文件描述符连接 |
| `disconnect` | `() throws -> Void` | 断开连接 |
| `serviceClose` | `() throws -> Void` | 关闭服务 |
| `serviceReload` | `() throws -> Void` | 重载服务 |
| `status` | `() throws -> StatusMessage` | 查询服务状态 |
| `connections` | `() throws -> ConnectionIterator` | 查询连接列表 |
| `group` | `(tag: String) throws -> OutboundGroup` | 查询出站组 |
| `groupSelect` | `(tag: String, selected: String) throws -> Void` | 切换出站组选中项 |
| `groupURLTest` | `(tag: String) throws -> Void` | 触发出站组 URL 测试 |
| `mode` | `() throws -> ModeList` | 查询模式列表 |
| `modeSet` | `(mode: String) throws -> Void` | 设置模式 |
| `logs` | `(level: Int32) throws -> LogIterator` | 查询日志 |
| `networkQuality` | `() throws -> NetworkQualityResult` | 网络质量测试 |
| `stun` | `(server: String) throws -> STUNResult` | STUN 测试 |
| `tailscale*` | 多种方法 | Tailscale 管理（peer/status/exit-node/ssh/taildrop/certificate） |
| `usbip*` | 多种方法 | USB/IP 共享（share/local/status） |
| `openconnect*` | 多种方法 | OpenConnect 认证 |
| `openvpn*` | 多种方法 | OpenVPN 认证 |

### LibboxSetXPCDialer — 设置 XPC 拨号器

```go
func SetXPCDialer(dialer XPCDialer)
```

用于 iOS 平台通过 XPC 与 Network Extension 通信，而不是本地 TCP 端口。

## 2.4 配置校验与格式化 API

| 函数 | 签名 | 说明 |
|------|------|------|
| `LibboxCheckConfig` | `(configContent: String) -> Error?` | 校验配置 JSON 是否合法 |
| `LibboxFormatConfig` | `(configContent: String) -> StringBox?` | 格式化配置 JSON |
| `LibboxGenerateConfigSchema` | `() -> StringBox?` | 生成配置 JSON Schema |
| `LibboxHasTunInbound` | `(configContent: String) -> Bool` | 检查配置是否包含 TUN 入站 |

**StringBox** 是 gomobile bind 中用于返回字符串的包装类型：
```swift
if let formatted = LibboxFormatConfig(configContent) {
    let formattedString = formatted.value
}
```

---

# 三、配置管理

## 3.1 配置 JSON 整体结构

sing-box 配置是一个 JSON 对象，包含以下顶层字段：

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `log` | `object` | 否 | 日志配置 |
| `dns` | `object` | 否 | DNS 配置 |
| `inbounds` | `[object]` | 是 | 入站配置列表（tun/socks/http/mixed/redirect/tproxy...） |
| `outbounds` | `[object]` | 是 | 出站配置列表（direct/block/dns/vmess/vless/trojan/shadowsocks/socks/http/wireguard/hysteria/hysteria2/tuic/shadowtls/anytls/selector/urltest/bridge/tor/ssh...） |
| `route` | `object` | 否 | 路由配置（规则/规则集/自动检测接口...） |
| `experimental` | `object` | 否 | 实验性功能配置 |
| `services` | `[object]` | 否 | 服务配置列表（api/clashapi/v2ray-api/mitm...） |
| `ntp` | `object` | 否 | NTP 时间同步配置 |
| `endpoint` | `object` | 否 | 端点配置（WireGuard 端点） |
| `acme` | `object` | 否 | ACME 证书配置 |
| `certificate` | `[object]` | 否 | 证书配置列表 |
| `certificate_provider` | `[object]` | 否 | 证书提供者列表 |
| `debug` | `object` | 否 | 调试配置 |

## 3.2 配置加载流程

```
SFI UI 编辑配置
    ↓
ProfileManager 保存配置到本地文件（或 iCloud）
    ↓
用户点击「连接」
    ↓
ExtensionProfile.generateProviderConfiguration() 读取配置内容
    ↓
配置内容通过 NEVPNProtocol.providerConfiguration["configContent"] 传递给 Extension
    ↓
PacketTunnelProvider.startTunnel(options:) 接收配置内容
    ↓
持久化配置内容到快照文件（用于崩溃后恢复）
    ↓
commandServer.startOrReloadService(configContent, options)
    ↓
libbox 内部调用 parseConfig(ctx, configContent) 解析为 option.Options
    ↓
sing-box 内核根据 option.Options 创建各个组件（inbounds/outbounds/router/dns/services...）
    ↓
服务启动完成
```

## 3.3 MITM 配置注入

MITM 配置通过 `MITMServiceManager.injectConfiguration(into:)` 方法注入到现有 profile JSON 的 `services[]` 数组中：

```swift
let mitmConfig = MITMServiceManager.shared.configuration
let injectedJSON = mitmConfig.injectConfiguration(into: originalProfileJSON)
```

注入逻辑：
1. 如果 MITM 未启用，移除 `services[]` 中 `type=mitm` 的配置
2. 如果 MITM 已启用，构建 MITM service 配置字典，追加到 `services[]` 数组
3. 序列化回 JSON 字符串

> **注意**：MITM 配置注入发生在主 App 进程中，注入后的完整 JSON 通过 VPN 启动选项传递给 Network Extension。

---

# 四、服务生命周期

## 4.1 启动流程

```
1. LibboxSetup(options) — 初始化 libbox 全局状态
2. LibboxNewCommandServer(handler, platformInterface) — 创建命令服务器
3. commandServer.start() — 启动 gRPC 服务器监听
4. commandServer.startOrReloadService(configContent, options) — 启动 sing-box 服务
   ├── parseConfig(ctx, configContent) — 解析配置 JSON
   ├── 创建日志器（log.NewContextLogger）
   ├── 创建 DNS 服务
   ├── 创建路由器（router.NewRouter）
   ├── 创建出站连接（outbound.NewManager）
   ├── 创建入站连接（inbound.NewManager）
   ├── 创建服务（service.Registry → 各个 service.NewService）
   ├── 启动各个组件（Start(stage, scope)）
   └── 服务启动完成，开始处理流量
```

## 4.2 重载流程

```
commandServer.startOrReloadService(newConfigContent, options)
    ↓
关闭旧服务（closeService）
    ↓
使用新配置启动新服务
    ↓
无缝切换（连接保持/中断取决于配置变化）
```

## 4.3 停止流程

```
用户点击「断开」
    ↓
NEVPNManager.stopVPNTunnel()
    ↓
PacketTunnelProvider.stopTunnel(reason:)
    ↓
commandServer.closeService() — 关闭 sing-box 服务
    ↓
commandServer.close() — 关闭 gRPC 服务器
    ↓
Extension 进程退出
```

## 4.4 崩溃恢复

Network Extension 崩溃后，系统会自动重启 Extension。重启后：
1. 从快照文件读取持久化的启动选项（`configContent`）
2. 重新执行启动流程
3. 恢复 VPN 连接

---

# 五、TUN 接入与 Network Extension

## 5.1 TUN 接口创建流程

sing-box 内核不直接创建 TUN 接口，而是通过 `PlatformInterface.OpenTun(options)` 回调请求 Swift 侧创建 TUN 接口：

```
sing-box 内核启动 TUN inbound
    ↓
调用 platformInterface.OpenTun(options)
    ↓
Swift 侧（ExtensionPlatformInterface）调用 NEPacketTunnelProvider.createTunnelInterface()
    ↓
创建 utun 接口，返回文件描述符（fd）
    ↓
sing-box 内核使用 fd 操作 TUN 接口
    ↓
读取 IP 包 → 解析 → 路由 → 出站 → 写入响应包
```

## 5.2 TunOptions — TUN 选项

| 字段 | 类型 | 说明 |
|------|------|------|
| `name` | `String` | 接口名称（如 `utun9`） |
| `mtu` | `Int32` | MTU（最大传输单元） |
| `inet4Address` | `StringIterator` | IPv4 地址列表 |
| `inet6Address` | `StringIterator` | IPv6 地址列表 |
| `autoRoute` | `Bool` | 是否自动设置路由 |
| `strictRoute` | `Bool` | 是否严格路由 |
| `includeInterface` | `StringIterator` | 包含的网络接口 |
| `excludeInterface` | `StringIterator` | 排除的网络接口 |
| `includeRoute` | `[String]` | 包含的路由段 |
| `excludeRoute` | `[String]` | 排除的路由段 |
| `includeAddress` | `[String]` | 包含的地址段 |
| `excludeAddress` | `[String]` | 排除的地址段 |

## 5.3 TUN Interceptor 接口（MITM 专项）

TUN 入站通过 `tun.Interceptor` 接口实现与 MITM 服务的解耦：

```go
type Interceptor interface {
    ShouldIntercept(metadata adapter.InboundContext) bool
    Intercept(ctx context.Context, conn net.Conn, metadata adapter.InboundContext, router adapter.Router, onClose N.CloseHandlerFunc)
}
```

TUN 入站在处理 TCP 连接时：
1. 通过 `service.FromContext[Interceptor](ctx)` 获取已注册的拦截器
2. 调用 `interceptor.ShouldIntercept(metadata)` 判断是否需要拦截
3. 如果需要拦截，调用 `interceptor.Intercept(...)` 接管连接
4. 如果不需要拦截，继续正常路由流程

MITM Service 在启动时同时注册为 `*Service` 和 `tun.Interceptor` 接口。

---

# 六、PlatformInterface 接口

`PlatformInterface` 是 libbox 定义的平台接口，Swift 侧必须实现此接口，为 sing-box 内核提供平台相关的功能。

## 6.1 接口方法清单

| 方法 | 签名 | 说明 | iOS 实现 |
|------|------|------|----------|
| `localDNSTransport` | `() -> LocalDNSTransport` | 本地 DNS 传输 | 使用 `NWListener` 实现 |
| `usePlatformAutoDetectInterfaceControl` | `() -> Bool` | 是否使用平台自动检测接口控制 | `true` |
| `autoDetectInterfaceControl` | `(fd: Int32) -> Error?` | 自动检测接口控制 | 绑定 socket 到默认接口 |
| `openTun` | `(options: TunOptions) -> (Int32, Error?)` | 打开 TUN 接口 | 通过 `NEPacketTunnelProvider` 创建 |
| `useProcFS` | `() -> Bool` | 是否使用 procfs（Android） | `false` |
| `findConnectionOwner` | `(ipProtocol, sourceAddress, sourcePort, destinationAddress, destinationPort) -> ConnectionOwner?` | 查找连接所有者（按进程） | iOS 不支持，返回 nil |
| `startDefaultInterfaceMonitor` | `(listener: InterfaceUpdateListener) -> Error?` | 启动默认接口监视器 | 通过 `NWPathMonitor` 实现 |
| `closeDefaultInterfaceMonitor` | `(listener: InterfaceUpdateListener) -> Error?` | 关闭默认接口监视器 | 停止 `NWPathMonitor` |
| `getInterfaces` | `() -> NetworkInterfaceIterator?` | 获取网络接口列表 | 通过 `ifaddrs` 获取 |
| `underNetworkExtension` | `() -> Bool` | 是否运行在 Network Extension 中 | `true` |
| `includeAllNetworks` | `() -> Bool` | 是否包含所有网络 | 取决于 VPN 配置 |
| `readWIFIState` | `() -> WIFIState?` | 读取 WiFi 状态（SSID/BSSID） | 通过 `NEHotspotNetwork` 获取 |
| `clearDNSCache` | `() -> Void` | 清除 DNS 缓存 | 调用系统 API |
| `sendNotification` | `(notification: Notification) -> Error?` | 发送通知 | 通过 `UNUserNotificationCenter` |
| `cancelNotification` | `(identifier: String, typeID: Int32) -> Error?` | 取消通知 | 通过 `UNUserNotificationCenter` |
| `startNeighborMonitor` | `(listener: NeighborUpdateListener) -> Error?` | 启动邻居表监视器 | iOS 不支持 |
| `closeNeighborMonitor` | `(listener: NeighborUpdateListener) -> Error?` | 关闭邻居表监视器 | iOS 不支持 |
| `registerMyInterface` | `(name: String) -> Void` | 注册自有接口 | 记录 utun 接口名 |
| `usePlatformShell` | `() -> Bool` | 是否使用平台 Shell | `false` |
| `checkPlatformShell` | `() -> Error?` | 检查平台 Shell | 返回错误 |
| `openShellSession` | `(...) -> ShellSession?` | 打开 Shell 会话 | iOS 不支持 |
| `lookupUser` | `(username: String) -> PlatformUser?` | 查找用户 | iOS 不支持 |
| `lookupSFTPServer` | `() -> String?` | 查找 SFTP 服务器 | iOS 不支持 |
| `readSystemSSHHostKey` | `() -> String?` | 读取系统 SSH 主机密钥 | iOS 不支持 |
| `tailscaleHostname` | `() -> String` | Tailscale 主机名 | 返回设备名称 |
| `usePlatformBridge` | `() -> Bool` | 是否使用平台桥接 | `false` |
| `createBridge` | `(options: BridgeOptions) -> BridgeSession?` | 创建桥接 | iOS 不支持 |
| `usePlatformAutoRedirect` | `() -> Bool` | 是否使用平台自动重定向 | `false` |
| `createAutoRedirect` | `(options: [Byte], handler: AutoRedirectHandler) -> AutoRedirectSession?` | 创建自动重定向 | iOS 不支持 |

## 6.2 关键数据结构

### ConnectionOwner — 连接所有者

| 字段 | 类型 | 说明 |
|------|------|------|
| `userId` | `Int32` | 用户 ID |
| `userName` | `String` | 用户名 |
| `processPath` | `String` | 进程路径 |
| `processPaths()` | `StringIterator` | 进程路径列表（Android） |
| `androidPackageNames()` | `StringIterator` | Android 包名列表 |

### NetworkInterface — 网络接口

| 字段 | 类型 | 说明 |
|------|------|------|
| `index` | `Int32` | 接口索引 |
| `mtu` | `Int32` | MTU |
| `name` | `String` | 接口名称 |
| `addresses` | `StringIterator` | IP 地址列表 |
| `flags` | `Int32` | 接口标志 |
| `type` | `Int32` | 接口类型（WIFI/Cellular/Ethernet/Other） |
| `dnsServer` | `StringIterator` | DNS 服务器列表 |
| `dnsSearchDomain` | `StringIterator` | DNS 搜索域列表 |
| `gateway` | `StringIterator` | 网关列表 |
| `metered` | `Bool` | 是否计费网络 |

### Notification — 通知

| 字段 | 类型 | 说明 |
|------|------|------|
| `identifier` | `String` | 通知唯一标识 |
| `typeName` | `String` | 类型名称 |
| `typeID` | `Int32` | 类型 ID |
| `title` | `String` | 标题 |
| `subtitle` | `String` | 副标题 |
| `body` | `String` | 正文 |
| `openURL` | `String` | 点击后打开的 URL |

---

# 七、CommandServerHandler 接口

`CommandServerHandler` 是 libbox 定义的命令处理器接口，Swift 侧必须实现此接口，处理来自 sing-box 内核的回调请求。

## 7.1 接口方法清单

| 方法 | 签名 | 说明 |
|------|------|------|
| `serviceStop` | `() -> Error?` | 停止服务（内核请求停止 VPN） |
| `serviceReload` | `() -> Error?` | 重载服务（内核请求重载配置） |
| `getSystemProxyStatus` | `() -> SystemProxyStatus?` | 获取系统代理状态（macOS） |
| `setSystemProxyEnabled` | `(enabled: Bool) -> Error?` | 设置系统代理开关（macOS） |
| `triggerNativeCrash` | `() -> Error?` | 触发原生崩溃（用于调试） |
| `writeDebugMessage` | `(message: String) -> Void` | 写入调试消息 |
| `connectSSHAgent` | `() -> (Int32, Error?)` | 连接 SSH Agent（返回 fd） |

## 7.2 SystemProxyStatus — 系统代理状态

| 字段 | 类型 | 说明 |
|------|------|------|
| `available` | `Bool` | 系统代理是否可用 |
| `enabled` | `Bool` | 系统代理是否已启用 |
| `httpServer` | `String` | HTTP 代理服务器地址 |
| `httpPort` | `Int32` | HTTP 代理端口 |
| `socksServer` | `String` | SOCKS 代理服务器地址 |
| `socksPort` | `Int32` | SOCKS 代理端口 |

> **注意**：iOS 不支持系统代理，这些方法在 iOS 上返回空值或错误。

---

# 八、MITM 专项接入

> 以下为 MITM（HTTPS 中间人攻击）功能的专项接入文档。MITM 是 sing-box 内核的可选服务，通过 `services[]` 数组中的 `type=mitm` 配置启用。

## 8.1 Swift 侧配置模型

文件：`MITM/MITMConfiguration.swift`

---

# 九、整体修改范围

分成两个工程：

## A. sing-box Core

负责：

```
MITM engine
TLS
HTTP
Router integration
Config
Service
```

修改：

```
sing-box/

option/
 └── mitm.go


constant/
 └── constant.go


protocol/
 └── mitm/


service/
 └── registry


experimental/libbox/
 └── API export
```

---

## B. SFI App

负责：

```
UI
配置生成
CA 管理
状态显示
Framework 更新
```

修改：

```
sing-box-for-apple/

SFI/

 ├── Views/
 ├── Settings/
 ├── Profile/
 └── Resources/


Library/

 └── Libbox bridge


Jailbreak/

 └── CA installer
```

---

# 十、Core 层修改

---

# 1. 新增 MITM Service Option

文件：

```
sing-box/option/mitm.go
```

新增：

```go
package option


type MITMServiceOptions struct {

    Enabled bool `json:"enabled"`

    CA MITMCAOptions `json:"ca"`

    Match MITMMatchOptions `json:"match"`

    Rewrite MITMRewriteOptions `json:"rewrite"`

}


type MITMCAOptions struct {

    Certificate string `json:"certificate"`

    PrivateKey string `json:"private_key"`

}
```

---

# 2. 注册 Service Type

文件：

```
constant/constant.go
```

增加：

```go
const (
    TypeMITM = "mitm"
)
```

---

# 3. Service Registry

文件：

```
include/service.go
```

增加：

```go
mitm.RegisterService(
    registry,
)
```

流程：

```
JSON

 ↓

services[]

 ↓

type=mitm

 ↓

MITMServiceOptions

 ↓

NewService()
```

---

# 4. 新增 MITM Core

目录：

```
protocol/mitm/
```

结构：

```
protocol/mitm/

service.go

interceptor.go

clienthello.go

tls.go

certificate.go

ca.go

cache.go

http1.go

http2.go

websocket.go

rewrite/
```

---

# 5. Service 生命周期

文件：

```
protocol/mitm/service.go
```

核心：

```go
type Service struct {

    ctx context.Context

    router adapter.Router

    ca *CA

    matcher *Matcher

}
```

启动：

```go
func NewService(
 ctx context.Context,
 logger log.ContextLogger,
 tag string,
 options option.MITMServiceOptions,
)(adapter.Service,error)
```

---

# 6. Router 接入

不要：

```go
route.NewRouter()
```

禁止重复创建。

应该：

```go
router :=
service.FromContext[
adapter.Router
](ctx)
```

数据流：

```
MITM

 ↓

HTTP Request

 ↓

sing-box Router

 ↓

Outbound
```

---

# 十一、libbox 修改

SFI 不直接调用 sing-box。

中间层：

```
sing-box core

        |

        |

libbox

        |

        |

Swift
```

---

# 1. 导出 MITM 状态 API

目录：

```
experimental/libbox
```

新增：

```
mitm.go
```

例如：

```go
package libbox


func GetMITMStatus()
MITMStatus
{

}
```

返回：

```go
type MITMStatus struct {

 Enabled bool

 CertificateInstalled bool

 Connections int

}
```

---

# 2. 导出 CA 操作

增加：

```go
func GenerateMITMCA(
 path string,
)
error
```

用途：

SFI UI：

```
Settings

 ↓

MITM

 ↓

Generate CA
```

---

# 十二、Libbox 编译

SFI 使用：

```
Libbox.xcframework
```

构建流程：

```
sing-box

 ↓

gomobile bind

 ↓

Libbox.framework

 ↓

xcframework

 ↓

SFI
```

当前 Apple 工程使用 libbox framework 作为核心集成方式。([Sing Box][3])

---

# 十三、SFI 工程修改

---

# 1. 增加 MITM 设置页面

位置：

```
SFI/
Views/
Settings/
```

新增：

```
MITMSettingsView.swift
```

UI：

```
设置

 ├── VPN
 |
 ├── Routing
 |
 └── MITM

      Enable

      CA Status

      Export CA

      Logs
```

---

# 2. 增加配置模型

Swift：

```
SFI/Models/
```

新增：

```swift
struct MITMSettings {

    var enabled: Bool

    var caPath:String

}
```

---

# 3. Profile JSON 生成

原：

```json
{
"inbounds":[]
}
```

增加：

```json
{
"services":[
 {
  "type":"mitm",

  "tag":"mitm",

  "options":{
    "enabled":true
  }
 }
]
}
```

---

# 十四、Network Extension 修改

位置：

```
Extension/
```

原则：

不修改 TLS。

不要：

```
Swift

 ↓

TLS parser
```

保持：

```
Packet Tunnel Provider

 ↓

libbox.Start()

 ↓

sing-box TUN

 ↓

MITM
```

---

# 十五、Jailbreak 模块

当前 Apple 工程包含 Jailbreak 相关组件。([GitHub][2])

目录：

```
Jailbreak/
```

新增：

```
MITMCAInstaller.swift
```

职责：

```
生成 CA

↓

保存

↓

调用安装流程
```

---

# 十六、CA 文件管理

路径：

推荐：

```
Library/Application Support/

SFI/

 └── MITM/

      ca.pem

      ca.key
```

不要：

```
Bundle/
```

原因：

* App 更新覆盖
* 不可写

---

# 十七、越狱版增强

越狱 SFI：

增加：

```
JailbreakDaemon
```

流程：

```
launchd

 |

sing-box daemon

 |

MITM Service

```

配置：

```
/var/mobile/singbox/
```

---

# 十八、SFI 配置同步

增加：

```
MITMConfigProvider.swift
```

职责：

Swift：

```
MITM UI

 ↓

JSON

 ↓

Profile

 ↓

libbox.Start()
```

---

# 十九、Bridge API

Swift 调用：

新增：

```swift
Libbox.getMITMStatus()
```

显示：

```
MITM

Enabled

✓ CA Installed

Connections: 12
```

---

# 二十、日志接口

增加：

```go
GetMITMLogs()
```

Swift：

```
MITM Debug Console
```

显示：

```
TLS ClientHello

SNI:
api.example.com


Certificate generated


Outbound:
proxy
```

---

# 二十一、Xcode 修改列表

## Target:

```
SFI
```

增加：

Framework:

```
Libbox.xcframework
```

Capabilities:

```
Network Extension

App Groups

VPN
```

---

# 二十二、Build 流程

完整：

```
修改 sing-box

↓

go test

↓

build libbox

↓

生成 xcframework

↓

复制 Framework

↓

Xcode build

↓

真机安装
```

---

# 二十三、Makefile 增加

建议：

```
make mitm-ios
```

执行：

```
build sing-box

+

build libbox

+

copy framework
```

例如：

```make
mitm-ios:

	go build ./...

	go run ./cmd/internal/build_libbox \
	-target apple \
	-platform ios

	cp -R \
	Libbox.xcframework \
	../sing-box-for-apple/Frameworks/
```

---

# 二十四、最终目录状态

完成后：

```
sing-box/

protocol/

 └── mitm/


experimental/libbox/

 └── mitm_api.go



sing-box-for-apple/


SFI/

 ├── MITMSettingsView.swift
 ├── MITMConfig.swift
 └── MITMStatus.swift


Jailbreak/

 └── MITMCAInstaller.swift


Frameworks/

 └── Libbox.xcframework
```

---

# 二十五、SFI 接入 API 完整清单（已实现）

> 本章节详细列出 sing-box-for-apple 分支中已实现的 MITM 接入 API、注册表、配置模型与目录结构。

---

## 18.1 Swift 侧配置模型

文件：`MITM/MITMConfiguration.swift`

### MITMConfiguration — MITM 服务配置

| 字段 | 类型 | 说明 | JSON 字段 |
|------|------|------|-----------|
| `enabled` | `Bool` | 是否启用 MITM | `enabled` |
| `ca` | `MITMCAConfiguration` | 根证书配置 | `ca` |
| `match` | `MITMMatchConfiguration` | 域名匹配规则 | `match` |
| `rewrite` | `MITMRewriteConfiguration` | 重写配置 | `rewrite` |
| `onError` | `MITMOnError` | 错误处理策略 | `on_error` |
| `upstreamTimeout` | `Int` | 上游连接超时（秒） | `upstream_timeout` |

### MITMCAConfiguration — 根证书配置

| 字段 | 类型 | 说明 | JSON 字段 |
|------|------|------|-----------|
| `certificate` | `String` | 证书文件路径（PEM） | `certificate` |
| `privateKey` | `String` | 私钥文件路径（PEM） | `private_key` |
| `isConfigured` | `Bool`（计算属性） | 是否已配置证书路径 | — |

### MITMMatchConfiguration — 域名匹配配置

> **注意**：Swift 侧配置模型包含 4 个匹配字段，但 Go 侧 `option.MITMMatchOptions` 当前仅支持 `domain` 和 `domain_suffix` 两个字段。`domain_keyword` 和 `domain_regex` 为 Swift 侧预留字段，注入 JSON 时会被 Go 侧忽略。

| 字段 | 类型 | 说明 | JSON 字段 | Go 侧支持 |
|------|------|------|-----------|-----------|
| `domain` | `[String]` | 精确匹配域名 | `domain` | ✅ 支持 |
| `domainSuffix` | `[String]` | 后缀匹配域名 | `domain_suffix` | ✅ 支持 |
| `domainKeyword` | `[String]` | 关键字匹配域名（预留） | `domain_keyword` | ❌ Go 侧暂不支持 |
| `domainRegex` | `[String]` | 正则匹配域名（预留） | `domain_regex` | ❌ Go 侧暂不支持 |
| `hasRules` | `Bool`（计算属性） | 是否有任何匹配规则 | — | — |

### MITMRewriteConfiguration — 重写配置

| 字段 | 类型 | 说明 | JSON 字段 |
|------|------|------|-----------|
| `enabled` | `Bool` | 是否启用重写 | `enabled` |
| `maxBodySize` | `Int` | 最大 Body 大小（字节，默认 10MB） | `max_body_size` |
| `rules` | `[MITMRewriteRule]` | 重写规则列表 | `rules` |

### MITMRewriteRule — 重写规则

> **注意**：Swift 侧配置模型包含 `domain` 数组和 `pathPrefix` 数组，但 Go 侧 `option.MITMRewriteRule` 当前仅支持 `domain_suffix`（数组）和 `path_prefix`（字符串）。`domain` 为 Swift 侧预留字段，`pathPrefix` 数组在注入 JSON 时取第一个元素。

| 字段 | 类型 | 说明 | JSON 字段 | Go 侧支持 |
|------|------|------|-----------|-----------|
| `id` | `UUID` | 规则唯一标识（仅 UI） | — | — |
| `name` | `String` | 规则名称（仅 UI） | — | — |
| `domain` | `[String]` | 精确匹配域名（预留） | `domain` | ❌ Go 侧暂不支持 |
| `domainSuffix` | `[String]` | 后缀匹配域名 | `domain_suffix` | ✅ 支持 |
| `pathPrefix` | `[String]` | 路径前缀匹配（注入时取第一个元素） | `path_prefix` | ✅ 支持（Go 侧为 string） |
| `method` | `[String]` | HTTP 方法匹配 | `method` | ✅ 支持 |
| `requestHeader` | `[String: String]` | 请求头添加/修改 | `request_header` | ✅ 支持 |
| `requestHeaderDelete` | `[String]` | 请求头删除 | `request_header_delete` | ✅ 支持 |
| `responseHeader` | `[String: String]` | 响应头添加/修改 | `response_header` | ✅ 支持 |
| `responseHeaderDelete` | `[String]` | 响应头删除 | `response_header_delete` | ✅ 支持 |
| `bodyReplace` | `[MITMBodyReplace]` | Body 替换规则 | `body_replace` | ✅ 支持 |

### MITMBodyReplace — Body 替换规则

| 字段 | 类型 | 说明 | JSON 字段 |
|------|------|------|-----------|
| `id` | `UUID` | 规则唯一标识 | — |
| `find` | `String` | 查找字符串 | `find` |
| `replace` | `String` | 替换字符串 | `replace` |

### MITMOnError — 错误处理策略（枚举）

> **注意**：Swift 侧枚举值为 `bypass` 和 `reject`，但 Go 侧 `option.MITMServiceOptions.OnError` 注释定义为 `bypass`（默认）和 `block`。注入 JSON 时 Swift 侧的 `reject` 会被序列化为 `"reject"`，Go 侧解析时未知值会回退到默认行为（bypass）。建议后续统一为 `block`。

| 枚举值 | 原始值 | 显示名称 | 说明 | Go 侧对应 |
|--------|--------|----------|------|-----------|
| `bypass` | `"bypass"` | 绕过（透传） | MITM 失败时直接透传原始流量 | ✅ `bypass`（默认） |
| `reject` | `"reject"` | 拒绝（断开） | MITM 失败时断开连接 | ⚠️ Go 侧为 `block`，当前不匹配 |

### MITMRuntimeStatus — MITM 运行状态

| 字段 | 类型 | 说明 |
|------|------|------|
| `enabled` | `Bool` | MITM 服务是否已启用 |
| `caInstalled` | `Bool` | 根证书是否已成功加载 |
| `activeConnections` | `Int32` | 当前活跃 MITM 连接数 |
| `unknown` | 静态常量 | 未知状态（服务未启动时） |

---

## 18.2 MITMServiceManager — 服务管理器 API

文件：`MITM/MITMServiceManager.swift`

单例类，负责 MITM 配置持久化、CA 证书管理、运行状态查询、配置注入。

### 属性

| 属性 | 类型 | 说明 |
|------|------|------|
| `shared` | `MITMServiceManager`（静态） | 共享单例实例 |
| `configuration` | `MITMConfiguration`（@Published） | MITM 配置，修改时自动保存 |
| `runtimeStatus` | `MITMRuntimeStatus`（@Published private(set)） | MITM 运行状态 |

### 方法

| 方法 | 签名 | 说明 |
|------|------|------|
| `generateCA` | `(certificatePath: String, privateKeyPath: String) -> Error?` | 调用 Libbox API 生成 MITM 根证书，成功返回 nil |
| `generateCAInAppGroup` | `() -> Result<(certificate: String, privateKey: String), Error>` | 在 App Group 共享目录中生成 CA 证书 |
| `isCAFileExists` | `Bool`（计算属性） | 检查 CA 证书文件是否存在 |
| `refreshRuntimeStatus` | `(commandServer: LibboxCommandServer?) -> Void` | 刷新 MITM 运行状态，调用 Libbox API |
| `injectConfiguration` | `(into profileJSON: String) -> String` | 将 MITM 配置注入到 sing-box profile JSON 中 |

### App Group 配置

- **App Group 标识**：`group.com.mitm.box`
- **配置文件路径**：`<App Group>/mitm-configuration.json`
- **CA 证书目录**：`<App Group>/mitm-ca/`
  - `ca.pem` — 根证书
  - `ca.key` — 私钥

---

## 18.3 Libbox 导出 API（Go 侧 → Swift 桥接）

文件：`experimental/libbox/mitm.go`（sing-box 分支）

通过 `gomobile bind` 导出为 Swift 可调用 API。

### LibboxGenerateMITMCA — 生成 CA 证书

```go
func LibboxGenerateMITMCA(certificatePath string, privateKeyPath string) error
```

**Swift 调用**：
```swift
let error = LibboxGenerateMITMCA(certificatePath, privateKeyPath)
```

**参数**：
- `certificatePath` — 证书输出路径（PEM 格式）
- `privateKeyPath` — 私钥输出路径（PEM 格式）

**返回**：成功返回 nil，失败返回错误。

### LibboxGetMITMStatus — 查询 MITM 运行状态

**Go 侧定义**（`experimental/libbox/mitm.go`）：
```go
func (s *CommandServer) GetMITMStatus() *MITMStatus
```

> 注意：此方法是 `CommandServer` 的实例方法，不是包级函数。通过 gomobile bind 导出到 Swift 时，需通过 `commandServer` 实例调用。

**Swift 调用**：
```swift
if let status = commandServer.getMITMStatus() {
    // status.enabled / status.caInstalled / status.activeConnections
}
```

**返回结构体 MITMStatus**：

| 字段 | 类型 | 说明 |
|------|------|------|
| `enabled` | `Bool` | MITM 服务是否已启用 |
| `caInstalled` | `Bool` | 根证书是否已成功加载 |
| `activeConnections` | `Int32` | 当前活跃 MITM 连接数 |

### LibboxGetMITMLogs — 获取 MITM 日志（迭代器）

**Go 侧定义**（`experimental/libbox/mitm.go`）：
```go
func (s *CommandServer) GetMITMLogs() *MITMLogIterator
```

> 注意：返回 `MITMLogIterator` 迭代器，不是 string。迭代器遵循 `Len()/HasNext()/Next()` 模式，供 Swift 端遍历日志条目，避免一次性把整个切片跨 gomobile 边界传递。

**MITMLogIterator 方法**：
| 方法 | 签名 | 说明 |
|------|------|------|
| `Len` | `() -> Int32` | 返回剩余日志条目总数 |
| `HasNext` | `() -> Bool` | 判断是否还有下一条日志 |
| `Next` | `() -> MITMLogEntry?` | 返回下一条日志条目，结束返回 nil |

**MITMLogEntry 结构体**：
| 字段 | 类型 | 说明 |
|------|------|------|
| `timestamp` | `String` | 时间戳（RFC3339 UTC 格式） |
| `level` | `Int32` | 日志级别（0=Trace, 1=Debug, 2=Info, 3=Warn, 4=Error） |
| `message` | `String` | 日志消息内容 |

### LibboxClearMITMLogs — 清空 MITM 日志

**Go 侧定义**（`experimental/libbox/mitm.go`）：
```go
func (s *CommandServer) ClearMITMLogs()
```

清空 MITM 服务内存环形缓冲区中的全部日志条目，对应 Debug Console 中的「清空日志」按钮。

---

## 18.4 Service Registry 注册表接入

### sing-box Core 侧

文件：`include/mitm.go`（sing-box 分支）

```go
package include

import (
	"github.com/sagernet/sing-box/adapter/service"
	"github.com/sagernet/sing-box/protocol/mitm"
)

// registerMITMService 注册 MITM 服务到服务注册表
func registerMITMService(registry *service.Registry) {
	mitm.RegisterService(registry)
}
```

### 注册流程

```
JSON 配置
    ↓
services[] 数组
    ↓
type = "mitm"
    ↓
option.MITMServiceOptions 解析
    ↓
mitm.NewService(ctx, logger, tag, options)
    ↓
adapter.Service 实例
    ↓
service.MustRegister[*Service](ctx, svc)
    ↓
service.MustRegister[tun.Interceptor](ctx, svc)
```

### 常量定义

文件：`constant/proxy.go`（第 49 行）

```go
const TypeMITM = "mitm"
```

---

## 18.5 TUN Interceptor 接口（解耦设计）

文件：`protocol/tun/interceptor.go`（sing-box 分支）

### 接口定义

```go
type Interceptor interface {
    // ShouldIntercept 判断是否需要拦截该连接
    ShouldIntercept(metadata M.Metadata) bool
    
    // Intercept 拦截连接，进行 MITM 处理
    Intercept(ctx context.Context, conn net.Conn, metadata adapter.InboundContext, router adapter.Router, closeHandler N.CloseHandlerFunc)
}
```

### TUN 入站接入

文件：`protocol/tun/inbound.go`

```go
// 通过 service.FromContext 获取拦截器，不直接 import mitm 包
interceptor := service.FromContext[Interceptor](ctx)
if interceptor != nil && interceptor.ShouldIntercept(metadata) {
    interceptor.Intercept(ctx, conn, metadata, router, closeHandler)
    return
}
```

### 解耦优势

- TUN 层不直接依赖 mitm 包
- 通过接口 + service registry 实现依赖反转
- MITM 服务可独立编译、测试、替换

---

## 18.6 完整配置 JSON 结构

### 启用 MITM 的 profile 配置示例

> 以下示例严格对应 Go 侧 `option.MITMServiceOptions` 的实际 JSON 字段。Swift 侧配置模型中预留的 `domain_keyword`、`domain_regex`、`domain`（rewrite 规则）字段不会出现在最终 JSON 中。

```json
{
  "services": [
    {
      "type": "mitm",
      "tag": "mitm",
      "enabled": true,
      "ca": {
        "certificate": "/path/to/ca.pem",
        "private_key": "/path/to/ca.key"
      },
      "match": {
        "domain": ["api.example.com"],
        "domain_suffix": ["example.com"]
      },
      "rewrite": {
        "enabled": true,
        "max_body_size": 10485760,
        "rules": [
          {
            "domain_suffix": ["example.com"],
            "path_prefix": "/api",
            "method": ["GET", "POST"],
            "request_header": {
              "X-Test": "rewritten"
            },
            "request_header_delete": ["X-Old-Header"],
            "response_header": {
              "X-Response": "modified"
            },
            "response_header_delete": ["X-Old-Response"],
            "body_replace": [
              {
                "find": "old-value",
                "replace": "new-value"
              }
            ]
          }
        ]
      },
      "on_error": "bypass",
      "upstream_timeout": 30
    }
  ]
}
```

### 字段说明

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `type` | `string` | 是 | 固定为 `"mitm"` |
| `tag` | `string` | 是 | 服务标签，固定为 `"mitm"` |
| `enabled` | `bool` | 是 | 是否启用 MITM |
| `ca.certificate` | `string` | 启用时必填 | 根证书 PEM 文件路径 |
| `ca.private_key` | `string` | 启用时必填 | 私钥 PEM 文件路径 |
| `match.domain` | `[string]` | 否 | 精确匹配域名列表 |
| `match.domain_suffix` | `[string]` | 否 | 后缀匹配域名列表 |
| `rewrite.enabled` | `bool` | 否 | 是否启用重写引擎 |
| `rewrite.max_body_size` | `int` | 否 | 最大 Body 大小（字节），默认 10MB |
| `rewrite.rules` | `[object]` | 否 | 重写规则列表 |
| `rewrite.rules[].domain_suffix` | `[string]` | 否 | 后缀匹配域名列表（Go 侧仅支持此字段，不支持 `domain`） |
| `rewrite.rules[].path_prefix` | `string` | 否 | 路径前缀匹配（Go 侧为 string，不是数组；Swift 侧数组注入时取第一个元素） |
| `rewrite.rules[].method` | `[string]` | 否 | HTTP 方法匹配列表 |
| `rewrite.rules[].request_header` | `object` | 否 | 需要设置/覆盖的请求头（key-value） |
| `rewrite.rules[].request_header_delete` | `[string]` | 否 | 需要删除的请求头列表 |
| `rewrite.rules[].response_header` | `object` | 否 | 需要设置/覆盖的响应头（key-value） |
| `rewrite.rules[].response_header_delete` | `[string]` | 否 | 需要删除的响应头列表 |
| `rewrite.rules[].body_replace` | `[object]` | 否 | Body 内容替换规则列表（仅对响应体生效） |
| `rewrite.rules[].body_replace[].find` | `string` | 否 | 要查找的字符串 |
| `rewrite.rules[].body_replace[].replace` | `string` | 否 | 替换为的字符串 |
| `on_error` | `string` | 否 | 错误处理策略：`bypass`（默认，回退到正常路由）或 `block`（关闭连接）。注意：Swift 侧枚举值为 `reject`，与 Go 侧 `block` 不匹配 |
| `upstream_timeout` | `int` | 否 | 上游连接超时（秒），默认 30 |

---

## 18.7 sing-box-for-apple 目录结构

```
sing-box-for-apple/
│
├── MITM/                          # MITM 功能模块（已实现）
│   ├── MITMConfiguration.swift    # 配置模型（对应 option.MITMServiceOptions）
│   ├── MITMServiceManager.swift   # 服务管理器（单例，配置持久化/CA管理/状态查询/配置注入）
│   ├── MITMView.swift             # MITM 主设置视图
│   ├── MITMSettingsPresenter.swift # 设置展示器
│   ├── MITMCAView.swift           # CA 证书管理视图
│   ├── MITMMatchView.swift        # 域名匹配规则视图
│   └── MITMRewriteView.swift      # 重写规则视图
│
├── SFI/                           # 主应用
│   ├── Application.swift
│   ├── ApplicationDelegate.swift
│   ├── MainView.swift
│   └── ...
│
├── Extension/                     # Network Extension
│   └── ...（Packet Tunnel Provider，调用 libbox.Start()）
│
├── Library/                       # 共享库
│   ├── Database/                  # 数据库（Profile 管理）
│   ├── Network/                   # 网络层（CommandServer/Extension 通信）
│   └── ...
│
├── Frameworks/
│   └── Libbox.xcframework/        # sing-box Core 框架（gomobile bind 生成）
│
├── README/
│   └── README.md                  # 本文档
│
├── Makefile                       # 构建脚本（含 libbox-mitm 目标）
├── .github/workflows/             # CI/CD 流水线
│   ├── mitm-ci.yml
│   ├── mitm-test-matrix.yml
│   ├── full-test-matrix.yml
│   ├── libbox-build.yml
│   ├── ios-deploy.yml
│   └── test.yml
│
└── ...
```

---

## 18.8 数据流完整链路

```
SFI UI（MITM 设置页面）
    │
    │  编辑 MITMConfiguration
    ▼
MITMServiceManager.configuration（@Published）
    │
    │  自动保存到 App Group
    ▼
<App Group>/mitm-configuration.json
    │
    │  VPN 启动时读取配置
    ▼
MITMServiceManager.injectConfiguration(into: profileJSON)
    │
    │  注入 services[] 数组
    ▼
完整 sing-box profile JSON
    │
    │  libbox.Start(profileJSON)
    ▼
Libbox.xcframework
    │
    │  gomobile bind 桥接
    ▼
sing-box Core
    │
    ├── Service Registry
    │       └── type=mitm → mitm.NewService()
    │
    ├── TUN Inbound
    │       └── service.FromContext[tun.Interceptor]()
    │
    ├── MITM Service
    │       ├── 读取 ClientHello（SNI/ALPN）
    │       ├── 域名匹配
    │       ├── 签发叶子证书（LRU 缓存）
    │       ├── TLS 终止
    │       ├── HTTP/1.1 引擎
    │       ├── HTTP/2 引擎
    │       ├── WebSocket 转发
    │       └── Rewrite 引擎（请求头/响应头/Body 替换）
    │
    └── Router
            └── Outbound（proxy/direct）
```

---

## 18.9 已完成 vs 待完成

### ✅ 已完成（sing-box 分支）

- [x] Phase 0：工程准备
- [x] Phase 1：MITM 配置体系（option/mitm.go）
- [x] Phase 2：Service Registry 接入
- [x] Phase 3：CA 管理模块
- [x] Phase 4：动态证书签发（含 LRU 缓存）
- [x] Phase 5：TLS ClientHello 解析
- [x] Phase 6：TUN Interceptor 接口解耦
- [x] Phase 7：MITM TLS Server
- [x] Phase 8：Router 融合
- [x] Phase 9：HTTP/1.1 引擎
- [x] Phase 10：HTTP/2 引擎
- [x] Phase 11：WebSocket 转发
- [x] Phase 12：Rewrite 引擎
- [x] Phase 13：libbox API 导出
- [x] Phase 17：Build 系统（Makefile libbox-mitm）
- [x] Phase 18：自动化测试（test/mitm/ 47 项测试 + protocol/mitm/ 165+ 项测试，覆盖率 98%）
- [x] CI/CD 体系（6 条流水线）
- [x] 生产 bug 修复（rewrite/body.go 解压/压缩失败恢复响应体）

### ✅ 已完成（sing-box-for-apple 分支）

- [x] MITM 配置模型（MITMConfiguration.swift）
- [x] MITM 服务管理器（MITMServiceManager.swift）
- [x] MITM 主设置视图（MITMView.swift）
- [x] CA 证书管理视图（MITMCAView.swift）
- [x] 域名匹配规则视图（MITMMatchView.swift）
- [x] 重写规则视图（MITMRewriteView.swift）
- [x] 设置展示器（MITMSettingsPresenter.swift）
- [x] App Group 配置持久化
- [x] Profile JSON 配置注入

### ⏳ 待完成

- [ ] **【高优先级】修正 Swift 侧配置模型与 Go 侧不匹配问题**：
  - `MITMRewriteRule.pathPrefix` 为 `[String]`，但 Go 侧 `option.MITMRewriteRule.PathPrefix` 为 `string`。注入 JSON 时数组类型会导致 Go 侧 `json.Unmarshal` 失败，MITM 服务无法启动。**修复方案**：Swift 侧注入时取 `pathPrefix.first` 转为字符串，或修改 Go 侧为数组类型。
  - `MITMOnError.reject` 原始值为 `"reject"`，但 Go 侧注释定义为 `"block"`。未知值会回退到默认 bypass 行为。**修复方案**：统一为 `"block"`。
  - `MITMMatchConfiguration.domainKeyword` 和 `domainRegex` 为 Swift 侧预留字段，Go 侧暂不支持，注入后会被忽略。
  - `MITMRewriteRule.domain` 为 Swift 侧预留字段，Go 侧暂不支持（仅支持 `domain_suffix`）。
- [ ] Phase 14：SFI 配置 UI 集成到主设置页面（需接入 MainView 导航）
- [ ] Phase 15：SFI CA 管理（证书生成/导出/安装引导）
- [ ] Phase 16：越狱版本支持（文档标注"暂时不做"，sing-box 分支已完成 deploy/ios 部署文件和 deb 构建流水线）
- [ ] Phase 19：真机测试（需 iOS 设备 + 开发者证书）
- [ ] Phase 20：最终 Release 分支合并
- [ ] Libbox.xcframework 重新编译（需 macOS + Xcode + gomobile，Linux 云电脑无法完成）
- [ ] SFI 替换 Framework 并编译验证

---

## 18.10 关键技术决策

1. **TUN 层解耦**：通过 `tun.Interceptor` 接口 + `service.FromContext` 实现 TUN 与 MITM 解耦，TUN 层不直接 import mitm 包。

2. **双重注册**：MITM Service 同时注册为 `*Service` 和 `tun.Interceptor` 接口，TUN 层通过接口获取。

3. **LRU 证书缓存**：叶子证书使用 LRU 缓存，避免重复签发，支持并发安全（双重检查锁定）。

4. **Rewrite 引擎错误恢复**：重写响应体解压/压缩失败时恢复原始响应体，避免 `resp.Write` 出现 `invalid Read on closed Body`。

5. **App Group 共享**：MITM 配置和 CA 证书存储在 App Group 共享目录，主 App 和 Network Extension 均可访问。

6. **配置注入而非替换**：MITM 配置通过 `injectConfiguration(into:)` 方法注入到现有 profile JSON 的 services[] 数组中，不破坏原有配置。

7. **全汉化**：所有 Swift 代码注释、变量名、错误提示、用户可见字符串均使用简体中文。

8. **生产级代码**：完整错误处理、边界判断、线程安全（@Published、ObservableObject）、适配深色模式和动态字体。

9. **Swift 侧与 Go 侧配置模型差异（需注意）**：
   - Go 侧 `option.MITMMatchOptions` 仅支持 `domain` 和 `domain_suffix`，不支持 `domain_keyword` 和 `domain_regex`。
   - Go 侧 `option.MITMRewriteRule` 仅支持 `domain_suffix`（数组）和 `path_prefix`（字符串），不支持 `domain`（数组），且 `path_prefix` 不是数组。
   - Go 侧 `on_error` 注释定义为 `bypass`（默认）和 `block`，Swift 侧枚举为 `bypass` 和 `reject`，两者不匹配。
   - **关键风险**：Swift 侧 `injectConfiguration` 方法会将 `path_prefix` 作为数组注入 JSON，但 Go 侧期望字符串类型，这会导致 `json.Unmarshal` 失败，MITM 服务无法启动。必须在 Swift 侧注入时取 `pathPrefix.first` 转为字符串。

---