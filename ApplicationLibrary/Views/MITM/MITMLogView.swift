import Libbox
import Library
import SwiftUI

/// MITM 日志查看页面
/// 展示 sing-box 运行时输出的 MITM 相关日志（解密、重写、错误等）
public struct MITMLogView: View {
    @EnvironmentObject private var environments: ExtensionEnvironments
    @State private var searchText = ""
    @State private var isAutoScroll = true

    public init() {}

    public var body: some View {
        NavigationStack {
            VStack(spacing: 0) {
                // 搜索栏
                searchBar

                // 日志列表
                ScrollViewReader { proxy in
                    List {
                        ForEach(filteredLogs) { log in
                            logRow(log)
                                .id(log.id)
                        }
                        if filteredLogs.isEmpty {
                            emptyState
                        }
                    }
                    .listStyle(.plain)
                    .onChange(of: filteredLogs.count) { _ in
                        if isAutoScroll, let lastID = filteredLogs.last?.id {
                            withAnimation {
                                proxy.scrollTo(lastID, anchor: .bottom)
                            }
                        }
                    }
                }
            }
            .navigationTitle("MITM 日志")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .navigationBarTrailing) {
                    Button {
                        isAutoScroll.toggle()
                    } label: {
                        Image(systemName: isAutoScroll ? "arrow.down.circle.fill" : "arrow.down.circle")
                            .foregroundColor(isAutoScroll ? .accentColor : .secondary)
                    }
                }
            }
        }
    }

    // MARK: - 搜索栏

    private var searchBar: some View {
        HStack {
            Image(systemName: "magnifyingglass")
                .foregroundColor(.secondary)
            TextField("搜索日志", text: $searchText)
                .textFieldStyle(.plain)
            if !searchText.isEmpty {
                Button {
                    searchText = ""
                } label: {
                    Image(systemName: "xmark.circle.fill")
                        .foregroundColor(.secondary)
                }
            }
        }
        .padding(.horizontal, 12)
        .padding(.vertical, 8)
        .background(Color(.systemGray6))
    }

    // MARK: - 日志行

    private func logRow(_ log: LogEntry) -> some View {
        HStack(alignment: .top, spacing: 8) {
            // 日志级别标记
            Circle()
                .fill(levelColor(log.level))
                .frame(width: 8, height: 8)
                .padding(.top, 6)

            VStack(alignment: .leading, spacing: 2) {
                Text(log.message)
                    .font(.system(.caption, design: .monospaced))
                    .foregroundColor(.primary)
                    .textSelection(.enabled)
            }
        }
        .padding(.vertical, 4)
        .padding(.horizontal, 8)
    }

    // MARK: - 空态

    private var emptyState: some View {
        VStack(spacing: 12) {
            Image(systemName: "doc.text")
                .font(.system(size: 40))
                .foregroundColor(.secondary)
            Text("暂无 MITM 日志")
                .font(.headline)
                .foregroundColor(.secondary)
            Text("启用 MITM 并连接 VPN 后，解密日志将显示在此处")
                .font(.caption)
                .foregroundColor(.secondary)
                .multilineTextAlignment(.center)
        }
        .padding(.vertical, 60)
        .frame(maxWidth: .infinity)
    }

    // MARK: - 辅助方法

    /// 过滤后的 MITM 日志（包含 mitm/MITM 关键字，或应用搜索条件）
    private var filteredLogs: [LogEntry] {
        let allLogs = environments.commandClient.logList
        // 过滤 MITM 相关日志：消息中包含 mitm/MITM 关键字
        let mitmLogs = allLogs.filter { log in
            log.message.localizedCaseInsensitiveContains("mitm")
        }
        // 应用搜索条件
        if searchText.isEmpty {
            return mitmLogs
        } else {
            return mitmLogs.filter { log in
                log.message.localizedCaseInsensitiveContains(searchText)
            }
        }
    }

    /// 日志级别颜色
    private func levelColor(_ level: Int) -> Color {
        switch level {
        case 0: return .gray      // trace
        case 1: return .blue      // debug
        case 2: return .green     // info
        case 3: return .orange    // warning
        case 4: return .red       // error
        case 5: return .red       // fatal
        default: return .gray
        }
    }
}
