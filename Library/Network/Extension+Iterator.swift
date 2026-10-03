import Foundation
import Libbox

public extension LibboxStringIterator {
    func toArray() -> [String] {
        var array: [String] = []
        while hasNext() {
            array.append(next())
        }
        return array
    }
}
