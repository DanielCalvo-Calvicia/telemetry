# python

The Python implementation of the telemetry standard ([PR-001](../docs/proposals/PR-001-telemetry-standard-go-python.md)). Python 3.10 or newer. Only the skeleton exists: the package imports, the four layers are empty and the extras are empty lists; the capabilities arrive with the tasks of [docs/tasks](../docs/tasks/README.md).

## Install

It is installed from the Git repository, not from a package index (Q07-3). Replace the tag with a released one:

```bash
pip install "telemetry[sqlite] @ git+https://github.com/DanielCalvo-Calvicia/telemetry@python-vX.Y.Z#subdirectory=python"
```

From a local checkout: `pip install ./python`. The extras are `sqlite`, `otel`, `prometheus` and `ai`. The core has no third-party dependency.

## Layout

```text
pyproject.toml                          build backend: setuptools; extras; coverage and ruff settings
src/telemetry/                          the package; py.typed marks it as typed
  internal/{domain,application,infrastructure,composition_root}/    the layers of the Go core
internal/tests/                         the tests, mirroring the code tree (same folders as go/internal/tests)
```

## Tests

From this directory, in a virtual environment with the package installed:

```bash
python -m unittest discover -s internal/tests -t .
coverage run -m unittest discover -s internal/tests -t . && coverage report   # at least 95%
```
