import ApplicationLibrary
import Foundation
import Libbox
import Library
import UIKit

class ApplicationDelegate: NSObject, UIApplicationDelegate {
    func application(_: UIApplication, didFinishLaunchingWithOptions _: [UIApplication.LaunchOptionsKey: Any]? = nil) -> Bool {
        NSLog("Here I stand")
        let options = LibboxSetupOptions()
        options.basePath = FilePath.sharedDirectory.relativePath
        options.workingPath = FilePath.workingDirectory.relativePath
        options.tempPath = FilePath.cacheDirectory.relativePath
        var port = SharedPreferences.commandServerPort.getBlocking()
        var secret = SharedPreferences.commandServerSecret.getBlocking()
        if port == 0 || secret.isEmpty {
            var error: NSError?
            LibboxAvailablePort(7990, &port, &error)
            if let error {
                port = 7990
                NSLog("Failed to get available port for control server: \(error.localizedDescription)")
            }
            if let randomHex = LibboxRandomHex(16) {
                secret = randomHex.value
            } else {
                secret = UUID().uuidString.replacingOccurrences(of: "-", with: "")
                NSLog("LibboxRandomHex 返回 nil，使用 UUID 作为备用密钥")
            }
            Task {
                await SharedPreferences.commandServerPort.set(port)
                await SharedPreferences.commandServerSecret.set(secret)
            }
        }
        options.commandServerListenPort = port
        options.commandServerSecret = secret
        var error: NSError?
        LibboxSetup(options, &error)
        LibboxSetLocale(Locale.current.identifier)
        setup()
        return true
    }

    private func setup() {
        do {
            try UIProfileUpdateTask.configure()
            NSLog("setup background task success")
        } catch {
            NSLog("setup background task error: \(error.localizedDescription)")
        }
    }
}
