import importlib.util
import sys
import unittest
from pathlib import Path


SCRIPT_PATH = Path(__file__).with_name("benchmark-compare.py")
SPEC = importlib.util.spec_from_file_location("benchmark_compare_under_test", SCRIPT_PATH)
assert SPEC and SPEC.loader
benchmark_compare = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = benchmark_compare
SPEC.loader.exec_module(benchmark_compare)


class BenchmarkCompareHelpersTest(unittest.TestCase):
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
