import importlib.util
from pathlib import Path
import unittest

spec = importlib.util.spec_from_file_location("compare", Path(__file__).with_name("compare.py"))
compare = importlib.util.module_from_spec(spec)
spec.loader.exec_module(compare)


class CompareTests(unittest.TestCase):
    def test_metrics(self):
        result = compare.metrics(
            "BenchmarkTracePipeline-1 256 1200 ns/op 50 cpu-ns/span 256 spans/op 1 allocated-B/span\n")
        self.assertEqual(result["cpu-ns/span"], 50)
        self.assertEqual(result["spans/op"], 256)

    def test_invalid_observations(self):
        with self.assertRaises(ValueError):
            compare.metrics("PASS")
        with self.assertRaises(ValueError):
            compare.metrics("BenchmarkTracePipeline-1 256 1200 ns/op 0 cpu-ns/span")

    def test_census_excludes_timing(self):
        value = compare.census('TRACEBENCH_CENSUS={"counters":[{"Elapsed":100,"Calls":8}]}\n')
        self.assertEqual(value, {"counters": [{"Calls": 8}]})

    def test_interval_sign(self):
        result = compare.interval([-0.1] * 8)
        self.assertGreater(result["less_cpu_percent"], 0)
        self.assertEqual(result["conditional_ci95_percent"][0], result["conditional_ci95_percent"][1])


if __name__ == "__main__":
    unittest.main()
