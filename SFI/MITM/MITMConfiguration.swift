import Foundation

// MARK: - MITM 配置模型
// 对应 sing-box option.MITMServiceOptions 的 Swift 侧表示
// 用于在 SFI 设置页面编辑 MITM 配置，并写入 profile 配置文件

/// MITM 服务配置
public struct MITMConfiguration: Codable, Equatable {
    /// 是否启用 MITM
    public var enabled: Bool
    /// 根证书配置
    public var ca: MITMCAConfiguration
    /// 域名匹配规则
    public var match: MITMMatchConfiguration
    /// 重写配置
    public var rewrite: MITMRewriteConfiguration
    /// 错误处理策略
    public var onError: MITMOnError
    /// 上游连接超时（秒）
    public var upstreamTimeout: Int

    public init(
        enabled: Bool = false,
        ca: MITMCAConfiguration = .init(),
        match: MITMMatchConfiguration = .init(),
        rewrite: MITMRewriteConfiguration = .init(),
        onError: MITMOnError = .bypass,
        upstreamTimeout: Int = 30
    ) {
        self.enabled = enabled
        self.ca = ca
        self.match = match
        self.rewrite = rewrite
        self.onError = onError
        self.upstreamTimeout = upstreamTimeout
    }

    /// 默认配置（禁用状态）
    public static let `default` = MITMConfiguration()
}

// MARK: - CA 证书配置

/// MITM 根证书配置
public struct MITMCAConfiguration: Codable, Equatable {
    /// 证书文件路径（PEM 格式）
    public var certificate: String
    /// 私钥文件路径（PEM 格式）
    public var privateKey: String

    public init(certificate: String = "", privateKey: String = "") {
        self.certificate = certificate
        self.privateKey = privateKey
    }

    /// 是否已配置证书路径
    public var isConfigured: Bool {
        !certificate.isEmpty && !privateKey.isEmpty
    }
}

// MARK: - 域名匹配配置

/// MITM 域名匹配配置
public struct MITMMatchConfiguration: Codable, Equatable {
    /// 精确匹配域名列表
    public var domain: [String]
    /// 后缀匹配域名列表
    public var domainSuffix: [String]
    /// 关键字匹配域名列表
    public var domainKeyword: [String]
    /// 正则匹配域名列表
    public var domainRegex: [String]

    public init(
        domain: [String] = [],
        domainSuffix: [String] = [],
        domainKeyword: [String] = [],
        domainRegex: [String] = []
    ) {
        self.domain = domain
        self.domainSuffix = domainSuffix
        self.domainKeyword = domainKeyword
        self.domainRegex = domainRegex
    }

    /// 是否有任何匹配规则
    public var hasRules: Bool {
        !domain.isEmpty || !domainSuffix.isEmpty || !domainKeyword.isEmpty || !domainRegex.isEmpty
    }
}

// MARK: - 重写配置

/// MITM 重写配置
public struct MITMRewriteConfiguration: Codable, Equatable {
    /// 是否启用重写
    public var enabled: Bool
    /// 最大 Body 大小（字节）
    public var maxBodySize: Int
    /// 重写规则列表
    public var rules: [MITMRewriteRule]

    public init(
        enabled: Bool = false,
        maxBodySize: Int = 10 * 1024 * 1024,
        rules: [MITMRewriteRule] = []
    ) {
        self.enabled = enabled
        self.maxBodySize = maxBodySize
        self.rules = rules
    }
}

/// MITM 重写规则
public struct MITMRewriteRule: Codable, Equatable, Identifiable {
    public var id: UUID
    /// 规则名称（仅用于 UI 显示）
    public var name: String
    /// 精确匹配域名
    public var domain: [String]
    /// 后缀匹配域名
    public var domainSuffix: [String]
    /// 路径前缀匹配
    public var pathPrefix: [String]
    /// HTTP 方法匹配
    public var method: [String]
    /// 请求头添加/修改
    public var requestHeader: [String: String]
    /// 请求头删除
    public var requestHeaderDelete: [String]
    /// 响应头添加/修改
    public var responseHeader: [String: String]
    /// 响应头删除
    public var responseHeaderDelete: [String]
    /// Body 替换规则
    public var bodyReplace: [MITMBodyReplace]

    public init(
        id: UUID = UUID(),
        name: String = "",
        domain: [String] = [],
        domainSuffix: [String] = [],
        pathPrefix: [String] = [],
        method: [String] = [],
        requestHeader: [String: String] = [:],
        requestHeaderDelete: [String] = [],
        responseHeader: [String: String] = [:],
        responseHeaderDelete: [String] = [],
        bodyReplace: [MITMBodyReplace] = []
    ) {
        self.id = id
        self.name = name
        self.domain = domain
        self.domainSuffix = domainSuffix
        self.pathPrefix = pathPrefix
        self.method = method
        self.requestHeader = requestHeader
        self.requestHeaderDelete = requestHeaderDelete
        self.responseHeader = responseHeader
        self.responseHeaderDelete = responseHeaderDelete
        self.bodyReplace = bodyReplace
    }
}

/// Body 替换规则
public struct MITMBodyReplace: Codable, Equatable, Identifiable {
    public var id: UUID
    /// 查找字符串
    public var find: String
    /// 替换字符串
    public var replace: String

    public init(id: UUID = UUID(), find: String = "", replace: String = "") {
        self.id = id
        self.find = find
        self.replace = replace
    }
}

// MARK: - 错误处理策略

/// MITM 错误处理策略
public enum MITMOnError: String, Codable, CaseIterable, Identifiable {
    public var id: String { rawValue }

    /// 绕过：MITM 失败时直接透传原始流量
    case bypass = "bypass"
    /// 拒绝：MITM 失败时断开连接
    case reject = "reject"

    /// 显示名称
    public var displayName: String {
        switch self {
        case .bypass:
            return "绕过（透传）"
        case .reject:
            return "拒绝（断开）"
        }
    }
}

// MARK: - MITM 运行状态

/// MITM 服务运行状态（对应 Libbox GetMITMStatus 返回值）
public struct MITMRuntimeStatus: Equatable {
    /// MITM 服务是否已启用
    public let enabled: Bool
    /// 根证书是否已成功加载
    public let caInstalled: Bool
    /// 当前活跃 MITM 连接数
    public let activeConnections: Int32

    public init(enabled: Bool = false, caInstalled: Bool = false, activeConnections: Int32 = 0) {
        self.enabled = enabled
        self.caInstalled = caInstalled
        self.activeConnections = activeConnections
    }

    /// 未知状态（服务未启动时）
    public static let unknown = MITMRuntimeStatus()
}
