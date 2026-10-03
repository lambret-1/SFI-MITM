import SwiftUI

// MARK: - CA 证书管理页面
// 功能：
// - 显示当前 CA 证书状态（是否已配置/文件是否存在）
// - 生成新的 CA 证书（调用 Libbox API）
// - 显示证书路径
// - 导出/分享证书文件（用于安装到系统）

/// CA 证书管理页面
public struct MITMCAView: View {
    @StateObject private var manager = MITMServiceManager.shared
    @State private var isGenerating = false
    @State private var alert: AlertState?
    @State private var showExportSheet = false
    @State private var certificateURL: URL?

    public init() {}

    public var body: some View {
        List {
            // 当前状态
            Section("当前状态") {
                statusRow(
                    title: "证书已配置",
                    value: manager.configuration.ca.isConfigured ? "是" : "否",
                    color: manager.configuration.ca.isConfigured ? .green : .secondary
                )
                statusRow(
                    title: "证书文件存在",
                    value: manager.isCAFileExists ? "是" : "否",
                    color: manager.isCAFileExists ? .green : .red
                )
            }

            // 证书路径
            if manager.configuration.ca.isConfigured {
                Section("证书路径") {
                    VStack(alignment: .leading, spacing: 4) {
                        Text("证书文件")
                            .font(.caption)
                            .foregroundColor(.secondary)
                        Text(manager.configuration.ca.certificate)
                            .font(.system(.footnote, design: .monospaced))
                            .textSelection(.enabled)
                    }

                    VStack(alignment: .leading, spacing: 4) {
                        Text("私钥文件")
                            .font(.caption)
                            .foregroundColor(.secondary)
                        Text(manager.configuration.ca.privateKey)
                            .font(.system(.footnote, design: .monospaced))
                            .textSelection(.enabled)
                    }
                }
            }

            // 操作
            Section {
                Button {
                    generateCA()
                } label: {
                    HStack {
                        if isGenerating {
                            ProgressView()
                        } else {
                            Image(systemName: "plus.circle.fill")
                                .foregroundColor(.accentColor)
                        }
                        Text(isGenerating ? "正在生成..." : "生成新 CA 证书")
                    }
                }
                .disabled(isGenerating)

                if manager.isCAFileExists {
                    Button {
                        exportCertificate()
                    } label: {
                        HStack {
                            Image(systemName: "square.and.arrow.up")
                                .foregroundColor(.accentColor)
                            Text("导出证书（用于安装）")
                        }
                    }
                }
            } footer: {
                Text("生成的 CA 证书使用 ECDSA P-256 曲线，有效期 10 年。生成后需要在 iOS 设置中安装并信任该证书。")
            }
        }
        .listStyle(.insetGrouped)
        .navigationTitle("CA 证书管理")
        .navigationBarTitleDisplayMode(.inline)
        .alert($alert)
        .sheet(isPresented: $showExportSheet) {
            if let url = certificateURL {
                ShareSheet(items: [url])
            }
        }
    }

    // MARK: - 生成 CA

    private func generateCA() {
        isGenerating = true
        DispatchQueue.global(qos: .userInitiated).async {
            let result = manager.generateCAInAppGroup()
            DispatchQueue.main.async {
                isGenerating = false
                switch result {
                case .success:
                    alert = AlertState(
                        title: "生成成功",
                        message: "CA 证书已生成并保存到 App Group 共享目录。请在设置中安装并信任该证书。"
                    )
                case .failure(let error):
                    alert = AlertState(
                        title: "生成失败",
                        message: error.localizedDescription
                    )
                }
            }
        }
    }

    // MARK: - 导出证书

    private func exportCertificate() {
        let certPath = manager.configuration.ca.certificate
        certificateURL = URL(fileURLWithPath: certPath)
        showExportSheet = true
    }

    // MARK: - 状态行

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

// MARK: - 分享 Sheet 包装

private struct ShareSheet: UIViewControllerRepresentable {
    let items: [Any]

    func makeUIViewController(context: Context) -> UIActivityViewController {
        UIActivityViewController(activityItems: items, applicationActivities: nil)
    }

    func updateUIViewController(_ uiViewController: UIActivityViewController, context: Context) {}
}

// MARK: - 预览

#Preview {
    NavigationStack {
        MITMCAView()
    }
}
