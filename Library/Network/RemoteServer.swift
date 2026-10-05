import Foundation

/// 远程服务器配置（本项目暂不支持远程控制，仅作类型占位）
public struct RemoteServer: Identifiable, Codable, Hashable {
    public let id: UUID
    public var name: String
    public var url: String
    public var secret: String

    public init(id: UUID = UUID(), name: String, url: String, secret: String) {
        self.id = id
        self.name = name
        self.url = url
        self.secret = secret
    }

    /// 生成连接URL（添加http://前缀）
    static func connectURL(_ urlString: String) -> String {
        var value = urlString.trimmingCharacters(in: .whitespacesAndNewlines)
        while value.hasSuffix("/") {
            value.removeLast()
        }
        if value.isEmpty {
            return ""
        }
        let lowercased = value.lowercased()
        if lowercased.hasPrefix("http://") || lowercased.hasPrefix("https://") {
            return value
        }
        return "http://" + value
    }
}
