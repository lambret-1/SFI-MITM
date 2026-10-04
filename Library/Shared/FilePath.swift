import Foundation

public enum FilePath {
    public static let packageName = AppConfiguration.packageName
    public static let groupName = AppConfiguration.appGroupID

    // 安全获取 App Group 共享目录
    // 注意：TrollStore 越狱环境下可能配置了 no-container entitlement，
    // 导致 containerURL(forSecurityApplicationGroupIdentifier:) 返回 nil。
    // 此时需要回退到可靠的本地路径，避免强制解包崩溃。
    private static let defaultSharedDirectory: URL = {
        // 优先尝试 App Group 容器路径
        if let groupContainerURL = FileManager.default.containerURL(forSecurityApplicationGroupIdentifier: groupName) {
            // 确保目录存在
            try? FileManager.default.createDirectory(at: groupContainerURL, withIntermediateDirectories: true)
            return groupContainerURL
        }

        // 回退方案 1：App 沙盒 Documents 目录（正常 App Store 环境）
        let documentDirectories = FileManager.default.urls(for: .documentDirectory, in: .userDomainMask)
        if let documentDirectory = documentDirectories.first {
            let fallbackURL = documentDirectory.appendingPathComponent("Shared", isDirectory: true)
            try? FileManager.default.createDirectory(at: fallbackURL, withIntermediateDirectories: true)
            return fallbackURL
        }

        // 回退方案 2：NSHomeDirectory 下的 Documents（越狱/TrollStore 环境）
        let homeFallbackURL = URL(fileURLWithPath: NSHomeDirectory()).appendingPathComponent("Documents").appendingPathComponent(packageName, isDirectory: true)
        try? FileManager.default.createDirectory(at: homeFallbackURL, withIntermediateDirectories: true)
        return homeFallbackURL
    }()

    #if os(iOS)
        public static let sharedDirectory = defaultSharedDirectory
    #elseif os(tvOS)
        public static let sharedDirectory = defaultSharedDirectory
            .appendingPathComponent("Library", isDirectory: true)
            .appendingPathComponent("Caches", isDirectory: true)
    #elseif os(macOS)
        public static var sharedDirectory: URL = defaultSharedDirectory
    #endif

    #if os(iOS)
        public static let cacheDirectory = sharedDirectory
            .appendingPathComponent("Library", isDirectory: true)
            .appendingPathComponent("Caches", isDirectory: true)
    #elseif os(tvOS)
        public static let cacheDirectory = sharedDirectory
    #elseif os(macOS)
        public static var cacheDirectory: URL {
            sharedDirectory
                .appendingPathComponent("Library", isDirectory: true)
                .appendingPathComponent("Caches", isDirectory: true)
        }
    #endif

    #if os(macOS)
        public static var workingDirectory: URL {
            cacheDirectory.appendingPathComponent("Working", isDirectory: true)
        }
    #else
        public static let workingDirectory = cacheDirectory.appendingPathComponent("Working", isDirectory: true)

    #endif

    public static var iCloudDirectory = FileManager.default.url(forUbiquityContainerIdentifier: nil)?.appendingPathComponent("Documents", isDirectory: true) ?? URL(string: "stub")!
}

public extension URL {
    var fileName: String {
        var path = relativePath
        if let index = path.lastIndex(of: "/") {
            path = String(path[path.index(index, offsetBy: 1)...])
        }
        return path
    }
}
