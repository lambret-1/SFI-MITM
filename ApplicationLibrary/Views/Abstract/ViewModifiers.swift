import SwiftUI

public struct PlatformSheetSize {
    let minWidth: CGFloat
    let minHeight: CGFloat

    public init(minWidth: CGFloat, minHeight: CGFloat) {
        self.minWidth = minWidth
        self.minHeight = minHeight
    }

    public static let `default` = PlatformSheetSize(minWidth: 500, minHeight: 400)
    public static let small = PlatformSheetSize(minWidth: 400, minHeight: 300)
}

public extension View {
    func platformSheet(
        isPresented: Binding<Bool>,
        size: PlatformSheetSize = .default,
        @ViewBuilder content: @escaping () -> some View
    ) -> some View {
        modifier(PlatformSheetModifier(isPresented: isPresented, size: size, content: content))
    }

    func platformSheet<Item: Identifiable>(
        item: Binding<Item?>,
        size: PlatformSheetSize = .default,
        @ViewBuilder content: @escaping (Item) -> some View
    ) -> some View {
        modifier(PlatformSheetItemModifier(item: item, size: size, content: content))
    }
}

private struct PlatformSheetModifier<SheetContent: View>: ViewModifier {
    @Binding var isPresented: Bool
    let size: PlatformSheetSize
    @ViewBuilder let content: () -> SheetContent

    func body(content: Content) -> some View {
        #if os(iOS)
            content.sheet(isPresented: $isPresented) {
                NavigationStackCompat {
                    self.content()
                }
            }
        #elseif os(macOS)
            content.sheet(isPresented: $isPresented) {
                NavigationStackCompat {
                    self.content()
                }
                .frame(minWidth: size.minWidth, minHeight: size.minHeight)
            }
        #elseif os(tvOS)
            content.fullScreenCover(isPresented: $isPresented) {
                NavigationStackCompat {
                    self.content()
                }
            }
        #endif
    }
}

private struct PlatformSheetItemModifier<Item: Identifiable, SheetContent: View>: ViewModifier {
    @Binding var item: Item?
    let size: PlatformSheetSize
    @ViewBuilder let content: (Item) -> SheetContent

    func body(content: Content) -> some View {
        #if os(iOS)
            content.sheet(item: $item) { item in
                NavigationStackCompat {
                    self.content(item)
                }
            }
        #elseif os(macOS)
            content.sheet(item: $item) { item in
                NavigationStackCompat {
                    self.content(item)
                }
                .frame(minWidth: size.minWidth, minHeight: size.minHeight)
            }
        #elseif os(tvOS)
            content.fullScreenCover(item: $item) { item in
                NavigationStackCompat {
                    self.content(item)
                }
            }
        #endif
    }
}

public extension View {
    @ViewBuilder
    func presentationDetentsIfAvailable() -> some View {
        #if os(iOS) || os(tvOS)
            if #available(iOS 16.0, tvOS 17.0, *) {
                presentationDetents([.large])
                    .presentationDragIndicator(.visible)
            } else {
                self
            }
        #else
            self
        #endif
    }

    #if os(iOS) || os(tvOS)
        @available(iOS 16.0, tvOS 17.0, *)
        @ViewBuilder
        func presentationDetentsIfAvailable(_ detents: PresentationDetent...) -> some View {
            let detentSet: Set<PresentationDetent> = detents.isEmpty ? [.large] : Set(detents)
            presentationDetents(detentSet)
                .presentationDragIndicator(.visible)
        }
    #endif

    @ViewBuilder
    func actionButtonStyle() -> some View {
        #if os(tvOS)
            ActionButtonWrapper { self }
        #else
            // 使用毛玻璃材质替代不存在的 glassEffect API，兼容 iOS 16+
            frame(width: 44, height: 32)
                .background(.ultraThinMaterial, in: RoundedRectangle(cornerRadius: 8))
        #endif
    }
}

#if os(tvOS)
    private struct ActionButtonWrapper<Content: View>: View {
        @Environment(\.isFocused) private var isFocused
        let content: () -> Content

        var body: some View {
            content()
                .frame(width: 70, height: 48)
                .background(isFocused ? Color.secondary.opacity(0.3) : Color.secondary.opacity(0.1))
                .clipShape(RoundedRectangle(cornerRadius: 12))
                .focusEffectDisabled()
        }
    }
#endif

public extension View {
    func cardStyle() -> some View {
        modifier(CardStyleModifier())
    }
}

private struct CardStyleModifier: ViewModifier {
    @Environment(\.colorScheme) private var colorScheme

    func body(content: Content) -> some View {
        // 使用毛玻璃材质替代不存在的 glassEffect API，兼容 iOS 16+
        content
            .background(.thinMaterial, in: RoundedRectangle(cornerRadius: 16))
    }
}
