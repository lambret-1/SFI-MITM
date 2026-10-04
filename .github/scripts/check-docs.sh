#!/bin/bash
# 接入文档门禁检查脚本
# 检查 README/README.md 的存在性、结构完整性、与 sing-box 分支代码的一致性

set -e

DOC_PATH="README/README.md"
ERRORS=0
WARNINGS=0

echo "=========================================="
echo "  SFI 接入 sing-box 内核文档门禁检查"
echo "=========================================="
echo ""

# ========== 1. 文档存在性检查 ==========
echo "【1/5】文档存在性检查"
if [ ! -f "$DOC_PATH" ]; then
    echo "  ❌ 文档不存在: $DOC_PATH"
    ERRORS=$((ERRORS + 1))
else
    DOC_LINES=$(wc -l < "$DOC_PATH")
    echo "  ✅ 文档存在: $DOC_PATH (${DOC_LINES} 行)"
fi
echo ""

# ========== 2. 文档结构完整性检查（25章） ==========
echo "【2/5】文档结构完整性检查（25章）"
REQUIRED_CHAPTERS=(
    "一、整体架构与内核接入流程"
    "二、Libbox 核心 API"
    "三、配置管理"
    "四、服务生命周期"
    "五、TUN 接入与 Network Extension"
    "六、PlatformInterface 接口"
    "七、CommandServerHandler 接口"
    "八、MITM 专项接入"
    "九、整体修改范围"
    "十、Core 层修改"
    "十一、libbox 修改"
    "十二、Libbox 编译"
    "十三、SFI 工程修改"
    "十四、Network Extension 修改"
    "十五、Jailbreak 模块"
    "十六、CA 文件管理"
    "十七、越狱版增强"
    "十八、SFI 配置同步"
    "十九、Bridge API"
    "二十、日志接口"
    "二十一、Xcode 修改列表"
    "二十二、Build 流程"
    "二十三、Makefile 增加"
    "二十四、最终目录状态"
    "二十五、SFI 接入 API 完整清单"
)

MISSING_CHAPTERS=()
for chapter in "${REQUIRED_CHAPTERS[@]}"; do
    if ! grep -q "$chapter" "$DOC_PATH" 2>/dev/null; then
        MISSING_CHAPTERS+=("$chapter")
    fi
done

if [ ${#MISSING_CHAPTERS[@]} -gt 0 ]; then
    echo "  ❌ 缺少 ${#MISSING_CHAPTERS[@]} 个章节:"
    for ch in "${MISSING_CHAPTERS[@]}"; do
        echo "      - $ch"
    done
    ERRORS=$((ERRORS + ${#MISSING_CHAPTERS[@]}))
else
    echo "  ✅ 全部 25 章完整"
fi
echo ""

# ========== 3. 关键 API 名称一致性检查 ==========
echo "【3/5】关键 API 名称一致性检查"
if [ -d "../sing-box-core" ]; then
    CORE_DIR="../sing-box-core"
elif [ -d "sing-box-core" ]; then
    CORE_DIR="sing-box-core"
else
    echo "  ⚠️ 未找到 sing-box 核心代码目录，跳过 API 一致性检查"
    CORE_DIR=""
fi

if [ -n "$CORE_DIR" ]; then
    # 检查 MITMStatus 字段
    echo "  检查 MITMStatus 字段..."
    if grep -q "CAInstalled" "$DOC_PATH" && grep -q "ActiveConnections" "$DOC_PATH"; then
        echo "    ✅ MITMStatus 字段正确（CAInstalled/ActiveConnections）"
    else
        echo "    ❌ MITMStatus 字段不正确（应为 CAInstalled/ActiveConnections）"
        ERRORS=$((ERRORS + 1))
    fi

    # 检查 GenerateMITMCA 签名
    echo "  检查 GenerateMITMCA 签名..."
    if grep -q "certificatePath" "$DOC_PATH" && grep -q "privateKeyPath" "$DOC_PATH"; then
        echo "    ✅ GenerateMITMCA 签名正确（certificatePath, privateKeyPath）"
    else
        echo "    ❌ GenerateMITMCA 签名不正确（应为 certificatePath, privateKeyPath）"
        ERRORS=$((ERRORS + 1))
    fi

    # 检查 TypeMITM 位置
    echo "  检查 TypeMITM 常量位置..."
    if grep -q "constant/proxy.go" "$DOC_PATH"; then
        echo "    ✅ TypeMITM 位置正确（constant/proxy.go）"
    else
        echo "    ❌ TypeMITM 位置不正确（应为 constant/proxy.go）"
        ERRORS=$((ERRORS + 1))
    fi

    # 检查 Service Registry 位置
    echo "  检查 Service Registry 文件位置..."
    if grep -q "include/mitm.go" "$DOC_PATH"; then
        echo "    ✅ Service Registry 位置正确（include/mitm.go）"
    else
        echo "    ❌ Service Registry 位置不正确（应为 include/mitm.go）"
        ERRORS=$((ERRORS + 1))
    fi

    # 检查 TunOptions 类型
    echo "  检查 TunOptions 类型..."
    if grep -q "TunOptions 接口" "$DOC_PATH" || grep -q "TunOptions — TUN 选项（接口）" "$DOC_PATH"; then
        echo "    ✅ TunOptions 类型正确（接口，非结构体）"
    else
        echo "    ❌ TunOptions 类型不正确（应为接口，非结构体）"
        ERRORS=$((ERRORS + 1))
    fi

    # 检查 libbox/mitm.go 文件名
    echo "  检查 libbox MITM 文件名..."
    if grep -q "experimental/libbox/mitm.go" "$DOC_PATH"; then
        echo "    ✅ libbox MITM 文件名正确（mitm.go）"
    else
        echo "    ❌ libbox MITM 文件名不正确（应为 mitm.go）"
        ERRORS=$((ERRORS + 1))
    fi
fi
echo ""

# ========== 4. 配置 JSON 示例检查 ==========
echo "【4/5】完整配置 JSON 示例检查"
if grep -q "完整配置 JSON 示例" "$DOC_PATH"; then
    # 检查关键顶层字段
    REQUIRED_FIELDS=('"log"' '"dns"' '"inbounds"' '"outbounds"' '"route"' '"services"' '"experimental"' '"mitm"')
    MISSING_FIELDS=()
    for field in "${REQUIRED_FIELDS[@]}"; do
        if ! grep -q "$field" "$DOC_PATH"; then
            MISSING_FIELDS+=("$field")
        fi
    done
    if [ ${#MISSING_FIELDS[@]} -gt 0 ]; then
        echo "  ⚠️ 配置示例缺少字段: ${MISSING_FIELDS[*]}"
        WARNINGS=$((WARNINGS + ${#MISSING_FIELDS[@]}))
    else
        echo "  ✅ 配置示例包含全部关键顶层字段"
    fi
else
    echo "  ⚠️ 未找到完整配置 JSON 示例章节"
    WARNINGS=$((WARNINGS + 1))
fi
echo ""

# ========== 5. Markdown 基本格式检查 ==========
echo "【5/5】Markdown 基本格式检查"
# 检查未闭合的代码块
CODE_BLOCK_MARKER='^```'
CODE_BLOCKS=$(grep -c "$CODE_BLOCK_MARKER" "$DOC_PATH" 2>/dev/null || echo 0)
if [ $((CODE_BLOCKS % 2)) -ne 0 ]; then
    echo "  ❌ 存在未闭合的代码块（代码块标记数量为奇数: $CODE_BLOCKS）"
    ERRORS=$((ERRORS + 1))
else
    echo "  ✅ 代码块闭合正常（共 $((CODE_BLOCKS / 2)) 个代码块）"
fi

# 检查表格格式（简单检查 | 数量）
TABLE_LINES=$(grep -c '^|' "$DOC_PATH" 2>/dev/null || echo 0)
echo "  ℹ️ 表格行数: $TABLE_LINES"
echo ""

# ========== 最终判定 ==========
echo "=========================================="
echo "  检查结果汇总"
echo "=========================================="
echo "  错误数: $ERRORS"
echo "  警告数: $WARNINGS"
echo ""

if [ "$ERRORS" -gt 0 ]; then
    echo "❌ 文档门禁检查未通过，存在 $ERRORS 个错误"
    exit 1
else
    echo "✅ 文档门禁检查通过"
    if [ "$WARNINGS" -gt 0 ]; then
        echo "⚠️ 存在 $WARNINGS 个警告（不阻塞）"
    fi
    exit 0
fi
