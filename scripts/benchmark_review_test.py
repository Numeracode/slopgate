import importlib.util
import subprocess
import sys
import unittest
from pathlib import Path
from unittest.mock import patch


SCRIPT_PATH = Path(__file__).with_name("benchmark_review.py")
SPEC = importlib.util.spec_from_file_location("benchmark_review_under_test", SCRIPT_PATH)
assert SPEC and SPEC.loader
benchmark_review = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = benchmark_review
SPEC.loader.exec_module(benchmark_review)


def completed(stdout: str = "", returncode: int = 0, stderr: str = "") -> subprocess.CompletedProcess[str]:
    return subprocess.CompletedProcess(args=[], returncode=returncode, stdout=stdout, stderr=stderr)


def call_commands(run_cmd: object) -> list[object]:
    return [next(iter(call.args), None) for call in run_cmd.call_args_list]


class ParseArgsTest(unittest.TestCase):
    def test_rejects_negative_fuzzy_range(self) -> None:
        argv = ["benchmark_review.py", "/repo", "20", "--fuzzy-range", "-1"]
        with patch.object(sys, "argv", argv):
            with self.assertRaises(SystemExit):
                benchmark_review.parse_args()


class GhApiJsonTest(unittest.TestCase):
    def test_raises_benchmark_error_on_malformed_paginated_json(self) -> None:
        with patch.object(
            benchmark_review,
            "run_cmd",
            return_value=completed(stdout='{"bad json"\n'),
        ):
            with self.assertRaisesRegex(benchmark_review.BenchmarkError, "gh api returned invalid JSON"):
                benchmark_review.gh_api_json(["repos/example/repo/pulls/20/comments", "--paginate"])


class ReviewCommentIngestionTest(unittest.TestCase):
    def test_collect_sentry_pr_comments_reads_sentry_bot_review_comments(self) -> None:
        comments = [
            {
                "id": 101,
                "user": {"login": "sentry[bot]"},
                "path": "api/src/app.js",
                "line": 10,
                "body": "**Bug:** invalid sample rate parses to NaN",
            },
            {
                "id": 102,
                "user": {"login": "coderabbitai[bot]"},
                "path": "api/src/app.js",
                "line": 12,
                "body": "_⚠️ Potential issue_",
            },
            {
                "id": 103,
                "user": {"login": "sentry[bot]"},
                "path": "api/src/app.js",
                "body": "missing line should be ignored",
            },
        ]

        with patch.object(benchmark_review, "gh_api_json", return_value=comments) as gh_api_json:
            findings = benchmark_review.collect_sentry_pr_comments("messagesgoel-blip/slopgate", 268)

        gh_api_json.assert_called_once_with(["repos/messagesgoel-blip/slopgate/pulls/268/comments", "--paginate"])
        self.assertEqual(len(findings), 1)
        [finding] = findings
        self.assertEqual(finding.path, "api/src/app.js")
        self.assertEqual(finding.line, 10)
        self.assertEqual(finding.source, "sentry")
        self.assertEqual(finding.item_id, "101")


class RunCmdTest(unittest.TestCase):
    def test_run_cmd_strips_hook_git_env_for_git_commands(self) -> None:
        with patch.dict(
            benchmark_review.os.environ,
            {
                "GIT_INDEX_FILE": ".git/index",
                "GIT_DIR": ".git",
                "GIT_WORK_TREE": "tmp-worktree",
                "GIT_PREFIX": "src/",
                "KEEP_ME": "1",
            },
            clear=False,
        ), patch.object(
            benchmark_review.subprocess,
            "run",
            return_value=completed(stdout="ok\n"),
        ) as run:
            benchmark_review.run_cmd(["git", "status"])

        env = run.call_args.kwargs["env"]
        self.assertIsNotNone(env)
        self.assertEqual(env.get("KEEP_ME"), "1")
        self.assertNotIn("GIT_INDEX_FILE", env)
        self.assertNotIn("GIT_DIR", env)
        self.assertNotIn("GIT_WORK_TREE", env)
        self.assertNotIn("GIT_PREFIX", env)

    def test_worktree_cleanup_strips_hook_git_env(self) -> None:
        context = benchmark_review.WorktreeContext(
            repo_root=Path("repo-root"),
            worktree_path=Path("worktree-path"),
            target_ref="target-ref",
            compare_base="base-ref",
            requested_base="main",
            base_branch="main",
            mode="open_pr_head",
            temp_ref="refs/slopgate-benchmark/pr-20-4321",
        )
        with patch.dict(
            benchmark_review.os.environ,
            {"GIT_INDEX_FILE": ".git/index"},
            clear=False,
        ), patch.object(
            benchmark_review.subprocess,
            "run",
            return_value=completed(),
        ) as run, patch.object(
            benchmark_review.shutil,
            "rmtree",
        ):
            context.cleanup()

        envs = [call.kwargs["env"] for call in run.call_args_list]
        self.assertEqual(len(envs), 2)
        self.assertTrue(all(env is not None for env in envs))
        self.assertTrue(all("GIT_INDEX_FILE" not in env for env in envs))


class PrepareWorktreeTest(unittest.TestCase):
    def setUp(self) -> None:
        self.repo_root = Path("repo-root")
        self.pr_number = 20
        self.worktree_path = "slopgate-benchmark-test"

    def test_prepare_worktree_uses_local_requested_base_sha(self) -> None:
        pr_meta = {"base": {"ref": "main"}, "merged": False}

        with patch.object(benchmark_review.tempfile, "mkdtemp", return_value=self.worktree_path), \
            patch.object(benchmark_review.os, "getpid", return_value=4321), \
            patch.object(
                benchmark_review,
                "run_cmd",
                side_effect=[
                    completed(),
                    completed(stdout="abc123\n"),
                    completed(),
                ],
            ) as run_cmd:
            context = benchmark_review.prepare_worktree(self.repo_root, pr_meta, self.pr_number, "feature/base")

        self.assertEqual(context.target_ref, "refs/slopgate-benchmark/pr-20-4321")
        self.assertEqual(context.compare_base, "abc123")
        self.assertEqual(context.temp_ref, "refs/slopgate-benchmark/pr-20-4321")
        self.assertEqual(call_commands(run_cmd), [
            ["git", "-C", "repo-root", "fetch", "origin", "refs/pull/20/head:refs/slopgate-benchmark/pr-20-4321"],
            ["git", "-C", "repo-root", "rev-parse", "--verify", "feature/base"],
            ["git", "-C", "repo-root", "worktree", "add", "--detach", self.worktree_path, "refs/slopgate-benchmark/pr-20-4321"],
        ])

    def test_prepare_worktree_uses_fetch_head_for_unnamed_requested_base(self) -> None:
        pr_meta = {"base": {"ref": "main"}, "merged": False}

        with patch.object(benchmark_review.tempfile, "mkdtemp", return_value=self.worktree_path), \
            patch.object(benchmark_review.os, "getpid", return_value=4321), \
            patch.object(
                benchmark_review,
                "run_cmd",
                side_effect=[
                    completed(),
                    completed(returncode=1, stderr="missing"),
                    completed(),
                    completed(returncode=1, stderr="still missing"),
                    completed(stdout="fedcba\n"),
                    completed(),
                ],
            ):
            context = benchmark_review.prepare_worktree(self.repo_root, pr_meta, self.pr_number, "deadbeef")

        self.assertEqual(context.compare_base, "fedcba")

    def test_prepare_worktree_raises_and_cleans_up_unresolved_requested_base(self) -> None:
        pr_meta = {"base": {"ref": "main"}, "merged": False}

        with patch.object(benchmark_review.tempfile, "mkdtemp", return_value=self.worktree_path), \
            patch.object(benchmark_review.os, "getpid", return_value=4321), \
            patch.object(
                benchmark_review,
                "run_cmd",
                side_effect=[
                    completed(),
                    completed(returncode=1, stderr="missing"),
                    completed(stderr="fetch ok"),
                    completed(returncode=1, stderr="still missing"),
                    completed(returncode=1, stderr="no fetch head"),
                    completed(returncode=1, stderr="no worktree"),
                    completed(),
                ],
            ) as run_cmd, \
            patch.object(benchmark_review.shutil, "rmtree") as rmtree:
            with self.assertRaises(benchmark_review.BenchmarkError):
                benchmark_review.prepare_worktree(self.repo_root, pr_meta, self.pr_number, "unknown-base")

        calls = call_commands(run_cmd)
        self.assertIn(["git", "-C", "repo-root", "worktree", "remove", "--force", self.worktree_path], calls)
        self.assertIn(["git", "-C", "repo-root", "update-ref", "-d", "refs/slopgate-benchmark/pr-20-4321"], calls)
        rmtree.assert_called_once_with(Path(self.worktree_path), ignore_errors=True)

    def test_prepare_worktree_merged_path_fetches_and_uses_parent_base(self) -> None:
        pr_meta = {"base": {"ref": "main"}, "merged": True, "merge_commit_sha": "merge-sha"}

        with patch.object(benchmark_review.tempfile, "mkdtemp", return_value=self.worktree_path), \
            patch.object(
                benchmark_review,
                "run_cmd",
                side_effect=[
                    completed(),
                    completed(stdout="merge-sha\n"),
                    completed(stdout="parent-sha\n"),
                    completed(),
                ],
            ) as run_cmd:
            context = benchmark_review.prepare_worktree(self.repo_root, pr_meta, self.pr_number, "")

        self.assertEqual(context.target_ref, "merge-sha")
        self.assertEqual(context.compare_base, "parent-sha")
        self.assertIsNone(context.temp_ref)
        self.assertEqual(call_commands(run_cmd), [
            ["git", "-C", "repo-root", "fetch", "origin", "main"],
            ["git", "-C", "repo-root", "rev-parse", "--verify", "merge-sha"],
            ["git", "-C", "repo-root", "rev-parse", "merge-sha^1"],
            ["git", "-C", "repo-root", "worktree", "add", "--detach", self.worktree_path, "merge-sha"],
        ])

    def test_prepare_worktree_merged_missing_merge_sha_requires_base(self) -> None:
        pr_meta = {"base": {"ref": "main"}, "merged": True}

        with patch.object(benchmark_review.tempfile, "mkdtemp", return_value=self.worktree_path), \
            patch.object(
                benchmark_review,
                "run_cmd",
                side_effect=[
                    completed(),
                    completed(returncode=1, stderr="no worktree"),
                ],
            ) as run_cmd, \
            patch.object(benchmark_review.shutil, "rmtree") as rmtree:
            with self.assertRaisesRegex(benchmark_review.BenchmarkError, "missing merge_commit_sha"):
                benchmark_review.prepare_worktree(self.repo_root, pr_meta, self.pr_number, "")

        calls = call_commands(run_cmd)
        self.assertIn(["git", "-C", "repo-root", "fetch", "origin", "main"], calls)
        self.assertIn(["git", "-C", "repo-root", "worktree", "remove", "--force", self.worktree_path], calls)
        rmtree.assert_called_once_with(Path(self.worktree_path), ignore_errors=True)

    def test_prepare_worktree_merged_missing_merge_sha_uses_pr_head_when_base_supplied(self) -> None:
        pr_meta = {"base": {"ref": "main"}, "merged": True}

        with patch.object(benchmark_review.tempfile, "mkdtemp", return_value=self.worktree_path), \
            patch.object(benchmark_review.os, "getpid", return_value=4321), \
            patch.object(
                benchmark_review,
                "run_cmd",
                side_effect=[
                    completed(),
                    completed(),
                    completed(stdout="base-sha\n"),
                    completed(),
                ],
            ) as run_cmd:
            context = benchmark_review.prepare_worktree(self.repo_root, pr_meta, self.pr_number, "feature/base")

        self.assertEqual(context.target_ref, "refs/slopgate-benchmark/pr-20-4321")
        self.assertEqual(context.compare_base, "base-sha")
        self.assertEqual(context.mode, "merged_pr_head")
        self.assertEqual(context.temp_ref, "refs/slopgate-benchmark/pr-20-4321")
        self.assertEqual(call_commands(run_cmd), [
            ["git", "-C", "repo-root", "fetch", "origin", "main"],
            ["git", "-C", "repo-root", "fetch", "origin", "refs/pull/20/head:refs/slopgate-benchmark/pr-20-4321"],
            ["git", "-C", "repo-root", "rev-parse", "--verify", "feature/base"],
            ["git", "-C", "repo-root", "worktree", "add", "--detach", self.worktree_path, "refs/slopgate-benchmark/pr-20-4321"],
        ])


class MatchStreamTest(unittest.TestCase):
    def test_match_stream_is_one_to_one(self) -> None:
        sg_findings = [
            {"file": "a.js", "line": 10, "rule_id": "SLP001", "severity": "warn", "message": "one"},
            {"file": "a.js", "line": 10, "rule_id": "SLP002", "severity": "warn", "message": "two"},
        ]
        review_findings = [
            benchmark_review.ReviewFinding(path="a.js", line=10, body="review", item_id="1", source="coderabbit", meta={}),
        ]

        result = benchmark_review.match_stream(sg_findings, review_findings, 0)

        self.assertEqual(result["comparison"]["overlap"], 1)
        self.assertEqual(result["comparison"]["slopgate_only"], 1)
        self.assertEqual(result["comparison"]["review_only"], 0)

    def test_match_stream_reports_unmatched_items(self) -> None:
        sg_findings = [
            {"file": "a.js", "line": 10, "rule_id": "SLP001", "severity": "warn", "message": "one"},
            {"file": "b.js", "line": 20, "rule_id": "SLP002", "severity": "warn", "message": "two"},
        ]
        review_findings = [
            benchmark_review.ReviewFinding(path="a.js", line=10, body="review one", item_id="1", source="coderabbit", meta={}),
            benchmark_review.ReviewFinding(path="c.js", line=30, body="review two", item_id="2", source="coderabbit", meta={}),
        ]

        result = benchmark_review.match_stream(sg_findings, review_findings, 0)

        self.assertEqual(result["comparison"]["overlap"], 1)
        self.assertEqual(result["comparison"]["slopgate_only"], 1)
        self.assertEqual(result["comparison"]["review_only"], 1)


class BenchmarkTieringTest(unittest.TestCase):
    def test_benchmark_rule_overrides_mark_known_noise_rules_as_advisory(self) -> None:
        self.assertEqual(benchmark_review.RULE_BENCHMARK_MODE_OVERRIDES["SLP017"], "advisory")
        self.assertEqual(benchmark_review.RULE_BENCHMARK_MODE_OVERRIDES["SLP035"], "advisory")
        self.assertEqual(benchmark_review.RULE_BENCHMARK_MODE_OVERRIDES["SLP053"], "advisory")

    def test_finding_to_compare_item_marks_info_as_advisory_by_default(self) -> None:
        finding = {
            "file": "a.js",
            "line": 10,
            "rule_id": "SLP035",
            "severity": "Info",
            "message": "style issue",
        }

        item = benchmark_review.finding_to_compare_item(finding)

        self.assertEqual(item["severity"], "info")
        self.assertEqual(item["benchmark_mode"], "advisory")
        self.assertFalse(item["benchmark_eligible"])

    def test_finding_to_compare_item_honors_rule_mode_override(self) -> None:
        finding = {
            "file": "a.js",
            "line": 10,
            "rule_id": "SLP999",
            "severity": "info",
            "message": "override issue",
        }

        with patch.dict(benchmark_review.RULE_BENCHMARK_MODE_OVERRIDES, {"SLP999": "parity"}, clear=True):
            item = benchmark_review.finding_to_compare_item(finding)

        self.assertEqual(item["benchmark_mode"], "parity")
        self.assertTrue(item["benchmark_eligible"])

    def test_finding_to_compare_item_rejects_unknown_severity(self) -> None:
        finding = {
            "file": "a.js",
            "line": 10,
            "rule_id": "SLP404",
            "severity": "criticalish",
            "message": "unknown severity issue",
        }

        with self.assertRaisesRegex(
            benchmark_review.BenchmarkError,
            "SLP404: criticalish|SLP404.*criticalish",
        ):
            benchmark_review.finding_to_compare_item(finding)

    def test_build_result_payload_keeps_legacy_fields_and_adds_tiers(self) -> None:
        context = benchmark_review.WorktreeContext(
            repo_root=Path("repo-root"),
            worktree_path=Path("worktree-path"),
            target_ref="target-ref",
            compare_base="base-sha",
            requested_base="main",
            base_branch="main",
            mode="open_pr_head",
        )
        report = {
            "summary": {"total": 2, "block": 0, "warn": 1, "info": 1},
            "findings": [
                {
                    "file": "a.js",
                    "line": 10,
                    "rule_id": "SLP001",
                    "severity": "warn",
                    "message": "warn finding",
                },
                {
                    "file": "b.js",
                    "line": 20,
                    "rule_id": "SLP035",
                    "severity": "info",
                    "message": "info finding",
                },
            ],
        }
        all_comments = [
            benchmark_review.ReviewFinding(
                path="a.js",
                line=10,
                body="matched review comment",
                item_id="1",
                source="coderabbit_all",
                meta={},
            )
        ]

        result = benchmark_review.build_result_payload(
            slug="messagesgoel-blip/slopgate",
            pr_number=64,
            context=context,
            pr_meta={"merged": False, "state": "open"},
            report=report,
            all_comments=all_comments,
            actionable_comments=all_comments,
            sentry_findings=[],
            combined_actionable=all_comments,
            fuzzy_range=0,
            slopgate_stderr="benchmark stderr",
        )

        self.assertEqual(result["slopgate"], report["summary"])
        self.assertEqual(result["comparison"], result["benchmark_tiers"]["all_rules"]["comparison"])
        self.assertEqual(result["scores"], result["benchmark_tiers"]["all_rules"]["scores"])
        self.assertEqual(result["sg_only_details"], result["benchmark_tiers"]["all_rules"]["sg_only_details"])
        self.assertEqual(
            result["benchmark_metadata"]["default_mode_by_severity"],
            benchmark_review.BENCHMARK_MODE_BY_SEVERITY,
        )
        self.assertEqual(set(result["benchmark_tiers"]), {"all_rules", "block_warn_only", "benchmark_eligible"})
        self.assertEqual(result["benchmark_tiers"]["all_rules"]["slopgate"]["total"], 2)
        self.assertEqual(result["benchmark_tiers"]["block_warn_only"]["slopgate"]["total"], 1)
        self.assertEqual(result["benchmark_tiers"]["benchmark_eligible"]["slopgate"]["total"], 1)
        self.assertEqual(result["benchmark_tiers"]["all_rules"]["scores"]["overlap_all"], 1)
        self.assertEqual(result["benchmark_tiers"]["benchmark_eligible"]["scores"]["overlap_all"], 1)


if __name__ == "__main__":
    unittest.main()
