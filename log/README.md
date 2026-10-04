# 🚀 PR 检查汇总报告

> 由 GitHub Actions 自动生成，包含静态检查与 SwiftLint 结果。

**更新时间 (中国标准时间)**: `2026-10-04 19:43:31`

## 📊 总体统计

- 🔴 **严重错误**: 1
- 🟡 **警告**: 0

## 🧱 静态检查结果

```text
❌ 缺少关键权限: com.apple.developer.networking.networkextension
```

## 🔍 SwiftLint 详细列表

| 文件 | 行号 | 级别 | 规则 | 描述 |
| :--- | :---: | :---: | :--- | :--- |
| `../../../../../work/SFT/ApplicationDelegate.swift` | L23 | 🟡 | `no_force_unwrap_core` | 核心网络模块请使用可选绑定或 guard 处理 nil |
| `../../../../../work/SFT/ApplicationDelegate.swift` | L23 | 🟡 | `force_unwrapping` | Force unwrapping should be avoided |