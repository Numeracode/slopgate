#!/usr/bin/env python3
"""Audit whether recent merged PRs have Slopgate benchmark artifacts."""

import argparse
import json
import subprocess
import sys
from pathlib import Path

BENCHMARK_DIR = Path("/srv/storage/shared/slopgate-benchmarks")
DEFAULT_PHASES = ("postmerge", "prepush", "precommit")
GH_PR_LIST_TIMEOUT_SECONDS = 30


def run_gh_pr_list(repo: str, limit: int) -> list[dict[str, str]]:
    cmd = [
        "gh",
        "pr",
        "list",
        "--repo",
        repo,
        "--state",
        "merged",
        "--limit",
        str(limit),
        "--json",
        "number,title,mergedAt,url",
    ]
    try:
        proc = subprocess.run(cmd, capture_output=True, text=True, check=False, timeout=GH_PR_LIST_TIMEOUT_SECONDS)
    except FileNotFoundError as exc:
        raise RuntimeError("GitHub CLI `gh` is not installed or not in PATH") from exc
    except subprocess.TimeoutExpired as exc:
        raise RuntimeError("gh pr list timed out after 30 seconds") from exc
    if proc.returncode != 0:
        raise RuntimeError(f"gh pr list failed: {proc.stderr.strip()}")
    return json.loads(proc.stdout)


def artifact_prefix(repo: str, explicit: str) -> str:
    if explicit:
        return explicit
    return repo.rsplit("/", 1)[-1]


def find_artifacts(benchmark_dir: Path, prefix: str, pr_number: int, phases: tuple[str, ...]) -> list[Path]:
    matches: list[Path] = []
    for phase in phases:
        matches.extend(sorted(benchmark_dir.glob(f"{prefix}-{pr_number}-{phase}.json")))
    # Finish-loop artifacts include owner/repo in the filename for some runs.
    matches.extend(sorted(benchmark_dir.glob(f"*-{prefix}-{pr_number}-finish.json")))
    return matches


def format_table(rows: list[dict[str, object]]) -> str:
    def markdown_cell(value: object) -> str:
        return str(value).strip().replace("|", r"\|").replace("\n", "<br>")

    lines = ["| PR | Merged | Benchmark | Artifacts | Title |", "|---:|---|---|---|---|"]
    for row in rows:
        artifacts = "<br>".join(f"`{Path(str(path)).name}`" for path in row["artifacts"])
        lines.append(
            f"| #{markdown_cell(row['number'])} | {markdown_cell(row['mergedAt'])} | "
            f"{markdown_cell(row['status'])} | {artifacts or '-'} | {markdown_cell(row['title'])} |"
        )
    return "\n".join(lines)


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Check benchmark artifact coverage for recent merged PRs.")
    parser.add_argument("repo", help="GitHub repo, e.g. messagesgoel-blip/slopgate")
    parser.add_argument("--artifact-prefix", default="", help="Benchmark artifact prefix; defaults to repo short name")
    parser.add_argument("--benchmark-dir", type=Path, default=BENCHMARK_DIR)
    parser.add_argument("--limit", type=int, default=20)
    parser.add_argument("--min-pr", type=int, default=0, help="Ignore merged PRs below this number")
    parser.add_argument("--phase", action="append", default=[], help="Accepted phase; repeatable. Defaults to postmerge/prepush/precommit")
    parser.add_argument("--fail-on-missing", action="store_true")
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    if not args.benchmark_dir.is_dir():
        print(f"Benchmark directory not found: {args.benchmark_dir}", file=sys.stderr)
        return 2
    prefix = artifact_prefix(args.repo, args.artifact_prefix)
    phases = tuple(args.phase) if args.phase else DEFAULT_PHASES
    prs = [pr for pr in run_gh_pr_list(args.repo, args.limit) if int(pr["number"]) >= args.min_pr]
    rows: list[dict[str, object]] = []
    missing = 0
    for pr in prs:
        artifacts = find_artifacts(args.benchmark_dir, prefix, int(pr["number"]), phases)
        if not artifacts:
            missing += 1
        rows.append({
            "number": pr["number"],
            "mergedAt": pr.get("mergedAt", ""),
            "title": pr.get("title", ""),
            "status": "tracked" if artifacts else "missing",
            "artifacts": [str(path) for path in artifacts],
        })

    print(format_table(rows))
    if missing:
        print(f"\nMissing benchmark artifacts: {missing}/{len(rows)}", file=sys.stderr)
    if missing and args.fail_on_missing:
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
