import Foundation
import Libbox

// MARK: - MITM 服务管理器
// 负责：
// 1. 调用 Libbox API 查询 MITM 运行状态
// 2. 调用 Libbox API 生成 CA 证书
// 3. 管理 MITM 配置的持久化（存储在 App Group 共享目录）
// 4. 将 MITM 配置注入到 sing-box profile 配置中

/// MITM 服务管理器（单例）
public final class MITMServiceManager: ObservableObject {
    /// 共享实例
    public static let shared = MITMServiceManager()

    /// MITM 配置
    @Published public var configuration: MITMConfiguration {
        didSet {
            saveConfiguration()
        }
    }

    /// MITM 运行状态
    @Published public private(set) var runtimeStatus: MITMRuntimeStatus = .unknown

    /// 配置文件路径（App Group 共享目录）
    private let configurationURL: URL?

    /// App Group 标识
    private let appGroupIdentifier = "group.com.mitm.box"

    private init() {
        // 初始化配置文件路径
        let containerURL = FileManager.default
            .containerURL(forSecurityApplicationGroupIdentifier: appGroupIdentifier)
        self.configurationURL = containerURL?
            .appendingPathComponent("mitm-configuration.json")

        // 加载已保存的配置
        if let url = configurationURL,
           FileManager.default.fileExists(atPath: url.path),
           let data = try? Data(contentsOf: url),
           let config = try? JSONDecoder().decode(MITMConfiguration.self, from: data) {
            self.configuration = config
        } else {
            self.configuration = .default
        }
    }

    // MARK: - 配置持久化

    /// 保存配置到 App Group 共享目录
    private func saveConfiguration() {
        guard let url = configurationURL else { return }
        do {
            let data = try JSONEncoder().encode(configuration)
            try data.write(to: url, options: .atomic)
        } catch {
            NSLog("MITMServiceManager: 保存配置失败 - \(error.localizedDescription)")
        }
    }

    // MARK: - CA 证书管理

    /// 生成 MITM 根证书
    /// - Parameters:
    ///   - certificatePath: 证书输出路径
    ///   - privateKeyPath: 私钥输出路径
    /// - Returns: 成功返回 nil，失败返回错误
    public func generateCA(certificatePath: String, privateKeyPath: String) -> Error? {
        // 调用 Libbox API 生成 CA 证书
        // 注意：此 API 在 Step 2 中添加到 Libbox，需要新版 Libbox.framework
        let error = LibboxGenerateMITMCA(certificatePath, privateKeyPath)
        if let error {
            NSLog("MITMServiceManager: 生成 CA 失败 - \(error.localizedDescription)")
            return error
        }

        // 更新配置中的证书路径
        configuration.ca.certificate = certificatePath
        configuration.ca.privateKey = privateKeyPath

        return nil
    }

    /// 在 App Group 共享目录中生成 CA 证书
    /// - Returns: 成功返回 (证书路径, 私钥路径)，失败返回错误
    public func generateCAInAppGroup() -> Result<(certificate: String, privateKey: String), Error> {
        guard let containerURL = FileManager.default
            .containerURL(forSecurityApplicationGroupIdentifier: appGroupIdentifier) else {
            return .failure(NSError(
                domain: "MITMServiceManager",
                code: -1,
                userInfo: [NSLocalizedDescriptionKey: "无法获取 App Group 共享目录"]
            ))
        }

        let caDirectory = containerURL.appendingPathComponent("mitm-ca", isDirectory: true)
        do {
            try FileManager.default.createDirectory(at: caDirectory, withIntermediateDirectories: true)
        } catch {
            return .failure(error)
        }

        let certificatePath = caDirectory.appendingPathComponent("ca.pem").path
        let privateKeyPath = caDirectory.appendingPathComponent("ca.key").path

        if let error = generateCA(certificatePath: certificatePath, privateKeyPath: privateKeyPath) {
            return .failure(error)
        }

        return .success((certificate: certificatePath, privateKey: privateKeyPath))
    }

    /// 检查 CA 证书文件是否存在
    public var isCAFileExists: Bool {
        guard configuration.ca.isConfigured else { return false }
        return FileManager.default.fileExists(atPath: configuration.ca.certificate)
            && FileManager.default.fileExists(atPath: configuration.ca.privateKey)
    }

    // MARK: - 运行状态查询

    /// 刷新 MITM 运行状态
    /// - Parameter commandServer: 当前运行的 CommandServer（可选，未启动时返回 unknown）
    public func refreshRuntimeStatus(commandServer: LibboxCommandServer?) {
        guard let commandServer else {
            runtimeStatus = .unknown
            return
        }

        // 调用 Libbox API 查询 MITM 状态
        // 注意：此 API 在 Step 2 中添加到 Libbox，需要新版 Libbox.framework
        if let status = commandServer.getMITMStatus() {
            runtimeStatus = MITMRuntimeStatus(
                enabled: status.enabled,
                caInstalled: status.caInstalled,
                activeConnections: status.activeConnections
            )
        } else {
            runtimeStatus = .unknown
        }
    }

    // MARK: - 配置注入

    /// 将 MITM 配置注入到 sing-box profile JSON 中
    /// - Parameter profileJSON: 原始 profile JSON 字符串
    /// - Returns: 注入 MITM 配置后的 JSON 字符串
    public func injectConfiguration(into profileJSON: String) -> String {
        guard configuration.enabled else {
            // MITM 未启用时，移除 services 中的 mitm 配置
            return removeMITMConfiguration(from: profileJSON)
        }

        guard let data = profileJSON.data(using: .utf8),
              var jsonObject = try? JSONSerialization.jsonObject(with: data) as? [String: Any] else {
            return profileJSON
        }

        // 构建 MITM service 配置
        var mitmService: [String: Any] = [
            "type": "mitm",
            "tag": "mitm",
            "enabled": true,
            "ca": [
                "certificate": configuration.ca.certificate,
                "private_key": configuration.ca.privateKey
            ],
            "on_error": configuration.onError.rawValue,
            "upstream_timeout": configuration.upstreamTimeout
        ]

        // match 配置
        var match: [String: Any] = [:]
        if !configuration.match.domain.isEmpty {
            match["domain"] = configuration.match.domain
        }
        if !configuration.match.domainSuffix.isEmpty {
            match["domain_suffix"] = configuration.match.domainSuffix
        }
        if !configuration.match.domainKeyword.isEmpty {
            match["domain_keyword"] = configuration.match.domainKeyword
        }
        if !configuration.match.domainRegex.isEmpty {
            match["domain_regex"] = configuration.match.domainRegex
        }
        if !match.isEmpty {
            mitmService["match"] = match
        }

        // rewrite 配置
        if configuration.rewrite.enabled {
            var rewrite: [String: Any] = [
                "enabled": true,
                "max_body_size": configuration.rewrite.maxBodySize
            ]
            if !configuration.rewrite.rules.isEmpty {
                rewrite["rules"] = configuration.rewrite.rules.map { rule in
                    var ruleDict: [String: Any] = [:]
                    if !rule.domain.isEmpty { ruleDict["domain"] = rule.domain }
                    if !rule.domainSuffix.isEmpty { ruleDict["domain_suffix"] = rule.domainSuffix }
                    if !rule.pathPrefix.isEmpty { ruleDict["path_prefix"] = rule.pathPrefix }
                    if !rule.method.isEmpty { ruleDict["method"] = rule.method }
                    if !rule.requestHeader.isEmpty { ruleDict["request_header"] = rule.requestHeader }
                    if !rule.requestHeaderDelete.isEmpty { ruleDict["request_header_delete"] = rule.requestHeaderDelete }
                    if !rule.responseHeader.isEmpty { ruleDict["response_header"] = rule.responseHeader }
                    if !rule.responseHeaderDelete.isEmpty { ruleDict["response_header_delete"] = rule.responseHeaderDelete }
                    if !rule.bodyReplace.isEmpty {
                        ruleDict["body_replace"] = rule.bodyReplace.map { [
                            "find": $0.find,
                            "replace": $0.replace
                        ] }
                    }
                    return ruleDict
                }
            }
            mitmService["rewrite"] = rewrite
        }

        // 注入到 services 数组
        var services = jsonObject["services"] as? [[String: Any]] ?? []
        // 移除已有的 mitm service
        services.removeAll { ($0["type"] as? String) == "mitm" }
        services.append(mitmService)
        jsonObject["services"] = services

        // 序列化回 JSON
        guard let outputData = try? JSONSerialization.data(
            withJSONObject: jsonObject,
            options: [.prettyPrinted, .sortedKeys]
        ) else {
            return profileJSON
        }
        return String(data: outputData, encoding: .utf8) ?? profileJSON
    }

    /// 从 profile JSON 中移除 MITM 配置
    private func removeMITMConfiguration(from profileJSON: String) -> String {
        guard let data = profileJSON.data(using: .utf8),
              var jsonObject = try? JSONSerialization.jsonObject(with: data) as? [String: Any] else {
            return profileJSON
        }

        guard var services = jsonObject["services"] as? [[String: Any]] else {
            return profileJSON
        }

        let originalCount = services.count
        services.removeAll { ($0["type"] as? String) == "mitm" }

        guard services.count != originalCount else {
            return profileJSON // 没有 mitm service，无需修改
        }

        jsonObject["services"] = services

        guard let outputData = try? JSONSerialization.data(
            withJSONObject: jsonObject,
            options: [.prettyPrinted, .sortedKeys]
        ) else {
            return profileJSON
        }
        return String(data: outputData, encoding: .utf8) ?? profileJSON
    }
}
