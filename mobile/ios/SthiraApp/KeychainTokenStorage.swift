import Foundation
import Security

/// KeychainTokenStorage: stores citizen session tokens and keys inside the iOS Keychain
/// with kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly to prevent backup extraction.
class KeychainTokenStorage {
    static let shared = KeychainTokenStorage()
    private let service = "org.sthira.mobile.security"

    func saveToken(token: String, forAccount account: String) -> Bool {
        guard let data = token.data(using: .utf8) else { return false }

        // Remove any existing entry first
        deleteToken(forAccount: account)

        let query: [String: Any] = [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrService as String: service,
            kSecAttrAccount as String: account,
            kSecValueData as String: data,
            kSecAttrAccessible as String: kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly
        ]

        let status = SecItemAdd(query as CFDictionary, nil)
        return status == errSecSuccess
    }

    func getToken(forAccount account: String) -> String? {
        let query: [String: Any] = [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrService as String: service,
            kSecAttrAccount as String: account,
            kSecReturnData as String: true,
            kSecMatchLimit as String: kSecMatchLimitOne
        ]

        var item: CFTypeRef?
        let status = SecItemCopyMatching(query as CFDictionary, &item)
        guard status == errSecSuccess, let data = item as? Data else {
            return nil
        }
        return String(data: data, encoding: .utf8)
    }

    func deleteToken(forAccount account: String) {
        let query: [String: Any] = [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrService as String: service,
            kSecAttrAccount as String: account
        ]
        SecItemDelete(query as CFDictionary)
    }
}
