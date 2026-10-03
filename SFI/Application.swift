import Foundation
import Library
import SwiftUI

@main
struct Application: App {
    @UIApplicationDelegateAdaptor private var appDelegate: ApplicationDelegate
    @StateObject private var environments = ExtensionEnvironments()
    @StateObject private var mitmPresenter = MITMSettingsPresenter.shared

    var body: some Scene {
        WindowGroup {
            MainView()
                .environmentObject(environments)
                .fullScreenCover(isPresented: $mitmPresenter.isPresented) {
                    MITMView {
                        mitmPresenter.isPresented = false
                    }
                }
        }
    }
}
