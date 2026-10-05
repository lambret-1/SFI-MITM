import Libbox
import Library
import SwiftUI

/// 工具页面
/// 对标官方 sing-box for iOS 工具页面结构，提供网络诊断和 MITM 专项工具
public struct ToolsView: View {
    public init() {}

    public var body: some View {
        NavigationStack {
            List {
                // 网络诊断（对标官方 Network 分类）
                Section("网络") {
                    NavigationLink {
                        URLTestView()
                    } label: {
                        Label("URL 测试", systemImage: "globe")
                    }

                    NavigationLink {
                        DNSTestView()
                    } label: {
                        Label("DNS 测试", systemImage: "network")
                    }
                }

                // MITM 专项工具（本项目特有）
                Section("MITM") {
                    Button {
                        MITMSettingsPresenter.show()
                    } label: {
                        HStack {
                            Label("MITM 设置", systemImage: "lock.shield")
                            Spacer()
                            Image(systemName: "chevron.right")
                                .foregroundColor(.secondary)
                                .font(.caption)
                        }
                    }

                    NavigationLink {
                        MITMLogView()
                    } label: {
                        Label("MITM 日志", systemImage: "doc.text.magnifyingglass")
                    }
                }

                // 关于
                Section("关于") {
                    HStack {
                        Text("版本")
                        Spacer()
                        Text(Bundle.main.infoDictionary?["CFBundleShortVersionString"] as? String ?? "未知")
                            .foregroundColor(.secondary)
                    }
                }
            }
            .listStyle(.insetGrouped)
            .navigationTitle("工具")
            .navigationBarTitleDisplayMode(.inline)
        }
    }
}

// MARK: - URL 测试视图

/// URL 测试页面
/// 测试指定 URL 的延迟和可用性
struct URLTestView: View {
    @State private var testURL = "https://www.google.com"
    @State private var isTesting = false
    @State private var result: String?
    @State private var delay: Int?

    var body: some View {
        Form {
            Section("测试地址") {
                TextField("URL", text: $testURL)
                    .textContentType(.URL)
                    .autocapitalization(.none)
            }

            Section {
                Button {
                    startTest()
                } label: {
                    HStack {
                        Spacer()
                        if isTesting {
                            ProgressView()
                            Text("测试中...")
                        } else {
                            Text("开始测试")
                        }
                        Spacer()
                    }
                }
                .disabled(isTesting || testURL.isEmpty)
            }

            if let result {
                Section("测试结果") {
                    HStack {
                        Text("状态")
                        Spacer()
                        Text(result)
                            .foregroundColor(result == "成功" ? .green : .red)
                    }
                    if let delay {
                        HStack {
                            Text("延迟")
                            Spacer()
                            Text("\(delay) ms")
                                .foregroundColor(delay < 200 ? .green : (delay < 500 ? .orange : .red))
                        }
                    }
                }
            }
        }
        .navigationTitle("URL 测试")
        .navigationBarTitleDisplayMode(.inline)
    }

    private func startTest() {
        isTesting = true
        result = nil
        delay = nil

        let startTime = Date()
        let task = URLSession.shared.dataTask(with: URL(string: testURL)!) { _, response, error in
            DispatchQueue.main.async {
                isTesting = false
                let elapsed = Int(Date().timeIntervalSince(startTime) * 1000)
                if error != nil {
                    result = "失败"
                } else if let httpResponse = response as? HTTPURLResponse {
                    result = httpResponse.statusCode == 200 ? "成功" : "HTTP \(httpResponse.statusCode)"
                    delay = elapsed
                } else {
                    result = "成功"
                    delay = elapsed
                }
            }
        }
        task.resume()
    }
}

// MARK: - DNS 测试视图

/// DNS 测试页面
/// 检测 DNS 解析和泄漏
struct DNSTestView: View {
    @State private var domain = "www.google.com"
    @State private var isTesting = false
    @State private var results: [String] = []

    var body: some View {
        Form {
            Section("测试域名") {
                TextField("域名", text: $domain)
                    .autocapitalization(.none)
            }

            Section {
                Button {
                    startTest()
                } label: {
                    HStack {
                        Spacer()
                        if isTesting {
                            ProgressView()
                            Text("解析中...")
                        } else {
                            Text("开始解析")
                        }
                        Spacer()
                    }
                }
                .disabled(isTesting || domain.isEmpty)
            }

            if !results.isEmpty {
                Section("解析结果") {
                    ForEach(results, id: \.self) { result in
                        Text(result)
                            .font(.system(.body, design: .monospaced))
                    }
                }
            }
        }
        .navigationTitle("DNS 测试")
        .navigationBarTitleDisplayMode(.inline)
    }

    private func startTest() {
        isTesting = true
        results = []

        DispatchQueue.global(qos: .userInitiated).async {
            let host = CFHostCreateWithName(nil, domain as CFString).takeRetainedValue()
            CFHostStartInfoResolution(host, .addresses, nil)

            var success: DarwinBoolean = false
            if let addresses = CFHostGetAddressing(host, &success)?.takeUnretainedValue() as? [Data] {
                for address in addresses {
                    var hostname = [CChar](repeating: 0, count: Int(NI_MAXHOST))
                    let result = address.withUnsafeBytes { ptr in
                        getnameinfo(
                            ptr.bindMemory(to: sockaddr.self).baseAddress,
                            socklen_t(address.count),
                            &hostname,
                            socklen_t(hostname.count),
                            nil,
                            0,
                            NI_NUMERICHOST
                        )
                    }
                    if result == 0 {
                        let ip = String(cString: hostname)
                        DispatchQueue.main.async {
                            results.append(ip)
                        }
                    }
                }
            }

            DispatchQueue.main.async {
                isTesting = false
                if results.isEmpty {
                    results.append("解析失败")
                }
            }
        }
    }
}
