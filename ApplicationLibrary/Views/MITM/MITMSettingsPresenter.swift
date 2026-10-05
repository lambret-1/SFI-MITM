import Foundation
import SwiftUI

// MARK: - MITM 设置页面展示控制器
// 通过 NotificationCenter 触发，在 App 顶层以 fullScreenCover 展示 MITM 设置页面
// 使用独立导航栈，不与 MainView 的 TabView 导航冲突

/// MITM 设置展示通知名称
public extension Notification.Name {
    static let showMITMSettings = Notification.Name("com.mitm.box.showMITMSettings")
}

/// MITM 设置页面展示控制器
@MainActor
public final class MITMSettingsPresenter: ObservableObject {
    /// 共享实例
    public static let shared = MITMSettingsPresenter()

    /// 是否展示 MITM 设置页面
    @Published public var isPresented = false

    private var observer: NSObjectProtocol?

    private init() {
        observer = NotificationCenter.default.addObserver(
            forName: .showMITMSettings,
            object: nil,
            queue: .main
        ) { [weak self] _ in
            self?.isPresented = true
        }
    }

    deinit {
        if let observer {
            NotificationCenter.default.removeObserver(observer)
        }
    }

    /// 请求展示 MITM 设置页面
    public static func show() {
        NotificationCenter.default.post(name: .showMITMSettings, object: nil)
    }
}

// MARK: - MITM 设置入口按钮
// 可在任意视图中使用，点击后打开 MITM 设置页面

/// MITM 设置入口按钮
public struct MITMSettingsButton: View {
    public init() {}

    public var body: some View {
        Button {
            MITMSettingsPresenter.show()
        } label: {
            HStack {
                Image(systemName: "lock.shield.fill")
                    .foregroundColor(.accentColor)
                Text("MITM 设置")
                Spacer()
                Image(systemName: "chevron.right")
                    .foregroundColor(.secondary)
            }
        }
    }
}
