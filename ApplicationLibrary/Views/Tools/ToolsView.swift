import Libbox
import Library
import SwiftUI

/// 工具页面
/// 提供网络诊断、端口转发、URL测试等实用工具入口
public struct ToolsView: View {
    @State private var showPortForward = false
    @State private var showURLTest = false
    @State private var showDNSTest = false

    public init() {}

    public var body: some View {
        NavigationStack {
            List {
                // 网络诊断工具
                Section("网络诊断") {
                    NavigationLink {
                        URLTestView()
                    } label: {
                        toolRow(
                            icon: "globe",
                            title: "URL 测试",
                            subtitle: "测试节点延迟和可用性",
                            color: .blue
                        )
                    }

                    NavigationLink {
                        DNSTestView()
                    } label: {
                        toolRow(
                            icon: "network",
                            title: "DNS 测试",
                            subtitle: "检测 DNS 解析和泄漏",
                            color: .green
                        )
                    }
                }

                // 流量工具
                Section("流量工具") {
                    NavigationLink {
                        PortForwardView()
                    } label: {
                        toolRow(
                            icon: "arrow.left.arrow.right",
                            title: "端口转发",
                            subtitle: "本地端口转发到远程地址",
                            color: .orange
                        )
                    }
                }

                // MITM 工具
                Section("MITM 工具") {
                    Button {
                        MITMSettingsPresenter.show()
                    } label: {
                        toolRow(
                            icon: "lock.shield",
                            title: "MITM 设置",
                            subtitle: "配置 HTTPS 解密和重写规则",
                            color: .purple
                        )
                    }

                    NavigationLink {
                        MITMLogView()
                    } label: {
                        toolRow(
                            icon: "doc.text.magnifyingglass",
                            title: "MITM 日志",
                            subtitle: "查看 HTTPS 解密日志",
                            color: .purple
                        )
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

    // MARK: - 工具行组件

    private func toolRow(icon: String, title: String, subtitle: String, color: Color) -> some View {
        HStack(spacing: 12) {
            Image(systemName: icon)
                .font(.system(size: 18))
                .foregroundColor(.white)
                .frame(width: 32, height: 32)
                .background(color)
                .cornerRadius(8)

            VStack(alignment: .leading, spacing: 2) {
                Text(title)
                    .font(.body)
                    .foregroundColor(.primary)
                Text(subtitle)
                    .font(.caption)
                    .foregroundColor(.secondary)
            }

            Spacer()

            Image(systemName: "chevron.right")
                .foregroundColor(.secondary)
                .font(.caption)
        }
        .padding(.vertical, 4)
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

// MARK: - 端口转发视图

/// 端口转发页面
/// 本地端口转发到远程地址
struct PortForwardView: View {
    @State private var localPort = "8080"
    @State private var remoteHost = "127.0.0.1"
    @State private var remotePort = "1080"
    @State private var isForwarding = false
    @State private var statusMessage = ""

    var body: some View {
        Form {
            Section("本地监听") {
                HStack {
                    Text("端口")
                    TextField("本地端口", text: $localPort)
                        .keyboardType(.numberPad)
                        .multilineTextAlignment(.trailing)
                }
            }

            Section("远程目标") {
                HStack {
                    Text("主机")
                    TextField("远程主机", text: $remoteHost)
                        .autocapitalization(.none)
                        .multilineTextAlignment(.trailing)
                }
                HStack {
                    Text("端口")
                    TextField("远程端口", text: $remotePort)
                        .keyboardType(.numberPad)
                        .multilineTextAlignment(.trailing)
                }
            }

            Section {
                Button {
                    toggleForwarding()
                } label: {
                    HStack {
                        Spacer()
                        Text(isForwarding ? "停止转发" : "开始转发")
                            .foregroundColor(isForwarding ? .red : .accentColor)
                        Spacer()
                    }
                }
            }

            if !statusMessage.isEmpty {
                Section("状态") {
                    Text(statusMessage)
                        .foregroundColor(.secondary)
                }
            }
        }
        .navigationTitle("端口转发")
        .navigationBarTitleDisplayMode(.inline)
    }

    private func toggleForwarding() {
        if isForwarding {
            isForwarding = false
            statusMessage = "已停止端口转发"
        } else {
            guard let localPortInt = Int(localPort),
                  let remotePortInt = Int(remotePort),
                  !remoteHost.isEmpty else {
                statusMessage = "请填写有效的端口和主机"
                return
            }
            isForwarding = true
            statusMessage = "正在转发 0.0.0.0:\(localPortInt) -> \(remoteHost):\(remotePortInt)"
        }
    }
}
