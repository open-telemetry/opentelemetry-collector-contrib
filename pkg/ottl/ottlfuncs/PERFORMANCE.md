# Reproduce literal-regex pipeline performance

Use the configurable trace-pipeline benchmark in the companion
[benchmark version 1.1.0](https://github.com/githubnext/opentelemetry-collector-contrib/tree/tracebench-v1.1.0/benchmarks/tracebench).
The runner and public GitHub-shaped workload share this version. Its public
normalization example includes literal-key replacement. The scenario is a
sanitized, production-informed synthetic model, not an exact production pipeline.

From that benchmark directory, with this branch fetched into a contrib checkout:

```sh
GOTOOLCHAIN=go1.26.0 python3 compare.py --candidate literal-regex \
  --repository /path/to/opentelemetry-collector-contrib \
  --base a1cd60723506d875a7a4a94b525b9615c11f08fb \
  --head 7ed811586d9311ed0ba40375b9dd290faff9ce60 \
  --workload github \
  --pgo --blocks 16 --iterations 192 --output /tmp/tracebench-literal-regex
```

The runner changes only the three replacement/matching source files in a copy
of the pinned OTTL dependency. Both binaries use the same profile trained on
the unchanged GitHub-shaped baseline. Raw CPU/allocated-byte observations, paired A/B
and same-binary A/A controls, module graphs and workload/stage/survival censuses
are retained. Add `--spec /path/to/scenario.json` for an external calibration;
iterations must be a whole multiple of the scenario bank size.

Run `make test` and `make lint` in the OTTL module as correctness checks.
Regression tests cover quoted literals, anchored/general/dynamic expressions,
Unicode and invalid patterns. Do not infer a fixed percentage, typical-workflow
benefit or production savings from an arbitrary public example.
