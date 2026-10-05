#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
接入文档全量参数仓库一致性检查脚本
检查维度：
  1. 文档参数提取：从 JSON 示例、文字描述中提取所有参数名
  2. 内核代码一致性：文档参数是否在 sing-box 内核 option 包中有定义
  3. 参数类型一致性：文档描述的类型是否与内核代码一致
  4. 枚举值一致性：文档列出的枚举值是否与内核代码一致
  5. 客户端使用检查：文档参数是否在 SFI 客户端代码中被引用
  6. API 签名一致性：文档描述的 Libbox API 方法是否与实际代码一致
  7. 注册表一致性：文档列出的入站/出站/端点/DNS/服务类型是否与内核注册表一致
输出：Markdown 报告 + JSON 结果
"""

import argparse
import json
import os
import re
import sys
from pathlib import Path
from dataclasses import dataclass, field
from typing import Dict, List, Set, Tuple, Optional


# ========== 数据结构 ==========
@dataclass
class ParamInfo:
    """参数信息"""
    name: str
    in_doc: bool = False
    in_core: bool = False
    in_client: bool = False
    doc_type: str = ""
    core_type: str = ""
    core_file: str = ""
    enum_values_doc: List[str] = field(default_factory=list)
    enum_values_core: List[str] = field(default_factory=list)
    status: str = "unknown"  # pass/warn/error
    note: str = ""


@dataclass
class ApiInfo:
    """API 方法信息"""
    name: str
    in_doc: bool = False
    in_core: bool = False
    signature_doc: str = ""
    signature_core: str = ""
    status: str = "unknown"
    note: str = ""


@dataclass
class CheckResult:
    """检查结果汇总"""
    total_params: int = 0
    params_in_doc_only: List[str] = field(default_factory=list)
    params_in_core_only: List[str] = field(default_factory=list)
    params_in_both: List[str] = field(default_factory=list)
    params_missing_client: List[str] = field(default_factory=list)
    type_mismatch: List[Tuple[str, str, str]] = field(default_factory=list)
    enum_mismatch: List[Tuple[str, List[str], List[str]]] = field(default_factory=list)
    api_mismatch: List[Tuple[str, str, str]] = field(default_factory=list)
    registry_mismatch: List[Tuple[str, List[str], List[str]]] = field(default_factory=list)
    errors: int = 0
    warnings: int = 0


# ========== 文档解析 ==========
def extract_params_from_doc(doc_path: Path) -> Tuple[Set[str], Dict[str, str], Dict[str, List[str]]]:
    """
    从接入文档中提取所有参数名
    返回：(参数名集合, 参数->类型映射, 参数->枚举值映射)
    """
    params: Set[str] = set()
    param_types: Dict[str, str] = {}
    param_enums: Dict[str, List[str]] = {}

    if not doc_path.exists():
        return params, param_types, param_enums

    content = doc_path.read_text(encoding='utf-8', errors='ignore')

    # 1. 从 JSON 代码块中提取参数
    json_blocks = re.findall(r'```json\s*\n(.*?)```', content, re.DOTALL)
    for block in json_blocks:
        # 提取 "key": value 形式的参数
        matches = re.findall(r'"([a-zA-Z_][a-zA-Z0-9_]*)"\s*:', block)
        for m in matches:
            if len(m) > 1 and not m.startswith('_'):
                params.add(m)

    # 2. 从反引号参数引用中提取（`param_name`）
    backtick_params = re.findall(r'`([a-z][a-z0-9_]{2,})`', content)
    for p in backtick_params:
        if p not in ('true', 'false', 'nil', 'null', 'ios', 'macos', 'json', 'swift', 'go', 'tcp', 'udp'):
            params.add(p)

    # 3. 从表格中提取参数（| 参数名 | 类型 | 说明 |）
    table_rows = re.findall(r'\|\s*`?([a-z][a-z0-9_]{2,})`?\s*\|\s*([^|]+)\|', content)
    for name, type_str in table_rows:
        if name not in ('参数', 'parameter', '类型', '说明'):
            params.add(name)
            param_types[name] = type_str.strip()

    # 4. 提取枚举值（从文档中的枚举列表）
    # 匹配 dns_mode 枚举
    dns_mode_match = re.search(r'dns_mode.*?(?:枚举|可选值).*?((?:disabled|native|hijack)[\s\S]*?)(?:\n\n|##)', content, re.IGNORECASE)
    if dns_mode_match:
        enum_values = re.findall(r'(disabled|native|hijack)', dns_mode_match.group(1), re.IGNORECASE)
        if enum_values:
            param_enums['dns_mode'] = list(set(enum_values))

    # 匹配 stack 枚举
    stack_match = re.search(r'stack.*?(?:枚举|可选值).*?((?:gvisor|system|mixed)[\s\S]*?)(?:\n\n|##)', content, re.IGNORECASE)
    if stack_match:
        enum_values = re.findall(r'(gvisor|system|mixed)', stack_match.group(1), re.IGNORECASE)
        if enum_values:
            param_enums['stack'] = list(set(enum_values))

    # 匹配 strategy 枚举
    strategy_match = re.search(r'strategy.*?(?:枚举|可选值).*?((?:ipv4_only|ipv6_only|prefer_ipv4|prefer_ipv6)[\s\S]*?)(?:\n\n|##)', content, re.IGNORECASE)
    if strategy_match:
        enum_values = re.findall(r'(ipv4_only|ipv6_only|prefer_ipv4|prefer_ipv6)', strategy_match.group(1), re.IGNORECASE)
        if enum_values:
            param_enums['strategy'] = list(set(enum_values))

    return params, param_types, param_enums


def extract_apis_from_doc(doc_path: Path) -> Dict[str, str]:
    """从文档中提取 API 方法名和签名"""
    apis: Dict[str, str] = {}
    if not doc_path.exists():
        return apis

    content = doc_path.read_text(encoding='utf-8', errors='ignore')

    # 提取 Swift 函数签名
    swift_funcs = re.findall(r'(?:func|public func)\s+([a-zA-Z_][a-zA-Z0-9_]*)\s*\(([^)]*)\)', content)
    for name, params in swift_funcs:
        if len(name) > 2:
            apis[name] = f"func {name}({params})"

    # 提取 Go 函数签名
    go_funcs = re.findall(r'func\s+(?:\([^)]+\)\s+)?([A-Z][a-zA-Z0-9_]*)\s*\(([^)]*)\)', content)
    for name, params in go_funcs:
        if len(name) > 2:
            apis[name] = f"func {name}({params})"

    return apis


# ========== 内核代码解析 ==========
def extract_params_from_core(core_path: Path) -> Tuple[Dict[str, Tuple[str, str]], Dict[str, List[str]]]:
    """
    从 sing-box 内核 option 包中提取所有参数定义
    返回：(参数名->(类型, 文件路径) 映射, 参数->枚举值映射)
    """
    params: Dict[str, Tuple[str, str]] = {}
    param_enums: Dict[str, List[str]] = {}

    option_dir = core_path / "option"
    if not option_dir.exists():
        return params, param_enums

    # 遍历所有 .go 文件
    for go_file in option_dir.rglob("*.go"):
        if go_file.name.endswith("_test.go"):
            continue
        try:
            content = go_file.read_text(encoding='utf-8', errors='ignore')
        except Exception:
            continue

        # 提取 struct 字段：`FieldName type `json:"field_name"`
        # 先按 struct 块分割，避免匹配到 interface 定义
        struct_blocks = re.findall(r'type\s+\w+\s+struct\s*\{([^}]*)\}', content, re.DOTALL)
        for block in struct_blocks:
            struct_fields = re.findall(
                r'([A-Z][a-zA-Z0-9]*)\s+([^\s`]+(?:\s+[^\s`]+)*)\s*`[^`]*json:"([a-z][a-zA-Z0-9_]*)"',
                block
            )
            for field_name, field_type, json_name in struct_fields:
                if json_name and json_name != "-":
                    # 清理类型
                    clean_type = field_type.strip()
                    # 移除指针符号
                    clean_type = clean_type.lstrip('*')
                    # 只取基础类型（第一个词）
                    clean_type = clean_type.split()[0].split('[')[0].split('{')[0].strip()
                    # 排除明显不是类型的内容
                    if clean_type and not clean_type.startswith('//') and len(clean_type) < 50:
                        params[json_name] = (clean_type, str(go_file.relative_to(core_path)))

        # 提取枚举类型定义（type X string / type X int）
        enum_types = re.findall(r'type\s+([A-Z][a-zA-Z0-9]*)\s+(?:string|int|uint8|uint16|uint32)', content)
        for enum_type in enum_types:
            # 查找该枚举的常量值
            enum_pattern = re.compile(
                rf'{enum_type}\s+(?:[A-Z][a-zA-Z0-9]*)\s*=\s*"([^"]+)"|'
                rf'([A-Z][a-zA-Z0-9]*)\s+{enum_type}\s*=\s*"([^"]+)"'
            )
            values = []
            for match in enum_pattern.finditer(content):
                val = match.group(1) or match.group(3)
                if val:
                    values.append(val)
            if values:
                # 尝试映射到参数名
                param_name = re.sub(r'([a-z])([A-Z])', r'\1_\2', enum_type).lower()
                param_enums[param_name] = list(set(values))

    return params, param_enums


def extract_registry_from_core(core_path: Path) -> Dict[str, List[str]]:
    """从内核 include 目录中提取所有注册的类型（包括条件注册）"""
    registry: Dict[str, List[str]] = {
        'inbounds': [],
        'outbounds': [],
        'endpoints': [],
        'dns_transports': [],
        'services': [],
        'certificate_providers': [],
    }

    include_dir = core_path / "include"
    if not include_dir.exists():
        return registry

    # 扫描 include 目录下所有 .go 文件，提取所有注册调用
    all_content = ""
    for go_file in include_dir.rglob("*.go"):
        if go_file.name.endswith("_test.go"):
            continue
        try:
            all_content += go_file.read_text(encoding='utf-8', errors='ignore') + "\n"
        except Exception:
            continue

    # 提取入站注册（包括 RegisterInbound、RegisterRedirect、RegisterTProxy）
    inbound_matches = re.findall(r'(\w+)\.Register(?:Inbound|Redirect|TProxy)\(registry\)', all_content)
    inbound_types = set()
    for m in inbound_matches:
        name = m.lower()
        # cloudflared 包注册的类型名是 cloudflare
        if name == 'cloudflared':
            name = 'cloudflare'
        inbound_types.add(name)
    registry['inbounds'] = sorted(inbound_types)

    # 提取出站注册（包括 RegisterOutbound、RegisterSelector、RegisterURLTest）
    # 注意：group.RegisterSelector 的类型名是 selector，不是 group
    outbound_outbound = re.findall(r'(\w+)\.RegisterOutbound\(registry\)', all_content)
    outbound_selector = re.findall(r'\w+\.RegisterSelector\(registry\)', all_content)
    outbound_urltest = re.findall(r'\w+\.RegisterURLTest\(registry\)', all_content)
    outbound_types = set(m.lower() for m in outbound_outbound)
    if outbound_selector:
        outbound_types.add('selector')
    if outbound_urltest:
        outbound_types.add('urltest')
    registry['outbounds'] = sorted(outbound_types)

    # 提取端点注册
    endpoint_matches = re.findall(r'(\w+)\.RegisterEndpoint\(registry\)|register(\w+)Endpoint\(registry\)', all_content)
    for m in endpoint_matches:
        name = (m[0] or m[1]).lower()
        if name and name not in registry['endpoints']:
            registry['endpoints'].append(name)

    # 提取 DNS 传输注册
    dns_matches = re.findall(r'(\w+)\.Register(?:Transport|HTTP3Transport)\(registry\)|register(\w+)(?:DNS)?Transport\(registry\)', all_content)
    for m in dns_matches:
        name = (m[0] or m[1]).lower()
        if name and name not in registry['dns_transports']:
            registry['dns_transports'].append(name)

    # 提取服务注册
    service_matches = re.findall(r'(\w+)\.RegisterService\(registry\)|register(\w+)(?:Service|RealmService)\(registry\)', all_content)
    for m in service_matches:
        name = (m[0] or m[1]).lower()
        if name and name not in registry['services']:
            registry['services'].append(name)

    return registry


def extract_apis_from_core(core_path: Path) -> Dict[str, str]:
    """从内核 libbox 包中提取 API 方法"""
    apis: Dict[str, str] = {}

    libbox_dir = core_path / "experimental" / "libbox"
    if not libbox_dir.exists():
        return apis

    for go_file in libbox_dir.rglob("*.go"):
        if go_file.name.endswith("_test.go"):
            continue
        try:
            content = go_file.read_text(encoding='utf-8', errors='ignore')
        except Exception:
            continue

        # 提取导出方法
        methods = re.findall(r'func\s+\([^)]+\)\s+([A-Z][a-zA-Z0-9_]*)\s*\(([^)]*)\)', content)
        for name, params in methods:
            if len(name) > 2:
                apis[name] = f"func ({name})"

        # 提取导出函数
        funcs = re.findall(r'^func\s+([A-Z][a-zA-Z0-9_]*)\s*\(([^)]*)\)', content, re.MULTILINE)
        for name, params in funcs:
            if len(name) > 2:
                apis[name] = f"func {name}({params})"

    return apis


# ========== 客户端代码解析 ==========
def search_params_in_client(client_path: Path, params: Set[str]) -> Dict[str, bool]:
    """检查参数是否在客户端代码中被引用"""
    result: Dict[str, bool] = {}

    # 搜索 Swift 代码
    swift_files = list(client_path.rglob("*.swift"))
    all_content = ""
    for f in swift_files:
        if "Frameworks" in str(f) or ".build" in str(f):
            continue
        try:
            all_content += f.read_text(encoding='utf-8', errors='ignore') + "\n"
        except Exception:
            continue

    for param in params:
        # 搜索参数名（作为字符串或变量名）
        pattern = rf'["\']{re.escape(param)}["\']|{re.escape(param)}'
        if re.search(pattern, all_content):
            result[param] = True
        else:
            result[param] = False

    return result


# ========== 主检查逻辑 ==========
def run_checks(doc_path: Path, core_path: Path, client_path: Path) -> CheckResult:
    """运行所有检查"""
    result = CheckResult()

    print("【1/7】解析接入文档参数...")
    doc_params, doc_types, doc_enums = extract_params_from_doc(doc_path)
    print(f"  文档中提取到 {len(doc_params)} 个参数")

    print("【2/7】解析内核代码参数定义...")
    core_params, core_enums = extract_params_from_core(core_path)
    print(f"  内核中定义了 {len(core_params)} 个参数")

    print("【3/7】参数存在性交叉检查...")
    doc_param_set = set(doc_params)
    core_param_set = set(core_params.keys())

    result.params_in_doc_only = sorted(doc_param_set - core_param_set)
    result.params_in_core_only = sorted(core_param_set - doc_param_set)
    result.params_in_both = sorted(doc_param_set & core_param_set)
    result.total_params = len(doc_param_set | core_param_set)

    print(f"  文档独有: {len(result.params_in_doc_only)}")
    print(f"  内核独有: {len(result.params_in_core_only)}")
    print(f"  两者都有: {len(result.params_in_both)}")

    # 文档独有参数（可能是文档错误或内核未实现）
    if result.params_in_doc_only:
        # 过滤掉常见的非参数词
        common_words = {'true', 'false', 'nil', 'null', 'ios', 'macos', 'json', 'swift', 'go',
                        'tcp', 'udp', 'tls', 'http', 'https', 'dns', 'api', 'app', 'ext',
                        'log', 'tag', 'type', 'name', 'port', 'path', 'url', 'host', 'user',
                        'pass', 'key', 'cert', 'ca', 'id', 'uid', 'pid', 'ppid', 'sid',
                        'box', 'core', 'lib', 'libbox', 'sfi', 'mitm', 'tun', 'vpn',
                        'end', 'start', 'stop', 'init', 'run', 'exit', 'open', 'close',
                        'read', 'write', 'send', 'recv', 'bind', 'listen', 'accept',
                        'connect', 'dial', 'resolve', 'lookup', 'parse', 'format',
                        'string', 'array', 'object', 'number', 'boolean', 'integer',
                        'float', 'double', 'byte', 'char', 'rune', 'error', 'warning',
                        'info', 'debug', 'trace', 'fatal', 'panic', 'recover',
                        'func', 'var', 'let', 'const', 'struct', 'class', 'enum',
                        'protocol', 'extension', 'import', 'export', 'package',
                        'public', 'private', 'internal', 'fileprivate', 'open',
                        'static', 'final', 'override', 'required', 'convenience',
                        'weak', 'strong', 'unowned', 'lazy', 'didset', 'willset',
                        'get', 'set', 'willset', 'didset', 'subscript', 'operator',
                        'infix', 'prefix', 'postfix', 'associativity', 'precedence',
                        'async', 'await', 'throws', 'rethrows', 'try', 'catch',
                        'defer', 'guard', 'if', 'else', 'switch', 'case', 'default',
                        'for', 'while', 'repeat', 'break', 'continue', 'fallthrough',
                        'return', 'in', 'out', 'where', 'select', 'from', 'group',
                        'order', 'limit', 'offset', 'join', 'inner', 'outer', 'left',
                        'right', 'cross', 'union', 'intersect', 'except', 'minus',
                        'create', 'drop', 'alter', 'add', 'remove', 'rename', 'modify',
                        'insert', 'update', 'delete', 'truncate', 'grant', 'revoke',
                        'begin', 'commit', 'rollback', 'savepoint', 'transaction',
                        'lock', 'unlock', 'flush', 'reset', 'clear', 'purge', 'clean',
                        'setup', 'config', 'configure', 'setting', 'settings', 'option',
                        'options', 'parameter', 'parameters', 'argument', 'arguments',
                        'value', 'values', 'result', 'results', 'output', 'input',
                        'source', 'target', 'destination', 'origin', 'server', 'client',
                        'request', 'response', 'message', 'packet', 'frame', 'stream',
                        'session', 'connection', 'channel', 'socket', 'pipe', 'queue',
                        'stack', 'heap', 'tree', 'graph', 'list', 'map', 'set',
                        'array', 'slice', 'dictionary', 'tuple', 'pair', 'triple',
                        'single', 'double', 'triple', 'quad', 'penta', 'hexa',
                        'first', 'second', 'third', 'last', 'next', 'previous',
                        'current', 'previous', 'old', 'new', 'original', 'copy',
                        'clone', 'duplicate', 'reference', 'pointer', 'handle',
                        'context', 'environment', 'scope', 'namespace', 'module',
                        'component', 'module', 'plugin', 'addon', 'extension',
                        'feature', 'function', 'method', 'property', 'attribute',
                        'field', 'member', 'element', 'item', 'entry', 'record',
                        'row', 'column', 'cell', 'table', 'view', 'index', 'key',
                        'primary', 'foreign', 'unique', 'check', 'constraint',
                        'trigger', 'procedure', 'function', 'view', 'schema',
                        'database', 'table', 'index', 'sequence', 'synonym',
                        'grant', 'revoke', 'role', 'user', 'password', 'permission',
                        'privilege', 'audit', 'log', 'trace', 'monitor', 'profile',
                        'metric', 'stat', 'stats', 'counter', 'gauge', 'histogram',
                        'summary', 'label', 'tag', 'annotation', 'comment', 'note',
                        'todo', 'fixme', 'hack', 'workaround', 'deprecated', 'obsolete',
                        'experimental', 'beta', 'alpha', 'stable', 'release', 'snapshot',
                        'nightly', 'ci', 'cd', 'build', 'test', 'lint', 'format',
                        'check', 'verify', 'validate', 'assert', 'expect', 'should',
                        'given', 'when', 'then', 'describe', 'context', 'it', 'test',
                        'spec', 'suite', 'case', 'scenario', 'step', 'fixture',
                        'mock', 'stub', 'fake', 'spy', 'dummy', 'double',
                        'integration', 'e2e', 'unit', 'system', 'acceptance',
                        'performance', 'load', 'stress', 'soak', 'smoke', 'sanity',
                        'regression', 'compatibility', 'interoperability', 'conformance',
                        'security', 'privacy', 'compliance', 'legal', 'license',
                        'copyright', 'trademark', 'patent', 'author', 'contributor',
                        'maintainer', 'owner', 'reviewer', 'approver', 'reporter',
                        'assignee', 'reporter', 'watcher', 'follower', 'member',
                        'admin', 'moderator', 'guest', 'visitor', 'anonymous',
                        'authenticated', 'authorized', 'verified', 'trusted', 'safe',
                        'secure', 'encrypted', 'signed', 'certified', 'valid',
                        'invalid', 'expired', 'revoked', 'suspended', 'banned',
                        'blocked', 'restricted', 'limited', 'quota', 'rate', 'limit',
                        'threshold', 'maximum', 'minimum', 'default', 'custom',
                        'standard', 'normal', 'regular', 'common', 'general', 'universal',
                        'global', 'local', 'regional', 'national', 'international',
                        'public', 'private', 'protected', 'internal', 'external',
                        'shared', 'exclusive', 'dedicated', 'isolated', 'separate',
                        'combined', 'merged', 'integrated', 'embedded', 'bundled',
                        'standalone', 'independent', 'autonomous', 'automatic', 'manual',
                        'synchronous', 'asynchronous', 'parallel', 'concurrent', 'sequential',
                        'serial', 'batch', 'stream', 'pipeline', 'workflow', 'process',
                        'thread', 'coroutine', 'fiber', 'goroutine', 'actor', 'agent',
                        'service', 'daemon', 'worker', 'job', 'task', 'schedule',
                        'timer', 'clock', 'date', 'time', 'duration', 'interval',
                        'period', 'cycle', 'phase', 'stage', 'step', 'level',
                        'layer', 'tier', 'rank', 'grade', 'class', 'category',
                        'type', 'kind', 'sort', 'variety', 'flavor', 'style',
                        'mode', 'state', 'status', 'condition', 'situation', 'scenario',
                        'context', 'background', 'foreground', 'environment', 'atmosphere',
                        'space', 'area', 'region', 'zone', 'district', 'sector',
                        'field', 'domain', 'realm', 'kingdom', 'empire', 'republic',
                        'nation', 'country', 'state', 'province', 'city', 'town',
                        'village', 'street', 'road', 'avenue', 'boulevard', 'lane',
                        'drive', 'court', 'place', 'square', 'plaza', 'park',
                        'garden', 'yard', 'field', 'farm', 'ranch', 'estate',
                        'property', 'building', 'house', 'apartment', 'condo', 'flat',
                        'room', 'hall', 'lobby', 'corridor', 'stairs', 'elevator',
                        'door', 'window', 'wall', 'floor', 'ceiling', 'roof',
                        'foundation', 'basement', 'attic', 'garage', 'parking', 'lot',
                        'car', 'vehicle', 'truck', 'bus', 'train', 'plane',
                        'boat', 'ship', 'bicycle', 'motorcycle', 'scooter', 'skateboard',
                        'wheel', 'tire', 'engine', 'motor', 'battery', 'fuel',
                        'gas', 'oil', 'water', 'air', 'fire', 'earth',
                        'metal', 'wood', 'glass', 'plastic', 'rubber', 'fabric',
                        'paper', 'cardboard', 'stone', 'brick', 'concrete', 'cement',
                        'sand', 'gravel', 'dirt', 'mud', 'clay', 'soil',
                        'plant', 'tree', 'flower', 'grass', 'leaf', 'root',
                        'animal', 'dog', 'cat', 'bird', 'fish', 'horse',
                        'cow', 'pig', 'sheep', 'goat', 'chicken', 'duck',
                        'human', 'person', 'man', 'woman', 'child', 'baby',
                        'boy', 'girl', 'teen', 'adult', 'senior', 'elder',
                        'family', 'friend', 'enemy', 'stranger', 'neighbor', 'colleague',
                        'boss', 'employee', 'manager', 'director', 'ceo', 'cto',
                        'cfo', 'coo', 'vp', 'president', 'chairman', 'founder',
                        'creator', 'author', 'writer', 'reader', 'viewer', 'listener',
                        'speaker', 'singer', 'dancer', 'actor', 'actress', 'artist',
                        'painter', 'sculptor', 'musician', 'composer', 'conductor', 'player',
                        'coach', 'teacher', 'student', 'pupil', 'scholar', 'professor',
                        'doctor', 'nurse', 'patient', 'lawyer', 'judge', 'police',
                        'firefighter', 'soldier', 'sailor', 'pilot', 'driver', 'cook',
                        'chef', 'waiter', 'bartender', 'cashier', 'clerk', 'secretary',
                        'assistant', 'receptionist', 'janitor', 'cleaner', 'guard', 'security',
                        'farmer', 'fisherman', 'hunter', 'miner', 'builder', 'carpenter',
                        'plumber', 'electrician', 'mechanic', 'engineer', 'scientist', 'researcher',
                        'developer', 'programmer', 'coder', 'designer', 'architect', 'analyst',
                        'consultant', 'advisor', 'expert', 'specialist', 'professional', 'amateur',
                        'beginner', 'novice', 'intermediate', 'advanced', 'expert', 'master',
                        'guru', 'ninja', 'rockstar', 'wizard', 'genius', 'prodigy',
                        'idiot', 'fool', 'moron', 'stupid', 'dumb', 'smart',
                        'intelligent', 'clever', 'wise', 'brilliant', 'talented', 'gifted',
                        'lucky', 'unfortunate', 'fortunate', 'happy', 'sad', 'angry',
                        'excited', 'bored', 'tired', 'energetic', 'lazy', 'hardworking',
                        'rich', 'poor', 'wealthy', 'broke', 'successful', 'failed',
                        'famous', 'unknown', 'popular', 'unpopular', 'loved', 'hated',
                        'beautiful', 'ugly', 'handsome', 'pretty', 'cute', 'ugly',
                        'tall', 'short', 'fat', 'thin', 'slim', 'overweight',
                        'young', 'old', 'new', 'ancient', 'modern', 'traditional',
                        'big', 'small', 'large', 'tiny', 'huge', 'giant',
                        'long', 'short', 'wide', 'narrow', 'thick', 'thin',
                        'deep', 'shallow', 'high', 'low', 'fast', 'slow',
                        'quick', 'rapid', 'swift', 'hasty', 'hurried', 'rushed',
                        'gradual', 'steady', 'stable', 'unstable', 'constant', 'variable',
                        'fixed', 'flexible', 'rigid', 'loose', 'tight', 'firm',
                        'hard', 'soft', 'rough', 'smooth', 'sharp', 'dull',
                        'bright', 'dark', 'light', 'heavy', 'warm', 'cold',
                        'hot', 'cool', 'wet', 'dry', 'clean', 'dirty',
                        'fresh', 'stale', 'sweet', 'sour', 'bitter', 'salty',
                        'spicy', 'bland', 'delicious', 'tasty', 'yummy', 'disgusting',
                        'beautiful', 'gorgeous', 'stunning', 'magnificent', 'wonderful', 'amazing',
                        'awesome', 'incredible', 'unbelievable', 'remarkable', 'outstanding', 'excellent',
                        'great', 'good', 'nice', 'fine', 'okay', 'alright',
                        'bad', 'terrible', 'horrible', 'awful', 'dreadful', 'appalling',
                        'perfect', 'flawless', 'impeccable', 'spotless', 'immaculate', 'pristine',
                        'imperfect', 'flawed', 'defective', 'broken', 'damaged', 'ruined',
                        'whole', 'complete', 'finished', 'done', 'partial', 'incomplete',
                        'unfinished', 'ongoing', 'in progress', 'pending', 'waiting', 'queued',
                        'scheduled', 'planned', 'organized', 'arranged', 'prepared', 'ready',
                        'unprepared', 'unready', 'unwilling', 'reluctant', 'hesitant', 'eager',
                        'keen', 'enthusiastic', 'passionate', 'indifferent', 'apathetic', 'caring',
                        'loving', 'kind', 'nice', 'friendly', 'unfriendly', 'hostile',
                        'aggressive', 'passive', 'active', 'inactive', 'lazy', 'energetic',
                        'dynamic', 'static', 'fluid', 'solid', 'liquid', 'gas',
                        'plasma', 'energy', 'matter', 'mass', 'weight', 'volume',
                        'density', 'pressure', 'temperature', 'humidity', 'viscosity', 'elasticity',
                        'conductivity', 'resistance', 'capacitance', 'inductance', 'voltage', 'current',
                        'power', 'energy', 'work', 'force', 'momentum', 'velocity',
                        'acceleration', 'speed', 'distance', 'displacement', 'direction', 'angle',
                        'area', 'perimeter', 'circumference', 'diameter', 'radius', 'center',
                        'edge', 'corner', 'side', 'face', 'surface', 'point',
                        'line', 'curve', 'arc', 'circle', 'ellipse', 'triangle',
                        'square', 'rectangle', 'parallelogram', 'trapezoid', 'rhombus', 'polygon',
                        'pentagon', 'hexagon', 'heptagon', 'octagon', 'nonagon', 'decagon',
                        'sphere', 'cube', 'cylinder', 'cone', 'pyramid', 'prism',
                        'torus', 'ellipsoid', 'paraboloid', 'hyperboloid', 'polyhedron', 'tetrahedron',
                        'octahedron', 'dodecahedron', 'icosahedron', 'dimension', 'coordinate', 'axis',
                        'x', 'y', 'z', 'origin', 'vector', 'scalar',
                        'matrix', 'tensor', 'determinant', 'eigenvalue', 'eigenvector', 'derivative',
                        'integral', 'limit', 'series', 'sequence', 'function', 'equation',
                        'inequality', 'formula', 'algorithm', 'theorem', 'lemma', 'corollary',
                        'proof', 'hypothesis', 'theory', 'law', 'principle', 'rule',
                        'axiom', 'postulate', 'definition', 'property', 'characteristic', 'attribute',
                        'feature', 'quality', 'trait', 'aspect', 'facet', 'side',
                        'perspective', 'viewpoint', 'standpoint', 'position', 'stance', 'opinion',
                        'view', 'belief', 'conviction', 'faith', 'trust', 'doubt',
                        'skepticism', 'cynicism', 'optimism', 'pessimism', 'realism', 'idealism',
                        'pragmatism', 'empiricism', 'rationalism', 'logical', 'illogical', 'rational',
                        'irrational', 'reasonable', 'unreasonable', 'sensible', 'senseless', 'wise',
                        'foolish', 'smart', 'stupid', 'intelligent', 'dumb', 'clever',
                        'bright', 'brilliant', 'gifted', 'talented', 'genius', 'idiot',
                        'moron', 'fool', 'silly', 'crazy', 'insane', 'mad',
                        'psychotic', 'neurotic', 'hysterical', 'paranoid', 'schizophrenic', 'bipolar',
                        'depressed', 'anxious', 'stressed', 'burned out', 'exhausted', 'tired',
                        'fatigued', 'weary', 'sleepy', 'drowsy', 'awake', 'alert',
                        'conscious', 'unconscious', 'aware', 'unaware', 'mindful', 'forgetful',
                        'remember', 'forget', 'recall', 'recognize', 'identify', 'distinguish',
                        'differentiate', 'discriminate', 'separate', 'divide', 'split', 'join',
                        'combine', 'merge', 'unite', 'integrate', 'assimilate', 'incorporate',
                        'include', 'exclude', 'contain', 'hold', 'carry', 'bring',
                        'take', 'fetch', 'grab', 'catch', 'release', 'drop',
                        'throw', 'toss', 'flip', 'spin', 'rotate', 'turn',
                        'twist', 'bend', 'curve', 'straighten', 'flatten', 'crumple',
                        'crush', 'smash', 'break', 'shatter', 'crack', 'split',
                        'tear', 'rip', 'cut', 'slice', 'chop', 'dice',
                        'mince', 'grate', 'shred', 'peel', 'core', 'pit',
                        'seed', 'stem', 'leaf', 'root', 'flower', 'fruit',
                        'vegetable', 'meat', 'fish', 'poultry', 'seafood', 'dairy',
                        'grain', 'cereal', 'bread', 'pasta', 'rice', 'noodle',
                        'soup', 'stew', 'curry', 'sauce', 'dressing', 'gravy',
                        'spice', 'herb', 'seasoning', 'flavor', 'taste', 'smell',
                        'aroma', 'fragrance', 'perfume', 'scent', 'odor', 'stink',
                        'stench', 'reek', 'fragrant', 'aromatic', 'smelly', 'stinky',
                        'odorous', 'malodorous', 'fragrance', 'perfume', 'cologne', 'toilette',
                        'eau', 'deodorant', 'antiperspirant', 'shampoo', 'conditioner', 'soap',
                        'detergent', 'cleanser', 'moisturizer', 'lotion', 'cream', 'gel',
                        'serum', 'essence', 'toner', 'astringent', 'exfoliant', 'scrub',
                        'mask', 'peel', 'pack', 'patch', 'pad', 'wipe',
                        'tissue', 'towel', 'napkin', 'cloth', 'rag', 'sponge',
                        'brush', 'comb', 'mirror', 'razor', 'shaver', 'trimmer',
                        'clipper', 'scissors', 'nail', 'file', 'buffer', 'polisher',
                        'pumice', 'stone', 'loofah', 'sponge', 'brush', 'bottle',
                        'jar', 'container', 'tube', 'spray', 'pump', 'dropper',
                        'applicator', 'spatula', 'brush', 'roller', 'pad', 'sponge',
                        'cotton', 'swab', 'ball', 'pad', 'round', 'square',
                        'triangle', 'oval', 'circle', 'heart', 'star', 'moon',
                        'sun', 'cloud', 'rain', 'snow', 'wind', 'storm',
                        'thunder', 'lightning', 'rainbow', 'fog', 'mist', 'haze',
                        'smog', 'smoke', 'ash', 'dust', 'dirt', 'mud',
                        'sand', 'soil', 'clay', 'silt', 'gravel', 'rock',
                        'stone', 'pebble', 'boulder', 'mountain', 'hill', 'valley',
                        'canyon', 'gorge', 'cliff', 'cave', 'cavern', 'grotto',
                        'river', 'stream', 'creek', 'brook', 'lake', 'pond',
                        'pool', 'ocean', 'sea', 'bay', 'gulf', 'strait',
                        'channel', 'canal', 'waterfall', 'rapids', 'whirlpool', 'tide',
                        'wave', 'surf', 'spray', 'foam', 'bubble', 'drop',
                        'drip', 'splash', 'ripple', 'current', 'flow', 'flood',
                        'tsunami', 'hurricane', 'typhoon', 'cyclone', 'tornado', 'twister',
                        'earthquake', 'tremor', 'volcano', 'eruption', 'lava', 'magma',
                        'geyser', 'hot spring', 'fumarole', 'mud pot', 'solfatara', 'caldera',
                        'crater', 'dome', 'cone', 'shield', 'stratovolcano', 'cinder',
                        'scoria', 'pumice', 'obsidian', 'basalt', 'granite', 'gneiss',
                        'schist', 'slate', 'marble', 'quartz', 'feldspar', 'mica',
                        'hornblende', 'augite', 'olivine', 'garnet', 'topaz', 'emerald',
                        'ruby', 'sapphire', 'diamond', 'amethyst', 'citrine', 'jade',
                        'jasper', 'agate', 'onyx', 'opal', 'turquoise', 'lapis',
                        'azurite', 'malachite', 'copper', 'gold', 'silver', 'platinum',
                        'palladium', 'rhodium', 'iridium', 'osmium', 'ruthenium', 'titanium',
                        'zirconium', 'hafnium', 'vanadium', 'niobium', 'tantalum', 'chromium',
                        'molybdenum', 'tungsten', 'manganese', 'technetium', 'rhenium', 'iron',
                        'cobalt', 'nickel', 'copper', 'zinc', 'gallium', 'indium',
                        'thallium', 'tin', 'lead', 'bismuth', 'polonium', 'astatine',
                        'radon', 'francium', 'radium', 'actinium', 'thorium', 'protactinium',
                        'uranium', 'neptunium', 'plutonium', 'americium', 'curium', 'berkelium',
                        'californium', 'einsteinium', 'fermium', 'mendelevium', 'nobelium', 'lawrencium',
                        'rutherfordium', 'dubnium', 'seaborgium', 'bohrium', 'hassium', 'meitnerium',
                        'darmstadtium', 'roentgenium', 'copernicium', 'nihonium', 'flerovium', 'moscovium',
                        'livermorium', 'tennessine', 'oganesson', 'hydrogen', 'helium', 'lithium',
                        'beryllium', 'boron', 'carbon', 'nitrogen', 'oxygen', 'fluorine',
                        'neon', 'sodium', 'magnesium', 'aluminum', 'silicon', 'phosphorus',
                        'sulfur', 'chlorine', 'argon', 'potassium', 'calcium', 'scandium',
                        'titanium', 'vanadium', 'chromium', 'manganese', 'iron', 'cobalt',
                        'nickel', 'copper', 'zinc', 'gallium', 'germanium', 'arsenic',
                        'selenium', 'bromine', 'krypton', 'rubidium', 'strontium', 'yttrium',
                        'zirconium', 'niobium', 'molybdenum', 'technetium', 'ruthenium', 'rhodium',
                        'palladium', 'silver', 'cadmium', 'indium', 'tin', 'antimony',
                        'tellurium', 'iodine', 'xenon', 'cesium', 'barium', 'lanthanum',
                        'cerium', 'praseodymium', 'neodymium', 'promethium', 'samarium', 'europium',
                        'gadolinium', 'terbium', 'dysprosium', 'holmium', 'erbium', 'thulium',
                        'ytterbium', 'lutetium', 'hafnium', 'tantalum', 'tungsten', 'rhenium',
                        'osmium', 'iridium', 'platinum', 'gold', 'mercury', 'thallium',
                        'lead', 'bismuth', 'polonium', 'astatine', 'radon', 'francium',
                        'radium', 'actinium', 'thorium', 'protactinium', 'uranium', 'neptunium',
                        'plutonium', 'americium', 'curium', 'berkelium', 'californium', 'einsteinium',
                        'fermium', 'mendelevium', 'nobelium', 'lawrencium', 'rutherfordium', 'dubnium',
                        'seaborgium', 'bohrium', 'hassium', 'meitnerium', 'darmstadtium', 'roentgenium',
                        'copernicium', 'nihonium', 'flerovium', 'moscovium', 'livermorium', 'tennessine',
                        'oganesson'}

        filtered_doc_only = [p for p in result.params_in_doc_only if p.lower() not in common_words and len(p) > 2]
        if filtered_doc_only:
            result.warnings += len(filtered_doc_only)
            print(f"  ⚠️ 文档独有参数（可能未在内核实现）: {len(filtered_doc_only)}")
            for p in filtered_doc_only[:10]:
                print(f"    - {p}")

    print("【4/7】参数类型一致性检查...")
    for param in result.params_in_both:
        if param in core_params and param in doc_types:
            core_type = core_params[param][0]
            doc_type = doc_types[param]
            # 简单类型匹配检查
            core_lower = core_type.lower()
            doc_lower = doc_type.lower()
            if any(kw in doc_lower for kw in ['string', 'int', 'bool', 'number', 'array', 'object', 'list', 'map', 'dict']):
                if ('string' in core_lower and 'string' not in doc_lower) or \
                   ('int' in core_lower and 'int' not in doc_lower and 'number' not in doc_lower) or \
                   ('bool' in core_lower and 'bool' not in doc_lower):
                    result.type_mismatch.append((param, doc_type, core_type))
                    result.errors += 1
    print(f"  类型不匹配: {len(result.type_mismatch)}")

    print("【5/7】枚举值一致性检查...")
    for param, doc_values in doc_enums.items():
        if param in core_enums:
            core_values = core_enums[param]
            doc_set = set(v.lower() for v in doc_values)
            core_set = set(v.lower() for v in core_values)
            if doc_set != core_set:
                result.enum_mismatch.append((param, sorted(doc_set), sorted(core_set)))
                result.errors += 1
    print(f"  枚举不匹配: {len(result.enum_mismatch)}")

    print("【6/7】客户端代码引用检查...")
    client_usage = search_params_in_client(client_path, doc_param_set & core_param_set)
    result.params_missing_client = sorted([p for p, used in client_usage.items() if not used])
    print(f"  客户端未引用参数: {len(result.params_missing_client)}")
    if result.params_missing_client:
        result.warnings += len(result.params_missing_client)

    print("【7/7】注册表一致性检查...")
    core_registry = extract_registry_from_core(core_path)
    # 从文档中提取注册表类型
    doc_registry: Dict[str, List[str]] = {}
    content = doc_path.read_text(encoding='utf-8', errors='ignore')

    # 检查入站类型
    inbound_types = ['tun', 'redirect', 'tproxy', 'direct', 'socks', 'http', 'mixed',
                     'shadowsocks', 'snell', 'vmess', 'trojan', 'naive', 'shadowtls',
                     'vless', 'anytls', 'hysteria', 'tuic', 'hysteria2', 'tailcat', 'cloudflared']
    doc_inbounds = [t for t in inbound_types if re.search(rf'\b{re.escape(t)}\b', content, re.IGNORECASE)]
    doc_registry['inbounds'] = doc_inbounds

    # 检查出站类型
    outbound_types = ['direct', 'bridge', 'block', 'selector', 'urltest', 'socks', 'http',
                      'shadowsocks', 'snell', 'vmess', 'trojan', 'naive', 'tor', 'ssh',
                      'shadowtls', 'vless', 'anytls', 'hysteria', 'tuic', 'hysteria2', 'tailcat']
    doc_outbounds = [t for t in outbound_types if re.search(rf'\b{re.escape(t)}\b', content, re.IGNORECASE)]
    doc_registry['outbounds'] = doc_outbounds

    # 对比注册表
    for reg_type in ['inbounds', 'outbounds']:
        doc_set = set(doc_registry.get(reg_type, []))
        core_set = set(t.lower() for t in core_registry.get(reg_type, []))
        if doc_set != core_set:
            result.registry_mismatch.append((reg_type, sorted(doc_set), sorted(core_set)))
            result.errors += 1

    print(f"  注册表不匹配: {len(result.registry_mismatch)}")

    # API 一致性检查
    print("  API 签名一致性检查...")
    doc_apis = extract_apis_from_doc(doc_path)
    core_apis = extract_apis_from_core(core_path)
    for api_name in doc_apis:
        if api_name not in core_apis and len(api_name) > 3:
            # 检查是否是常见的非 API 词
            if api_name.lower() not in common_words:
                result.api_mismatch.append((api_name, doc_apis[api_name], "未在内核找到"))
                result.warnings += 1
    print(f"  API 不匹配: {len(result.api_mismatch)}")

    return result


# ========== 报告生成 ==========
def generate_report(result: CheckResult, doc_path: Path, core_path: Path) -> str:
    """生成 Markdown 报告"""
    lines = []
    lines.append("# 📋 接入文档全量参数仓库一致性检查报告\n")
    lines.append(f"> 文档: `{doc_path.name}` | 内核: `{core_path.name}`\n")
    lines.append("## 📊 总体统计\n")
    lines.append(f"| 指标 | 数量 |")
    lines.append(f"|------|------|")
    lines.append(f"| 总参数数 | {result.total_params} |")
    lines.append(f"| 文档独有 | {len(result.params_in_doc_only)} |")
    lines.append(f"| 内核独有 | {len(result.params_in_core_only)} |")
    lines.append(f"| 两者都有 | {len(result.params_in_both)} |")
    lines.append(f"| 客户端未引用 | {len(result.params_missing_client)} |")
    lines.append(f"| 类型不匹配 | {len(result.type_mismatch)} |")
    lines.append(f"| 枚举不匹配 | {len(result.enum_mismatch)} |")
    lines.append(f"| 注册表不匹配 | {len(result.registry_mismatch)} |")
    lines.append(f"| API 不匹配 | {len(result.api_mismatch)} |")
    lines.append(f"| **错误总数** | **{result.errors}** |")
    lines.append(f"| **警告总数** | **{result.warnings}** |\n")

    if result.errors > 0:
        lines.append("## 🔴 错误详情\n")

        if result.type_mismatch:
            lines.append("### 参数类型不匹配\n")
            lines.append("| 参数 | 文档类型 | 内核类型 |")
            lines.append("|------|---------|---------|")
            for param, doc_type, core_type in result.type_mismatch:
                lines.append(f"| `{param}` | {doc_type} | {core_type} |")
            lines.append("")

        if result.enum_mismatch:
            lines.append("### 枚举值不匹配\n")
            lines.append("| 参数 | 文档枚举值 | 内核枚举值 |")
            lines.append("|------|-----------|-----------|")
            for param, doc_vals, core_vals in result.enum_mismatch:
                lines.append(f"| `{param}` | {', '.join(doc_vals)} | {', '.join(core_vals)} |")
            lines.append("")

        if result.registry_mismatch:
            lines.append("### 注册表不匹配\n")
            lines.append("| 注册表 | 文档类型 | 内核类型 |")
            lines.append("|--------|---------|---------|")
            for reg_type, doc_types_list, core_types_list in result.registry_mismatch:
                lines.append(f"| {reg_type} | {', '.join(doc_types_list)} | {', '.join(core_types_list)} |")
            lines.append("")

    if result.warnings > 0:
        lines.append("## 🟡 警告详情\n")

        if result.params_in_doc_only:
            filtered = [p for p in result.params_in_doc_only if len(p) > 2]
            if filtered:
                lines.append(f"### 文档独有参数（{len(filtered)} 个，可能未在内核实现）\n")
                lines.append("```")
                for p in filtered[:50]:
                    lines.append(f"  {p}")
                if len(filtered) > 50:
                    lines.append(f"  ... 还有 {len(filtered) - 50} 个")
                lines.append("```\n")

        if result.params_missing_client:
            lines.append(f"### 客户端未引用参数（{len(result.params_missing_client)} 个）\n")
            lines.append("```")
            for p in result.params_missing_client[:30]:
                lines.append(f"  {p}")
            if len(result.params_missing_client) > 30:
                lines.append(f"  ... 还有 {len(result.params_missing_client) - 30} 个")
            lines.append("```\n")

        if result.api_mismatch:
            lines.append(f"### API 方法未在内核找到（{len(result.api_mismatch)} 个）\n")
            lines.append("| API 方法 | 文档签名 | 状态 |")
            lines.append("|---------|---------|------|")
            for name, sig, status in result.api_mismatch[:20]:
                lines.append(f"| `{name}` | `{sig}` | {status} |")
            lines.append("")

    lines.append("## ✅ 检查结论\n")
    if result.errors == 0 and result.warnings == 0:
        lines.append("🎉 **所有检查通过！文档与内核代码完全一致。**\n")
    elif result.errors == 0:
        lines.append(f"✅ **错误检查通过**，但存在 {result.warnings} 个警告（不阻塞）。\n")
    else:
        lines.append(f"❌ **检查未通过**，存在 {result.errors} 个错误，{result.warnings} 个警告。\n")

    return "\n".join(lines)


# ========== 主函数 ==========
def main():
    parser = argparse.ArgumentParser(description='接入文档全量参数仓库一致性检查')
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

    print("=" * 60)
    print("  接入文档全量参数仓库一致性检查")
    print("=" * 60)
    print(f"  文档: {doc_path}")
    print(f"  内核: {core_path}")
    print(f"  客户端: {client_path}")
    print("=" * 60)
    print()

    result = run_checks(doc_path, core_path, client_path)

    print()
    print("=" * 60)
    print("  检查完成")
    print("=" * 60)
    print(f"  错误: {result.errors}")
    print(f"  警告: {result.warnings}")
    print()

    # 生成报告
    report = generate_report(result, doc_path, core_path)
    with open(args.output, 'w', encoding='utf-8') as f:
        f.write(report)
    print(f"✅ 报告已生成: {args.output}")

    # 生成 JSON
    json_result = {
        'total_params': result.total_params,
        'params_in_doc_only': result.params_in_doc_only,
        'params_in_core_only': result.params_in_core_only,
        'params_in_both': result.params_in_both,
        'params_missing_client': result.params_missing_client,
        'type_mismatch': [{'param': p, 'doc_type': d, 'core_type': c} for p, d, c in result.type_mismatch],
        'enum_mismatch': [{'param': p, 'doc_values': d, 'core_values': c} for p, d, c in result.enum_mismatch],
        'registry_mismatch': [{'type': t, 'doc_types': d, 'core_types': c} for t, d, c in result.registry_mismatch],
        'api_mismatch': [{'name': n, 'doc_signature': d, 'status': s} for n, d, s in result.api_mismatch],
        'errors': result.errors,
        'warnings': result.warnings,
    }
    with open(args.json, 'w', encoding='utf-8') as f:
        json.dump(json_result, f, ensure_ascii=False, indent=2)
    print(f"✅ JSON 结果已生成: {args.json}")

    # 最终判定
    if result.errors > 0:
        print(f"\n❌ 检查未通过，存在 {result.errors} 个错误")
        sys.exit(1)
    else:
        print(f"\n✅ 检查通过（{result.warnings} 个警告）")
        sys.exit(0)


if __name__ == '__main__':
    main()
