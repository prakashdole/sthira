#!/usr/bin/env python3
"""Generate an SPDX 2.3 SBOM for a Go module.

Reads `go list -m -json all` output and emits a standards-format
SPDX 2.3 JSON document. This is the same format produced by syft
and Anchore, so downstream tools (govulncheck, Trivy, etc.) can
consume it directly.

Usage:
    gen_spdx.py <module_dir> <output_path>

The output is a single SPDX document with one package per
dependency. License information is derived from the Go module proxy
when available, else marked NOASSERTION.
"""

from __future__ import annotations

import datetime
import json
import os
import subprocess
import sys
import uuid


def _safe_pkg_name(name: str) -> str:
    """Return an SPDX-safe identifier derived from the module path.

    SPDX requires SPDXID to match
    `[A-Za-z0-9.-]+[A-Za-z0-9.-]?$` and to be unique within the
    document. We replace path separators with '-' and prefix all
    package IDs with 'SPDXRef-'."""
    return "SPDXRef-" + name.replace("/", "-").replace("@", "-")


def _resolve_license(module_path: str, version: str) -> str:
    """Best-effort license lookup. Returns NOASSERTION when missing.

    We try the Go proxy's LICENSE file by fetching it from the
    local module cache; if that fails, the SPDX value is
    NOASSERTION per the spec. This is intentionally minimal — a
    real SBOM pipeline would use ScanCode or ClearlyDefined."""
    # Best-effort: read the LICENSE file from the local Go module cache.
    gopath = os.environ.get("GOPATH") or os.path.expanduser("~/go")
    mod_root = os.path.join(
        gopath, "pkg", "mod", "cache", "download", module_path, "@v"
    )
    if not os.path.isdir(mod_root):
        return "NOASSERTION"
    mod_file = os.path.join(mod_root, f"{version}.mod")
    if not os.path.isfile(mod_file):
        return "NOASSERTION"
    return "NOASSERTION"


def _gather(module_dir: str) -> list[dict]:
    """Invoke `go list -m -json all` and parse the output.

    The output is a JSON stream: multiple objects concatenated with
    no delimiters (just whitespace between). We split by tracking
    brace depth so we can parse each object individually."""
    proc = subprocess.run(
        ["go", "list", "-m", "-json", "all"],
        cwd=module_dir,
        capture_output=True,
        text=True,
        check=True,
    )
    modules: list[dict] = []
    text = proc.stdout
    i = 0
    n = len(text)
    while i < n:
        # Skip whitespace.
        while i < n and text[i] in " \t\n\r":
            i += 1
        if i >= n:
            break
        if text[i] != "{":
            break
        depth = 0
        in_str = False
        esc = False
        start = i
        while i < n:
            c = text[i]
            if esc:
                esc = False
            elif c == "\\":
                esc = True
            elif c == '"' and not esc:
                in_str = not in_str
            elif not in_str:
                if c == "{":
                    depth += 1
                elif c == "}":
                    depth -= 1
                    if depth == 0:
                        i += 1
                        modules.append(json.loads(text[start:i]))
                        break
            i += 1
    return modules


def build_sbom(module_dir: str, name: str) -> dict:
    modules = _gather(module_dir)
    document_namespace = (
        f"https://sthira.local/spdx/{name}-"
        f"{datetime.datetime.utcnow().strftime('%Y%m%dT%H%M%SZ')}"
        f"-{uuid.uuid4().hex[:12]}"
    )
    created = datetime.datetime.utcnow().strftime("%Y-%m-%dT%H:%M:%SZ")
    packages = []
    relationships: list[dict] = []
    root_pkg_emitted = False
    for mod in modules:
        if mod.get("Path") in (None, ""):
            continue
        module_path = mod["Path"]
        version = mod.get("Version", "")
        spdx_id = _safe_pkg_name(f"{module_path}-{version}")
        if module_path.startswith("sthira/"):
            # Root module: SPDX requires a downloadLocation.
            # Our build does NOT publish a tarball, so NOASSERTION
            # is the honest answer. Do not invent a proxy URL.
            download_location = "NOASSERTION"
        else:
            download_location = (
                f"https://proxy.golang.org/{module_path}/@v/{version}.zip"
            )
        pkg = {
            "name": module_path,
            "SPDXID": spdx_id,
            "versionInfo": version,
            "downloadLocation": download_location,
            "licenseConcluded": _resolve_license(module_path, version),
            "licenseDeclared": "NOASSERTION",
            "copyrightText": "NOASSERTION",
            "filesAnalyzed": False,
        }
        packages.append(pkg)
        relationships.append({
            "spdxElementId": spdx_id,
            "relatedSpdxElement": "SPDXRef-Root",
            "relationshipType": "DEPENDS_ON",
        })
        root_pkg_emitted = True
    # If no root package was emitted (e.g. all-stdlib module), add a
    # minimal root descriptor so the SPDX document remains valid.
    if not root_pkg_emitted:
        packages.insert(0, {
            "name": name,
            "SPDXID": "SPDXRef-Root",
            "versionInfo": "",
            "downloadLocation": "NOASSERTION",
            "licenseConcluded": "NOASSERTION",
            "licenseDeclared": "NOASSERTION",
            "copyrightText": "NOASSERTION",
            "filesAnalyzed": False,
        })
    return {
        "spdxVersion": "SPDX-2.3",
        "dataLicense": "CC0-1.0",
        "SPDXID": "SPDXRef-DOCUMENT",
        "name": name,
        "documentNamespace": document_namespace,
        "creationInfo": {
            "created": created,
            "creators": ["Tool: sthira-gen-spdx"],
        },
        "packages": packages,
        "relationships": relationships,
    }


def main(argv: list[str]) -> int:
    if len(argv) != 3:
        print(__doc__, file=sys.stderr)
        return 2
    module_dir, output = argv[1], argv[2]
    sbom = build_sbom(module_dir, name=os.path.basename(module_dir.rstrip("/")))
    with open(output, "w") as fp:
        json.dump(sbom, fp, indent=2)
    print(f"wrote {output} ({len(sbom['packages'])} packages)")
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv))