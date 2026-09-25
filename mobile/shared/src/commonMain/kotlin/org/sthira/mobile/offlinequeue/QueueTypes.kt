package org.sthira.mobile.offlinequeue

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable

/**
 * Durable state of an offline pending write operation matching backend/internal/offlinequeue.
 */
enum class OpState {
    PENDING,
    IN_FLIGHT,
    PENDING_RECONCILIATION,
    COMMITTED,
    FAILED_STALE,
    FAILED_PERM
}

@Serializable
data class PendingOperation(
    @SerialName("id") val id: String,
    @SerialName("op_name") val opName: String,
    @SerialName("idempotency_key") val idempotencyKey: String,
    @SerialName("payload_json") val payloadJson: String,
    @SerialName("payload_hash") val payloadHash: String,
    @SerialName("token_ref") val tokenRef: String, // Opaque reference, never raw token
    @SerialName("state") val state: OpState,
    @SerialName("snapshot_version") val snapshotVersion: Int,
    @SerialName("selection_expires_epoch") val selectionExpiresEpoch: Long,
    @SerialName("attempts") val attempts: Int = 0,
    @SerialName("last_error") val lastError: String? = null,
    @SerialName("created_at_epoch") val createdAtEpoch: Long,
    @SerialName("updated_at_epoch") val updatedAtEpoch: Long
)

sealed class QueueException(message: String) : Exception(message) {
    class OperationConflict(key: String) : QueueException("Operation with idempotency key '$key' already pending")
    class PayloadConflict(key: String) : QueueException("Idempotency key '$key' reused with different payload")
    class NoSuchOperation(id: String) : QueueException("No operation found with ID '$id'")
    class TerminalState(id: String, state: OpState) : QueueException("Operation '$id' in terminal state $state cannot transition")
    class SelectionExpired(id: String) : QueueException("Selection window expired for operation '$id'")
}
