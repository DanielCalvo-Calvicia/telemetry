"""Tests of the validators that the generator wrote for its own fixture contract.

The fixture contract uses every construct of the validation: the generated module is loaded from its golden file and run
over the shared tables of fixtures/event_validation/constructs, the same ones the Go test runs on the same rules.
"""

import unittest

from internal.tests.domain.operations.event_validation.helpers import load_cases, load_generated_fixture_validators

EVENT_TYPE = "demo.thing.happened"


class GeneratedFixtureValidatorsTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.generated = load_generated_fixture_validators()
        cls.validator = staticmethod(cls.generated.EVENT_VALIDATORS[EVENT_TYPE])

    def test_the_generated_module_registers_the_event_type(self):
        self.assertEqual(list(self.generated.EVENT_VALIDATORS), [EVENT_TYPE])

    def test_every_valid_construct_case_is_accepted(self):
        for case in load_cases("constructs/valid.json"):
            with self.subTest(case["name"]):
                problem = self.validator(case["data"], "data")
                self.assertIsNone(problem, None if problem is None else f"{problem.path}: {problem.reason}")

    def test_every_invalid_construct_case_gives_the_expected_field_path_and_reason(self):
        for case in load_cases("constructs/invalid.json"):
            with self.subTest(case["name"]):
                problem = self.validator(case["data"], "data")
                self.assertIsNotNone(problem)
                self.assertEqual((problem.path, problem.reason), (case["field_path"], case["reason"]))
                secret = case.get("secret")
                if secret is not None:
                    self.assertNotIn(secret, problem.path + problem.reason)

    def test_the_nested_validator_is_a_function_of_the_module(self):
        self.assertIsNone(self.generated.validate_resource({"service_name": "s"}, "data.resource"))
        problem = self.generated.validate_resource({}, "data.resource")
        self.assertEqual((problem.path, problem.reason), ("data.resource.service_name", "required field is missing"))
