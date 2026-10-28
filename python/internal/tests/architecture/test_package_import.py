"""Checks that the package and its four layers import with no third-party package installed."""

import importlib
import unittest

LAYER_NAMES = ("domain", "application", "infrastructure", "composition_root")


class PackageImportTest(unittest.TestCase):
    def test_the_package_imports(self):
        package = importlib.import_module("telemetry")
        self.assertTrue(package.__doc__)

    def test_each_layer_imports(self):
        for layer_name in LAYER_NAMES:
            with self.subTest(layer=layer_name):
                layer = importlib.import_module(f"telemetry.internal.{layer_name}")
                self.assertTrue(layer.__doc__)
