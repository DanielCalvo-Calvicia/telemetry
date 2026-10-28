"""Reads the import lists of a Python package laid out in the layers of the standard and reports the imports that
break the direction of the dependencies (REQ-ARC-001, REQ-ARC-002, REQ-ARC-003). It uses only the standard ``ast``."""

import ast
import dataclasses
import pathlib
import sys

RULE_DOMAIN_ISOLATED = "domain-isolated"
RULE_APPLICATION_IMPORTS_ONLY_DOMAIN = "application-imports-only-domain"
RULE_INFRASTRUCTURE_NOT_COMPOSITION_ROOT = "infrastructure-not-composition-root"
RULE_ADAPTERS_WIRED_BY_COMPOSITION_ROOT = "adapters-wired-only-by-the-composition-root"
RULE_IO_ONLY_IN_EXTERNAL = "io-only-in-external"
RULE_CONSUMER_NEVER_IMPORTS_INTERNAL = "consumer-never-imports-internal"

LAYER_NAMES = ("domain", "application", "infrastructure", "composition_root")

# The modules that do I/O: network, files, processes, databases, and the common client libraries.
IO_MODULES = frozenset(
    {
        "http",
        "urllib",
        "socket",
        "ssl",
        "sqlite3",
        "os",
        "pathlib",
        "shutil",
        "subprocess",
        "tempfile",
        "requests",
        "httpx",
    }
)

# The third-party modules the domain never imports: telemetry backends and AI frameworks (REQ-ARC-002).
FORBIDDEN_FOR_DOMAIN = IO_MODULES | frozenset(
    {"opentelemetry", "prometheus_client", "langchain", "llama_index", "langfuse"}
)


@dataclasses.dataclass(frozen=True)
class Violation:
    """One import that breaks a rule."""

    rule: str
    file: str
    imported: str

    def message(self) -> str:
        """Name the rule, the offending file and the forbidden import."""
        return f"{self.rule}: {self.file} imports {self.imported!r}"


def check(root: pathlib.Path, package_name: str, examples_root: pathlib.Path) -> list[Violation]:
    """Apply every rule to the package in ``root`` (the folder that holds ``internal/``).

    ``package_name`` is the import name of the package; ``examples_root`` holds the consumers of the library.
    """
    internal_prefix = f"{package_name}.internal"
    violations: list[Violation] = []
    for layer_name in LAYER_NAMES:
        for file in _python_files_under(root / "internal" / layer_name):
            relative_file = file.relative_to(root).as_posix()
            for imported in _imports_of(file, root, package_name):
                violations.extend(_check_import(layer_name, relative_file, imported, internal_prefix))
    for file in _python_files_under(examples_root):
        for imported in _imports_of(file, examples_root, "examples"):
            if _under(imported, internal_prefix):
                violations.append(
                    Violation(
                        RULE_CONSUMER_NEVER_IMPORTS_INTERNAL,
                        file.relative_to(examples_root.parent).as_posix(),
                        imported,
                    )
                )
    return violations


def of_rule(violations: list[Violation], rule: str) -> list[Violation]:
    """Keep the violations of one rule."""
    return [violation for violation in violations if violation.rule == rule]


def _python_files_under(folder: pathlib.Path) -> list[pathlib.Path]:
    if not folder.is_dir():
        return []
    return sorted(folder.rglob("*.py"))


def _imports_of(file: pathlib.Path, root: pathlib.Path, package_name: str) -> list[str]:
    """Return the absolute names a file imports, resolving the relative imports against its own package."""
    tree = ast.parse(file.read_text(encoding="utf-8"), filename=str(file))
    package_parts = [package_name, *file.relative_to(root).parent.parts]
    imported: list[str] = []
    for node in ast.walk(tree):
        if isinstance(node, ast.Import):
            imported.extend(alias.name for alias in node.names)
        elif isinstance(node, ast.ImportFrom):
            if node.level:
                base_parts = package_parts[: len(package_parts) - (node.level - 1)]
                module_name = ".".join([*base_parts, *([node.module] if node.module else [])])
            else:
                module_name = node.module or ""
            imported.append(module_name)
            imported.extend(f"{module_name}.{alias.name}" for alias in node.names)
    return imported


def _under(imported: str, base: str) -> bool:
    return imported == base or imported.startswith(base + ".")


def _top_level(imported: str) -> str:
    return imported.split(".", 1)[0]


def _is_standard_library(imported: str) -> bool:
    return _top_level(imported) in sys.stdlib_module_names


def _check_import(layer_name: str, relative_file: str, imported: str, internal_prefix: str) -> list[Violation]:
    violations: list[Violation] = []

    def add(rule: str) -> None:
        violations.append(Violation(rule, relative_file, imported))

    def layer(name: str) -> str:
        return f"{internal_prefix}.{name}"

    if layer_name == "domain":
        other_layer = any(
            _under(imported, layer(name)) for name in ("application", "infrastructure", "composition_root")
        )
        if other_layer or _top_level(imported) in FORBIDDEN_FOR_DOMAIN:
            add(RULE_DOMAIN_ISOLATED)
    elif layer_name == "application":
        allowed = (
            _is_standard_library(imported)
            or _under(imported, layer("domain"))
            or _under(imported, layer("application"))
        )
        if not allowed:
            add(RULE_APPLICATION_IMPORTS_ONLY_DOMAIN)
    elif layer_name == "infrastructure":
        if _under(imported, layer("composition_root")):
            add(RULE_INFRASTRUCTURE_NOT_COMPOSITION_ROOT)
        in_adapter = relative_file.startswith("internal/infrastructure/outbound/")
        if in_adapter and "/external/" not in relative_file and _top_level(imported) in IO_MODULES:
            add(RULE_IO_ONLY_IN_EXTERNAL)
    inside_adapters = layer_name == "infrastructure" and relative_file.startswith("internal/infrastructure/outbound/")
    if layer_name != "composition_root" and not inside_adapters and _under(imported, layer("infrastructure.outbound")):
        add(RULE_ADAPTERS_WIRED_BY_COMPOSITION_ROOT)
    return violations
