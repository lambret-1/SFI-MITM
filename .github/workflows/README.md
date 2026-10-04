# GitHub Actions 工作流目录说明书

## 目录概述

本目录包含 sing-box 分支的全部 CI/CD 工作流，共 5 条流水线，按职责严格划分，触发条件互不重叠。

## 职责划分总表

| 工作流文件 | 名称 | 职责 | 触发时机 |
|---|---|---|---|
| `mitm-ci.yml` | MITM CI | 基础编译冒烟门禁 | 核心 Go 代码修改（排除 experimental/、deploy/） |
| `mitm-test-matrix.yml` | MITM 测试矩阵 | MITM 专项深度测试 | MITM 核心代码修改（protocol/mitm/、option/mitm.go、libbox/mitm.go） |
| `full-test-matrix.yml` | 全项目测试矩阵 | 全项目深度测试（非 MITM） | 非 MITM 核心 Go 代码修改 |
| `libbox-build.yml` | Libbox 构建（iOS 设备专用） | Libbox.framework 构建 | libbox 相关代码修改 |
| `ios-deploy.yml` | iOS 部署包构建 | 越狱部署包打包 | deploy/ios 配置或 CA 生成命令修改 |

## 触发规则设计

### 互不重叠原则

修改某类文件时，仅触发职责对应的流水线，避免重复构建浪费资源。

| 修改内容 | 触发的流水线 |
|---|---|
| `protocol/mitm/**`、`option/mitm.go` | MITM CI + MITM 测试矩阵 |
| `daemon/**`、`route/**`、`dns/**` 等非 MITM 核心代码 | MITM CI + 全项目测试矩阵 |
| `experimental/libbox/**`、`cmd/internal/build_libbox/**` | Libbox 构建（仅） |
| `deploy/ios/**`、`cmd/sing-box/cmd_generate_mitm_ca.go` | iOS 部署包构建（仅） |
| 任意 `*.yml` 工作流文件 | 仅触发对应工作流自身 |
| `*.md` 文档 | 不触发任何流水线 |

### 各工作流触发路径明细

#### mitm-ci.yml（基础冒烟门禁）
```yaml
paths-ignore:
  - experimental/**        # 排除实验性包
  - deploy/**              # 排除部署包
  - .github/workflows/mitm-test-matrix.yml
  - .github/workflows/full-test-matrix.yml
  - .github/workflows/libbox-build.yml
  - .github/workflows/ios-deploy.yml
  - '**.md'
```

#### mitm-test-matrix.yml（MITM 专项）
```yaml
paths:
  - protocol/mitm/**
  - option/mitm.go
  - experimental/libbox/mitm.go
  - .github/workflows/mitm-test-matrix.yml
```

#### full-test-matrix.yml（全项目深度）
```yaml
paths-ignore:
  - protocol/mitm/**           # 排除 MITM 专项
  - option/mitm.go
  - experimental/libbox/mitm.go
  - experimental/**
  - deploy/**
  - .github/workflows/mitm-ci.yml
  - .github/workflows/mitm-test-matrix.yml
  - .github/workflows/libbox-build.yml
  - .github/workflows/ios-deploy.yml
  - '**.md'
```

#### libbox-build.yml（Libbox 构建）
```yaml
paths:
  - experimental/libbox/**
  - cmd/internal/build_libbox/**
  - .github/workflows/libbox-build.yml
```

#### ios-deploy.yml（iOS 部署包）
```yaml
paths:
  - deploy/ios/**
  - cmd/sing-box/cmd_generate_mitm_ca.go
  - .github/workflows/ios-deploy.yml
```

---

## 各工作流详细说明

### 1. mitm-ci.yml — MITM CI（基础冒烟门禁）

**职责**：快速验证代码可编译、核心测试通过，作为提交的基础质量门禁。

**运行环境**：ubuntu-latest

**执行步骤**：
1. 编译 sing-box 主程序
2. 编译全部包（排除 experimental 平台限制包）
3. 运行 MITM 模块单元测试
4. 运行相关模块测试（option/constant/include/adapter）
5. 生成测试 CA 证书
6. 验证 MITM 配置解析（启用 + 未启用两种场景）

**产物**：
| 产物名 | 格式 | 用途 | 保留期 |
|---|---|---|---|
| `singbox-mitm-linux-amd64` | ELF 二进制 | Linux 平台 sing-box 可执行文件，用于快速验证和调试 | 7 天 |

**预期耗时**：约 2-3 分钟

---

### 2. mitm-test-matrix.yml — MITM 测试矩阵（专项深度测试）

**职责**：对 MITM 模块进行多维度、全覆盖的深度测试，确保 MITM 功能正确性和稳定性。

**运行环境**：ubuntu-latest + macos-14

**七大测试维度（共 31 个 Job）**：

| 维度 | Job 数 | 覆盖内容 |
|---|---|---|
| 单元测试矩阵 | 13 | 配置解析、Service 生命周期、CA 证书管理、动态证书签发、TLS 终止握手、HTTP/1.1、Router 分流、WebSocket、日志缓冲区、重写引擎、失败绕过策略、上游超时、域名匹配器 |
| Race Detector | 1 | 全量并发安全检测（MITM 涉及多连接并发处理） |
| 覆盖率收集 | 1 | 行覆盖率统计 + HTML 覆盖率报告 |
| 配置校验矩阵 | 8 | 完整 MITM 配置、未启用、TUN+MITM、多域名匹配、重写规则全字段、on_error=block、空匹配、多 outbound |
| 负向测试 | 1 | 缺少证书路径、缺少私钥路径、非 CA 证书拒绝、未知 service 类型 |
| macOS 跨平台 | 1 | macOS 平台全量 MITM 测试 + 配置校验 |
| 交叉编译 | 6 | linux/amd64、linux/arm64、darwin/arm64、darwin/amd64、freebsd/amd64、windows/amd64 |

**产物**：
| 产物名 | 格式 | 用途 | 保留期 |
|---|---|---|---|
| `mitm-coverage-report` | `.out` + `.html` | MITM 模块行覆盖率数据和可视化报告，用于评估测试覆盖度 | 30 天 |

**预期耗时**：约 5-8 分钟

---

### 3. full-test-matrix.yml — 全项目测试矩阵（非 MITM 深度测试）

**职责**：对 sing-box 全项目（非 MITM 部分）进行深度测试，确保核心功能（路由、DNS、协议、入站/出站等）的正确性。

**运行环境**：ubuntu-latest + macos-14

**七大测试维度（共 23 个 Job）**：

| 维度 | Job 数 | 覆盖内容 |
|---|---|---|
| 全量单元测试 | 1 | 全部包单元测试（排除 experimental 平台限制包，包含 MITM） |
| Race Detector | 1 | 全量并发安全检测 |
| 全量覆盖率 | 1 | 全项目行覆盖率统计 + HTML 报告 |
| macOS 全量测试 | 1 | macOS 平台全量测试（包含 MITM） |
| 交叉编译矩阵 | 8 | linux/amd64、linux/arm64、linux/arm(v7)、darwin/arm64、darwin/amd64、freebsd/amd64、windows/amd64、android/arm64 |
| 构建产物验证 | 1 | version/help/schema/generate mitm-ca 命令验证 |
| 配置校验矩阵 | 10 | 基础配置、TUN 模式、SOCKS 入站、HTTP 入站、Mixed 入站、多入站多出站、DNS 配置、路由规则、MITM 完整配置、MITM 未启用 |

**产物**：
| 产物名 | 格式 | 用途 | 保留期 |
|---|---|---|---|
| `full-coverage-report` | `.out` + `.html` | 全项目行覆盖率数据和可视化报告 | 30 天 |
| `singbox-linux-amd64` | ELF 二进制 | 全项目测试通过的 Linux 可执行文件 | 7 天 |

**预期耗时**：约 8-12 分钟

---

### 4. libbox-build.yml — Libbox 构建（iOS 设备专用）

**职责**：构建供 iOS App（sing-box-for-apple）使用的 Libbox.framework，仅包含 ios-arm64 设备切片。

**运行环境**：macos-14 + Xcode 15.4

**执行步骤**：
1. 安装 sagernet/gomobile
2. Go 构建缓存（libbox 依赖预编译）
3. 构建 Libbox.framework（仅 ios-arm64 设备切片）
4. 验证 framework 架构
5. 打包产物

**产物**：
| 产物名 | 格式 | 用途 | 保留期 |
|---|---|---|---|
| `Libbox.framework` | `.framework`  bundle | iOS App Network Extension 嵌入的 Go 核心框架，提供 sing-box 全部功能（含 MITM）的 Swift 绑定接口 | 30 天 |

**用途说明**：
- 构建完成后需手动或通过 Step 4 复制到 `sing-box-for-apple/Frameworks/` 目录
- 供 Xcode 编译 iOS App 时链接使用
- 仅包含 iOS 真机切片（ios-arm64），不包含模拟器切片，以缩短构建时间

**预期耗时**：约 10-15 分钟

---

### 5. ios-deploy.yml — iOS 部署包构建

**职责**：构建越狱环境使用的 sing-box iOS 部署包（.deb + tar.gz），供越狱设备直接安装运行 MITM 代理。

**运行环境**：macos-14 + Xcode 15.4

**执行步骤**：
1. 原生构建 darwin/arm64 sing-box（用于生成 CA 证书）
2. 生成 MITM CA 证书（预生成，随包分发）
3. 交叉编译 iOS/arm64 二进制（使用 iPhoneOS SDK）
4. ldid 伪签名（越狱环境无需 Apple 证书）
5. 验证 Mach-O 格式
6. 组装部署包目录（二进制 + 配置 + LaunchDaemon + CA + 安装脚本）
7. 组装 .deb 安装包
8. 打包 tar.gz
9. 可选：创建 GitHub Release

**产物**：
| 产物名 | 格式 | 用途 | 保留期 |
|---|---|---|---|
| `sing-box-mitm-ios-deb` | `.deb` | 越狱设备 .deb 安装包，可通过 Filza/Sileo/Cydia 直接安装 | 30 天 |
| `sing-box-mitm-ios-deploy` | `.tar.gz` + 目录 | 手动部署包，包含二进制、配置模板、CA 证书、LaunchDaemon 配置、安装脚本 | 30 天 |
| `sing-box-ios-arm64` | Mach-O 二进制 | 单独的 iOS arm64 sing-box 可执行文件（已 ldid 签名） | 30 天 |

**用途说明**：
- 适用于多巴胺 Dopamine、palera1n、checkra1n 等越狱环境
- .deb 包可直接在 iOS 设备上安装，无需电脑
- 预生成 CA 证书随包分发，安装后按提示完成信任设置
- 支持开机自启（LaunchDaemon）

**预期耗时**：约 5-8 分钟

---

## 产物总览

| 产物 | 来源工作流 | 格式 | 主要用途 |
|---|---|---|---|
| singbox-mitm-linux-amd64 | mitm-ci | ELF | Linux 快速验证二进制 |
| mitm-coverage-report | mitm-test-matrix | HTML/out | MITM 模块覆盖率报告 |
| full-coverage-report | full-test-matrix | HTML/out | 全项目覆盖率报告 |
| singbox-linux-amd64 | full-test-matrix | ELF | 全项目测试通过二进制 |
| Libbox.framework | libbox-build | framework | iOS App 嵌入框架 |
| sing-box-mitm-ios-deb | ios-deploy | .deb | 越狱设备安装包 |
| sing-box-mitm-ios-deploy | ios-deploy | tar.gz | 手动部署包 |
| sing-box-ios-arm64 | ios-deploy | Mach-O | iOS 单独二进制 |

## 维护说明

- 修改工作流文件时，仅会触发对应工作流自身，不会影响其他流水线
- 新增工作流时，需在现有工作流的 `paths-ignore` 中添加新工作流文件名，确保互不影响
- 所有工作流均支持 `workflow_dispatch` 手动触发
- 并发策略：同一分支同一工作流的多次推送会取消旧运行（`cancel-in-progress: true`）
