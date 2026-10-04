import SwiftUI
import Libbox

// MARK: - MITM 设置主页面
// 独立全屏页面（fullScreenCover），包含：
// - MITM 总开关
// - 运行状态显示（启用状态/CA 状态/活跃连接数）
// - CA 证书管理入口
// - 域名匹配规则编辑
// - 重写规则管理
// - 错误处理策略选择
// - 上游超时设置

/// MITM 设置页面（独立导航栈）
public struct MITMView: View {
    /// 关闭回调（由 presenting 视图传入）
    public var onDismiss: () -> Void

    @StateObject private var manager = MITMServiceManager.shared
    @State private var showCAView = false
    @State private var showMatchView = false
    @State private var showRewriteView = false
    @State private var alert: AlertState?

    public init(onDismiss: @escaping () -> Void) {
        self.onDismiss = onDismiss
    }

    public var body: some View {
        NavigationStack {
            List {
                // 总开关
                Section {
                    Toggle("启用 MITM", isOn: $manager.configuration.enabled)
                        .tint(.accentColor)
                } footer: {
                    Text("启用后，匹配的 HTTPS 流量将被解密并可进行重写。需要安装并信任 CA 证书。")
                }

                // 运行状态
                Section("运行状态") {
                    statusRow(
                        title: "服务状态",
                        value: manager.runtimeStatus.enabled ? "运行中" : "未运行",
                        color: manager.runtimeStatus.enabled ? .green : .secondary
                    )
                    statusRow(
                        title: "CA 证书",
                        value: manager.runtimeStatus.caInstalled ? "已加载" : "未加载",
                        color: manager.runtimeStatus.caInstalled ? .green : .orange
                    )
                    statusRow(
                        title: "活跃连接",
                        value: "\(manager.runtimeStatus.activeConnections)",
                        color: .primary
                    )
                }

                // CA 证书管理
                Section("CA 证书") {
                    NavigationLink {
                        MITMCAView()
                    } label: {
                        HStack {
                            Image(systemName: "lock.shield.fill")
                                .foregroundColor(.accentColor)
                            Text("证书管理")
                            Spacer()
                            if manager.isCAFileExists {
                                Image(systemName: "checkmark.circle.fill")
                                    .foregroundColor(.green)
                            } else {
                                Image(systemName: "exclamationmark.triangle.fill")
                                    .foregroundColor(.orange)
                            }
                        }
                    }
                }

                // 域名匹配
                Section("域名匹配") {
                    NavigationLink {
                        MITMMatchView()
                    } label: {
                        HStack {
                            Image(systemName: "text.magnifyingglass")
                                .foregroundColor(.accentColor)
                            Text("匹配规则")
                            Spacer()
                            Text("\(manager.configuration.match.domain.count + manager.configuration.match.domainSuffix.count) 条")
                                .foregroundColor(.secondary)
                        }
                    }
                }

                // 重写规则
                Section("流量重写") {
                    Toggle("启用重写", isOn: $manager.configuration.rewrite.enabled)
                        .tint(.accentColor)

                    if manager.configuration.rewrite.enabled {
                        NavigationLink {
                            MITMRewriteView()
                        } label: {
                            HStack {
                                Image(systemName: "pencil.line")
                                    .foregroundColor(.accentColor)
                                Text("重写规则")
                                Spacer()
                                Text("\(manager.configuration.rewrite.rules.count) 条")
                                    .foregroundColor(.secondary)
                            }
                        }

                        VStack(alignment: .leading, spacing: 4) {
                            Text("最大 Body 大小")
                                .font(.subheadline)
                            Stepper(
                                "\(ByteCountFormatter.string(fromByteCount: Int64(manager.configuration.rewrite.maxBodySize), countStyle: .file))",
                                value: $manager.configuration.rewrite.maxBodySize,
                                in: 1024 * 1024...100 * 1024 * 1024,
                                step: 1024 * 1024
                            )
                        }
                    }
                }

                // 高级设置
                Section("高级设置") {
                    Picker("错误处理策略", selection: $manager.configuration.onError) {
                        ForEach(MITMOnError.allCases) { strategy in
                            Text(strategy.displayName).tag(strategy)
                        }
                    }

                    VStack(alignment: .leading, spacing: 4) {
                        Text("上游连接超时")
                            .font(.subheadline)
                        Stepper(
                            "\(manager.configuration.upstreamTimeout) 秒",
                            value: $manager.configuration.upstreamTimeout,
                            in: 5...120,
                            step: 5
                        )
                    }
                }
            }
            .listStyle(.insetGrouped)
            .navigationTitle("MITM 设置")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .navigationBarLeading) {
                    Button {
                        onDismiss()
                    } label: {
                        Image(systemName: "chevron.left")
                            .font(.body.weight(.semibold))
                    }
                }
            }
        }
        // 支持边缘左滑手势返回
        .gesture(
            DragGesture(minimumDistance: 20, coordinateSpace: .local)
                .onEnded { value in
                    if value.startLocation.x < 30 && value.translation.width > 80 {
                        onDismiss()
                    }
                }
        )
    }

    // MARK: - 状态行组件

    private func statusRow(title: String, value: String, color: Color) -> some View {
        HStack {
            Text(title)
            Spacer()
            Text(value)
                .foregroundColor(color)
                .fontWeight(.medium)
        }
    }
}

// MARK: - 预览

#Preview {
    MITMView(onDismiss: {})
}
