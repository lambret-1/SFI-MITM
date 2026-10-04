# 《SFI 接入 sing-box MITM 的源码修改清单》

> 目标：将前面设计的 **sing-box MITM Service** 接入 **SFI（sing-box for iOS）客户端体系**，形成：
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
> SFI 当前属于 sing-box Apple 客户端体系，负责配置管理、TUN/VPN Extension 和 libbox 集成；

---

# 一、源码仓库结构 

工作区：

```
workspace/

├── sing-box/
│
│   ├── option/
│   ├── protocol/
│   │    └── mitm/
│   ├── service/
│   ├── experimental/libbox/
│   └── cmd/
│
│
└── sing-box-for-apple/
    │
    ├── SFI/
    │
    ├── Extension/
    │
    ├── Library/
    │
    ├── Jailbreak/
    │
    └── Frameworks/
         └── Libbox.xcframework
```

`sng-box-for-apple` 当前包含 SFI、Extension、Jailbreak 等 Apple 平台组件。

---

# 二、整体修改范围

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

# 三、Core 层修改

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

# 四、libbox 修改

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

# 五、Libbox 编译

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

# 六、SFI 工程修改

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

# 七、Network Extension 修改

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

# 八、Jailbreak 模块

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

# 九、CA 文件管理

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

# 十、越狱版增强

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

# 十一、SFI 配置同步

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

# 十二、Bridge API

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

# 十三、日志接口

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

# 十四、Xcode 修改列表

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

# 十五、Build 流程

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

# 十六、Makefile 增加

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

# 十七、测试矩阵

## Core

```
go test ./...
```

测试：

* JSON
* Service
* CA
* Certificate
* TLS

---

## SFI

测试：

```
启动 VPN

↓

访问 HTTPS

↓

MITM 捕获

↓

Router 分流

↓

Outbound
```

---

# 十八、最终目录状态

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

# 十九、推荐开发顺序

不要同时修改两个仓库。

顺序：

```
Step 1

sing-box MITM Service

↓

Step 2

libbox export API

↓

Step 3

build Libbox.xcframework

↓

Step 4

SFI 替换 Framework

↓

Step 5

SFI UI

↓

Step 6

Jailbreak CA

↓

Step 7

真机测试
```

---

# 二十、最终目标

完成后 SFI：

```
SFI App

 |

VPN Extension

 |

libbox

 |

sing-box

 |

+----------------+

| TUN            |

| MITM Service   |

| Router         |

| Outbound       |

+----------------+

 |

Internet
```

实现：

* SFI 原生配置 MITM
* sing-box Router 保持一致
* TUN 透明代理
* HTTPS interception
* 动态证书
* HTTP/2
* WebSocket
* iOS 越狱部署（暂时不做）

---




# 《sing-box MITM + SFI 完整阶段源码 Patch 计划》

> 目标：基于 `sing-box testing` 分支 + `sing-box-for-apple`（SFI）工程，实现完整 MITM 能力接入。
> 最终产物：
>
> ```
> SFI.app
>    |
>    |
> Libbox.xcframework
>    |
>    |
> sing-box MITM Core
>    |
>    +── TUN
>    +── TLS MITM
>    +── HTTP/2
>    +── WebSocket
>    +── Rewrite
>    +── Router
>    +── Outbound
> ```
>
> SFI 当前架构本身就是 Apple UI + Network Extension + Libbox Core 模式，MITM 应放在 Core 层，而不是 Swift 网络层。

---

# 总体 Git 分支规划

建议维护两个仓库：

## sing-box fork

```text
sing-box

main
 |
 └── feature/mitm-core
```

---

## SFI fork

```text
sing-box-for-apple

dev
 |
 └── feature/mitm-support
```

---

# Patch 总览

最终拆成：

```text
0001  MITM configuration model
0002  MITM service registry
0003  MITM CA engine
0004  MITM certificate generator
0005  TLS ClientHello interceptor
0006  TUN interception interface
0007  MITM router integration
0008  HTTP/1.1 proxy engine
0009  HTTP/2 engine
0010  WebSocket relay
0011  Rewrite engine
0012  libbox MITM API
0013  SFI configuration UI
0014  SFI CA manager
0015  Jailbreak CA deployment
0016  Integration tests
```

---

# Phase 0：工程准备

## Patch 0000

### 建立开发环境

目录：

```text
workspace/

├── sing-box
└── sing-box-for-apple
```

确认：

```bash
go version

xcodebuild -version
```

---

生成 baseline：

```bash
git checkout testing

git tag mitm-base
```

---

# Phase 1：MITM 配置体系

## Commit 0001

### 新增：

```
option/mitm.go
```

---

代码：

```go
type MITMServiceOptions struct {

    Enabled bool `json:"enabled"`

    CA MITMCAOptions `json:"ca"`

    Match MITMMatchOptions `json:"match"`

    Rewrite MITMRewriteOptions `json:"rewrite"`

}
```

---

影响：

```text
JSON

 ↓

option.Options

 ↓

schema

 ↓

check
```

---

验证：

```bash
sing-box check -c mitm.json
```

---

# Phase 2：Service Registry 接入

## Commit 0002

目标：

让：

```json
{
"type":"mitm"
}
```

可以创建实例。

---

修改：

```
include/service.go
```

加入：

```go
mitm.RegisterService(
    registry,
)
```

---

新增：

```
protocol/mitm/service.go
```

骨架：

```go
type Service struct {

ctx context.Context

options option.MITMServiceOptions

}
```

---

完成后：

流程：

```text
config.json

↓

Service Registry

↓

MITM Service

↓

Service Manager
```

---

# Phase 3：CA 管理模块

## Commit 0003

新增：

```
protocol/mitm/ca.go
```

---

结构：

```go
type CA struct {

Certificate *x509.Certificate

PrivateKey crypto.PrivateKey

}
```

---

功能：

* 加载 PEM
* 验证 CA
* 验证私钥匹配

测试：

```
TestLoadCA()
```

---

# Phase 4：动态证书签发

## Commit 0004

新增：

```
protocol/mitm/certificate.go
```

---

实现：

```go
func IssueCertificate(
    domain string,
)(
    *tls.Certificate,
    error,
)
```

---

流程：

```text
example.com

↓

Generate RSA/ECDSA key

↓

Create x509

↓

CA Sign

↓

tls.Certificate
```

---

增加：

```
certificate_cache.go
```

缓存：

```go
map[string]*tls.Certificate
```

---

测试：

```
TestCertificateIssue
```

---

# Phase 5：TLS ClientHello

## Commit 0005

新增：

```
protocol/mitm/clienthello.go
```

---

目标：

从：

```
TCP
```

读取：

```
TLS ClientHello
```

解析：

```text
SNI

ALPN

TLS Version
```

---

结构：

```go
type ClientHelloInfo struct {

ServerName string

ALPN []string

}
```

---

测试：

输入：

```
api.example.com
```

输出：

```
SNI=api.example.com
```

---

# Phase 6：TUN interception

## Commit 0006

修改：

```
protocol/tun/
```

不要加入 MITM 逻辑。

只增加接口：

```go
type Interceptor interface {


ShouldIntercept(
metadata.Metadata,
) bool


Intercept(
conn net.Conn,
metadata.Metadata,
) error


}
```

---

数据：

```text
TUN

 |

Interceptor

 |

MITM
```

---

# Phase 7：MITM TLS Server

## Commit 0007

新增：

```
protocol/mitm/tls.go
```

---

实现：

```go
tls.Server(
clientConn,
config,
)
```

---

流程：

```text
iOS App

TLS Client

↓

MITM

tls.Server

↓

HTTP plaintext
```

---

# Phase 8：Router 融合

## Commit 0008

重点 patch。

---

禁止：

```go
net.Dial()
```

直接连接。

---

必须：

```text
HTTP Request

↓

Router

↓

Outbound
```

---

新增：

```
router.go
```

接口：

```go
func RouteRequest(
request *http.Request,
)
```

---

数据：

```text
MITM

 ↓

sing-box Router

 ↓

proxy/direct
```

---

# Phase 9：HTTP/1.1

## Commit 0009

新增：

```
http1.go
```

---

支持：

* GET
* POST
* Header
* Cookie
* Body

---

流程：

```text
TLS

↓

http.ReadRequest

↓

rewrite

↓

router

↓

response
```

---

# Phase 10：HTTP/2

## Commit 0010

新增：

```
http2.go
```

---

使用：

```
golang.org/x/net/http2
```

---

支持：

* stream
* header
* data frame
* flow control

---

流程：

```text
ALPN=h2

↓

HTTP2 Server

↓

HTTP2 Client
```

---

# Phase 11：WebSocket

## Commit 0011

新增：

```
websocket.go
```

---

支持：

```
HTTP Upgrade

↓

101 Switching Protocols

↓

TCP relay
```

---

核心：

```go
io.Copy()
```

双向：

```text
client

 ⇄

MITM

 ⇄

server
```

---

# Phase 12：Rewrite Engine

## Commit 0012

目录：

```
protocol/mitm/rewrite/
```

---

结构：

```
engine.go

rule.go

body.go
```

---

支持：

```json
{
"domain_suffix":[
 "example.com"
],

"replace":[
 {
  "from":"abc",
  "to":"xyz"
 }
]
}
```

---

处理：

```
gzip

br

zstd
```

---

# Phase 13：libbox API

## Commit 0013

进入：

```
experimental/libbox
```

---

新增：

```
mitm.go
```

---

导出：

```go
GetMITMStatus()
```

返回：

```json
{
"enabled":true,

"caInstalled":true,

"connections":5
}
```

---

生成：

```
Libbox.xcframework
```

SFI 使用 Libbox framework 作为核心桥接

---

# Phase 14：SFI 配置支持

## Commit 0014

仓库：

```
sing-box-for-apple
```

---

新增：

```
SFI/Settings/MITM/
```

---

文件：

```
MITMSettingsView.swift

MITMSettingsModel.swift

MITMConfigBuilder.swift
```

---

UI：

```
Settings

 └── MITM

      Enable

      CA Status

      Export CA

      Debug Logs
```

---

生成：

```json
services:[
 {
 "type":"mitm"
 }
]
```

---

# Phase 15：SFI CA 管理

## Commit 0015

新增：

```
Library/MITM/
```

---

负责：

```text
Generate CA

↓

Store

↓

Export

↓

Install
```

---

路径：

```
Library/Application Support/SFI/MITM/
```

---

保存：

```
ca.pem

ca.key
```

---

# Phase 16：越狱版本支持(暂时不做)

## Commit 0016

仓库：

```
Jailbreak/
```

以及：

```
JailbreakDaemon/
```

---

增加：

```
MITMCAInstaller.swift
```

---

功能：

* 文件部署
* 权限处理
* daemon 配置

---

目录：

```
/var/jb/mobile/singbox/

├── config.json

└── mitm/

    ├── ca.pem

    └── ca.key
```

---

# Phase 17：Build 系统

## Commit 0017

修改：

```
Makefile
```

增加：

```make
make libbox-mitm
```

---

流程：

```text
go build

↓

gomobile bind

↓

xcframework

↓

Xcode
```

---

# Phase 18：自动化测试

## Commit 0018

新增：

```
test/mitm/
```

---

测试：

```
config_test.go

ca_test.go

certificate_test.go

tls_test.go

http_test.go

router_test.go

websocket_test.go
```

---

# Phase 19：真机测试

测试链：

```text
iPhone

↓

SFI

↓

VPN Extension

↓

TUN

↓

MITM

↓

Router

↓

Outbound

↓

Internet
```

---

验证：

## HTTPS

```
https://example.com
```

日志：

```
MITM:
SNI example.com

certificate issued

route proxy
```

---

## HTTP/2

```
ALPN=h2
```

---

## WebSocket

```
101 Upgrade
```

---

# Phase 20：最终 Release 分支

最终：

```
sing-box

feature/mitm-core

        merge

testing
```

---

SFI：

```
feature/mitm-support

        merge

dev
```

---

# 最终 Patch 顺序总结

| Patch | 内容               | 仓库       |
| ----- | ---------------- | -------- |
| 0001  | MITM options     | sing-box |
| 0002  | Service Registry | sing-box |
| 0003  | CA engine        | sing-box |
| 0004  | Certificate      | sing-box |
| 0005  | TLS ClientHello  | sing-box |
| 0006  | TUN interceptor  | sing-box |
| 0007  | TLS server       | sing-box |
| 0008  | Router           | sing-box |
| 0009  | HTTP/1.1         | sing-box |
| 0010  | HTTP/2           | sing-box |
| 0011  | WebSocket        | sing-box |
| 0012  | Rewrite          | sing-box |
| 0013  | Libbox API       | sing-box |
| 0014  | SFI UI           | SFI      |
| 0015  | CA manager       | SFI      |
| 0016  | Jailbreak        | SFI      |
| 0017  | Build            | Both     |
| 0018  | Tests            | Both     |

---

# 最终完成状态

完成后：

```
SFI

↓

NetworkExtension

↓

Libbox

↓

sing-box

↓

MITM Service

↓

TLS interception

↓

HTTP engine

↓

Rewrite

↓

Router

↓

Outbound
```

形成完整：

**SFI + sing-box Core + MITM + iOS 越狱部署平台。**

---

# 二十一、SFI 接入 API 完整清单（已实现）

> 本章节详细列出 sing-box-for-apple 分支中已实现的 MITM 接入 API、注册表、配置模型与目录结构。

---

## 21.1 Swift 侧配置模型

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

| 字段 | 类型 | 说明 | JSON 字段 |
|------|------|------|-----------|
| `domain` | `[String]` | 精确匹配域名 | `domain` |
| `domainSuffix` | `[String]` | 后缀匹配域名 | `domain_suffix` |
| `domainKeyword` | `[String]` | 关键字匹配域名 | `domain_keyword` |
| `domainRegex` | `[String]` | 正则匹配域名 | `domain_regex` |
| `hasRules` | `Bool`（计算属性） | 是否有任何匹配规则 | — |

### MITMRewriteConfiguration — 重写配置

| 字段 | 类型 | 说明 | JSON 字段 |
|------|------|------|-----------|
| `enabled` | `Bool` | 是否启用重写 | `enabled` |
| `maxBodySize` | `Int` | 最大 Body 大小（字节，默认 10MB） | `max_body_size` |
| `rules` | `[MITMRewriteRule]` | 重写规则列表 | `rules` |

### MITMRewriteRule — 重写规则

| 字段 | 类型 | 说明 | JSON 字段 |
|------|------|------|-----------|
| `id` | `UUID` | 规则唯一标识（仅 UI） | — |
| `name` | `String` | 规则名称（仅 UI） | — |
| `domain` | `[String]` | 精确匹配域名 | `domain` |
| `domainSuffix` | `[String]` | 后缀匹配域名 | `domain_suffix` |
| `pathPrefix` | `[String]` | 路径前缀匹配 | `path_prefix` |
| `method` | `[String]` | HTTP 方法匹配 | `method` |
| `requestHeader` | `[String: String]` | 请求头添加/修改 | `request_header` |
| `requestHeaderDelete` | `[String]` | 请求头删除 | `request_header_delete` |
| `responseHeader` | `[String: String]` | 响应头添加/修改 | `response_header` |
| `responseHeaderDelete` | `[String]` | 响应头删除 | `response_header_delete` |
| `bodyReplace` | `[MITMBodyReplace]` | Body 替换规则 | `body_replace` |

### MITMBodyReplace — Body 替换规则

| 字段 | 类型 | 说明 | JSON 字段 |
|------|------|------|-----------|
| `id` | `UUID` | 规则唯一标识 | — |
| `find` | `String` | 查找字符串 | `find` |
| `replace` | `String` | 替换字符串 | `replace` |

### MITMOnError — 错误处理策略（枚举）

| 枚举值 | 原始值 | 显示名称 | 说明 |
|--------|--------|----------|------|
| `bypass` | `"bypass"` | 绕过（透传） | MITM 失败时直接透传原始流量 |
| `reject` | `"reject"` | 拒绝（断开） | MITM 失败时断开连接 |

### MITMRuntimeStatus — MITM 运行状态

| 字段 | 类型 | 说明 |
|------|------|------|
| `enabled` | `Bool` | MITM 服务是否已启用 |
| `caInstalled` | `Bool` | 根证书是否已成功加载 |
| `activeConnections` | `Int32` | 当前活跃 MITM 连接数 |
| `unknown` | 静态常量 | 未知状态（服务未启动时） |

---

## 21.2 MITMServiceManager — 服务管理器 API

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

## 21.3 Libbox 导出 API（Go 侧 → Swift 桥接）

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

```go
func LibboxGetMITMStatus() *MITMStatus
```

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

### LibboxGetMITMLogs — 获取 MITM 日志（预留）

```go
func LibboxGetMITMLogs() string
```

返回 MITM 内存日志缓冲区中的日志内容，用于调试控制台显示。

---

## 21.4 Service Registry 注册表接入

### sing-box Core 侧

文件：`include/service.go`（sing-box 分支）

```go
mitm.RegisterService(registry)
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

文件：`constant/constant.go`

```go
const TypeMITM = "mitm"
```

---

## 21.5 TUN Interceptor 接口（解耦设计）

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

## 21.6 完整配置 JSON 结构

### 启用 MITM 的 profile 配置示例

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
        "domain_suffix": ["example.com"],
        "domain_keyword": ["test"],
        "domain_regex": ["^.*\\.example\\.com$"]
      },
      "rewrite": {
        "enabled": true,
        "max_body_size": 10485760,
        "rules": [
          {
            "domain_suffix": ["example.com"],
            "path_prefix": ["/api"],
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
| `match.domain_keyword` | `[string]` | 否 | 关键字匹配域名列表 |
| `match.domain_regex` | `[string]` | 否 | 正则匹配域名列表 |
| `rewrite.enabled` | `bool` | 否 | 是否启用重写引擎 |
| `rewrite.max_body_size` | `int` | 否 | 最大 Body 大小（字节），默认 10MB |
| `rewrite.rules` | `[object]` | 否 | 重写规则列表 |
| `on_error` | `string` | 否 | 错误处理策略：`bypass`（默认）或 `reject` |
| `upstream_timeout` | `int` | 否 | 上游连接超时（秒），默认 30 |

---

## 21.7 sing-box-for-apple 目录结构

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

## 21.8 数据流完整链路

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

## 21.9 已完成 vs 待完成

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

- [ ] Phase 14：SFI 配置 UI 集成到主设置页面（需接入 MainView 导航）
- [ ] Phase 15：SFI CA 管理（证书生成/导出/安装引导）
- [ ] Phase 16：越狱版本支持（文档标注"暂时不做"）
- [ ] Phase 19：真机测试（需 iOS 设备 + 开发者证书）
- [ ] Phase 20：最终 Release 分支合并
- [ ] Libbox.xcframework 重新编译（需 macOS + Xcode + gomobile，Linux 云电脑无法完成）
- [ ] SFI 替换 Framework 并编译验证

---

## 21.10 关键技术决策

1. **TUN 层解耦**：通过 `tun.Interceptor` 接口 + `service.FromContext` 实现 TUN 与 MITM 解耦，TUN 层不直接 import mitm 包。

2. **双重注册**：MITM Service 同时注册为 `*Service` 和 `tun.Interceptor` 接口，TUN 层通过接口获取。

3. **LRU 证书缓存**：叶子证书使用 LRU 缓存，避免重复签发，支持并发安全（双重检查锁定）。

4. **Rewrite 引擎错误恢复**：重写响应体解压/压缩失败时恢复原始响应体，避免 `resp.Write` 出现 `invalid Read on closed Body`。

5. **App Group 共享**：MITM 配置和 CA 证书存储在 App Group 共享目录，主 App 和 Network Extension 均可访问。

6. **配置注入而非替换**：MITM 配置通过 `injectConfiguration(into:)` 方法注入到现有 profile JSON 的 services[] 数组中，不破坏原有配置。

7. **全汉化**：所有 Swift 代码注释、变量名、错误提示、用户可见字符串均使用简体中文。

8. **生产级代码**：完整错误处理、边界判断、线程安全（@Published、ObservableObject）、适配深色模式和动态字体。

---