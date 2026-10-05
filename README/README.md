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

> **注意**：`status`/`connections`/`group`/`logs`/`mode` 等状态查询不是直接的同步方法调用，而是通过 gRPC 流式订阅（`CommandClientHandler` 回调）实时推送数据。以下为实际导出的同步方法。

| 方法 | 签名 | 说明 |
|------|------|------|
| `connect` | `() throws -> Void` | 连接到 CommandServer |
| `connectWithFD` | `(fd: Int32) throws -> Void` | 通过文件描述符连接 |
| `disconnect` | `() throws -> Void` | 断开连接 |
| `serviceClose` | `() throws -> Void` | 关闭服务 |
| `serviceReload` | `() throws -> Void` | 重载服务 |
| `selectOutbound` | `(groupTag: String, outboundTag: String) throws -> Void` | 切换出站组选中项 |
| `urlTest` | `(outboundTag: String) throws -> Void` | 触发单个出站 URL 测试 |
| `setClashMode` | `(newMode: String) throws -> Void` | 设置 Clash 模式（rule/global/direct） |
| `closeConnection` | `(connId: String) throws -> Void` | 关闭单个连接 |
| `closeConnections` | `() throws -> Void` | 关闭所有连接 |
| `clearLogs` | `() throws -> Void` | 清空日志 |
| `getSystemProxyStatus` | `() throws -> SystemProxyStatus` | 获取系统代理状态（macOS） |
| `setSystemProxyEnabled` | `(isEnabled: Bool) throws -> Void` | 设置系统代理开关（macOS） |
| `triggerGoCrash` | `() throws -> Void` | 触发 Go 运行时崩溃（调试用） |
| `triggerNativeCrash` | `() throws -> Void` | 触发原生崩溃（调试用） |
| `triggerOOMReport` | `() throws -> Void` | 触发 OOM 内存报告 |
| `getDeprecatedNotes` | `() throws -> DeprecatedNoteIterator` | 获取弃用配置说明 |
| `getStartedAt` | `() throws -> Int64` | 获取服务启动时间戳 |
| `getAPIVersion` | `() throws -> Int32` | 获取 API 版本号 |
| `setGroupExpand` | `(groupTag: String, isExpand: Bool) throws -> Void` | 设置出站组 UI 展开状态 |
| `startNetworkQualityTest` | `(configURL: String, outboundTag: String, serial: Bool, maxRuntimeSeconds: Int32, http3: Bool, handler: NetworkQualityTestHandler) throws -> NetworkQualityTestSession` | 启动网络质量测试 |
| `startSTUNTest` | `(server: String, outboundTag: String, handler: STUNTestHandler) throws -> STUNTestSession` | 启动 STUN NAT 类型测试 |
| `subscribeTailscaleStatus` | `(handler: TailscaleStatusHandler) throws -> TailscaleStatusSubscription` | 订阅 Tailscale 状态 |
| `subscribeUSBIPServerStatus` | `(handler: USBIPServerStatusHandler) throws -> USBIPServerStatusSubscription` | 订阅 USB/IP 服务器状态 |
| `subscribeOpenConnectStatus` | `(handler: OpenConnectStatusHandler) throws -> OpenConnectStatusSubscription` | 订阅 OpenConnect 认证状态 |
| `submitOpenConnectAuthResponse` | `(endpointTag: String, challengeID: String, response: OpenConnectAuthResponse) throws -> Void` | 提交 OpenConnect 认证响应 |
| `cancelOpenConnectAuthChallenge` | `(endpointTag: String, challengeID: String) throws -> Void` | 取消 OpenConnect 认证挑战 |
| `subscribeOpenVPNStatus` | `(handler: OpenVPNStatusHandler) throws -> OpenVPNStatusSubscription` | 订阅 OpenVPN 认证状态 |
| `submitOpenVPNChallengeResponse` | `(endpointTag: String, challengeID: String, response: OpenVPNChallengeResponse) throws -> Void` | 提交 OpenVPN 挑战响应 |
| `cancelOpenVPNChallenge` | `(endpointTag: String, challengeID: String) throws -> Void` | 取消 OpenVPN 挑战 |

### CommandClientHandler — 命令客户端回调接口

通过 gRPC 流式订阅接收实时数据，Swift 侧需实现此接口：

| 回调方法 | 说明 |
|----------|------|
| `updateStatus(status: StatusMessage)` | 服务状态更新（内存/CPU/连接数/上行下行速率） |
| `updateConnections(connections: ConnectionIterator)` | 连接列表更新 |
| `updateGroups(groups: OutboundGroupIterator)` | 出站组状态更新 |
| `updateClashMode(mode: String)` | Clash 模式更新 |
| `writeLogs(level: Int32, message: String)` | 实时日志推送 |
| `updateOutbounds(outbounds: OutboundIterator)` | 出站列表更新 |

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

sing-box 配置是一个 JSON 对象，包含以下顶层字段（对应 `option.Options` 结构体）：

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `$schema` | `string` | 否 | JSON Schema 引用（如 `https://sing-box.sagernet.org/schema.json`） |
| `log` | `object` | 否 | 日志配置（level/output/timestamp...） |
| `dns` | `object` | 否 | DNS 配置（servers/rules/final...） |
| `ntp` | `object` | 否 | NTP 时间同步配置 |
| `certificate` | `[object]` | 否 | 证书配置列表 |
| `certificate_providers` | `[object]` | 否 | 证书提供者列表（ACME 是其中一种类型，注意是复数） |
| `http_clients` | `[object]` | 否 | HTTP 客户端配置列表 |
| `network_namespaces` | `[object]` | 否 | 网络命名空间配置列表（Linux） |
| `endpoints` | `[object]` | 否 | 端点配置列表（WireGuard 端点，注意是复数） |
| `inbounds` | `[object]` | 是 | 入站配置列表（tun/socks/http/mixed/redirect/tproxy/direct...） |
| `outbounds` | `[object]` | 是 | 出站配置列表（direct/block/dns/vmess/vless/trojan/shadowsocks/socks/http/wireguard/hysteria/hysteria2/tuic/shadowtls/anytls/selector/urltest/bridge/tor/ssh...） |
| `route` | `object` | 否 | 路由配置（rules/rule_set/auto_detect_interface/final...） |
| `services` | `[object]` | 否 | 服务配置列表（api/clashapi/v2ray-api/mitm...） |
| `experimental` | `object` | 否 | 实验性功能配置（clash_api/v2ray_api/debug...） |

> **注意**：`acme` 不是顶层字段，而是 `certificate_providers` 中 `type=acme` 的证书提供者配置。`debug` 配置位于 `experimental` 对象下。

## 3.2 完整配置 JSON 示例（含 MITM）

以下为 iOS 越狱环境下包含 MITM 功能的完整 sing-box 配置示例，涵盖日志、DNS、NTP、入站、出站、路由、服务、实验性功能等全部顶层字段：

```json
{
  "$schema": "https://sing-box.sagernet.org/schema.json",
  "log": {
    "level": "info",
    "output": "/var/log/sing-box.log",
    "timestamp": true
  },
  "dns": {
    "servers": [
      {
        "tag": "dns-direct",
        "address": "223.5.5.5",
        "detour": "direct"
      },
      {
        "tag": "dns-proxy",
        "address": "https://1.1.1.1/dns-query",
        "detour": "proxy"
      },
      {
        "tag": "dns-fakeip",
        "address": "fakeip"
      }
    ],
    "rules": [
      {
        "domain_suffix": ["cn"],
        "server": "dns-direct"
      }
    ],
    "final": "dns-proxy",
    "reverse_mapping": false,
    "strategy": "prefer_ipv4",
    "disable_cache": false,
    "disable_expire": false,
    "cache_capacity": 4096,
    "independent_cache": true,
    "client_subnet": "0.0.0.0/0",
    "fakeip": {
      "enabled": true,
      "inet4_range": "198.18.0.0/15",
      "inet6_range": "fc00::/18"
    }
  },
  "ntp": {
    "enabled": true,
    "server": "time.apple.com",
    "server_port": 123,
    "interval": "30m",
    "detour": "direct"
  },
  "inbounds": [
    {
      "type": "tun",
      "tag": "tun-in",
      "interface_name": "utun9",
      "address": ["198.18.0.1/30", "fdfe:dcba:9876::1/126"],
      "mtu": 9000,
      "auto_route": true,
      "strict_route": false,
      "stack": "gvisor",
      "dns_mode": "hijack",
      "endpoint_independent_nat": false,
      "exclude_mptcp": false,
      "route_address": ["198.18.0.0/15"],
      "route_exclude_address": ["10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"],
      "include_interface": ["en0"],
      "exclude_interface": ["utun*"],
      "loopback_address": ["127.0.0.1"],
      "dns_handler": "dns-proxy"
    },
    {
      "type": "mixed",
      "tag": "mixed-in",
      "listen": "127.0.0.1",
      "listen_port": 7890
    }
  ],
  "outbounds": [
    {
      "type": "selector",
      "tag": "proxy",
      "outbounds": ["proxy-vmess", "proxy-ss", "direct"],
      "default": "proxy-vmess"
    },
    {
      "type": "urltest",
      "tag": "proxy-auto",
      "outbounds": ["proxy-vmess", "proxy-ss"],
      "url": "http://www.gstatic.com/generate_204",
      "interval": "5m",
      "tolerance": 50
    },
    {
      "type": "vmess",
      "tag": "proxy-vmess",
      "server": "example.com",
      "server_port": 443,
      "uuid": "00000000-0000-0000-0000-000000000000",
      "security": "auto",
      "alter_id": 0,
      "global_padding": true,
      "authenticated_length": true,
      "tls": {
        "enabled": true,
        "server_name": "example.com",
        "insecure": false,
        "utls": {
          "enabled": true,
          "fingerprint": "chrome"
        }
      },
      "transport": {
        "type": "ws",
        "path": "/vmess",
        "headers": {
          "Host": "example.com"
        }
      }
    },
    {
      "type": "shadowsocks",
      "tag": "proxy-ss",
      "server": "example.com",
      "server_port": 8388,
      "method": "aes-256-gcm",
      "password": "your-password"
    },
    {
      "type": "direct",
      "tag": "direct"
    },
    {
      "type": "block",
      "tag": "block"
    },
    {
      "type": "dns",
      "tag": "dns-out"
    }
  ],
  "route": {
    "rules": [
      {
        "protocol": "dns",
        "outbound": "dns-out"
      },
      {
        "domain_suffix": ["cn"],
        "outbound": "direct"
      },
      {
        "ip_cidr": ["10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"],
        "outbound": "direct"
      },
      {
        "ip_is_private": true,
        "outbound": "direct"
      },
      {
        "source_ip_cidr": ["192.168.1.0/24"],
        "source_ip_is_private": true,
        "outbound": "direct"
      },
      {
        "port": [80, 443],
        "port_range": ["1000-2000"],
        "source_port": [1024, 65535],
        "source_port_range": ["50000-60000"],
        "outbound": "proxy"
      },
      {
        "network": ["tcp", "udp"],
        "ip_version": 4,
        "auth_user": ["user1"],
        "client": ["client1"],
        "outbound": "proxy"
      },
      {
        "process_name": ["com.apple.Safari"],
        "process_path": ["/Applications/Safari.app"],
        "outbound": "direct"
      },
      {
        "domain_suffix": ["google.com", "youtube.com"],
        "outbound": "proxy",
        "sniff": true,
        "sniff_override_destination": false,
        "sniff_timeout": "300ms",
        "udp_disable_domain_unification": false
      },
      {
        "domain_keyword": ["ads"],
        "action": "reject",
        "invert": false
      },
      {
        "domain_regex": "^.*\\.example\\.com$",
        "action": "route",
        "route_options": {
          "outbound": "proxy"
        },
        "rewrite_url": "https://new.example.com",
        "rewrite_path": "/new-path"
      },
      {
        "rule_set": ["geosite-cn"],
        "outbound": "direct"
      }
    ],
    "rule_set": [
      {
        "tag": "geosite-cn",
        "type": "remote",
        "format": "binary",
        "url": "https://raw.githubusercontent.com/SagerNet/sing-geosite/rule-set/geosite-cn.srs",
        "download_detour": "proxy"
      }
    ],
    "final": "proxy",
    "auto_detect_interface": true,
    "override_android_vpn": true
  },
  "services": [
    {
      "type": "api",
      "tag": "api",
      "listen": "127.0.0.1",
      "listen_port": 9090
    },
    {
      "type": "mitm",
      "tag": "mitm",
      "enabled": true,
      "ca": {
        "certificate": "/var/jb/etc/sing-box/ca.pem",
        "private_key": "/var/jb/etc/sing-box/ca.key"
      },
      "match": {
        "domain": ["api.example.com"],
        "domain_suffix": ["example.com", "example.net"]
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
              "X-MITM": "sing-box",
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
  ],
  "experimental": {
    "cache_file": {
      "enabled": true,
      "path": "cache.db",
      "cache_id": "sfi",
      "store_fakeip": true,
      "store_rdrc": true
    },
    "clash_api": {
      "external_controller": "127.0.0.1:9097",
      "secret": "",
      "default_mode": "rule",
      "store_mode": true,
      "store_selected": true
    },
    "v2ray_api": {
      "listen": "127.0.0.1:10085",
      "stats": {
        "enabled": true,
        "inbounds": ["tun-in"],
        "outbounds": ["proxy", "direct"]
      }
    },
    "debug": {
      "listen": "127.0.0.1:9098",
      "gc_percent": 20,
      "max_fds": 10000
    }
  }
}
```

### 配置要点说明

| 配置项 | 说明 |
|--------|------|
| `$schema` | JSON Schema 引用，用于编辑器智能提示和校验 |
| `dns.final` | 最终 DNS 服务器，未匹配规则的域名使用此服务器解析 |
| `inbounds[].type=tun` | TUN 虚拟网卡入站，iOS Network Extension 必须使用此类型 |
| `inbounds[].stack` | TUN 协议栈，可选 `gvisor`（纯 Go 实现）或 `system`（系统栈） |
| `outbounds[].type=selector` | 手动选择出站组，UI 可切换节点 |
| `outbounds[].type=urltest` | 自动测速选择最快节点 |
| `route.final` | 最终出站，未匹配规则的流量使用此出站 |
| `services[].type=api` | sing-box 原生 API 服务 |
| `services[].type=mitm` | MITM HTTPS 中间人攻击服务（本项目核心） |
| `experimental.clash_api` | Clash 兼容 API，供第三方 GUI 客户端使用 |
| `experimental.v2ray_api` | V2Ray 兼容 API，用于流量统计 |
| `experimental.debug` | 调试 API，用于 pprof 性能分析 |

### 内核支持的完整入站类型（19种）

| 类型 | 说明 | iOS 可用性 |
|------|------|-----------|
| `tun` | TUN 虚拟网卡入站，iOS Network Extension 必须使用 | ✅ 核心 |
| `redirect` | 重定向入站（透明代理） | ❌ 仅 Linux/macOS |
| `direct` | 直接入站 | ✅ |
| `socks` | SOCKS5 代理入站 | ✅ |
| `http` | HTTP 代理入站 | ✅ |
| `mixed` | SOCKS+HTTP 混合入站 | ✅ |
| `shadowsocks` | Shadowsocks 入站 | ✅ |
| `snell` | Snell 入站（Surge 协议） | ✅ |
| `vmess` | VMess 入站 | ✅ |
| `trojan` | Trojan 入站 | ✅ |
| `naive` | NaiveProxy 入站 | ✅ |
| `shadowtls` | ShadowTLS 入站 | ✅ |
| `vless` | VLESS 入站 | ✅ |
| `anytls` | AnyTLS 入站 | ✅ |
| `hysteria` | Hysteria 入站（QUIC） | ✅ |
| `hysteria2` | Hysteria2 入站（QUIC） | ✅ |
| `tuic` | TUIC 入站（QUIC） | ✅ |
| `cloudflare` | Cloudflare 入站 | ✅ |

### 内核支持的完整出站类型（19种）

| 类型 | 说明 | iOS 可用性 |
|------|------|-----------|
| `direct` | 直接出站 | ✅ |
| `block` | 阻断出站 | ✅ |
| `bridge` | 桥接出站 | ✅ |
| `selector` | 手动选择出站组 | ✅ UI 核心 |
| `urltest` | 自动测速选择出站 | ✅ |
| `socks` | SOCKS5 代理出站 | ✅ |
| `http` | HTTP 代理出站 | ✅ |
| `shadowsocks` | Shadowsocks 出站 | ✅ |
| `snell` | Snell 出站（Surge 协议） | ✅ |
| `vmess` | VMess 出站 | ✅ |
| `trojan` | Trojan 出站 | ✅ |
| `naive` | NaiveProxy 出站 | ✅ |
| `tor` | Tor 出站 | ✅ |
| `ssh` | SSH 隧道出站 | ✅ |
| `shadowtls` | ShadowTLS 出站 | ✅ |
| `vless` | VLESS 出站 | ✅ |
| `anytls` | AnyTLS 出站 | ✅ |
| `hysteria` | Hysteria 出站（QUIC） | ✅ |
| `hysteria2` | Hysteria2 出站（QUIC） | ✅ |
| `tuic` | TUIC 出站（QUIC） | ✅ |

> **注意**：WireGuard 出站已在 sing-box 1.11.0 废弃，改用 WireGuard endpoint（端点）类型。iOS 实际使用中主要依赖 `tun` 入站 + `selector`/`urltest` 出站组 + 各协议出站。

> **注意**：以上配置为示例，实际使用时需替换服务器地址、UUID、密码等敏感信息。iOS App Store 版本（非越狱）通常使用更简洁的配置，由 SFI UI 动态生成。

## 3.3 配置加载流程

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

## 3.4 MITM 配置注入

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

## 5.2 TunOptions — TUN 选项（接口）

> **注意**：`TunOptions` 是 Go 接口（interface），不是结构体。Swift 侧通过实现此接口为 sing-box 内核提供 TUN 配置信息。libbox 内部的 `tunOptions` 结构体实现此接口，从 `option.TunInboundOptions` 读取配置。

### 接口方法清单

| 方法 | 签名 | 说明 |
|------|------|------|
| `getInet4Address` | `() -> RoutePrefixIterator` | 获取 IPv4 地址列表（含前缀长度） |
| `getInet6Address` | `() -> RoutePrefixIterator` | 获取 IPv6 地址列表（含前缀长度） |
| `getDNSMode` | `() -> StringBox?` | 获取 DNS 模式（如 `local`、`remote`） |
| `getDNSServerAddress` | `() -> (StringIterator, Error?)` | 获取 DNS 服务器地址列表 |
| `getMTU` | `() -> Int32` | 获取 MTU（最大传输单元） |
| `getAutoRoute` | `() -> Bool` | 是否自动设置路由 |
| `getStrictRoute` | `() -> Bool` | 是否严格路由 |
| `getInet4RouteAddress` | `() -> RoutePrefixIterator` | 获取 IPv4 包含路由地址 |
| `getInet6RouteAddress` | `() -> RoutePrefixIterator` | 获取 IPv6 包含路由地址 |
| `getInet4RouteExcludeAddress` | `() -> RoutePrefixIterator` | 获取 IPv4 排除路由地址 |
| `getInet6RouteExcludeAddress` | `() -> RoutePrefixIterator` | 获取 IPv6 排除路由地址 |
| `getInet4RouteRange` | `() -> RoutePrefixIterator` | 获取 IPv4 路由范围 |
| `getInet6RouteRange` | `() -> RoutePrefixIterator` | 获取 IPv6 路由范围 |
| `getIncludePackage` | `() -> StringIterator` | 获取包含的应用包名（Android） |
| `getExcludePackage` | `() -> StringIterator` | 获取排除的应用包名（Android） |
| `isHTTPProxyEnabled` | `() -> Bool` | 是否启用 HTTP 代理 |
| `getHTTPProxyServer` | `() -> String` | 获取 HTTP 代理服务器地址 |
| `getHTTPProxyServerPort` | `() -> Int32` | 获取 HTTP 代理服务器端口 |
| `getHTTPProxyBypassDomain` | `() -> StringIterator` | 获取 HTTP 代理绕过域名列表 |
| `getHTTPProxyMatchDomain` | `() -> StringIterator` | 获取 HTTP 代理匹配域名列表 |

### RoutePrefix — 路由前缀结构体

| 方法 | 签名 | 说明 |
|------|------|------|
| `address` | `() -> String` | IP 地址（字符串格式） |
| `prefix` | `() -> Int32` | 前缀长度 |
| `mask` | `() -> String` | 子网掩码（字符串格式） |

### RoutePrefixIterator — 路由前缀迭代器

| 方法 | 签名 | 说明 |
|------|------|------|
| `next` | `() -> RoutePrefix?` | 返回下一个路由前缀，结束返回 nil |
| `hasNext` | `() -> Bool` | 是否还有下一个 |

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

新增完整结构体（含全部字段）：

```go
package option

// MITMServiceOptions MITM 服务配置选项
type MITMServiceOptions struct {
    Enabled         bool                `json:"enabled,omitempty"`
    CA              MITMCAOptions       `json:"ca,omitempty"`
    Match           MITMMatchOptions    `json:"match,omitempty"`
    Rewrite         MITMRewriteOptions  `json:"rewrite,omitempty"`
    OnError         string              `json:"on_error,omitempty"`  // "bypass"(默认) / "block"
    UpstreamTimeout int                 `json:"upstream_timeout,omitempty"` // 默认30秒
}

// MITMCAOptions MITM 根证书配置
type MITMCAOptions struct {
    Certificate string `json:"certificate"`
    PrivateKey  string `json:"private_key"`
}

// MITMMatchOptions MITM 域名匹配规则
type MITMMatchOptions struct {
    Domain       []string `json:"domain,omitempty"`
    DomainSuffix []string `json:"domain_suffix,omitempty"`
}

// MITMRewriteOptions HTTP 重写配置
type MITMRewriteOptions struct {
    Enabled     bool               `json:"enabled,omitempty"`
    MaxBodySize int64              `json:"max_body_size,omitempty"` // 默认10MB
    Rules       []MITMRewriteRule `json:"rules,omitempty"`
}

// MITMRewriteRule 单条 HTTP 重写规则
type MITMRewriteRule struct {
    DomainSuffix         []string              `json:"domain_suffix,omitempty"`
    PathPrefix           string                `json:"path_prefix,omitempty"`
    Method               []string              `json:"method,omitempty"`
    RequestHeader        map[string]string     `json:"request_header,omitempty"`
    RequestHeaderDelete  []string              `json:"request_header_delete,omitempty"`
    ResponseHeader       map[string]string     `json:"response_header,omitempty"`
    ResponseHeaderDelete []string              `json:"response_header_delete,omitempty"`
    BodyReplace          []MITMBodyReplaceRule `json:"body_replace,omitempty"`
}

// MITMBodyReplaceRule Body 内容替换规则
type MITMBodyReplaceRule struct {
    Find    string `json:"find"`
    Replace string `json:"replace"`
}
```

---

# 2. 注册 Service Type

文件：

```
constant/proxy.go
```

> **注意**：TypeMITM 常量定义在 `constant/proxy.go`，不是 `constant/constant.go`。

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
include/mitm.go
```

> **注意**：MITM 服务注册文件是 `include/mitm.go`，不是 `include/service.go`。

完整内容：

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

结构（源代码文件，不含测试文件）：

```
protocol/mitm/
├── service.go          # Service 核心实现，双重注册（*Service + tun.Interceptor）
├── interceptor.go      # TUN Interceptor 接口实现，ShouldIntercept/Intercept
├── clienthello.go      # TLS ClientHello 解析与 SNI 提取
├── tls.go              # TLS 终止与动态证书签发
├── certificate.go      # 证书缓存与管理
├── ca.go               # 根证书加载与叶子证书签发
├── cache.go            # 会话缓存（TLS Session Ticket）
├── log_buffer.go       # MITM 日志环形缓冲区
├── matcher.go          # 域名匹配器（domain + domain_suffix）
├── router.go           # MITM 内部路由（解密后流量转发）
├── upstream.go         # 上游连接管理（与目标服务器通信）
├── http1.go            # HTTP/1.1 处理与重写
├── http2.go            # HTTP/2 处理与重写
├── websocket.go        # WebSocket 透传处理
└── rewrite/            # HTTP 重写引擎
    ├── engine.go       # 重写引擎核心
    ├── matcher.go      # 重写规则匹配器
    ├── rule.go         # 重写规则定义
    └── body.go         # Body 替换（含 gzip/br/zstd 解压重压缩）
```

> **测试文件**：目录下另有 20+ 个 `*_test.go` 测试文件，覆盖率 98%，详见 sing-box 分支 `test/mitm/` 独立测试目录。

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
experimental/libbox/mitm.go
```

> **注意**：MITM 相关 API 分为两类：
> - **包级函数**：`GenerateMITMCA`（不需要运行中的服务实例）
> - **CommandServer 方法**：`GetMITMStatus`/`GetMITMLogs`/`ClearMITMLogs`（需要运行中的服务实例，通过 `commandServer.xxx()` 调用）

## 1.1 MITMStatus 结构体

```go
type MITMStatus struct {
    // Enabled MITM 服务是否已启用
    Enabled bool
    // CAInstalled 根证书是否已成功加载
    CAInstalled bool
    // ActiveConnections 当前正在进行 MITM 解密的活跃连接数
    ActiveConnections int32
}
```

## 1.2 GetMITMStatus 方法（CommandServer 方法）

```go
// GetMITMStatus 获取 MITM 服务运行状态
// 此方法是 CommandServer 的方法，不是包级函数
// 服务未启动或未配置 MITM 时返回零值（Enabled=false）
func (s *CommandServer) GetMITMStatus() *MITMStatus
```

Swift 调用示例：

```swift
let status = commandServer.getMITMStatus()
if status.enabled {
    print("MITM 已启用，活跃连接：\(status.activeConnections)")
}
```

## 1.3 MITM 日志 API（CommandServer 方法）

```go
// MITMLogEntry MITM 日志条目
type MITMLogEntry struct {
    Timestamp string  // RFC3339 UTC 格式
    Level     int32   // 0=Trace, 1=Debug, 2=Info, 3=Warn, 4=Error
    Message   string
}

// MITMLogIterator MITM 日志迭代器（Len/HasNext/Next 模式）
type MITMLogIterator struct { ... }

// GetMITMLogs 获取 MITM 服务日志（返回迭代器）
func (s *CommandServer) GetMITMLogs() *MITMLogIterator

// ClearMITMLogs 清空 MITM 服务日志
func (s *CommandServer) ClearMITMLogs()
```

---

# 2. 导出 CA 操作

## GenerateMITMCA 包级函数

```go
// GenerateMITMCA 生成 MITM 根证书与私钥
// 使用 ECDSA P-256 曲线生成自签名 CA 根证书
// 证书有效期 10 年，具备 CA:TRUE 基本约束和 keyCertSign 密钥用途
//
// 参数：
//   - certificatePath: 证书输出文件路径（PEM 格式）
//   - privateKeyPath: 私钥输出文件路径（PEM 格式，权限 0o600）
func GenerateMITMCA(certificatePath string, privateKeyPath string) error
```

> **注意**：此函数是**包级函数**，不需要运行中的服务实例，可在 SFI UI 进程中直接调用。

用途：

SFI UI：

```
Settings
    ↓
   MITM
    ↓
Generate CA
```

Swift 调用示例：

```swift
let 证书路径 = "\(NSHomeDirectory())/Documents/mitm-ca.pem"
let 私钥路径 = "\(NSHomeDirectory())/Documents/mitm-ca.key"
do {
    try LibboxGenerateMITMCA(证书路径, privateKeyPath: 私钥路径)
    print("CA 证书生成成功")
} catch {
    print("CA 证书生成失败：\(error)")
}
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
JailbreakDaemon（通过 deb 包部署的 sing-box 守护进程）
```

流程：

```
launchd
   |
sing-box daemon（/var/jb/usr/bin/sing-box）
   |
MITM Service
```

配置与文件路径（根less越狱，/var/jb 前缀）：

```
/var/jb/etc/sing-box/
├── config.json    # sing-box 配置（含 MITM 服务）
├── ca.pem         # MITM 根证书
└── ca.key         # MITM 根证书私钥

/var/jb/Library/LaunchDaemons/
└── com.sb1.mitm.plist   # launchd 守护进程配置

/var/jb/usr/bin/
└── sing-box              # sing-box 二进制（darwin/arm64）
```

deb 包信息：
- 包名：`com.sb1.mitm`
- 版本：`1.11.0-1`
- 架构：`iphoneos-arm64`
- 产物：`deploy/ios/output/com.sb1.mitm_1.11.0-1_iphoneos-arm64.deb`
- CI 流水线：`.github/workflows/mitm-ios-deb.yml`（Linux 交叉编译）

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

sing-box 分支 Makefile 已新增以下 MITM 相关构建目标：

## 23.1 libbox-mitm — 构建 MITM 版本 Libbox.framework

```bash
make libbox-mitm
```

等价于 `make libbox-ios`，仅构建 iOS 真机（arm64）切片的 Libbox.framework，作为 MITM 专项构建目标提供语义化入口。

实际定义：

```make
# libbox-mitm 构建 MITM 版本的 Libbox.framework（iOS 真机专用）
# 等价于 libbox-ios，作为 MITM 专项构建目标提供语义化入口
libbox-mitm: libbox-ios
```

## 23.2 libbox-apple — 构建全平台 Libbox.xcframework

```bash
make libbox-apple
```

构建全平台 Apple 切片（iOS/iOS Simulator/macOS/macOS Catalyst）的 Libbox.xcframework，对应 CI 工作流 `.github/workflows/libbox-build.yml`。

## 23.3 mitm-ios-deb — 构建 iOS 越狱版 .deb 安装包

```bash
make mitm-ios-deb
```

Linux 交叉编译 darwin/arm64 构建 sing-box 二进制，打包为越狱版 .deb 安装包。

产物：`deploy/ios/output/com.sb1.mitm_1.11.0-1_iphoneos-arm64.deb`

对应 CI 工作流：`.github/workflows/mitm-ios-deb.yml`

---

# 二十四、最终目录状态

## 24.1 sing-box 分支（Go 核心代码）

```
sing-box/
├── option/
│   └── mitm.go                          # MITM 配置选项（6个结构体）
├── constant/
│   └── proxy.go                         # TypeMITM = "mitm" 常量
├── include/
│   └── mitm.go                          # MITM 服务注册入口
├── protocol/
│   ├── mitm/                            # MITM 核心实现（14个源文件 + rewrite/子目录）
│   │   ├── service.go                   # Service 核心，双重注册
│   │   ├── interceptor.go               # TUN Interceptor 实现
│   │   ├── clienthello.go               # TLS ClientHello 解析
│   │   ├── tls.go                       # TLS 终止与动态证书
│   │   ├── certificate.go               # 证书缓存管理
│   │   ├── ca.go                        # 根证书加载与签发
│   │   ├── cache.go                     # 会话缓存
│   │   ├── log_buffer.go                # 日志环形缓冲区
│   │   ├── matcher.go                   # 域名匹配器
│   │   ├── router.go                    # MITM 内部路由
│   │   ├── upstream.go                  # 上游连接管理
│   │   ├── http1.go                     # HTTP/1.1 处理
│   │   ├── http2.go                     # HTTP/2 处理
│   │   ├── websocket.go                 # WebSocket 透传
│   │   └── rewrite/                     # HTTP 重写引擎
│   │       ├── engine.go
│   │       ├── matcher.go
│   │       ├── rule.go
│   │       └── body.go
│   └── tun/
│       └── interceptor.go               # TUN Interceptor 接口定义
├── experimental/libbox/
│   └── mitm.go                          # MITM libbox 导出 API
├── cmd/sing-box/
│   └── cmd_generate_mitm_ca.go          # sing-box generate mitm-ca 命令
├── test/mitm/                           # MITM 独立测试目录（9个测试文件）
├── deploy/ios/                          # iOS 越狱部署文件
│   ├── build-deb.sh                     # deb 包构建脚本
│   ├── build-ios.sh                     # iOS 交叉编译脚本
│   ├── install.sh                       # 越狱设备安装脚本
│   ├── com.sb1.mitm.plist              # LaunchDaemon 配置
│   ├── config/config.json               # MITM 完整配置示例
│   ├── deb/                             # deb 包目录结构
│   │   └── DEBIAN/                     # 控制脚本（control/preinst/postinst/prerm/postrm）
│   └── output/                          # 构建产物（.deb 文件）
└── Makefile                             # 新增 libbox-mitm / mitm-ios-deb 目标
```

## 24.2 sing-box-for-apple 分支（iOS 客户端 Swift 代码）

```
sing-box-for-apple/
├── MITM/                                # MITM 功能模块（7个 Swift 文件）
│   ├── MITMConfiguration.swift          # 配置模型（与 Go 侧 option/mitm.go 对齐）
│   ├── MITMServiceManager.swift         # 服务管理器（状态查询/日志获取/CA生成）
│   ├── MITMView.swift                   # MITM 主页面
│   ├── MITMSettingsPresenter.swift      # 设置页面 Presenter
│   ├── MITMCAView.swift                 # CA 证书管理页面
│   ├── MITMMatchView.swift              # 域名匹配规则页面
│   └── MITMRewriteView.swift            # HTTP 重写规则页面
├── Jailbreak/                           # 越狱模块
│   └── MITMCAInstaller.swift            # 越狱环境 CA 安装器
├── Frameworks/
│   └── Libbox.xcframework               # sing-box 内核框架（含 MITM API）
└── README/
    └── README.md                        # 本文档（SFI 接入 sing-box 内核完整指南）
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