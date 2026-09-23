package org.sthira.mobile.session

import org.sthira.mobile.contracts.CitizenSession
import org.sthira.mobile.storage.SecureTokenStorage

/**
 * Manages citizen session credentials and ensures clean separation
 * between public offline data and private capability tokens.
 */
class SessionManager(
    private val secureStorage: SecureTokenStorage
) {
    private var activeSession: CitizenSession? = null

    fun getActiveSession(): CitizenSession? = activeSession

    suspend fun saveSession(session: CitizenSession) {
        this.activeSession = session
        secureStorage.saveSessionToken(session.token)
    }

    suspend fun clearSession() {
        this.activeSession = null
        secureStorage.clearSessionToken()
    }

    fun isSessionExpired(nowEpochSeconds: Long, issuedEpochSeconds: Long): Boolean {
        val session = activeSession ?: return true
        return (nowEpochSeconds - issuedEpochSeconds) >= session.expiresInSeconds
    }
}
