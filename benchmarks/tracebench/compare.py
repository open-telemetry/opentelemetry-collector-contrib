#!/usr/bin/env python3
"""Compare a single source patch with identical public or external inputs."""
import argparse
import hashlib
import json
import math
import os
from pathlib import Path
import random
import re
import shutil
import statistics
import subprocess

PATCHES = {
    "map-get": [
        ("go.opentelemetry.io/collector/pdata", 2, ["pdata/pcommon/map.go"]),
    ],
    "literal-regex": [
        ("github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl", 3, [
            "pkg/ottl/ottlfuncs/regex_helpers.go",
            "pkg/ottl/ottlfuncs/func_replace_all_patterns.go",
            "pkg/ottl/ottlfuncs/func_replace_pattern.go",
        ]),
    ],
    "kind-aware": [
        ("github.com/DataDog/datadog-agent/pkg/trace", 3, ["pkg/trace/transform/otelutil.go"]),
    ],
    "batched-tags": [
        ("github.com/DataDog/datadog-agent/pkg/trace", 3, ["pkg/trace/transform/transform.go"]),
        ("github.com/DataDog/datadog-agent/pkg/trace/otel", 4, ["pkg/trace/otel/stats/otel_util.go"]),
    ],
}


def run(command, cwd, env, log):
    result = subprocess.run(command, cwd=cwd, env=env, text=True,
                            stdout=subprocess.PIPE, stderr=subprocess.STDOUT)
    log.write_text(result.stdout)
    if result.returncode:
        raise RuntimeError(f"Command failed ({result.returncode}); see {log}")
    return result.stdout


def interval(ratios):
    if len(ratios) < 8:
        raise ValueError("At least eight blocks are required for the length-four bootstrap")
    mean = statistics.mean(ratios)
    rng = random.Random(1)
    draws = []
    for _ in range(5000):
        sample = []
        while len(sample) < len(ratios):
            start = rng.randrange(len(ratios))
            sample.extend(ratios[(start + j) % len(ratios)] for j in range(min(4, len(ratios))))
        draws.append(statistics.mean(sample[:len(ratios)]))
    gains = sorted(100 * -math.expm1(value) for value in draws)
    return {"less_cpu_percent": 100 * -math.expm1(mean),
            "conditional_ci95_percent": [gains[125], gains[4874]]}


def census(text):
    values = re.findall(r"TRACEBENCH_CENSUS=(\{[^\n]+\})", text)
    if len(values) != 1:
        raise ValueError("Expected one workload census")
    result = json.loads(values[0])
    for counter in result["counters"]:
        counter.pop("Elapsed", None)
    return result


def modules(text):
    decoder = json.JSONDecoder()
    result = {}
    while text.strip():
        value, end = decoder.raw_decode(text.lstrip())
        text = text.lstrip()[end:]
        result[value["Path"]] = value
    return result


def metrics(text):
    lines = [line for line in text.splitlines() if line.startswith("BenchmarkTracePipeline")]
    if len(lines) != 1:
        raise ValueError("Expected one benchmark observation")
    fields = lines[0].split()
    result = {}
    for index in range(2, len(fields), 2):
        result[fields[index + 1]] = float(fields[index])
    if result["cpu-ns/span"] <= 0:
        raise ValueError("Invalid process CPU measurement")
    return result


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--candidate", choices=PATCHES, required=True)
    parser.add_argument("--repository", type=Path, required=True)
    parser.add_argument("--base", required=True)
    parser.add_argument("--head", required=True)
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--spec", type=Path)
    parser.add_argument("--blocks", type=int, default=8)
    parser.add_argument("--iterations", type=int, default=256)
    parser.add_argument("--pgo", action="store_true")
    args = parser.parse_args()
    if args.blocks < 8 or args.iterations < 1:
        parser.error("use at least eight blocks and a positive whole-bank iteration count")
    root = Path(__file__).resolve().parent
    output = args.output.resolve()
    if output == root or root in output.parents:
        parser.error("output must be outside the benchmark source directory")
    output.mkdir(mode=0o700, parents=True, exist_ok=False)
    env = dict(os.environ, GOWORK="off", GOFLAGS="", GOMAXPROCS="1", GOGC="100",
               GOMEMLIMIT="off", CGO_ENABLED="0", GOAMD64="v1")
    env.pop("TRACEBENCH_CPU_PROFILE", None)
    env.pop("TRACEBENCH_SPEC", None)
    if args.spec:
        env["TRACEBENCH_SPEC"] = str(args.spec.resolve())
    harness = output / "harness"
    shutil.copytree(root, harness, ignore=shutil.ignore_patterns("__pycache__"))
    run(["go", "mod", "edit",
         "-replace=github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl=" +
         str(root.parent.parent / "pkg/ottl")], harness, env, output / "baseline-mod-edit.log")
    graph = modules(run(["go", "list", "-mod=readonly", "-m", "-json", "all"],
                        harness, env, output / "baseline-modules.json"))
    build = ["go", "test", "-mod=readonly", "-p=2", "-c"]
    baseline = output / "baseline.test"
    run([*build, "-pgo=off", "-o", str(baseline)], harness, env, output / "baseline-build.log")
    expected = census(run([str(baseline), "-test.run=^TestConfiguredCensus$", "-test.v"],
                          output, env, output / "baseline-census.log"))
    if args.iterations % expected["batches"]:
        raise ValueError("Iterations must be a whole multiple of the workload bank size")
    profile = output / "training.pprof"
    if args.pgo:
        run([str(baseline), "-test.run=^$", "-test.bench=^BenchmarkTracePipeline$",
             f"-test.benchtime={args.iterations}x"], output,
            dict(env, TRACEBENCH_CPU_PROFILE=str(profile)), output / "training.log")
        run([*build, "-pgo=" + str(profile), "-o", str(baseline)], harness, env,
            output / "baseline-pgo-build.log")
    patch_hashes = {}
    for index, (module, strip, paths) in enumerate(PATCHES[args.candidate]):
        source = output / ("source-" + str(index))
        original = graph[module]["Dir"]
        shutil.copytree(original, source)
        for path in [source, *source.rglob("*")]:
            path.chmod(0o700 if path.is_dir() else 0o600)
        patch = run(["git", "--no-pager", "diff", "--no-ext-diff", "--no-textconv",
                     "-U0", args.base, args.head, "--", *paths],
                    args.repository.resolve(), env, output / ("proposal-" + str(index) + ".patch"))
        if not patch.strip():
            raise ValueError("Empty candidate patch")
        patch_file = output / ("proposal-" + str(index) + ".patch")
        patch_hashes[module] = hashlib.sha256(patch.encode()).hexdigest()
        run(["git", "apply", "--no-index", "--unidiff-zero", f"-p{strip}", str(patch_file)],
            source, dict(env, GIT_CEILING_DIRECTORIES=str(output)),
            output / ("apply-" + str(index) + ".log"))
        run(["go", "mod", "edit", f"-replace={module}={source}"], harness, env,
            output / ("mod-edit-" + str(index) + ".log"))
    after = modules(run(["go", "list", "-mod=readonly", "-m", "-json", "all"],
                        harness, env, output / "candidate-modules.json"))
    if {k: v.get("Version") for k, v in graph.items()} != {k: v.get("Version") for k, v in after.items()}:
        raise ValueError("Dependency versions changed")
    candidate = output / "candidate.test"
    run([*build, "-pgo=" + str(profile) if args.pgo else "-pgo=off", "-o", str(candidate)],
        harness, env, output / "candidate-build.log")
    run([str(candidate), "-test.run=^Test", "-test.v"], output, env, output / "candidate-tests.log")
    actual = census(run([str(candidate), "-test.run=^TestConfiguredCensus$", "-test.v"],
                        output, env, output / "candidate-census.log"))
    if actual != expected:
        raise ValueError("Candidate workload/stage/survival census changed")
    observations = []
    binaries = {"A": baseline, "B": candidate}
    for label, binary in binaries.items():
        run([str(binary), "-test.run=^$", "-test.bench=^BenchmarkTracePipeline$",
             f"-test.benchtime={args.iterations}x"], output, env, output / ("warmup-" + label + ".log"))
    plan = ["AB"] * args.blocks + ["AA"] * args.blocks
    random.Random(1).shuffle(plan)
    counters = {"AB": 0, "AA": 0}
    ratios = {"AB": [], "AA": []}
    allocations = []
    for phase in plan:
        block = counters[phase]
        counters[phase] += 1
        group = []
        for position, label in enumerate("ABBA" if block % 2 == 0 else "BAAB"):
            binary = baseline if phase == "AA" else binaries[label]
            log = output / f"{phase}-{block}-{position}.log"
            value = metrics(run([str(binary), "-test.run=^$", "-test.bench=^BenchmarkTracePipeline$",
                                 f"-test.benchtime={args.iterations}x"], output, env, log))
            if abs(value["spans/op"] - expected["spans"] / expected["batches"]) > 0.1:
                raise ValueError("Measured input work changed")
            group.append((label, value))
            observations.append({"phase": phase, "block": block, "label": label, "metrics": value})
        ratios[phase].append(statistics.mean(math.log(v["cpu-ns/span"]) for label, v in group if label == "B") -
                             statistics.mean(math.log(v["cpu-ns/span"]) for label, v in group if label == "A"))
        if phase == "AB":
            allocations.append(statistics.mean(math.log(v["allocated-B/span"]) for label, v in group if label == "B") -
                               statistics.mean(math.log(v["allocated-B/span"]) for label, v in group if label == "A"))
    result = {"candidate": args.candidate, "base": args.base, "head": args.head,
              "pgo": args.pgo, "iterations": args.iterations, "blocks": args.blocks,
              "patch_sha256": patch_hashes, "census": expected,
              "cpu": interval(ratios["AB"]), "aa_control": interval(ratios["AA"]),
              "allocated_bytes_change_percent": 100 * math.expm1(statistics.mean(allocations)),
              "observations": observations,
              "caveat": "Synthetic workload; conditional intervals on this host, no A/A correction "
                        "or production inference. Census does not verify all derived APM metrics."}
    (output / "result.json").write_text(json.dumps(result, indent=2) + "\n")
    print(json.dumps({key: value for key, value in result.items() if key != "observations"}, indent=2))


if __name__ == "__main__":
    main()
