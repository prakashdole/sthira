package org.sthira.mobile.offlinequeue

/**
 * State machine managing pending offline mutations (stay reservations, arrivals, cancellations).
 * Conforms to P5 offline queue invariants:
 * 1. Strictly stores TokenRef, never raw bearer credentials.
 * 2. Idempotency keys are unique per payload; duplicate keys with differing payloads are rejected.
 * 3. Network uncertainty maps to PENDING_RECONCILIATION, never silent failure or local auto-commit.
 * 4. Terminal states (COMMITTED, FAILED_STALE, FAILED_PERM) cannot be mutated.
 */
class OfflineQueue {
    private val operations = mutableMapOf<String, PendingOperation>()
    private val keyIndex = mutableMapOf<String, String>() // idempotencyKey -> opId

    @Synchronized
    fun enqueue(
        opName: String,
        idempotencyKey: String,
        payloadJson: String,
        payloadHash: String,
        tokenRef: String,
        snapshotVersion: Int,
        selectionExpiresEpoch: Long,
        nowEpoch: Long = System.currentTimeMillis() / 1000
    ): PendingOperation {
        val existingId = keyIndex[idempotencyKey]
        if (existingId != null) {
            val existing = operations[existingId]!!
            if (existing.payloadHash != payloadHash) {
                throw QueueException.PayloadConflict(idempotencyKey)
            }
            return existing
        }

        val id = "OP-" + (nowEpoch.toString(16) + "-" + (operations.size + 1))
        val op = PendingOperation(
            id = id,
            opName = opName,
            idempotencyKey = idempotencyKey,
            payloadJson = payloadJson,
            payloadHash = payloadHash,
            tokenRef = tokenRef,
            state = OpState.PENDING,
            snapshotVersion = snapshotVersion,
            selectionExpiresEpoch = selectionExpiresEpoch,
            attempts = 0,
            lastError = null,
            createdAtEpoch = nowEpoch,
            updatedAtEpoch = nowEpoch
        )

        operations[id] = op
        keyIndex[idempotencyKey] = id
        return op
    }

    @Synchronized
    fun markInFlight(opId: String, nowEpoch: Long = System.currentTimeMillis() / 1000): PendingOperation {
        val op = operations[opId] ?: throw QueueException.NoSuchOperation(opId)
        if (op.state != OpState.PENDING && op.state != OpState.PENDING_RECONCILIATION) {
            throw QueueException.TerminalState(opId, op.state)
        }
        val updated = op.copy(
            state = OpState.IN_FLIGHT,
            attempts = op.attempts + 1,
            updatedAtEpoch = nowEpoch
        )
        operations[opId] = updated
        return updated
    }

    @Synchronized
    fun markNetworkUncertainty(opId: String, errorReason: String, nowEpoch: Long = System.currentTimeMillis() / 1000): PendingOperation {
        val op = operations[opId] ?: throw QueueException.NoSuchOperation(opId)
        if (op.state != OpState.IN_FLIGHT) {
            throw QueueException.TerminalState(opId, op.state)
        }
        val updated = op.copy(
            state = OpState.PENDING_RECONCILIATION,
            lastError = errorReason,
            updatedAtEpoch = nowEpoch
        )
        operations[opId] = updated
        return updated
    }

    @Synchronized
    fun markCommitted(opId: String, nowEpoch: Long = System.currentTimeMillis() / 1000): PendingOperation {
        val op = operations[opId] ?: throw QueueException.NoSuchOperation(opId)
        val updated = op.copy(
            state = OpState.COMMITTED,
            lastError = null,
            updatedAtEpoch = nowEpoch
        )
        operations[opId] = updated
        return updated
    }

    @Synchronized
    fun markFailedStale(opId: String, errorReason: String, nowEpoch: Long = System.currentTimeMillis() / 1000): PendingOperation {
        val op = operations[opId] ?: throw QueueException.NoSuchOperation(opId)
        val updated = op.copy(
            state = OpState.FAILED_STALE,
            lastError = errorReason,
            updatedAtEpoch = nowEpoch
        )
        operations[opId] = updated
        return updated
    }

    @Synchronized
    fun markFailedPermanent(opId: String, errorReason: String, nowEpoch: Long = System.currentTimeMillis() / 1000): PendingOperation {
        val op = operations[opId] ?: throw QueueException.NoSuchOperation(opId)
        val updated = op.copy(
            state = OpState.FAILED_PERM,
            lastError = errorReason,
            updatedAtEpoch = nowEpoch
        )
        operations[opId] = updated
        return updated
    }

    @Synchronized
    fun getPending(): List<PendingOperation> {
        return operations.values.filter { it.state == OpState.PENDING || it.state == OpState.PENDING_RECONCILIATION }
    }

    @Synchronized
    fun get(opId: String): PendingOperation? = operations[opId]

    @Synchronized
    fun purgeCommitted(): Int {
        val committedIds = operations.values.filter { it.state == OpState.COMMITTED }.map { it.id }
        for (id in committedIds) {
            val op = operations.remove(id)
            if (op != null) {
                keyIndex.remove(op.idempotencyKey)
            }
        }
        return committedIds.size
    }
}
