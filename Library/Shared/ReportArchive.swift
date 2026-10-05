import Foundation

/// 报告归档基类（简化版，提供通用的文件操作）
public enum ReportArchive {
    /// 元数据文件名
    public static let metadataFileName = "metadata.json"
    /// 配置文件名
    public static let configFileName = "config.json"
    /// Go日志文件名
    public static let goLogFileName = "go.log"
    /// 原生日志文件名
    public static let nativeLogFileName = "native.log"
    /// 已读标记文件名
    public static let readMarkerFileName = ".read"
    /// tvOS设备来源标记
    public static let tvOSDeviceOrigin = "tvos"

    /// 移除报告归档目录
    public static func removeArtifact(at artifactURL: URL) {
        try? FileManager.default.removeItem(at: artifactURL)
    }

    /// 解析报告归档日期（从目录名解析）
    public static func parseArtifactDate(for artifactURL: URL) -> Date? {
        let directoryName = artifactURL.lastPathComponent
        // 目录名格式：YYYY-MM-DD_HH-MM-SS
        let formatter = DateFormatter()
        formatter.dateFormat = "yyyy-MM-dd_HH-mm-ss"
        formatter.timeZone = TimeZone.current
        return formatter.date(from: directoryName)
    }
}
