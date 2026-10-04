import SwiftUI

// MARK: - 域名匹配规则编辑页面
// 支持四种匹配方式：
// - 精确匹配（domain）
// - 后缀匹配（domain_suffix）
// - 关键字匹配（domain_keyword）
// - 正则匹配（domain_regex）

/// 域名匹配规则编辑页面
public struct MITMMatchView: View {
    @StateObject private var manager = MITMServiceManager.shared
    @State private var newDomain = ""
    @State private var newSuffix = ""
    @State private var newKeyword = ""
    @State private var newRegex = ""

    public init() {}

    public var body: some View {
        List {
            // 精确匹配
            matchSection(
                title: "精确匹配",
                subtitle: "域名完全一致时命中",
                items: $manager.configuration.match.domain,
                newItem: $newDomain,
                placeholder: "example.com",
                systemImage: "target"
            )

            // 后缀匹配
            matchSection(
                title: "后缀匹配",
                subtitle: "域名以指定后缀结尾时命中（如 example.com 匹配 www.example.com）",
                items: $manager.configuration.match.domainSuffix,
                newItem: $newSuffix,
                placeholder: "example.com",
                systemImage: "text.line.last"
            )

            // 关键字匹配
            matchSection(
                title: "关键字匹配",
                subtitle: "域名包含指定关键字时命中",
                items: $manager.configuration.match.domainKeyword,
                newItem: $newKeyword,
                placeholder: "cdn",
                systemImage: "text.magnifyingglass"
            )

            // 正则匹配
            matchSection(
                title: "正则匹配",
                subtitle: "域名匹配指定正则表达式时命中",
                items: $manager.configuration.match.domainRegex,
                newItem: $newRegex,
                placeholder: "^api\\..+\\.com$",
                systemImage: "ellipsis.curlybraces"
            )
        }
        .listStyle(.insetGrouped)
        .navigationTitle("域名匹配规则")
        .navigationBarTitleDisplayMode(.inline)
    }

    // MARK: - 匹配规则区块

    private func matchSection(
        title: String,
        subtitle: String,
        items: Binding<[String]>,
        newItem: Binding<String>,
        placeholder: String,
        systemImage: String
    ) -> some View {
        Section {
            ForEach(items.wrappedValue.indices, id: \.self) { index in
                HStack {
                    Image(systemName: systemImage)
                        .foregroundColor(.accentColor)
                        .frame(width: 24)
                    Text(items.wrappedValue[index])
                        .font(.system(.body, design: .monospaced))
                    Spacer()
                    Button(role: .destructive) {
                        items.wrappedValue.remove(at: index)
                    } label: {
                        Image(systemName: "minus.circle.fill")
                            .foregroundColor(.red)
                    }
                }
            }

            HStack {
                Image(systemName: "plus.circle.fill")
                    .foregroundColor(.green)
                    .frame(width: 24)
                TextField(placeholder, text: newItem)
                    .textInputAutocapitalization(.never)
                    .autocorrectionDisabled()
                Button {
                    let value = newItem.wrappedValue.trimmingCharacters(in: .whitespacesAndNewlines)
                    guard !value.isEmpty, !items.wrappedValue.contains(value) else { return }
                    items.wrappedValue.append(value)
                    newItem.wrappedValue = ""
                } label: {
                    Image(systemName: "plus.circle.fill")
                        .foregroundColor(.accentColor)
                }
                .disabled(newItem.wrappedValue.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty)
            }
        } header: {
            Text(title)
        } footer: {
            Text(subtitle)
        }
    }
}

// MARK: - 预览

#Preview {
    NavigationStack {
        MITMMatchView()
    }
}
