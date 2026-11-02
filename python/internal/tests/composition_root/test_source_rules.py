"""Source rules of REQ-ARC-006 and REQ-ARC-013: no global mutable state and no adapter that registers itself on import.

The rules read the sources with ``ast``. Each rule is a function of one source text, so the tests can show that it
finds what it must; a test then runs it over every file of the package. The environment is already checked by the
configuration tests (no ``os.environ``, no ``getenv``, no ``dotenv``).
"""

import ast
import pathlib
import re
import unittest
from typing import Optional

PYTHON_ROOT = pathlib.Path(__file__).resolve().parents[3]
SOURCE_ROOT = PYTHON_ROOT / "src" / "telemetry"

MUTABLE_CALLS = frozenset({"list", "dict", "set", "defaultdict", "deque", "OrderedDict", "Counter", "bytearray"})
MUTATING_METHODS = frozenset(
    {"append", "extend", "insert", "add", "update", "setdefault", "pop", "popitem", "remove", "clear", "discard"}
)
FORBIDDEN_AT_IMPORT = frozenset({"atexit", "entry_points", "iter_entry_points", "pkg_resources", "pkgutil"})

# Module-level names that hold a mutable container on purpose. Each one is a constant table written once by a
# literal; the rule "nothing mutates a module-level name" below proves it is never changed afterwards.
ALLOWED_MUTABLE_NAMES = {
    "__all__": "the list of the public names of a module, read by the import machinery",
    "_ESCAPES": "the escapes of canonical JSON: a table written by a literal, read only",
    "EVENT_VALIDATORS": "generated from the event schemas: a table written by a literal, read only",
    "_FLAG_VALUES": "the regular expression flags of the shared pattern: a table written by a literal, read only",
    "_FOLDED_LETTER": "the case folding of the shared pattern: a table written by a literal, read only",
    "ERROR_CODES": "generated from the error catalog: a table written by a literal, read only",
    "ENVIRONMENT_VARIABLES": "the names the consumer loads the configuration with: a table written by a literal",
}
# The one ContextVar of the library: the call context (REQ-ARC-006: each thread and task sees its own value).
CONTEXT_VAR_FILE = "internal/application/call_context/call_context.py"


def _assigned(node: ast.stmt) -> tuple[list[str], Optional[ast.expr]]:
    if isinstance(node, ast.Assign):
        return [target.id for target in node.targets if isinstance(target, ast.Name)], node.value
    if isinstance(node, ast.AnnAssign) and isinstance(node.target, ast.Name):
        return [node.target.id], node.value
    if isinstance(node, ast.AugAssign) and isinstance(node.target, ast.Name):
        return [node.target.id], node.value
    return [], None


def _call_name(call: ast.Call) -> str:
    if isinstance(call.func, ast.Name):
        return call.func.id
    if isinstance(call.func, ast.Attribute):
        return call.func.attr
    return ""


def _is_mutable_value(value: ast.expr) -> bool:
    if isinstance(value, (ast.List, ast.Dict, ast.Set, ast.ListComp, ast.DictComp, ast.SetComp)):
        return True
    return isinstance(value, ast.Call) and _call_name(value) in MUTABLE_CALLS


def module_state_violations(source: str, relative_path: str = "module.py") -> list[str]:
    """Return what breaks the rules of global state and of registration by import in one module."""
    tree = ast.parse(source)
    violations: list[str] = []
    module_names: set[str] = set()
    for node in tree.body:
        names, value = _assigned(node)
        module_names.update(names)
        if value is None:
            continue
        if isinstance(node, ast.AugAssign):
            violations.append(f"{relative_path}:{node.lineno}: module-level augmented assignment of {names}")
        if _is_mutable_value(value):
            for name in names:
                if name not in ALLOWED_MUTABLE_NAMES:
                    violations.append(f"{relative_path}:{node.lineno}: module-level mutable state {name!r}")
        if isinstance(value, ast.Call) and _call_name(value) == "ContextVar" and relative_path != CONTEXT_VAR_FILE:
            violations.append(f"{relative_path}:{node.lineno}: a ContextVar outside the call context")
    for node in ast.walk(tree):
        if isinstance(node, ast.Global):
            violations.append(f"{relative_path}:{node.lineno}: a global statement")
        violations.extend(_mutation_violations(node, module_names, relative_path))
        violations.extend(_registration_violations(node, relative_path))
    return violations


def _target_name(target: ast.expr) -> str:
    while isinstance(target, (ast.Subscript, ast.Attribute)):
        target = target.value
    return target.id if isinstance(target, ast.Name) else ""


def _mutation_violations(node: ast.AST, module_names: set[str], relative_path: str) -> list[str]:
    """A change of a module-level name from inside a function or a class is a registry in the making."""
    found: list[str] = []
    if isinstance(node, (ast.Assign, ast.AugAssign)):
        targets = node.targets if isinstance(node, ast.Assign) else [node.target]
        for target in targets:
            if isinstance(target, (ast.Subscript, ast.Attribute)) and _target_name(target) in module_names:
                found.append(f"{relative_path}:{node.lineno}: changes the module-level name {_target_name(target)!r}")
    if isinstance(node, ast.Delete):
        for target in node.targets:
            if isinstance(target, ast.Subscript) and _target_name(target) in module_names:
                found.append(f"{relative_path}:{node.lineno}: deletes from {_target_name(target)!r}")
    if (
        isinstance(node, ast.Call)
        and isinstance(node.func, ast.Attribute)
        and node.func.attr in MUTATING_METHODS
        and isinstance(node.func.value, ast.Name)
        and node.func.value.id in module_names
    ):
        found.append(f"{relative_path}:{node.lineno}: {node.func.attr} on the module-level name {node.func.value.id!r}")
    return found


def _registration_violations(node: ast.AST, relative_path: str) -> list[str]:
    found: list[str] = []
    if isinstance(node, ast.FunctionDef) and node.name in ("__init_subclass__", "__set_name__"):
        found.append(f"{relative_path}:{node.lineno}: {node.name} (a class that registers itself)")
    if isinstance(node, ast.ClassDef):
        for keyword in node.keywords:
            if keyword.arg == "metaclass":
                found.append(f"{relative_path}:{node.lineno}: a metaclass (a class that registers itself)")
    if isinstance(node, ast.Import):
        found.extend(
            f"{relative_path}:{node.lineno}: imports {alias.name}"
            for alias in node.names
            if alias.name.split(".")[0] in FORBIDDEN_AT_IMPORT
        )
    if isinstance(node, ast.ImportFrom):
        names = [node.module or "", *(alias.name for alias in node.names)]
        found.extend(
            f"{relative_path}:{node.lineno}: imports {name}"
            for name in names
            if name.split(".")[0] in FORBIDDEN_AT_IMPORT
        )
    if isinstance(node, ast.Subscript) and isinstance(node.value, ast.Attribute) and node.value.attr == "modules":
        if isinstance(node.ctx, (ast.Store, ast.Del)):
            found.append(f"{relative_path}:{node.lineno}: changes sys.modules")
    return found


def source_files() -> list[pathlib.Path]:
    """Every source file of the package."""
    paths = sorted(SOURCE_ROOT.rglob("*.py"))
    if not paths:
        raise AssertionError(f"no source files in {SOURCE_ROOT}")
    return paths


class ModuleStateRulesTest(unittest.TestCase):
    def test_no_module_of_the_package_has_global_mutable_state_or_registers_itself(self):
        for path in source_files():
            relative_path = path.relative_to(SOURCE_ROOT).as_posix()
            with self.subTest(file=relative_path):
                self.assertEqual(module_state_violations(path.read_text(encoding="utf-8"), relative_path), [])

    def test_the_allow_list_names_only_what_exists(self):
        present: set[str] = set()
        for path in source_files():
            for node in ast.parse(path.read_text(encoding="utf-8")).body:
                present.update(_assigned(node)[0])
        for name in ALLOWED_MUTABLE_NAMES:
            self.assertIn(name, present, f"{name} is in the allow-list and in no module")

    def test_there_is_exactly_one_context_var(self):
        found = []
        for path in source_files():
            tree = ast.parse(path.read_text(encoding="utf-8"))
            for node in tree.body:
                _, value = _assigned(node)
                if isinstance(value, ast.Call) and _call_name(value) == "ContextVar":
                    found.append(path.relative_to(SOURCE_ROOT).as_posix())
        self.assertEqual(found, [CONTEXT_VAR_FILE])


class RuleSelfTest(unittest.TestCase):
    """The rules find what they must: a rule that finds nothing proves nothing."""

    def assert_found(self, source: str, text: str) -> None:
        violations = module_state_violations(source)
        self.assertTrue(any(text in violation for violation in violations), f"{text!r} not in {violations}")

    def test_module_level_containers_are_found(self):
        self.assert_found("CACHE = {}\n", "'CACHE'")
        self.assert_found("items = []\n", "'items'")
        self.assert_found("seen: set = set()\n", "'seen'")
        self.assert_found("table = dict(a=1)\n", "'table'")
        self.assert_found("import collections\nqueue = collections.deque()\n", "'queue'")
        self.assert_found("squares = [n for n in range(3)]\n", "'squares'")
        self.assert_found("COUNT = 0\nCOUNT += 1\n", "augmented assignment")

    def test_read_only_values_are_not_found(self):
        source = "NAMES = ('a', 'b')\nKINDS = frozenset({'a'})\nTABLE = MappingProxyType({'a': 1})\nVALUE = 3\n"
        self.assertEqual(module_state_violations(source), [])

    def test_a_global_statement_and_a_registry_by_decorator_are_found(self):
        self.assert_found("x = 1\ndef set_it():\n    global x\n    x = 2\n", "a global statement")
        registry_source = "REGISTRY = ()\ndef register(function):\n    REGISTRY.append(function)\n    return function\n"
        self.assert_found(registry_source, "append on the module-level name 'REGISTRY'")
        self.assert_found("TABLE = ()\ndef register(key, value):\n    TABLE[key] = value\n", "changes the module-level")
        self.assert_found("TABLE = ()\ndef forget(key):\n    del TABLE[key]\n", "deletes from")

    def test_a_class_that_registers_itself_is_found(self):
        self.assert_found("class Base:\n    def __init_subclass__(cls):\n        pass\n", "__init_subclass__")
        self.assert_found("class Base(metaclass=type):\n    pass\n", "a metaclass")

    def test_the_machinery_of_registration_by_import_is_found(self):
        self.assert_found("import atexit\n", "imports atexit")
        self.assert_found("from importlib.metadata import entry_points\n", "imports entry_points")
        self.assert_found("import sys\nsys.modules['x'] = None\n", "changes sys.modules")

    def test_a_second_context_var_is_found(self):
        self.assert_found("import contextvars\nOTHER = contextvars.ContextVar('x')\n", "a ContextVar outside")
        self.assertEqual(
            module_state_violations("import contextvars\nOTHER = contextvars.ContextVar('x')\n", CONTEXT_VAR_FILE), []
        )


class PyprojectTest(unittest.TestCase):
    def test_the_library_has_no_runtime_dependency(self):
        text = (PYTHON_ROOT / "pyproject.toml").read_text(encoding="utf-8")
        project_table = re.search(r"^\[project\]\s(.*?)^\[", text, re.MULTILINE | re.DOTALL)
        self.assertIsNotNone(project_table)
        dependencies = re.search(r"^dependencies\s*=\s*\[(.*?)\]", project_table.group(1), re.MULTILINE | re.DOTALL)
        self.assertIsNotNone(dependencies, "the [project] table has no dependencies entry")
        self.assertEqual(dependencies.group(1).strip(), "")
        self.assertNotIn("dynamic", project_table.group(1))


if __name__ == "__main__":
    unittest.main()
