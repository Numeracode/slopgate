import importlib.util
import sys
import tempfile
import unittest
from pathlib import Path


SCRIPT_PATH = Path(__file__).with_name("benchmark-pr-coverage.py")
SPEC = importlib.util.spec_from_file_location("benchmark_pr_coverage_under_test", SCRIPT_PATH)
assert SPEC is not None
assert SPEC.loader is not None
coverage = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = coverage
SPEC.loader.exec_module(coverage)


class BenchmarkPRCoverageTest(unittest.TestCase):
    def test_artifact_prefix_defaults_to_repo_short_name(self) -> None:
        self.assertEqual(coverage.artifact_prefix("messagesgoel-blip/slopgate", ""), "slopgate")
        self.assertEqual(coverage.artifact_prefix("messagesgoel-blip/slopgate", "custom"), "custom")

    def test_find_artifacts_matches_requested_phases_and_finish_artifact(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            bench_dir = Path(tmp)
            (bench_dir / "slopgate-75-postmerge.json").write_text("{}")
            (bench_dir / "messagesgoel-blip-slopgate-75-finish.json").write_text("{}")
            (bench_dir / "slopgate-76-prepush.json").write_text("{}")

            matches = coverage.find_artifacts(bench_dir, "slopgate", 75, ("postmerge",))

            self.assertEqual(
                sorted(path.name for path in matches),
                ["messagesgoel-blip-slopgate-75-finish.json", "slopgate-75-postmerge.json"],
            )

    def test_find_artifacts_respects_phase_filter(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            bench_dir = Path(tmp)
            (bench_dir / "slopgate-75-postmerge.json").write_text("{}")
            (bench_dir / "slopgate-75-prepush.json").write_text("{}")

            matches = coverage.find_artifacts(bench_dir, "slopgate", 75, ("postmerge",))

            self.assertEqual([path.name for path in matches], ["slopgate-75-postmerge.json"])


if __name__ == "__main__":
    unittest.main()
