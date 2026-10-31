// swift-tools-version: 6.2
import PackageDescription

let package = Package(
    name: "tomoru-engine",
    platforms: [.macOS("26.0")],
    targets: [
        .executableTarget(
            name: "tomoru-engine",
            path: "Sources/tomoru-engine",
            swiftSettings: [.swiftLanguageMode(.v5)]
        )
    ]
)
