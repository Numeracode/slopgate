#!/usr/bin/env python3
"""Build a rule-pruning scorecard from archived Slopgate benchmark JSON.

The scorecard is intentionally rule-centric: for each sampled PR it records
which rules fired, what they reported, and whether the finding corresponded to
CodeRabbit or Sentry review feedback. Use it to decide which rules should stay
in the default gate, move to advisory/quarantine, or be disabled.
"""

import argparse
import csv
import json
import os
import sys
from collections import Counter, defaultdict
from pathlib import Path
from typing import Any

BENCHMARK_DIR = Path(os.environ.get("SLOPGATE_BENCHMARK_DIR", "./benchmark-results"))
PHASE_RANK = {
    "postmerge": 4,
    "prepush": 3,
    "precommit": 2,
    "finish": 1,
    "unknown": 0,
}
STREAMS = ("coderabbit_all", "coderabbit_actionable", "sentry", "actionable_plus_sentry")


def load_benchmark(path: Path) -> dict[str, Any] | None:
    data: dict[str, Any] | None = None
    try:
        data = json.loads(path.read_text())
    except (OSError, json.JSONDecodeError):
        pass
    if not isinstance(data, dict) or "slopgate" not in data or "pr" not in data or "repo" not in data:
        return None
    data["_source"] = str(path)
    data["_mtime"] = path.stat().st_mtime
    data["_phase"] = artifact_phase(path)
    return data


def artifact_phase(path: Path) -> str:
    stem = path.stem
    for phase in PHASE_RANK:
        if stem.endswith(f"-{phase}"):
            return phase
    return "unknown"


def repo_matches(repo_value: str, filters: set[str]) -> bool:
    if not filters:
        return True
    short = repo_value.rsplit("/", 1)[-1]
    return repo_value in filters or short in filters


def parse_int(value: object, default: int = 0) -> int:
    try:
        return int(value or default)
    except (TypeError, ValueError):
        return default


def collect_sample(
    benchmark_dir: Path,
    *,
    repos: set[str],
    prs: set[int],
    limit: int,
) -> list[dict[str, Any]]:
    best_by_pr: dict[tuple[str, int], dict[str, Any]] = {}
    for path in benchmark_dir.glob("*.json"):
        data = load_benchmark(path)
        if data is None:
            continue
        repo = str(data.get("repo", ""))
        pr = parse_int(data.get("pr"), -1)
        if pr < 0:
            continue
        if not repo_matches(repo, repos):
            continue
        if prs and pr not in prs:
            continue
        key = (repo, pr)
        current = best_by_pr.get(key)
        if current is None or benchmark_sort_key(data) > benchmark_sort_key(current):
            best_by_pr[key] = data

    sample = sorted(best_by_pr.values(), key=lambda d: float(d["_mtime"]), reverse=True)
    if limit > 0:
        sample = sample[:limit]
    return sorted(sample, key=lambda d: (str(d.get("repo", "")), int(d.get("pr", 0))))


def benchmark_sort_key(data: dict[str, Any]) -> tuple[int, float]:
    return (PHASE_RANK.get(str(data.get("_phase", "unknown")), 0), float(data.get("_mtime", 0)))


def stream_details(data: dict[str, Any], stream: str, key: str) -> list[dict[str, Any]]:
    comparison_streams = data.get("comparison_streams")
    if isinstance(comparison_streams, dict):
        stream_data = comparison_streams.get(stream)
        if isinstance(stream_data, dict):
            details = stream_data.get(key)
            if isinstance(details, list):
                return [item for item in details if isinstance(item, dict)]
    if stream == "coderabbit_all":
        details = data.get(key)
        if isinstance(details, list):
            return [item for item in details if isinstance(item, dict)]
    return []


def build_rows(sample: list[dict[str, Any]]) -> tuple[list[dict[str, Any]], list[dict[str, Any]], list[dict[str, Any]]]:
    pr_rows: list[dict[str, Any]] = []
    review_miss_rows: list[dict[str, Any]] = []
    rule_stats: dict[str, dict[str, Any]] = defaultdict(new_rule_stats)

    for data in sample:
        repo = str(data.get("repo", ""))
        pr = int(data.get("pr", 0))
        source = str(data.get("_source", ""))
        phase = str(data.get("_phase", "unknown"))

        seen_rule_in_pr: set[str] = set()
        for item in stream_details(data, "coderabbit_all", "sg_only_details"):
            rule_id = str(item.get("rule_id", "?"))
            row = finding_row(repo, pr, phase, source, item, "slopgate_only", "", "")
            pr_rows.append(row)
            update_rule_stats(rule_stats[rule_id], row, seen_rule_in_pr)

        for item in stream_details(data, "coderabbit_all", "overlap_details"):
            rule_id = str(item.get("rule_id", "?"))
            review_source = str(item.get("review_source", "coderabbit_all"))
            review_summary = str(item.get("review_summary", ""))
            row = finding_row(repo, pr, phase, source, item, "overlap", review_source, review_summary)
            pr_rows.append(row)
            update_rule_stats(rule_stats[rule_id], row, seen_rule_in_pr)

        actionable_rules = {
            (
                str(item.get("file") or item.get("path") or ""),
                parse_int(item.get("line")),
                str(item.get("rule_id", "?")),
            )
            for item in stream_details(data, "actionable_plus_sentry", "overlap_details")
        }
        for row in pr_rows:
            key = (row["file"], parse_int(row["line"]), row["rule_id"])
            if row["repo"] == repo and int(row["pr"]) == pr and key in actionable_rules:
                if row["match_status"] != "overlap":
                    continue
                row["actionable_overlap"] = "yes"
                rule_stats[row["rule_id"]]["actionable_overlaps"] += 1

        for stream in STREAMS:
            for item in stream_details(data, stream, "review_only_details"):
                review_miss_rows.append(review_miss_row(repo, pr, phase, source, stream, item))

    rule_rows = [rule_row(rule_id, stats) for rule_id, stats in sorted(rule_stats.items())]
    rule_rows.sort(key=lambda row: (action_rank(row["suggested_action"]), -int(row["slopgate_only"]), row["rule_id"]))
    return rule_rows, pr_rows, review_miss_rows


def new_rule_stats() -> dict[str, Any]:
    return {
        "prs": set(),
        "findings_total": 0,
        "slopgate_only": 0,
        "overlaps_all": 0,
        "actionable_overlaps": 0,
        "severity": Counter(),
        "messages": Counter(),
        "examples": [],
    }


def finding_row(
    repo: str,
    pr: int,
    phase: str,
    source: str,
    item: dict[str, Any],
    status: str,
    review_source: str,
    review_summary: str,
) -> dict[str, Any]:
    file_path = str(item.get("file") or item.get("path") or "")
    line = parse_int(item.get("line"))
    return {
        "repo": repo,
        "pr": pr,
        "phase": phase,
        "artifact": source,
        "rule_id": str(item.get("rule_id", "?")),
        "severity": str(item.get("severity", "")),
        "file": file_path,
        "line": line,
        "message": str(item.get("message", "")),
        "match_status": status,
        "review_source": review_source,
        "review_summary": review_summary,
        "actionable_overlap": "",
        "manual_usefulness": "",
    }


def update_rule_stats(stats: dict[str, Any], row: dict[str, Any], seen_rule_in_pr: set[str]) -> None:
    rule_id = row["rule_id"]
    if rule_id not in seen_rule_in_pr:
        stats["prs"].add((row["repo"], row["pr"]))
        seen_rule_in_pr.add(rule_id)
    stats["findings_total"] += 1
    if row["match_status"] == "slopgate_only":
        stats["slopgate_only"] += 1
    if row["match_status"] == "overlap":
        stats["overlaps_all"] += 1
    if row["severity"]:
        stats["severity"][row["severity"]] += 1
    if row["message"]:
        stats["messages"][row["message"]] += 1
    if len(stats["examples"]) < 3:
        stats["examples"].append(f"{row['repo']}#{row['pr']} {row['file']}:{row['line']} {row['message'] or row['review_summary']}")


def review_miss_row(repo: str, pr: int, phase: str, source: str, stream: str, item: dict[str, Any]) -> dict[str, Any]:
    file_path = str(item.get("file") or item.get("path") or "")
    line = parse_int(item.get("line"))
    return {
        "repo": repo,
        "pr": pr,
        "phase": phase,
        "artifact": source,
        "stream": stream,
        "file": file_path,
        "line": line,
        "summary": str(item.get("body", "")),
        "id": str(item.get("id", "")),
        "manual_family": "",
    }


def rule_row(rule_id: str, stats: dict[str, Any]) -> dict[str, Any]:
    findings_total = int(stats["findings_total"])
    overlaps_all = int(stats["overlaps_all"])
    actionable_overlaps = int(stats["actionable_overlaps"])
    slopgate_only = int(stats["slopgate_only"])
    top_message = ""
    for message, _count in stats["messages"].most_common(1):
        top_message = message
    return {
        "rule_id": rule_id,
        "prs_fired": len(stats["prs"]),
        "findings_total": findings_total,
        "overlaps_all": overlaps_all,
        "actionable_overlaps": actionable_overlaps,
        "slopgate_only": slopgate_only,
        "precision_proxy_pct": round((overlaps_all / findings_total * 100) if findings_total else 0, 1),
        "severity_counts": "; ".join(f"{k}:{v}" for k, v in sorted(stats["severity"].items())),
        "top_message": top_message,
        "examples": " | ".join(stats["examples"]),
        "suggested_action": suggest_action(findings_total, slopgate_only, overlaps_all, actionable_overlaps, len(stats["prs"])),
        "manual_decision": "",
    }


def suggest_action(findings_total: int, slopgate_only: int, overlaps_all: int, actionable_overlaps: int, prs_fired: int) -> str:
    if actionable_overlaps > 0:
        return "keep"
    if overlaps_all > 0 and slopgate_only <= overlaps_all * 3:
        return "watch"
    if findings_total >= 5 and slopgate_only == findings_total:
        return "quarantine"
    if prs_fired >= 2 and slopgate_only == findings_total:
        return "disable_candidate"
    return "review"


def action_rank(action: str) -> int:
    return {
        "quarantine": 0,
        "disable_candidate": 1,
        "review": 2,
        "watch": 3,
        "keep": 4,
    }.get(action, 5)


def write_csv(path: Path, rows: list[dict[str, Any]]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    if not rows:
        path.write_text("")
        return
    with path.open("w", newline="") as f:
        first_row = next(iter(rows))
        writer = csv.DictWriter(f, fieldnames=list(first_row.keys()))
        writer.writeheader()
        writer.writerows(rows)


def format_markdown(sample: list[dict[str, Any]], rule_rows: list[dict[str, Any]], review_rows: list[dict[str, Any]]) -> str:
    lines = ["# Slopgate Rule Pruning Scorecard", ""]
    lines.append(f"Sampled PR artifacts: {len(sample)}")
    if sample:
        lines.append("")
        lines.append("| Repo | PR | Phase | Artifact |")
        lines.append("|---|---:|---|---|")
        for data in sample:
            lines.append(f"| {data.get('repo')} | #{data.get('pr')} | {data.get('_phase')} | `{data.get('_source')}` |")
    lines.append("")
    lines.append("## Rule Scorecard")
    lines.append("")
    lines.append("| Rule | PRs | Findings | Overlap | Actionable | SG-only | Precision | Suggested | Top message |")
    lines.append("|---|---:|---:|---:|---:|---:|---:|---|---|")
    for row in rule_rows:
        lines.append(
            f"| {row['rule_id']} | {row['prs_fired']} | {row['findings_total']} | "
            f"{row['overlaps_all']} | {row['actionable_overlaps']} | {row['slopgate_only']} | "
            f"{row['precision_proxy_pct']}% | {row['suggested_action']} | {row['top_message']} |"
        )
    lines.append("")
    lines.append("## Review Misses")
    lines.append("")
    lines.append(f"Review-only rows: {len(review_rows)}")
    lines.append("")
    lines.append(
        "Manual review fields are intentionally blank in CSV output. Fill them "
        "with `useful`, `noise`, `unclear`, or `safety_exception` during pruning review."
    )
    lines.append("")
    return "\n".join(lines)


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Build a rule-level pruning scorecard from benchmark artifacts.")
    parser.add_argument("--benchmark-dir", type=Path, default=BENCHMARK_DIR)
    parser.add_argument("--repo", action="append", default=[], help="Repo filter; accepts owner/repo or short repo name")
    parser.add_argument("--pr", action="append", type=int, default=[], help="Specific PR number to include; repeatable")
    parser.add_argument("--limit", type=int, default=20, help="Maximum distinct repo/PR artifacts to sample; 0 means all")
    parser.add_argument("--output-dir", type=Path, default=None, help="Write CSV and markdown files to this directory")
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    if not args.benchmark_dir.is_dir():
        print(f"Benchmark directory not found: {args.benchmark_dir}", file=sys.stderr)
        return 1
    sample = collect_sample(args.benchmark_dir, repos=set(args.repo), prs=set(args.pr), limit=args.limit)
    rule_rows, pr_rows, review_rows = build_rows(sample)
    markdown = format_markdown(sample, rule_rows, review_rows)

    if args.output_dir:
        args.output_dir.mkdir(parents=True, exist_ok=True)
        write_csv(args.output_dir / "rule_scorecard.csv", rule_rows)
        write_csv(args.output_dir / "pr_findings.csv", pr_rows)
        write_csv(args.output_dir / "review_misses.csv", review_rows)
        (args.output_dir / "pruning_candidates.md").write_text(markdown)
        print(f"Wrote pruning scorecard to {args.output_dir}")
    else:
        print(markdown)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
