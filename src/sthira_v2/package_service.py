"""Authenticated, versioned operational-package lifecycle for synthetic use."""

from __future__ import annotations

from dataclasses import dataclass
from datetime import datetime, timezone
from threading import RLock
from typing import Any, Mapping

from .operational_package import PackageValidation, OperationalPackageError, validate_operational_package


class PackageAuthorizationError(PermissionError):
    """Operator lacks the jurisdiction or action permission."""


@dataclass(frozen=True)
class PackageRecord:
    package: dict[str, Any]
    validation: PackageValidation
    checksum: str
    state: str
    published_at: datetime | None = None
    cancelled_at: datetime | None = None


class OperationalPackageService:
    def __init__(self) -> None:
        self._records: dict[str, PackageRecord] = {}
        self._active_by_jurisdiction: dict[str, str] = {}
        self._lock = RLock()

    def preview(self, package: Mapping[str, Any], *, jurisdiction: str) -> PackageRecord:
        validation = validate_operational_package(package, expected_jurisdiction=jurisdiction)
        package_copy = __import__("copy").deepcopy(package)
        record = PackageRecord(package_copy, validation, validation.checksum_sha256, "PREVIEW")
        with self._lock:
            existing = self._records.get(validation.package_id)
            if existing is not None and existing.validation.version != validation.version:
                raise OperationalPackageError("package identifier cannot change version")
            self._records[validation.package_id] = record
        return record

    def publish(self, package_id: str, *, operator_jurisdiction: str, authenticated: bool) -> PackageRecord:
        if not authenticated:
            raise PackageAuthorizationError("operator authentication required")
        with self._lock:
            record = self._records.get(package_id)
            if record is None:
                raise OperationalPackageError("package has not been previewed")
            if record.package.get("provenance", {}).get("jurisdiction") != operator_jurisdiction:
                raise PackageAuthorizationError("operator jurisdiction does not match package")
            prior_id = self._active_by_jurisdiction.get(operator_jurisdiction)
            if prior_id and prior_id != package_id:
                prior = self._records[prior_id]
                if record.validation.version <= prior.validation.version:
                    raise OperationalPackageError("package version conflict")
                self._records[prior_id] = PackageRecord(prior.package, prior.validation, prior.checksum, "SUPERSEDED", prior.published_at)
            published = PackageRecord(record.package, record.validation, record.checksum, "PUBLISHED", datetime.now(timezone.utc))
            self._records[package_id] = published
            self._active_by_jurisdiction[operator_jurisdiction] = package_id
            return published

    def active(self, *, jurisdiction: str) -> PackageRecord | None:
        package_id = self._active_by_jurisdiction.get(jurisdiction)
        with self._lock:
            return self._records.get(package_id) if package_id else None

    def cancel(self, package_id: str, *, operator_jurisdiction: str, authenticated: bool) -> PackageRecord:
        record = self._authorized_record(package_id, operator_jurisdiction, authenticated)
        cancelled = PackageRecord(record.package, record.validation, record.checksum, "CANCELLED", record.published_at)
        with self._lock:
            cancelled = PackageRecord(record.package, record.validation, record.checksum, "CANCELLED", record.published_at, datetime.now(timezone.utc))
            self._records[package_id] = cancelled
            if self._active_by_jurisdiction.get(operator_jurisdiction) == package_id:
                del self._active_by_jurisdiction[operator_jurisdiction]
            return cancelled

    def rollback(self, *, jurisdiction: str, package_id: str, authenticated: bool) -> PackageRecord:
        if not authenticated:
            raise PackageAuthorizationError("operator authentication required")
        record = self._records.get(package_id)
        if record is None or record.package.get("provenance", {}).get("jurisdiction") != jurisdiction:
            raise PackageAuthorizationError("package is outside operator jurisdiction")
        if record.state not in {"SUPERSEDED", "PUBLISHED"}:
            raise OperationalPackageError("only a valid published package can be restored")
        restored = PackageRecord(record.package, record.validation, record.checksum, "PUBLISHED", datetime.now(timezone.utc))
        current_id = self._active_by_jurisdiction.get(jurisdiction)
        if current_id and current_id != package_id:
            current = self._records[current_id]
            self._records[current_id] = PackageRecord(current.package, current.validation, current.checksum, "SUPERSEDED", current.published_at)
        with self._lock:
            self._records[package_id] = restored
            self._active_by_jurisdiction[jurisdiction] = package_id
            return restored

    def _authorized_record(self, package_id: str, jurisdiction: str, authenticated: bool) -> PackageRecord:
        if not authenticated:
            raise PackageAuthorizationError("operator authentication required")
        record = self._records.get(package_id)
        if record is None or record.package.get("provenance", {}).get("jurisdiction") != jurisdiction:
            raise PackageAuthorizationError("package is outside operator jurisdiction")
        return record
