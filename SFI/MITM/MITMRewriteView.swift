import SwiftUI

// MARK: - 重写规则管理页面
// 功能：
// - 列出所有重写规则
// - 添加/编辑/删除重写规则
// - 每个规则包含：域名匹配、路径匹配、方法匹配、请求头修改、响应头修改、Body 替换

/// 重写规则管理页面
public struct MITMRewriteView: View {
    @StateObject private var manager = MITMServiceManager.shared
    @State private var editingRule: MITMRewriteRule?
    @State private var showEditor = false

    public init() {}

    public var body: some View {
        List {
            if manager.configuration.rewrite.rules.isEmpty {
                ContentUnavailableView(
                    "暂无重写规则",
                    systemImage: "pencil.line",
                    description: Text("点击右上角添加第一条重写规则")
                )
            } else {
                ForEach($manager.configuration.rewrite.rules) { $rule in
                    ruleRow(rule: rule)
                }
                .onDelete { indexSet in
                    manager.configuration.rewrite.rules.remove(atOffsets: indexSet)
                }
            }
        }
        .listStyle(.insetGrouped)
        .navigationTitle("重写规则")
        .navigationBarTitleDisplayMode(.inline)
        .toolbar {
            ToolbarItem(placement: .navigationBarTrailing) {
                Button {
                    editingRule = MITMRewriteRule(name: "新规则")
                    showEditor = true
                } label: {
                    Image(systemName: "plus")
                }
            }
        }
        .sheet(isPresented: $showEditor) {
            if let rule = editingRule {
                MITMRewriteRuleEditor(rule: rule) { updatedRule in
                    if let index = manager.configuration.rewrite.rules.firstIndex(where: { $0.id == updatedRule.id }) {
                        manager.configuration.rewrite.rules[index] = updatedRule
                    } else {
                        manager.configuration.rewrite.rules.append(updatedRule)
                    }
                    showEditor = false
                    editingRule = nil
                }
            }
        }
    }

    // MARK: - 规则行

    private func ruleRow(rule: MITMRewriteRule) -> some View {
        Button {
            editingRule = rule
            showEditor = true
        } label: {
            HStack {
                VStack(alignment: .leading, spacing: 4) {
                    Text(rule.name.isEmpty ? "未命名规则" : rule.name)
                        .font(.headline)
                        .foregroundColor(.primary)

                    HStack(spacing: 8) {
                        if !rule.domainSuffix.isEmpty {
                            label("后缀: \(rule.domainSuffix.count)")
                        }
                        if !rule.domain.isEmpty {
                            label("精确: \(rule.domain.count)")
                        }
                        if !rule.requestHeader.isEmpty {
                            label("请求头: \(rule.requestHeader.count)")
                        }
                        if !rule.responseHeader.isEmpty {
                            label("响应头: \(rule.responseHeader.count)")
                        }
                        if !rule.bodyReplace.isEmpty {
                            label("Body: \(rule.bodyReplace.count)")
                        }
                    }
                }
                Spacer()
                Image(systemName: "chevron.right")
                    .foregroundColor(.secondary)
            }
        }
    }

    private func label(_ text: String) -> some View {
        Text(text)
            .font(.caption2)
            .padding(.horizontal, 6)
            .padding(.vertical, 2)
            .background(Color.accentColor.opacity(0.15))
            .foregroundColor(.accentColor)
            .cornerRadius(4)
    }
}

// MARK: - 重写规则编辑器

/// 重写规则编辑器（全屏 Sheet）
private struct MITMRewriteRuleEditor: View {
    @State private var rule: MITMRewriteRule
    let onSave: (MITMRewriteRule) -> Void
    @Environment(\.dismiss) private var dismiss

    init(rule: MITMRewriteRule, onSave: @escaping (MITMRewriteRule) -> Void) {
        self._rule = State(initialValue: rule)
        self.onSave = onSave
    }

    var body: some View {
        NavigationStack {
            Form {
                Section("基本信息") {
                    TextField("规则名称", text: $rule.name)
                }

                Section("域名匹配") {
                    editableList(title: "精确匹配", items: $rule.domain, placeholder: "api.example.com")
                    editableList(title: "后缀匹配", items: $rule.domainSuffix, placeholder: "example.com")
                    editableList(title: "路径前缀", items: $rule.pathPrefix, placeholder: "/api/v1")
                }

                Section("HTTP 方法") {
                    ScrollView(.horizontal, showsIndicators: false) {
                        HStack {
                            ForEach(["GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"], id: \.self) { method in
                                methodButton(method)
                            }
                        }
                    }
                }

                Section("请求头修改") {
                    keyValueEditor(title: "添加/修改", dict: $rule.requestHeader)
                    editableList(title: "删除", items: $rule.requestHeaderDelete, placeholder: "X-Original-Forwarded-For")
                }

                Section("响应头修改") {
                    keyValueEditor(title: "添加/修改", dict: $rule.responseHeader)
                    editableList(title: "删除", items: $rule.responseHeaderDelete, placeholder: "X-Powered-By")
                }

                Section("Body 替换") {
                    ForEach($rule.bodyReplace) { $replace in
                        HStack {
                            TextField("查找", text: $replace.find)
                                .textInputAutocapitalization(.never)
                            Image(systemName: "arrow.right")
                                .foregroundColor(.secondary)
                            TextField("替换", text: $replace.replace)
                                .textInputAutocapitalization(.never)
                        }
                    }
                    .onDelete { indexSet in
                        rule.bodyReplace.remove(atOffsets: indexSet)
                    }
                    Button {
                        rule.bodyReplace.append(MITMBodyReplace())
                    } label: {
                        HStack {
                            Image(systemName: "plus.circle.fill")
                                .foregroundColor(.green)
                            Text("添加替换规则")
                        }
                    }
                }
            }
            .navigationTitle("编辑重写规则")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .navigationBarLeading) {
                    Button("取消") {
                        dismiss()
                    }
                }
                ToolbarItem(placement: .navigationBarTrailing) {
                    Button("保存") {
                        onSave(rule)
                    }
                    .fontWeight(.semibold)
                }
            }
        }
    }

    private func methodButton(_ method: String) -> some View {
        let isSelected = rule.method.contains(method)
        return Button {
            if isSelected {
                rule.method.removeAll { $0 == method }
            } else {
                rule.method.append(method)
            }
        } label: {
            Text(method)
                .font(.caption.weight(.semibold))
                .padding(.horizontal, 12)
                .padding(.vertical, 6)
                .background(isSelected ? Color.accentColor : Color.gray.opacity(0.2))
                .foregroundColor(isSelected ? .white : .primary)
                .cornerRadius(8)
        }
    }

    private func editableList(title: String, items: Binding<[String]>, placeholder: String) -> some View {
        VStack(alignment: .leading, spacing: 8) {
            Text(title)
                .font(.subheadline)
                .foregroundColor(.secondary)
            ForEach(items.wrappedValue.indices, id: \.self) { index in
                HStack {
                    TextField(placeholder, text: items[index])
                        .textInputAutocapitalization(.never)
                        .autocorrectionDisabled()
                    Button(role: .destructive) {
                        items.wrappedValue.remove(at: index)
                    } label: {
                        Image(systemName: "minus.circle.fill")
                            .foregroundColor(.red)
                    }
                }
            }
            Button {
                items.wrappedValue.append("")
            } label: {
                HStack {
                    Image(systemName: "plus.circle.fill")
                        .foregroundColor(.green)
                    Text("添加")
                }
            }
        }
    }

    private func keyValueEditor(title: String, dict: Binding<[String: String]>) -> some View {
        VStack(alignment: .leading, spacing: 8) {
            Text(title)
                .font(.subheadline)
                .foregroundColor(.secondary)
            ForEach(Array(dict.wrappedValue.keys), id: \.self) { key in
                HStack {
                    TextField("Header", text: Binding(
                        get: { key },
                        set: { newValue in
                            let value = dict.wrappedValue[key] ?? ""
                            dict.wrappedValue.removeValue(forKey: key)
                            dict.wrappedValue[newValue] = value
                        }
                    ))
                    .textInputAutocapitalization(.never)
                    TextField("Value", text: Binding(
                        get: { dict.wrappedValue[key] ?? "" },
                        set: { dict.wrappedValue[key] = $0 }
                    ))
                    .textInputAutocapitalization(.never)
                    Button(role: .destructive) {
                        dict.wrappedValue.removeValue(forKey: key)
                    } label: {
                        Image(systemName: "minus.circle.fill")
                            .foregroundColor(.red)
                    }
                }
            }
            Button {
                dict.wrappedValue["X-Custom-Header"] = "value"
            } label: {
                HStack {
                    Image(systemName: "plus.circle.fill")
                        .foregroundColor(.green)
                    Text("添加 Header")
                }
            }
        }
    }
}

// MARK: - 预览

#Preview {
    NavigationStack {
        MITMRewriteView()
    }
}
