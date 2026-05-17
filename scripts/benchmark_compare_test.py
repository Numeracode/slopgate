import importlib.util
import sys
import unittest
from pathlib import Path


SCRIPT_PATH = Path(__file__).with_name("benchmark-compare.py")
SPEC = importlib.util.spec_from_file_location("benchmark_compare_under_test", SCRIPT_PATH)
assert SPEC is not None, f"Failed to create spec for {SCRIPT_PATH}"
assert SPEC.loader is not None, f"Spec has no loader: {SPEC}"
benchmark_compare = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = benchmark_compare
SPEC.loader.exec_module(benchmark_compare)


class BenchmarkCompareHelpersTest(unittest.TestCase):
    def test_get_stream_total_returns_nested_stream_total(self) -> None:
        data = {
            "streams": {
                "coderabbit_all": {"total": 3},
            },
        }

        self.assertEqual(benchmark_compare.get_stream_total(data, "coderabbit_all"), 3)

    def test_get_stream_total_returns_zero_for_unknown_stream(self) -> None:
        data = {
            "streams": {
                "coderabbit_all": {"total": 3},
            },
            "coderabbit": {"total": 2},
        }

        self.assertEqual(benchmark_compare.get_stream_total(data, "unknown_stream"), 0)

    def test_get_stream_total_uses_legacy_fallback(self) -> None:
        data = {
            "coderabbit": {"total": 4},
        }

        self.assertEqual(benchmark_compare.get_stream_total(data, "coderabbit_all"), 4)

    def test_tier_metric_value_returns_metric_value_for_known_key(self) -> None:
        data = {
            "benchmark_tiers": {
                "all_rules": {
                    "slopgate": {"total": 2},
                    "scores": {"overlap_all": 2},
                }
            },
            "streams": {
                "coderabbit_all": {"total": 3},
            },
        }

        self.assertEqual(
            benchmark_compare._tier_metric_value(data, "all_rules", "overlap_all"),
            2,
        )

    def test_tier_metric_value_returns_dash_for_unknown_metric_key(self) -> None:
        data = {
            "benchmark_tiers": {
                "all_rules": {
                    "slopgate": {"total": 1},
                    "scores": {"overlap_all": 1},
                }
            },
            "streams": {
                "coderabbit_all": {"total": 1},
            },
        }

        self.assertEqual(
            benchmark_compare._tier_metric_value(data, "all_rules", "missing_metric"),
            "-",
        )


if __name__ == "__main__":
    unittest.main()
