# Reproduce literal-regex pipeline performance

Use the configurable trace-pipeline benchmark in the companion
[benchmark proposal](https://github.com/githubnext/opentelemetry-collector-contrib/tree/oss/trace-pipeline-benchmark/benchmarks/tracebench).
Its public example includes literal-key replacement. It is an arbitrary
synthetic workload, not a production calibration.

From that benchmark directory, with this branch fetched into a contrib checkout:

```sh
python3 compare.py --candidate literal-regex \
  --repository /path/to/opentelemetry-collector-contrib \
  --base a1cd60723506d875a7a4a94b525b9615c11f08fb \
  --head refs/remotes/githubnext/oss/ottl-literal-regex \
  --pgo --blocks 8 --iterations 256 --output /tmp/tracebench-literal-regex
```

The runner changes only the three replacement/matching source files in a copy
of the pinned OTTL dependency. Both binaries use the same profile trained on
the unchanged public baseline. Raw CPU/allocated-byte observations, paired A/B
and same-binary A/A controls, module graphs and workload/stage/survival censuses
are retained. Add `--spec /path/to/scenario.json` for an external calibration;
iterations must be a whole multiple of the scenario bank size.

Run `make test` and `make lint` in the OTTL module as correctness checks.
Regression tests cover quoted literals, anchored/general/dynamic expressions,
Unicode and invalid patterns. Do not infer a fixed percentage, typical-workflow
benefit or production savings from an arbitrary public example.
