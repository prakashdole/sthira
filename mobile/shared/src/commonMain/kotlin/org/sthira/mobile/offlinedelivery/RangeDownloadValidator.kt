package org.sthira.mobile.offlinedelivery

/**
 * Validates HTTP Range resumption and protects local storage against traversal and overflow.
 * Conforms to P5 resumable download specification.
 */
object RangeDownloadValidator {

    data class RangeHeader(
        val startByte: Long,
        val endByte: Long,
        val totalBytes: Long
    )

    sealed class RangeValidationResult {
        data class Valid(val range: RangeHeader) : RangeValidationResult()
        data class Invalid(val error: String) : RangeValidationResult()
    }

    /**
     * Parses and validates Content-Range header: "bytes 1000-1999/5000"
     */
    fun validateContentRange(
        contentRangeHeader: String?,
        contentLengthHeader: Long,
        expectedTotalBytes: Long
    ): RangeValidationResult {
        if (contentRangeHeader.isNullOrBlank()) {
            return RangeValidationResult.Invalid("Missing Content-Range header on 206 Partial Content response")
        }

        val prefix = "bytes "
        if (!contentRangeHeader.startsWith(prefix)) {
            return RangeValidationResult.Invalid("Malformed Content-Range: must begin with '$prefix'")
        }

        val rangePart = contentRangeHeader.removePrefix(prefix).trim()
        val parts = rangePart.split("/")
        if (parts.size != 2) {
            return RangeValidationResult.Invalid("Malformed Content-Range format; expected 'bytes start-end/total'")
        }

        val total = parts[1].toLongOrNull() ?: return RangeValidationResult.Invalid("Invalid total bytes in Content-Range")
        if (total != expectedTotalBytes) {
            return RangeValidationResult.Invalid("Total bytes ($total) does not match expected size ($expectedTotalBytes)")
        }

        val byteIndices = parts[0].split("-")
        if (byteIndices.size != 2) {
            return RangeValidationResult.Invalid("Invalid range indices in Content-Range")
        }

        val start = byteIndices[0].toLongOrNull() ?: return RangeValidationResult.Invalid("Invalid start byte")
        val end = byteIndices[1].toLongOrNull() ?: return RangeValidationResult.Invalid("Invalid end byte")

        if (start < 0 || end < start || end >= total) {
            return RangeValidationResult.Invalid("Inconsistent byte bounds: $start-$end for total $total")
        }

        val expectedLength = end - start + 1
        if (contentLengthHeader != expectedLength) {
            return RangeValidationResult.Invalid("Content-Length ($contentLengthHeader) does not match range span ($expectedLength)")
        }

        return RangeValidationResult.Valid(RangeHeader(start, end, total))
    }

    /**
     * Sanitizes file paths to prevent directory traversal attacks (e.g. "../../../etc/passwd").
     */
    fun sanitizeStorageFilename(rawFilename: String): String {
        val sanitized = rawFilename.replace("\\", "/")
            .split("/")
            .filter { it.isNotBlank() && it != "." && it != ".." }
            .joinToString("_")

        if (sanitized.isBlank() || sanitized.contains("..")) {
            throw IllegalArgumentException("Unsafe or path-traversing resource filename: '$rawFilename'")
        }
        return sanitized
    }
}
