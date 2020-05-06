# Oasis Test Runner

`oasis-test-runner` initializes and executes end-to-end and remote signer test
suites.

To list all supported tests and corresponding parameters, type:

```bash
oasis-test-runner list
```

If no flags provided, all tests are executed. If you want to run specific test,
pass `--test` parameter:

```bash
oasis-test-runner --test e2e/runtime/runtime-dynamic
```

## Benchmarking

To benchmark tests, set the `--metrics.address` parameter to the address of the
Prometheus push gateway along with `--metrics.interval`. Additionally, you
can set test-specific parameters and the number of runs each test should be run
with `--num_runs` parameter.

## Benchmark analysis with `oasis-test-runner cmp` command

`cmp` command connects to Prometheus server instance containing benchmark
results generated by `oasis-test-runner` and corresponding `oasis-node`
workers. It compares the benchmark results of the last (`source`) and
pre-last (`target`) test batch (also called *instance*). You need to pass the
address of Prometheus query server using `--metrics.address` flag:

```bash
oasis-test-runner cmp \
  --metrics.address http://prometheus.myorg.com:9090
```

By default `cmp` command will fetch the results of all tests and metrics
supported by `oasis-test-runner`. If you want to compare specific metric(s) and
test(s), provide `--metrics` and `--test` flags respectively:

```bash
oasis-test-runner cmp \
  --metrics time \
  --test e2e/runtime/multiple-runtimes
```

`cmp` takes *thresholds* for each metric into account. Currently, `avg_ratio`
and `max_ratio` are supported which correspond to the average and maximum ratio
of metric values of all test runs in the benchmark batch. For each ratio, you
can set the `max_threshold` and `min_threshold`. The former requires that the
ratio should not be exceeded and the latter that the provided threshold must be
reached. If not, `oasis-test-runner cmp` will exit with error code. This is
useful for integration into CI pipelines. Thresholds can be using the flag
`--{min|max}_threshold.<metric name>.{avg|max}_ratio`.

For example:

```bash
oasis-test-runner cmp \
  --metrics time \
  --test e2e/runtime/multiple-runtimes \
  --max_threshold.time.avg_ratio 1.1
```

will require that the average duration of the test in the last benchmark batch
should be at most 10\% slower than from the pre-last benchmark batch.

If you are developing or improving a feature in a separate git branch, you will
want to perform benchmarks and compare the results of your branch to the ones
from the master branch. `oasis-test-runner` automatically sends information of
the git branch it was compiled on to Prometheus, so all you need to do is
perform benchmarks on both master and feature branches using the same Prometheus
instance. Then, pass the name of source and target branch names to `cmp` and it
will compare the last benchmark batch from each git branch respectively:

```bash
oasis-test-runner cmp \
  --metrics.source.git_branch=feature_branch \
  --metrics.target.git_branch=master
```

For detailed help, run:

```bash
oasis-test-runner cmp --help
```