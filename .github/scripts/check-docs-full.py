#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
SFI 客户端接入文档参数一致性检查脚本

聚焦于 SFI iOS 客户端实际接入 sing-box 内核的参数，
排除 Linux 特有、服务端功能等与 SFI 无关的参数。

检查维度：
1. 顶层字段完整性（SFI 必须/可选/不适用）
2. 入站类型（SFI 实际使用的类型）
3. 出站类型（SFI 实际使用的类型）
4. DNS 配置参数
5. 路由配置参数
6. 实验配置参数
7. 服务配置参数（api/mitm）
8. TUN 入站参数（iOS 必须）
9. libbox API 方法
10. 客户端代码引用检查
"""

import argparse
import json
import re
import sys
from pathlib import Path
from typing import Dict, List, Tuple, Set, Optional


# ========== SFI 客户端接入参数定义 ==========

# 顶层字段定义：(字段名, 是否SFI必须, 说明)
SFI_TOP_LEVEL_FIELDS = [
    ("log", False, "日志配置（level/output/timestamp）"),
    ("dns", False, "DNS 配置（servers/rules/final/strategy）"),
    ("inbounds", True, "入站配置列表（iOS 必须使用 tun）"),
    ("outbounds", True, "出站配置列表（direct/selector/各协议）"),
    ("route", False, "路由配置（rules/final/auto_detect_interface）"),
    ("services", False, "服务配置列表（api/mitm）"),
    ("experimental", False, "实验性功能（clash_api/v2ray_api/cache_file）"),
]

# 不适用 SFI 的顶层字段
SFI_NOT_APPLICABLE_TOP_LEVEL = [
    ("ntp", "iOS 系统自动时间同步，无需配置"),
    ("certificate", "证书管理，SFI 客户端不直接使用"),
    ("certificate_providers", "ACME 等证书提供者，服务端功能"),
    ("http_clients", "高级 HTTP 客户端配置，SFI 不使用"),
    ("network_namespaces", "Linux 网络命名空间，iOS 不支持"),
    ("endpoints", "WireGuard 端点，SFI 使用出站而非端点"),
]

# SFI 入站类型（实际使用的）
SFI_INBOUND_TYPES = [
    ("tun", True, "TUN 虚拟网卡，iOS Network Extension 必须使用"),
    ("socks", False, "SOCKS5 本地代理入站"),
    ("http", False, "HTTP 本地代理入站"),
    ("mixed", False, "SOCKS+HTTP 混合入站"),
    ("direct", False, "直接入站"),
]

# 不适用 SFI 的入站类型（服务端/Linux 特有）
SFI_NOT_APPLICABLE_INBOUNDS = [
    ("redirect", "Linux/macOS 透明代理，iOS 不支持"),
    ("tproxy", "Linux 透明代理，iOS 不支持"),
    ("shadowsocks", "服务端入站，SFI 作为客户端不使用"),
    ("snell", "服务端入站，SFI 作为客户端不使用"),
    ("vmess", "服务端入站，SFI 作为客户端不使用"),
    ("trojan", "服务端入站，SFI 作为客户端不使用"),
    ("naive", "服务端入站，SFI 作为客户端不使用"),
    ("shadowtls", "服务端入站，SFI 作为客户端不使用"),
    ("vless", "服务端入站，SFI 作为客户端不使用"),
    ("anytls", "服务端入站，SFI 作为客户端不使用"),
    ("hysteria", "服务端入站，SFI 作为客户端不使用"),
    ("hysteria2", "服务端入站，SFI 作为客户端不使用"),
    ("tuic", "服务端入站，SFI 作为客户端不使用"),
    ("cloudflare", "服务端入站，SFI 作为客户端不使用"),
]

# SFI 出站类型（实际使用的）
SFI_OUTBOUND_TYPES = [
    ("direct", True, "直连出站"),
    ("block", False, "阻断出站"),
    ("selector", True, "手动选择出站组，UI 切换节点"),
    ("urltest", False, "自动测速选择最快节点"),
    ("shadowsocks", True, "Shadowsocks 协议出站"),
    ("vmess", True, "VMess 协议出站"),
    ("trojan", True, "Trojan 协议出站"),
    ("vless", True, "VLESS 协议出站"),
    ("socks", False, "SOCKS5 代理出站"),
    ("http", False, "HTTP 代理出站"),
    ("shadowtls", False, "ShadowTLS 协议出站"),
    ("hysteria", False, "Hysteria 协议出站（QUIC）"),
    ("hysteria2", False, "Hysteria2 协议出站（QUIC）"),
    ("tuic", False, "TUIC 协议出站（QUIC）"),
    ("naive", False, "NaiveProxy 协议出站"),
    ("anytls", False, "AnyTLS 协议出站"),
    ("snell", False, "Snell 协议出站（Surge）"),
    ("tor", False, "Tor 出站"),
    ("ssh", False, "SSH 隧道出站"),
    ("bridge", False, "桥接出站"),
]

# DNS 配置参数（SFI 相关）
SFI_DNS_PARAMS = [
    ("servers", True, "DNS 服务器列表"),
    ("rules", False, "DNS 规则列表"),
    ("final", True, "最终 DNS 服务器"),
    ("strategy", False, "DNS 解析策略（ipv4_only/ipv6_only/prefer_ipv4）"),
    ("disable_cache", False, "禁用 DNS 缓存"),
    ("disable_expire", False, "禁用 DNS 缓存过期"),
    ("independent_cache", False, "独立 DNS 缓存"),
    ("reverse_mapping", False, "反向 DNS 映射"),
    ("fakeip", False, "FakeIP 配置"),
]

# 路由配置参数（SFI 相关）
SFI_ROUTE_PARAMS = [
    ("rules", False, "路由规则列表"),
    ("rule_set", False, "路由规则集"),
    ("final", True, "最终出站"),
    ("auto_detect_interface", False, "自动检测出口网卡"),
    ("auto_route", False, "自动路由（全局流量接管）"),
    ("default_interface", False, "默认出口网卡"),
    ("endpoint_independent_nat", False, "端点无关 NAT"),
    ("exclude_interface", False, "排除网卡列表"),
    ("exclude_routable", False, "排除可路由地址"),
]

# 实验配置参数（SFI 相关）
SFI_EXPERIMENTAL_PARAMS = [
    ("clash_api", True, "Clash 兼容 API（供第三方 GUI 使用）"),
    ("v2ray_api", True, "V2Ray 兼容 API（流量统计）"),
    ("cache_file", True, "缓存文件（保存节点测速/选择状态）"),
    ("debug", False, "调试 API（pprof 性能分析）"),
]

# 服务配置参数（SFI 相关）
SFI_SERVICE_TYPES = [
    ("api", True, "sing-box 原生 API 服务"),
    ("mitm", True, "MITM HTTPS 中间人攻击服务（本项目核心）"),
    ("clashapi", False, "Clash API 服务（已废弃，用 experimental.clash_api）"),
    ("v2ray-api", False, "V2Ray API 服务（已废弃，用 experimental.v2ray_api）"),
]

# TUN 入站参数（SFI 必须）
SFI_TUN_PARAMS = [
    ("type", True, "入站类型，必须为 tun"),
    ("tag", True, "入站标签"),
    ("interface_name", False, "虚拟网卡名称"),
    ("mtu", False, "最大传输单元"),
    ("gso", False, "通用分段卸载"),
    ("address", True, "虚拟网卡 IP 地址"),
    ("stack", True, "协议栈（gvisor/system）"),
    ("route_address", False, "路由地址列表"),
    ("route_exclude_address", False, "排除路由地址"),
    ("auto_route", False, "自动路由"),
    ("strict_route", False, "严格路由"),
    ("endpoint_independent_nat", False, "端点无关 NAT"),
    ("udp_timeout", False, "UDP 超时时间"),
]

# API 方法清单（SFI 客户端调用的 libbox API）
SFI_API_METHODS = [
    ("LibboxSetup", "初始化 libbox 服务"),
    ("LibboxNewCommandServer", "创建命令服务器"),
    ("LibboxNewStandaloneCommandClient", "创建独立命令客户端"),
    ("LibboxSetXPCDialer", "设置 XPC 拨号器"),
    ("LibboxCheckConfig", "校验配置"),
    ("LibboxFormatConfig", "格式化配置"),
    ("LibboxGenerateMITMCA", "生成 MITM CA 证书"),
]


# ========== 文档解析 ==========

def extract_doc_params(doc_path: Path) -> Dict[str, any]:
    """从接入文档中提取参数信息"""
    content = doc_path.read_text(encoding='utf-8', errors='ignore')

    result = {
        'top_level_fields': set(),
        'inbound_types': set(),
        'outbound_types': set(),
        'dns_params': set(),
        'route_params': set(),
        'experimental_params': set(),
        'service_types': set(),
        'tun_params': set(),
        'api_methods': set(),
        'raw_content': content,
    }

    # 提取顶层字段（从表格中）
    top_level_pattern = re.findall(r'\| `(\w+)` \|', content)
    for field in top_level_pattern:
        if field in [f[0] for f in SFI_TOP_LEVEL_FIELDS] or field in [f[0] for f in SFI_NOT_APPLICABLE_TOP_LEVEL]:
            result['top_level_fields'].add(field)

    # 提取入站类型
    for inbound_type, _, _ in SFI_INBOUND_TYPES + SFI_NOT_APPLICABLE_INBOUNDS:
        if re.search(rf'`{re.escape(inbound_type)}`', content):
            result['inbound_types'].add(inbound_type)

    # 提取出站类型
    for outbound_type, _, _ in SFI_OUTBOUND_TYPES:
        if re.search(rf'`{re.escape(outbound_type)}`', content):
            result['outbound_types'].add(outbound_type)

    # 提取 DNS 参数
    for param, _, _ in SFI_DNS_PARAMS:
        if re.search(rf'`dns\.{re.escape(param)}`|`{re.escape(param)}`', content):
            result['dns_params'].add(param)

    # 提取路由参数
    for param, _, _ in SFI_ROUTE_PARAMS:
        if re.search(rf'`route\.{re.escape(param)}`|`{re.escape(param)}`', content):
            result['route_params'].add(param)

    # 提取实验配置参数
    for param, _, _ in SFI_EXPERIMENTAL_PARAMS:
        if re.search(rf'`experimental\.{re.escape(param)}`|`{re.escape(param)}`', content):
            result['experimental_params'].add(param)

    # 提取服务类型
    for service_type, _, _ in SFI_SERVICE_TYPES:
        if re.search(rf'`{re.escape(service_type)}`', content):
            result['service_types'].add(service_type)

    # 提取 TUN 参数
    for param, _, _ in SFI_TUN_PARAMS:
        if re.search(rf'`{re.escape(param)}`', content):
            result['tun_params'].add(param)

    # 提取 API 方法
    for api_method, _ in SFI_API_METHODS:
        if api_method in content:
            result['api_methods'].add(api_method)

    return result


# ========== 内核代码解析 ==========

def extract_core_registry(core_path: Path) -> Dict[str, Set[str]]:
    """从内核注册表中提取支持的类型"""
    registry = {
        'inbounds': set(),
        'outbounds': set(),
        'services': set(),
    }

    include_dir = core_path / "include"
    if not include_dir.exists():
        return registry

    all_content = ""
    for go_file in include_dir.rglob("*.go"):
        if go_file.name.endswith("_test.go"):
            continue
        try:
            all_content += go_file.read_text(encoding='utf-8', errors='ignore') + "\n"
        except Exception:
            continue

    # 入站注册
    inbound_matches = re.findall(r'(\w+)\.Register(?:Inbound|Redirect|TProxy)\(registry\)', all_content)
    for m in inbound_matches:
        name = m.lower()
        if name == 'cloudflared':
            name = 'cloudflare'
        registry['inbounds'].add(name)

    # 出站注册
    outbound_outbound = re.findall(r'(\w+)\.RegisterOutbound\(registry\)', all_content)
    outbound_selector = re.findall(r'\w+\.RegisterSelector\(registry\)', all_content)
    outbound_urltest = re.findall(r'\w+\.RegisterURLTest\(registry\)', all_content)
    for m in outbound_outbound:
        registry['outbounds'].add(m.lower())
    if outbound_selector:
        registry['outbounds'].add('selector')
    if outbound_urltest:
        registry['outbounds'].add('urltest')

    # 服务注册
    service_matches = re.findall(r'(\w+)\.RegisterService\(registry\)|register(\w+)(?:Service|RealmService)\(registry\)', all_content)
    for m in service_matches:
        name = (m[0] or m[1]).lower()
        if name:
            registry['services'].add(name)

    return registry


def extract_core_params(core_path: Path) -> Dict[str, Set[str]]:
    """从内核 option 包中提取参数定义"""
    params = {
        'dns': set(),
        'route': set(),
        'experimental': set(),
        'tun': set(),
        'top_level': set(),
    }

    option_dir = core_path / "option"
    if not option_dir.exists():
        return params

    # 顶层字段（Options 结构体）
    options_file = option_dir / "options.go"
    if options_file.exists():
        content = options_file.read_text(encoding='utf-8', errors='ignore')
        struct_blocks = re.findall(r'type\s+Options\s+struct\s*\{([^}]*)\}', content, re.DOTALL)
        for block in struct_blocks:
            fields = re.findall(r'`[^`]*json:"([a-z][a-zA-Z0-9_]*)"', block)
            for field in fields:
                if field and field != "-":
                    params['top_level'].add(field)

    # DNS 参数
    dns_file = option_dir / "dns.go"
    if dns_file.exists():
        content = dns_file.read_text(encoding='utf-8', errors='ignore')
        struct_blocks = re.findall(r'type\s+\w*DNS\w*\s+struct\s*\{([^}]*)\}', content, re.DOTALL)
        for block in struct_blocks:
            fields = re.findall(r'`[^`]*json:"([a-z][a-zA-Z0-9_]*)"', block)
            for field in fields:
                if field and field != "-":
                    params['dns'].add(field)

    # 路由参数
    route_file = option_dir / "route.go"
    if route_file.exists():
        content = route_file.read_text(encoding='utf-8', errors='ignore')
        struct_blocks = re.findall(r'type\s+\w*Route\w*\s+struct\s*\{([^}]*)\}', content, re.DOTALL)
        for block in struct_blocks:
            fields = re.findall(r'`[^`]*json:"([a-z][a-zA-Z0-9_]*)"', block)
            for field in fields:
                if field and field != "-":
                    params['route'].add(field)

    # 实验配置参数
    for exp_file in ["experimental.go", "clash_api.go", "v2ray_api.go", "cache_file.go"]:
        file_path = option_dir / exp_file
        if file_path.exists():
            content = file_path.read_text(encoding='utf-8', errors='ignore')
            fields = re.findall(r'`[^`]*json:"([a-z][a-zA-Z0-9_]*)"', content)
            for field in fields:
                if field and field != "-":
                    params['experimental'].add(field)

    # TUN 参数
    tun_file = option_dir / "inbound_tun.go"
    if tun_file.exists():
        content = tun_file.read_text(encoding='utf-8', errors='ignore')
        struct_blocks = re.findall(r'type\s+\w*TUN\w*\s+struct\s*\{([^}]*)\}', content, re.DOTALL)
        for block in struct_blocks:
            fields = re.findall(r'`[^`]*json:"([a-z][a-zA-Z0-9_]*)"', block)
            for field in fields:
                if field and field != "-":
                    params['tun'].add(field)

    return params


# ========== 客户端代码解析 ==========

def extract_client_usage(client_path: Path) -> Dict[str, Set[str]]:
    """从客户端 Swift 代码中提取参数使用情况"""
    usage = {
        'top_level': set(),
        'inbound_types': set(),
        'outbound_types': set(),
        'dns_params': set(),
        'route_params': set(),
        'experimental_params': set(),
        'service_types': set(),
        'api_methods': set(),
    }

    swift_files = list(client_path.rglob("*.swift"))
    all_content = ""
    for swift_file in swift_files:
        if ".build" in str(swift_file) or "Frameworks" in str(swift_file):
            continue
        try:
            all_content += swift_file.read_text(encoding='utf-8', errors='ignore') + "\n"
        except Exception:
            continue

    # 顶层字段
    for field, _, _ in SFI_TOP_LEVEL_FIELDS:
        if f'"{field}"' in all_content or f'`{field}`' in all_content:
            usage['top_level'].add(field)

    # 入站类型
    for inbound_type, _, _ in SFI_INBOUND_TYPES:
        if f'"{inbound_type}"' in all_content:
            usage['inbound_types'].add(inbound_type)

    # 出站类型
    for outbound_type, _, _ in SFI_OUTBOUND_TYPES:
        if f'"{outbound_type}"' in all_content:
            usage['outbound_types'].add(outbound_type)

    # API 方法
    for api_method, _ in SFI_API_METHODS:
        if api_method in all_content:
            usage['api_methods'].add(api_method)

    return usage


# ========== 检查逻辑 ==========

def run_checks(doc_path: Path, core_path: Path, client_path: Path) -> Tuple[Dict, List[str], List[str]]:
    """运行所有检查，返回 (结果字典, 错误列表, 警告列表)"""
    errors = []
    warnings = []
    results = {
        'top_level': {'required': [], 'optional': [], 'not_applicable': [], 'missing_required': []},
        'inbounds': {'sfi_used': [], 'not_applicable': [], 'missing_required': []},
        'outbounds': {'sfi_used': [], 'missing_required': []},
        'dns': {'documented': [], 'missing_required': []},
        'route': {'documented': [], 'missing_required': []},
        'experimental': {'documented': [], 'missing_required': []},
        'services': {'documented': [], 'missing_required': []},
        'tun': {'documented': [], 'missing_required': []},
        'api': {'documented': [], 'missing_required': []},
        'client_usage': {},
    }

    # 解析文档
    doc = extract_doc_params(doc_path)

    # 解析内核
    core_registry = extract_core_registry(core_path)
    core_params = extract_core_params(core_path)

    # 解析客户端
    client_usage = extract_client_usage(client_path)

    # ===== 1. 顶层字段检查 =====
    for field, required, desc in SFI_TOP_LEVEL_FIELDS:
        in_doc = field in doc['top_level_fields']
        in_core = field in core_params['top_level']
        in_client = field in client_usage['top_level']

        item = {
            'field': field,
            'required': required,
            'description': desc,
            'in_doc': in_doc,
            'in_core': in_core,
            'in_client': in_client,
        }

        if required:
            results['top_level']['required'].append(item)
            if not in_doc:
                errors.append(f"[顶层字段] 必须字段 `{field}` 未在文档中说明")
                results['top_level']['missing_required'].append(field)
            if not in_core:
                warnings.append(f"[顶层字段] 必须字段 `{field}` 未在内核 option 中找到定义")
        else:
            results['top_level']['optional'].append(item)
            if not in_doc:
                warnings.append(f"[顶层字段] 可选字段 `{field}` 未在文档中说明")

    # 不适用的顶层字段（文档中不应作为 SFI 推荐配置）
    for field, reason in SFI_NOT_APPLICABLE_TOP_LEVEL:
        in_doc = field in doc['top_level_fields']
        results['top_level']['not_applicable'].append({
            'field': field,
            'reason': reason,
            'in_doc': in_doc,
        })
        if in_doc:
            warnings.append(f"[顶层字段] 文档包含不适用 SFI 的字段 `{field}`（{reason}）")

    # ===== 2. 入站类型检查 =====
    for inbound_type, required, desc in SFI_INBOUND_TYPES:
        in_doc = inbound_type in doc['inbound_types']
        in_core = inbound_type in core_registry['inbounds']
        in_client = inbound_type in client_usage['inbound_types']

        item = {
            'type': inbound_type,
            'required': required,
            'description': desc,
            'in_doc': in_doc,
            'in_core': in_core,
            'in_client': in_client,
        }

        results['inbounds']['sfi_used'].append(item)

        if required:
            if not in_doc:
                errors.append(f"[入站类型] 必须类型 `{inbound_type}` 未在文档中说明")
                results['inbounds']['missing_required'].append(inbound_type)
            if not in_core:
                errors.append(f"[入站类型] 必须类型 `{inbound_type}` 未在内核注册表中注册")

    # 不适用的入站类型
    for inbound_type, reason in SFI_NOT_APPLICABLE_INBOUNDS:
        in_doc = inbound_type in doc['inbound_types']
        results['inbounds']['not_applicable'].append({
            'type': inbound_type,
            'reason': reason,
            'in_doc': in_doc,
        })

    # ===== 3. 出站类型检查 =====
    for outbound_type, required, desc in SFI_OUTBOUND_TYPES:
        in_doc = outbound_type in doc['outbound_types']
        in_core = outbound_type in core_registry['outbounds']
        in_client = outbound_type in client_usage['outbound_types']

        item = {
            'type': outbound_type,
            'required': required,
            'description': desc,
            'in_doc': in_doc,
            'in_core': in_core,
            'in_client': in_client,
        }

        results['outbounds']['sfi_used'].append(item)

        if required:
            if not in_doc:
                errors.append(f"[出站类型] 必须类型 `{outbound_type}` 未在文档中说明")
                results['outbounds']['missing_required'].append(outbound_type)
            if not in_core:
                errors.append(f"[出站类型] 必须类型 `{outbound_type}` 未在内核注册表中注册")

    # ===== 4. DNS 参数检查 =====
    for param, required, desc in SFI_DNS_PARAMS:
        in_doc = param in doc['dns_params']
        in_core = param in core_params['dns']

        item = {
            'param': param,
            'required': required,
            'description': desc,
            'in_doc': in_doc,
            'in_core': in_core,
        }
        results['dns']['documented'].append(item)

        if required and not in_doc:
            errors.append(f"[DNS参数] 必须参数 `dns.{param}` 未在文档中说明")
            results['dns']['missing_required'].append(param)

    # ===== 5. 路由参数检查 =====
    for param, required, desc in SFI_ROUTE_PARAMS:
        in_doc = param in doc['route_params']
        in_core = param in core_params['route']

        item = {
            'param': param,
            'required': required,
            'description': desc,
            'in_doc': in_doc,
            'in_core': in_core,
        }
        results['route']['documented'].append(item)

        if required and not in_doc:
            errors.append(f"[路由参数] 必须参数 `route.{param}` 未在文档中说明")
            results['route']['missing_required'].append(param)

    # ===== 6. 实验配置参数检查 =====
    for param, required, desc in SFI_EXPERIMENTAL_PARAMS:
        in_doc = param in doc['experimental_params']
        in_core = param in core_params['experimental']

        item = {
            'param': param,
            'required': required,
            'description': desc,
            'in_doc': in_doc,
            'in_core': in_core,
        }
        results['experimental']['documented'].append(item)

        if required and not in_doc:
            errors.append(f"[实验参数] 必须参数 `experimental.{param}` 未在文档中说明")
            results['experimental']['missing_required'].append(param)

    # ===== 7. 服务类型检查 =====
    for service_type, required, desc in SFI_SERVICE_TYPES:
        in_doc = service_type in doc['service_types']
        in_core = service_type in core_registry['services']

        item = {
            'type': service_type,
            'required': required,
            'description': desc,
            'in_doc': in_doc,
            'in_core': in_core,
        }
        results['services']['documented'].append(item)

        if required and not in_doc:
            errors.append(f"[服务类型] 必须类型 `{service_type}` 未在文档中说明")
            results['services']['missing_required'].append(service_type)

    # ===== 8. TUN 参数检查 =====
    for param, required, desc in SFI_TUN_PARAMS:
        in_doc = param in doc['tun_params']
        in_core = param in core_params['tun']

        item = {
            'param': param,
            'required': required,
            'description': desc,
            'in_doc': in_doc,
            'in_core': in_core,
        }
        results['tun']['documented'].append(item)

        if required and not in_doc:
            errors.append(f"[TUN参数] 必须参数 `{param}` 未在文档中说明")
            results['tun']['missing_required'].append(param)

    # ===== 9. API 方法检查 =====
    for api_method, desc in SFI_API_METHODS:
        in_doc = api_method in doc['api_methods']
        in_client = api_method in client_usage['api_methods']

        item = {
            'method': api_method,
            'description': desc,
            'in_doc': in_doc,
            'in_client': in_client,
        }
        results['api']['documented'].append(item)

        if not in_doc:
            warnings.append(f"[API方法] `{api_method}` 未在文档中说明")

    # 客户端使用情况
    results['client_usage'] = {
        'top_level': sorted(client_usage['top_level']),
        'inbound_types': sorted(client_usage['inbound_types']),
        'outbound_types': sorted(client_usage['outbound_types']),
        'api_methods': sorted(client_usage['api_methods']),
    }

    return results, errors, warnings


# ========== 报告生成 ==========

def generate_report(results: Dict, errors: List[str], warnings: List[str], doc_path: Path) -> str:
    """生成 Markdown 格式的检查报告"""
    lines = []

    lines.append("# 📋 SFI 客户端接入文档参数一致性检查报告")
    lines.append("")
    lines.append(f"> 文档: `{doc_path.name}` | 检查范围: SFI iOS 客户端实际接入参数")
    lines.append("")

    # 总体统计
    total_checks = 0
    passed_checks = 0
    for category in ['top_level', 'inbounds', 'outbounds', 'dns', 'route', 'experimental', 'services', 'tun', 'api']:
        if category == 'top_level':
            total_checks += len(results['top_level']['required']) + len(results['top_level']['optional'])
            for item in results['top_level']['required'] + results['top_level']['optional']:
                if item['in_doc']:
                    passed_checks += 1
        elif category in ['inbounds', 'outbounds']:
            total_checks += len(results[category]['sfi_used'])
            for item in results[category]['sfi_used']:
                if item['in_doc'] and item['in_core']:
                    passed_checks += 1
        elif category in ['dns', 'route', 'experimental', 'services', 'tun']:
            total_checks += len(results[category]['documented'])
            for item in results[category]['documented']:
                if item['in_doc']:
                    passed_checks += 1
        elif category == 'api':
            total_checks += len(results['api']['documented'])
            for item in results['api']['documented']:
                if item['in_doc']:
                    passed_checks += 1

    lines.append("## 📊 总体统计")
    lines.append("")
    lines.append("| 指标 | 数量 |")
    lines.append("|------|------|")
    lines.append(f"| 检查项总数 | {total_checks} |")
    lines.append(f"| 通过检查项 | {passed_checks} |")
    lines.append(f"| 错误总数 | {len(errors)} |")
    lines.append(f"| 警告总数 | {len(warnings)} |")
    lines.append("")

    # 错误详情
    if errors:
        lines.append("## 🔴 错误详情（必须修复）")
        lines.append("")
        for i, error in enumerate(errors, 1):
            lines.append(f"{i}. {error}")
        lines.append("")

    # 各模块详细检查
    lines.append("## 📝 各模块详细检查")
    lines.append("")

    # 顶层字段
    lines.append("### 1. 顶层字段")
    lines.append("")
    lines.append("#### 必须字段")
    lines.append("")
    lines.append("| 字段 | 说明 | 文档 | 内核 | 状态 |")
    lines.append("|------|------|------|------|------|")
    for item in results['top_level']['required']:
        status = "✅" if item['in_doc'] and item['in_core'] else "❌"
        lines.append(f"| `{item['field']}` | {item['description']} | {'✅' if item['in_doc'] else '❌'} | {'✅' if item['in_core'] else '❌'} | {status} |")
    lines.append("")

    lines.append("#### 可选字段")
    lines.append("")
    lines.append("| 字段 | 说明 | 文档 | 内核 | 状态 |")
    lines.append("|------|------|------|------|------|")
    for item in results['top_level']['optional']:
        status = "✅" if item['in_doc'] else "⚠️"
        lines.append(f"| `{item['field']}` | {item['description']} | {'✅' if item['in_doc'] else '⚠️'} | {'✅' if item['in_core'] else '❓'} | {status} |")
    lines.append("")

    lines.append("#### 不适用 SFI 的字段")
    lines.append("")
    lines.append("| 字段 | 原因 | 文档中出现 |")
    lines.append("|------|------|-----------|")
    for item in results['top_level']['not_applicable']:
        lines.append(f"| `{item['field']}` | {item['reason']} | {'⚠️ 是' if item['in_doc'] else '✅ 否'} |")
    lines.append("")

    # 入站类型
    lines.append("### 2. 入站类型")
    lines.append("")
    lines.append("#### SFI 实际使用的入站类型")
    lines.append("")
    lines.append("| 类型 | 说明 | 必须 | 文档 | 内核 | 状态 |")
    lines.append("|------|------|------|------|------|------|")
    for item in results['inbounds']['sfi_used']:
        status = "✅" if item['in_doc'] and item['in_core'] else "❌"
        lines.append(f"| `{item['type']}` | {item['description']} | {'是' if item['required'] else '否'} | {'✅' if item['in_doc'] else '❌'} | {'✅' if item['in_core'] else '❌'} | {status} |")
    lines.append("")

    lines.append("#### 不适用 SFI 的入站类型（服务端/Linux 特有）")
    lines.append("")
    lines.append("| 类型 | 原因 | 文档中出现 |")
    lines.append("|------|------|-----------|")
    for item in results['inbounds']['not_applicable']:
        lines.append(f"| `{item['type']}` | {item['reason']} | {'⚠️ 是' if item['in_doc'] else '✅ 否'} |")
    lines.append("")

    # 出站类型
    lines.append("### 3. 出站类型")
    lines.append("")
    lines.append("| 类型 | 说明 | 必须 | 文档 | 内核 | 状态 |")
    lines.append("|------|------|------|------|------|------|")
    for item in results['outbounds']['sfi_used']:
        status = "✅" if item['in_doc'] and item['in_core'] else "❌"
        lines.append(f"| `{item['type']}` | {item['description']} | {'是' if item['required'] else '否'} | {'✅' if item['in_doc'] else '❌'} | {'✅' if item['in_core'] else '❌'} | {status} |")
    lines.append("")

    # DNS 参数
    lines.append("### 4. DNS 配置参数")
    lines.append("")
    lines.append("| 参数 | 说明 | 必须 | 文档 | 内核 | 状态 |")
    lines.append("|------|------|------|------|------|------|")
    for item in results['dns']['documented']:
        status = "✅" if item['in_doc'] else ("❌" if item['required'] else "⚠️")
        lines.append(f"| `dns.{item['param']}` | {item['description']} | {'是' if item['required'] else '否'} | {'✅' if item['in_doc'] else '❌'} | {'✅' if item['in_core'] else '❓'} | {status} |")
    lines.append("")

    # 路由参数
    lines.append("### 5. 路由配置参数")
    lines.append("")
    lines.append("| 参数 | 说明 | 必须 | 文档 | 内核 | 状态 |")
    lines.append("|------|------|------|------|------|------|")
    for item in results['route']['documented']:
        status = "✅" if item['in_doc'] else ("❌" if item['required'] else "⚠️")
        lines.append(f"| `route.{item['param']}` | {item['description']} | {'是' if item['required'] else '否'} | {'✅' if item['in_doc'] else '❌'} | {'✅' if item['in_core'] else '❓'} | {status} |")
    lines.append("")

    # 实验配置参数
    lines.append("### 6. 实验配置参数")
    lines.append("")
    lines.append("| 参数 | 说明 | 必须 | 文档 | 内核 | 状态 |")
    lines.append("|------|------|------|------|------|------|")
    for item in results['experimental']['documented']:
        status = "✅" if item['in_doc'] else ("❌" if item['required'] else "⚠️")
        lines.append(f"| `experimental.{item['param']}` | {item['description']} | {'是' if item['required'] else '否'} | {'✅' if item['in_doc'] else '❌'} | {'✅' if item['in_core'] else '❓'} | {status} |")
    lines.append("")

    # 服务类型
    lines.append("### 7. 服务配置类型")
    lines.append("")
    lines.append("| 类型 | 说明 | 必须 | 文档 | 内核 | 状态 |")
    lines.append("|------|------|------|------|------|------|")
    for item in results['services']['documented']:
        status = "✅" if item['in_doc'] else ("❌" if item['required'] else "⚠️")
        lines.append(f"| `{item['type']}` | {item['description']} | {'是' if item['required'] else '否'} | {'✅' if item['in_doc'] else '❌'} | {'✅' if item['in_core'] else '❓'} | {status} |")
    lines.append("")

    # TUN 参数
    lines.append("### 8. TUN 入站参数（iOS 必须）")
    lines.append("")
    lines.append("| 参数 | 说明 | 必须 | 文档 | 内核 | 状态 |")
    lines.append("|------|------|------|------|------|------|")
    for item in results['tun']['documented']:
        status = "✅" if item['in_doc'] else ("❌" if item['required'] else "⚠️")
        lines.append(f"| `{item['param']}` | {item['description']} | {'是' if item['required'] else '否'} | {'✅' if item['in_doc'] else '❌'} | {'✅' if item['in_core'] else '❓'} | {status} |")
    lines.append("")

    # API 方法
    lines.append("### 9. libbox API 方法")
    lines.append("")
    lines.append("| 方法 | 说明 | 文档 | 客户端调用 | 状态 |")
    lines.append("|------|------|------|-----------|------|")
    for item in results['api']['documented']:
        status = "✅" if item['in_doc'] else "⚠️"
        lines.append(f"| `{item['method']}` | {item['description']} | {'✅' if item['in_doc'] else '⚠️'} | {'✅' if item['in_client'] else '❓'} | {status} |")
    lines.append("")

    # 客户端使用情况
    lines.append("### 10. 客户端代码实际引用")
    lines.append("")
    lines.append("| 类别 | 引用的参数/类型 |")
    lines.append("|------|----------------|")
    for category, items in results['client_usage'].items():
        if items:
            lines.append(f"| {category} | {', '.join(f'`{i}`' for i in items)} |")
        else:
            lines.append(f"| {category} | （未检测到） |")
    lines.append("")

    # 警告详情
    if warnings:
        lines.append("## 🟡 警告详情（建议优化）")
        lines.append("")
        for i, warning in enumerate(warnings, 1):
            lines.append(f"{i}. {warning}")
        lines.append("")

    # 结论
    lines.append("## ✅ 检查结论")
    lines.append("")
    if errors:
        lines.append(f"❌ **检查未通过**，存在 {len(errors)} 个错误必须修复。")
    else:
        lines.append(f"✅ **错误检查通过**，存在 {len(warnings)} 个警告（不阻塞，建议优化）。")
    lines.append("")
    lines.append(f"- 检查项总数：{total_checks}")
    lines.append(f"- 通过检查项：{passed_checks}")
    lines.append(f"- 覆盖率：{passed_checks}/{total_checks} = {passed_checks*100//total_checks if total_checks > 0 else 0}%")
    lines.append("")

    return "\n".join(lines)


# ========== 主函数 ==========

def main():
    parser = argparse.ArgumentParser(description='SFI 客户端接入文档参数一致性检查')
    parser.add_argument('--doc', required=True, help='接入文档路径')
    parser.add_argument('--core', required=True, help='sing-box 内核代码路径')
    parser.add_argument('--client', required=True, help='SFI 客户端代码路径')
    parser.add_argument('--output', default='doc-full-check-report.md', help='报告输出路径')
    parser.add_argument('--json', default='doc-full-check-result.json', help='JSON 结果输出路径')
    args = parser.parse_args()

    doc_path = Path(args.doc)
    core_path = Path(args.core)
    client_path = Path(args.client)

    if not doc_path.exists():
        print(f"❌ 文档不存在: {doc_path}")
        sys.exit(1)
    if not core_path.exists():
        print(f"❌ 内核代码不存在: {core_path}")
        sys.exit(1)

    print("==========================================")
    print("  SFI 客户端接入文档参数一致性检查")
    print("  （聚焦 SFI iOS 客户端实际接入参数）")
    print("==========================================")
    print("")

    # 运行检查
    results, errors, warnings = run_checks(doc_path, core_path, client_path)

    # 生成报告
    report = generate_report(results, errors, warnings, doc_path)

    # 保存报告
    output_path = Path(args.output)
    output_path.parent.mkdir(parents=True, exist_ok=True)
    output_path.write_text(report, encoding='utf-8')
    print(f"📄 报告已保存: {output_path}")

    # 保存 JSON 结果
    json_path = Path(args.json)
    json_path.parent.mkdir(parents=True, exist_ok=True)
    json_result = {
        'errors': errors,
        'warnings': warnings,
        'results': results,
    }
    json_path.write_text(json.dumps(json_result, ensure_ascii=False, indent=2), encoding='utf-8')
    print(f"📊 JSON 结果已保存: {json_path}")
    print("")

    # 输出摘要
    print("=== 检查摘要 ===")
    print(f"  错误: {len(errors)}")
    print(f"  警告: {len(warnings)}")
    print("")

    if errors:
        print("🔴 错误列表:")
        for error in errors:
            print(f"  - {error}")
        print("")

    if errors:
        print(f"❌ 检查未通过，存在 {len(errors)} 个错误")
        sys.exit(1)
    else:
        print(f"✅ 检查通过（{len(warnings)} 个警告）")
        sys.exit(0)


if __name__ == '__main__':
    main()
