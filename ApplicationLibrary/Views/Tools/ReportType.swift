import Foundation

/// 报告类型（简化版，移除BinaryCodable依赖）
public enum ReportType: String, Codable {
    case crash
    case oom
    case power

    public var directoryName: String {
        switch self {
        case .crash: return "crash_reports"
        case .oom: return "oom_reports"
        case .power: return "power_reports"
        }
    }
}

/// 报告接收通知
public extension Notification.Name {
    static let reportReceived = Notification.Name("com.mitm.box.reportReceived")
}
