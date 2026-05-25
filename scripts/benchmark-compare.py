#!/usr/bin/env python3
"""benchmark-compare.py — compare slopgate benchmark results across time or PRs.

Usage:
  # Compare all benchmarks for a repo
  benchmark-compare.py slopgate

  # Compare benchmarks for specific PRs
  benchmark-compare.py slopgate 16 20

  # Compare two specific benchmark files
  benchmark-compare.py --file bench1.json bench2.json

  # Show trend over time for all repos
  benchmark-compare.py --trend

Outputs a markdown table suitable for PR descriptions or reports.
"""

from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path
from typing import Any

BENCHMARK_DIR = Path("/srv/storage/shared/slopgate-benchmarks")
TIER_ORDER = ("all_rules", "block_warn_only", "benchmark_eligible")
TIER_LABELS = {
    "all_rules": "All Rules",
    "block_warn_only": "Block/Warn",
    "benchmark_eligible": "Eligible",
}


def load_benchmark(path: Path) -> dict[str, Any] | None:
    """Load a single benchmark JSON file."""
    try:
        with open(path) as f:
            data = json.load(f)
        if "slopgate" not in data:
            return None
        return data
    except (json.JSONDecodeError, KeyError):
        # Skip malformed benchmark files silently
        return None


def collect_benchmarks(repo: str | None = None, pr_nums: list[int] | None = None) -> list[dict[str, Any]]:
    """Collect benchmarks from the shared directory."""
    results = []
    if not BENCHMARK_DIR.exists():
        return results

    for f in sorted(BENCHMARK_DIR.glob("*.json")):
        data = load_benchmark(f)
        if data is None:
            continue
        if repo and data.get("repo") != repo:
            continue
        if pr_nums and data.get("pr") not in pr_nums:
            continue
        data["_source"] = str(f)
        results.append(data)

    return results


def _legacy_all_rules_tier(data: dict[str, Any]) -> dict[str, Any]:
    return {
        "slopgate": data.get("slopgate", {}),
        "scores": data.get("scores", {}),
        "streams": data.get("streams", {}),
    }


def get_tier(data: dict[str, Any], tier_name: str) -> dict[str, Any] | None:
    tiers = data.get("benchmark_tiers")
    if isinstance(tiers, dict):
        tier = tiers.get(tier_name)
        if isinstance(tier, dict):
            return tier
    if tier_name == "all_rules":
        return _legacy_all_rules_tier(data)
    return None


def get_stream_total(data: dict[str, Any], stream_name: str) -> int:
    legacy_map = {
        "coderabbit_all": "coderabbit",
        "coderabbit_actionable": "coderabbit_actionable",
        "sentry": "sentry",
        "gemini": "gemini",
        "deepsource": "deepsource",
        "qodo": "qodo",
        "actionable_plus_sentry": "actionable_plus_sentry",
    }
    streams = data.get("streams", {})
    stream = streams.get(stream_name, {}) if isinstance(streams, dict) else {}
    total = stream.get("total")
    if isinstance(total, int):
        return total
    legacy_key = legacy_map.get(stream_name)
    if not legacy_key:
        return 0
    legacy = data.get(legacy_key, {})
    if not isinstance(legacy, dict):
        return 0
    return int(legacy.get("total", 0))


def tier_metrics(data: dict[str, Any], tier_name: str) -> dict[str, Any] | None:
    tier = get_tier(data, tier_name)
    if tier is None:
        return None
    scores = tier.get("scores", {})
    return {
        "slopgate_total": int(tier.get("slopgate", {}).get("total", 0)),
        "review_total": get_stream_total(data, "coderabbit_all"),
        "overlap_all": int(scores.get("overlap_all", 0)),
        "coverage_all_pct": float(scores.get("coverage_all_pct", 0)),
        "precision_proxy_all_pct": float(scores.get("precision_proxy_all_pct", 0)),
    }


def format_tier_summary(data: dict[str, Any], tier_name: str) -> str:
    metrics = tier_metrics(data, tier_name)
    if metrics is None:
        return "-"
    return (
        f"SG {metrics['slopgate_total']} "
        f"Ov {metrics['overlap_all']} "
        f"Cov {metrics['coverage_all_pct']:.1f}% "
        f"Prec {metrics['precision_proxy_all_pct']:.1f}%"
    )


def aggregate_tier_metrics(benchmarks: list[dict[str, Any]], tier_name: str) -> dict[str, Any] | None:
    available = [tier_metrics(b, tier_name) for b in benchmarks]
    available = [metrics for metrics in available if metrics is not None]
    if not available:
        return None
    slopgate_total = sum(metrics["slopgate_total"] for metrics in available)
    review_total = sum(metrics["review_total"] for metrics in available)
    overlap_all = sum(metrics["overlap_all"] for metrics in available)
    return {
        "slopgate_total": slopgate_total,
        "overlap_all": overlap_all,
        "coverage_all_pct": round((overlap_all / review_total * 100) if review_total else 0, 1),
        "precision_proxy_all_pct": round((overlap_all / slopgate_total * 100) if slopgate_total else 0, 1),
    }


def format_table(benchmarks: list[dict[str, Any]], title: str = "") -> str:
    """Format benchmarks as a markdown table."""
    if not benchmarks:
        return f"## {title}\n\nNo benchmarks found.\n"

    lines = [f"## {title}\n" if title else ""]
    lines.append("| Repo | PR | CR | CR Act | Sentry | Gemini | DeepSrc | Qodo | Combined | All Rules | Block/Warn | Eligible |")
    lines.append("|------|-----|-----|--------|--------|--------|---------|------|----------|-----------|------------|----------|")

    for b in benchmarks:
        repo = b.get("repo", "?")
        pr = b.get("pr", "?")
        cr = get_stream_total(b, "coderabbit_all")
        cr_act = get_stream_total(b, "coderabbit_actionable")
        sentry = get_stream_total(b, "sentry")
        gemini = get_stream_total(b, "gemini")
        deepsource = get_stream_total(b, "deepsource")
        qodo = get_stream_total(b, "qodo")
        combined = get_stream_total(b, "actionable_plus_sentry")

        lines.append(
            f"| {repo} | #{pr} | {cr} | {cr_act} | {sentry} | {gemini} | {deepsource} | {qodo} | {combined} | "
            f"{format_tier_summary(b, 'all_rules')} | "
            f"{format_tier_summary(b, 'block_warn_only')} | "
            f"{format_tier_summary(b, 'benchmark_eligible')} |"
        )

    total_cr = sum(get_stream_total(b, "coderabbit_all") for b in benchmarks)
    total_act = sum(get_stream_total(b, "coderabbit_actionable") for b in benchmarks)
    total_sentry = sum(get_stream_total(b, "sentry") for b in benchmarks)
    total_gemini = sum(get_stream_total(b, "gemini") for b in benchmarks)
    total_deepsource = sum(get_stream_total(b, "deepsource") for b in benchmarks)
    total_qodo = sum(get_stream_total(b, "qodo") for b in benchmarks)
    total_combined = sum(get_stream_total(b, "actionable_plus_sentry") for b in benchmarks)

    tier_totals = {tier_name: aggregate_tier_metrics(benchmarks, tier_name) for tier_name in TIER_ORDER}

    def total_cell(tier_name: str) -> str:
        metrics = tier_totals[tier_name]
        if metrics is None:
            return "-"
        return (
            f"SG {metrics['slopgate_total']} "
            f"Ov {metrics['overlap_all']} "
            f"Cov {metrics['coverage_all_pct']:.1f}% "
            f"Prec {metrics['precision_proxy_all_pct']:.1f}%"
        )

    lines.append(
        f"| **Total** | | **{total_cr}** | **{total_act}** | **{total_sentry}** | **{total_gemini}** | **{total_deepsource}** | **{total_qodo}** | **{total_combined}** | "
        f"**{total_cell('all_rules')}** | "
        f"**{total_cell('block_warn_only')}** | "
        f"**{total_cell('benchmark_eligible')}** |"
    )
    lines.append("")

    return "\n".join(lines)


def show_trend() -> str:
    """Show benchmark trends over time."""
    all_benchmarks = collect_benchmarks()
    if not all_benchmarks:
        return "No benchmarks found."

    # Group by repo
    by_repo: dict[str, list[dict[str, Any]]] = {}
    for b in all_benchmarks:
        repo = b.get("repo", "unknown")
        by_repo.setdefault(repo, []).append(b)

    lines = ["# Slopgate Benchmark Trends\n"]

    for repo, benchmarks in sorted(by_repo.items()):
        # Sort by PR number
        benchmarks.sort(key=lambda x: x.get("pr", 0))
        lines.append(format_table(benchmarks, title=f"Repo: {repo}"))

    # Overall summary
    lines.append(format_table(all_benchmarks, title="All Benchmarks"))

    return "\n".join(lines)


def compare_files(file1: str, file2: str) -> str:
    """Compare two specific benchmark files."""
    b1 = load_benchmark(Path(file1))
    b2 = load_benchmark(Path(file2))

    if not b1 or not b2:
        return "Error: Could not load one or both benchmark files."

    lines = ["# Benchmark Comparison\n"]
    lines.append("| Metric | Before | After | Change |")
    lines.append("|--------|--------|-------|--------|")

    metrics: list[tuple[str, Any, Any]] = [
        ("CodeRabbit Total", get_stream_total(b1, "coderabbit_all"), get_stream_total(b2, "coderabbit_all")),
        ("CodeRabbit Actionable", get_stream_total(b1, "coderabbit_actionable"), get_stream_total(b2, "coderabbit_actionable")),
        ("Sentry Total", get_stream_total(b1, "sentry"), get_stream_total(b2, "sentry")),
        ("Gemini Total", get_stream_total(b1, "gemini"), get_stream_total(b2, "gemini")),
        ("DeepSource Total", get_stream_total(b1, "deepsource"), get_stream_total(b2, "deepsource")),
        ("Qodo Total", get_stream_total(b1, "qodo"), get_stream_total(b2, "qodo")),
        ("Combined (all bots)", get_stream_total(b1, "actionable_plus_sentry"), get_stream_total(b2, "actionable_plus_sentry")),
        ("Overlap (actionable)", _get_nested(b1, "scores.overlap_actionable", 0), _get_nested(b2, "scores.overlap_actionable", 0)),
        (
            "Overlap (combined)",
            _get_nested(b1, "scores.overlap_actionable_plus_sentry", 0),
            _get_nested(b2, "scores.overlap_actionable_plus_sentry", 0),
        ),
    ]
    for tier_name in TIER_ORDER:
        metrics.extend(
            [
                (f"{TIER_LABELS[tier_name]} Slopgate", _tier_metric_value(b1, tier_name, "slopgate_total"), _tier_metric_value(b2, tier_name, "slopgate_total")),
                (f"{TIER_LABELS[tier_name]} Overlap", _tier_metric_value(b1, tier_name, "overlap_all"), _tier_metric_value(b2, tier_name, "overlap_all")),
                (
                    f"{TIER_LABELS[tier_name]} Coverage %",
                    _tier_metric_value(b1, tier_name, "coverage_all_pct"),
                    _tier_metric_value(b2, tier_name, "coverage_all_pct"),
                ),
                (
                    f"{TIER_LABELS[tier_name]} Precision %",
                    _tier_metric_value(b1, tier_name, "precision_proxy_all_pct"),
                    _tier_metric_value(b2, tier_name, "precision_proxy_all_pct"),
                ),
            ]
        )

    for label, v1, v2 in metrics:
        if isinstance(v1, (int, float)) and isinstance(v2, (int, float)):
            diff = v2 - v1
            sign = "+" if diff > 0 else ""
            lines.append(f"| {label} | {v1} | {v2} | {sign}{diff} |")
        else:
            lines.append(f"| {label} | {v1} | {v2} | - |")

    lines.append("")
    return "\n".join(lines)


def _get_nested(data: dict[str, Any], path: str, default: Any = None) -> Any:
    """Get a nested value from a dict using dot notation."""
    keys = path.split(".")
    current = data
    for key in keys:
        if isinstance(current, dict):
            current = current.get(key, default)
        else:
            return default
    return current


def _tier_metric_value(data: dict[str, Any], tier_name: str, key: str) -> Any:
    metrics = tier_metrics(data, tier_name)
    if metrics is None:
        return "-"
    return metrics.get(key, "-")


def main() -> int:
    parser = argparse.ArgumentParser(description="Compare slopgate benchmark results")
    parser.add_argument("repo", nargs="?", help="Repo name to filter by")
    parser.add_argument("pr_nums", nargs="*", type=int, help="PR numbers to filter by")
    parser.add_argument("--file", nargs=2, metavar=("FILE1", "FILE2"), help="Compare two specific benchmark files")
    parser.add_argument("--trend", action="store_true", help="Show trend over time for all repos")
    parser.add_argument("--output", "-o", help="Write output to file instead of stdout")

    args = parser.parse_args()

    if args.file and len(args.file) >= 2:
        output = compare_files(args.file[0], args.file[1])
    elif args.trend:
        output = show_trend()
    else:
        benchmarks = collect_benchmarks(args.repo, args.pr_nums if args.pr_nums else None)
        title = "Slopgate Benchmarks"
        if args.repo:
            title += f" — {args.repo}"
        if args.pr_nums:
            title += f" (PRs: {', '.join(f'#{p}' for p in args.pr_nums)})"
        output = format_table(benchmarks, title=title)

    if args.output:
        Path(args.output).write_text(output)
        print(f"Written to {args.output}", file=sys.stderr)
    else:
        print(output)

    return 0


if __name__ == "__main__":
    sys.exit(main())
