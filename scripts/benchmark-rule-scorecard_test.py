import importlib.util
import json
import sys
import tempfile
import unittest
from pathlib import Path


SCRIPT_PATH = Path(__file__).with_name("benchmark-rule-scorecard.py")
SPEC = importlib.util.spec_from_file_location("benchmark_rule_scorecard_under_test", SCRIPT_PATH)
assert SPEC is not None
assert SPEC.loader is not None
scorecard = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = scorecard
SPEC.loader.exec_module(scorecard)


class BenchmarkRuleScorecardTest(unittest.TestCase):
    def test_collect_sample_prefers_postmerge_artifact_for_same_pr(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            bench_dir = Path(tmp)
            self.write_benchmark(bench_dir / "demo-7-prepush.json", pr=7, phase_note="prepush")
            self.write_benchmark(bench_dir / "demo-7-postmerge.json", pr=7, phase_note="postmerge")

            sample = scorecard.collect_sample(bench_dir, repos={"demo"}, prs=set(), limit=20)

            self.assertEqual(len(sample), 1)
            only_sample = next(iter(sample))
            self.assertEqual(only_sample["_phase"], "postmerge")
            self.assertTrue(only_sample["_source"].endswith("demo-7-postmerge.json"))

    def test_build_rows_marks_actionable_overlap_for_rule(self) -> None:
        data = {
            "repo": "messagesgoel-blip/demo",
            "pr": 9,
            "_source": "demo-9-postmerge.json",
            "_phase": "postmerge",
            "slopgate": {"total": 2},
            "comparison_streams": {
                "coderabbit_all": {
                    "sg_only_details": [
                        {
                            "file": "a.go",
                            "line": 3,
                            "rule_id": "SLP043",
                            "severity": "warn",
                            "message": "duplicate key field",
                        },
                        {
                            "file": "b.go",
                            "line": 5,
                            "rule_id": "SLP205",
                            "severity": "warn",
                            "message": "colliding slopgate-only row",
                        }
                    ],
                    "overlap_details": [
                        {
                            "file": "b.go",
                            "line": 5,
                            "rule_id": "SLP205",
                            "review_source": "coderabbit_all",
                            "review_summary": "real bug",
                        }
                    ],
                    "review_only_details": [],
                },
                "actionable_plus_sentry": {
                    "overlap_details": [
                        {
                            "file": "b.go",
                            "line": 5,
                            "rule_id": "SLP205",
                        }
                    ],
                    "review_only_details": [],
                },
            },
        }

        rule_rows, pr_rows, review_rows = scorecard.build_rows([data])

        self.assertEqual(review_rows, [])
        by_rule = {row["rule_id"]: row for row in rule_rows}
        self.assertEqual(by_rule["SLP205"]["actionable_overlaps"], 1)
        self.assertEqual(by_rule["SLP205"]["suggested_action"], "keep")
        self.assertEqual(by_rule["SLP043"]["slopgate_only"], 1)
        self.assertEqual(by_rule["SLP205"]["slopgate_only"], 1)
        self.assertEqual(len(pr_rows), 3)

    def test_malformed_line_and_pr_values_do_not_abort_scorecard(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            bench_dir = Path(tmp)
            (bench_dir / "demo-bad-postmerge.json").write_text(json.dumps({
                "repo": "messagesgoel-blip/demo",
                "pr": "not-a-number",
                "slopgate": {"total": 0},
            }))
            (bench_dir / "demo-10-postmerge.json").write_text(json.dumps({
                "repo": "messagesgoel-blip/demo",
                "pr": 10,
                "slopgate": {"total": 1},
                "comparison_streams": {
                    "coderabbit_all": {
                        "sg_only_details": [
                            {
                                "file": "a.go",
                                "line": None,
                                "rule_id": "SLP001",
                                "severity": "warn",
                                "message": "sample",
                            }
                        ],
                        "review_only_details": [],
                    }
                },
            }))

            sample = scorecard.collect_sample(bench_dir, repos={"demo"}, prs=set(), limit=20)
            _rule_rows, pr_rows, _review_rows = scorecard.build_rows(sample)

            self.assertEqual([row["pr"] for row in pr_rows], [10])
            only_row = next(iter(pr_rows))
            self.assertEqual(only_row["line"], 0)

    def test_actionable_overlap_accepts_missing_line_and_path_key(self) -> None:
        data = {
            "repo": "messagesgoel-blip/demo",
            "pr": 11,
            "_source": "demo-11-postmerge.json",
            "_phase": "postmerge",
            "slopgate": {"total": 1},
            "comparison_streams": {
                "coderabbit_all": {
                    "sg_only_details": [],
                    "overlap_details": [
                        {
                            "file": "a.go",
                            "line": None,
                            "rule_id": "SLP050",
                            "review_source": "coderabbit_all",
                            "review_summary": "needs guard",
                        }
                    ],
                    "review_only_details": [],
                },
                "actionable_plus_sentry": {
                    "overlap_details": [
                        {
                            "path": "a.go",
                            "line": None,
                            "rule_id": "SLP050",
                        }
                    ],
                    "review_only_details": [],
                },
            },
        }

        rule_rows, pr_rows, _review_rows = scorecard.build_rows([data])

        only_pr_row = next(iter(pr_rows))
        only_rule_row = next(iter(rule_rows))
        self.assertEqual(only_pr_row["actionable_overlap"], "yes")
        self.assertEqual(only_rule_row["actionable_overlaps"], 1)

    def write_benchmark(self, path: Path, *, pr: int, phase_note: str) -> None:
        path.write_text(json.dumps({
            "repo": "messagesgoel-blip/demo",
            "pr": pr,
            "slopgate": {"total": 0},
            "phase_note": phase_note,
        }))


if __name__ == "__main__":
    unittest.main()
