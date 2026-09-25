package org.sthira.mobile.storage

/**
 * Platform-agnostic interface for secure token storage.
 * Android: EncryptedSharedPreferences backed by Android Keystore.
 * iOS: Keychain Services with kSecAttrAccessibleAfterFirstUnlock.
 */
expect class SecureTokenStorage {
    suspend fun saveSessionToken(token: String)
    suspend fun getSessionToken(): String?
    suspend fun clearSessionToken()
}
