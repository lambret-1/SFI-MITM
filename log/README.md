# 🚀 PR 检查汇总报告

> 由 GitHub Actions 自动生成，包含静态检查与 SwiftLint 结果。

**更新时间 (中国标准时间)**: `2026-10-04 19:52:08`

## 📊 总体统计

- 🔴 **严重错误**: 1
- 🟡 **警告**: 0

## 🧱 静态检查详情

| 检查项 | 状态 | 详情 |
| :--- | :---: | :--- |
| 检查冲突标记 | ✅ 通过 | 无冲突标记 |
| 检查 Info.plist 语法 | ✅ 通过 | 所有文件合法 |
| 检查意外提交的文件 | ✅ 通过 | 无异常文件 |
| 检查 pbxproj 有效性 | ✅ 通过 | 结构完整 |
| 检查 entitlements | ❌ 失败 | 缺少网络扩展权限 |
| 检查大文件 | ✅ 通过 | 无超过 5MB 的文件 |
| 检查硬编码密钥 | ✅ 通过 | 无硬编码密钥 |
| 检查空白错误 | ✅ 通过 | 无空白错误 |
| 检查重复文件名 | ⚠️ 警告 | 存在重复文件名（不阻塞） |

### 📋 静态检查错误详情

```text
❌ 缺少关键权限: com.apple.developer.networking.networkextension
```

## 🔍 SwiftLint 详细列表

| 文件 | 行号 | 级别 | 规则 | 描述 |
| :--- | :---: | :---: | :--- | :--- |
| `../../../../../work/SFT/ApplicationDelegate.swift` | L23 | 🟡 | `no_force_unwrap_core` | 核心网络模块请使用可选绑定或 guard 处理 nil |
| `../../../../../work/SFT/ApplicationDelegate.swift` | L23 | 🟡 | `force_unwrapping` | Force unwrapping should be avoided |