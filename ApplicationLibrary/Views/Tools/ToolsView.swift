import Library
import NetworkExtension
import SwiftUI

@MainActor
public struct ToolsView: View {
    @EnvironmentObject private var environments: ExtensionEnvironments
    #if os(iOS)
        @State private var showOOMReportList = false
        @State private var showPowerReportList = false
    #endif

    public init() {}

    public var body: some View {
        FormView {
            Section("网络") {
                FormNavigationLink {
                    NetworkQualityView()
                } label: {
                    Label("网络质量", systemImage: "network")
                }
                FormNavigationLink {
                    STUNTestView()
                } label: {
                    Label("STUN 测试", systemImage: "arrow.triangle.swap")
                }
            }

            // OOM/电源报告读取本地设备
            if environments.remoteServer == nil {
                Section("调试") {
                    #if os(iOS)
                        NavigationLink(isActive: $showOOMReportList) {
                            OOMReportListView()
                        } label: {
                            Label("OOM 报告", systemImage: "memorychip")
                                .badge(environments.oomReportManager.unreadCount)
                        }
                        .onReceive(NotificationCenter.default.publisher(for: .reportReceived)) { notification in
                            Task {
                                try? await Task.sleep(nanoseconds: NSEC_PER_MSEC * 300)
                                if let reportType = notification.object as? ReportType {
                                    switch reportType {
                                    case .oom:
                                        showOOMReportList = true
                                    case .power:
                                        showPowerReportList = true
                                    default:
                                        break
                                    }
                                }
                            }
                        }
                        NavigationLink(isActive: $showPowerReportList) {
                            PowerReportListView()
                        } label: {
                            Label("电源报告", systemImage: "battery.50percent")
                                .badge(environments.powerReportManager.unreadCount)
                        }
                    #else
                        FormNavigationLink {
                            OOMReportListView()
                        } label: {
                            Label("OOM 报告", systemImage: "memorychip")
                                .badge(environments.oomReportManager.unreadCount)
                        }
                        FormNavigationLink {
                            PowerReportListView()
                        } label: {
                            Label("电源报告", systemImage: "battery.50percent")
                                .badge(environments.powerReportManager.unreadCount)
                        }
                    #endif
                }
            }
        }
        .navigationTitle("工具")
    }
}
