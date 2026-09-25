#!/usr/bin/env python3
"""Read every scenario's k6 summary and the fixture's /metrics snapshot,
emit a per-scenario one-line summary so the baseline report can quote it
verbatim. NOT a Go binary; stdlib only.
"""
import glob, json, os, sys

WANT_METRICS = [
    "http_reqs",
    "http_req_duration",
    "http_req_duration{expected_response:true}",
    "http_req_failed",
]

def metric_block(metrics, key):
    v = metrics.get(key)
    if not v:
        return None
    keep = ("avg", "min", "med", "max", "p(90)", "p(95)", "p(99)", "count", "rate", "value", "passes", "fails")
    out = {}
    for k, val in v.items():
        if k in keep:
            out[k] = round(val, 3) if isinstance(val, float) else val
    return out

def main():
    root = sys.argv[1] if len(sys.argv) > 1 else "loadmodel/reports"
    for sdir in sorted(glob.glob(os.path.join(root, "*"))):
        if not os.path.isdir(sdir):
            continue
        name = os.path.basename(sdir)
        summary_path = os.path.join(sdir, "summary.json")
        if not os.path.exists(summary_path):
            continue
        d = json.load(open(summary_path))
        metrics = d.get("metrics", {})
        print(f"=== {name} ===")
        for k in WANT_METRICS:
            v = metric_block(metrics, k)
            if v:
                print(f"  {k}: {v}")
        checks = d.get("root_group", {}).get("checks", {})
        for cname, c in checks.items():
            print(f"  check {cname}: passes={c['passes']} fails={c['fails']}")
        stats_path = os.path.join(sdir, "loadtestd-stats.json")
        if os.path.exists(stats_path):
            print(f"  fixture_stats: {json.load(open(stats_path))}")
        print()

if __name__ == "__main__":
    main()
