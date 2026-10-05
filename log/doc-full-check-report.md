# 📋 接入文档全量参数仓库一致性检查报告

> 文档: `README.md` | 内核: `sing-box-core`

## 📊 总体统计

| 指标 | 数量 |
|------|------|
| 总参数数 | 231 |
| 文档独有 | 191 |
| 内核独有 | 20 |
| 两者都有 | 20 |
| 客户端未引用 | 1 |
| 类型不匹配 | 0 |
| 枚举不匹配 | 0 |
| 注册表不匹配 | 0 |
| API 不匹配 | 3 |
| **错误总数** | **0** |
| **警告总数** | **153** |

## 🟡 警告详情

### 文档独有参数（190 个，可能未在内核实现）

```
  Host
  acme
  alter_id
  anytls
  auth_user
  authenticated_length
  auto_detect_interface
  auto_route
  available
  block
  body
  body_replace
  bool
  bridge
  bypass
  cache_capacity
  cache_file
  cache_id
  certificate_providers
  clash_api
  client
  client_subnet
  close
  cloudflare
  configuration
  connect
  connections
  debug
  default
  default_mode
  detour
  direct
  disable_cache
  disable_expire
  disconnect
  dns
  dns_handler
  dns_mode
  domain
  domain_keyword
  domain_regex
  domain_suffix
  download_detour
  enabled
  endpoint_independent_nat
  endpoints
  exclude_interface
  exclude_mptcp
  experimental
  external_controller
  ... 还有 140 个
```

### 客户端未引用参数（1 个）

```
  password
```

### API 方法未在内核找到（3 个）

| API 方法 | 文档签名 | 状态 |
|---------|---------|------|
| `registerMITMService` | `func registerMITMService(registry *service.Registry)` | 未在内核找到 |
| `NewService` | `func NewService(
 ctx context.Context,
 logger log.ContextLogger,
 tag string,
 options option.MITMServiceOptions,
)` | 未在内核找到 |
| `LibboxGenerateMITMCA` | `func LibboxGenerateMITMCA(certificatePath string, privateKeyPath string)` | 未在内核找到 |

## ✅ 检查结论

✅ **错误检查通过**，但存在 153 个警告（不阻塞）。
