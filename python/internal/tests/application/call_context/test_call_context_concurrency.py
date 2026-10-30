"""Two calls at the same time never see each other's values: threads and asyncio tasks (Q07-7)."""

import asyncio
import random
import threading
import unittest

from internal.tests.fakes import FakeIdGenerator
from telemetry.internal.application.call_context.call_context import (
    CallContext,
    current_call_context,
    deeper_indentation,
    use_call_context,
)
from telemetry.internal.application.call_context.request_context import start_request

THREAD_COUNT = 64


class ThreadsTest(unittest.TestCase):
    def test_no_thread_sees_the_values_of_another(self):
        generator = FakeIdGenerator()
        barrier = threading.Barrier(THREAD_COUNT)
        problems: list[str] = []
        lock = threading.Lock()

        def run(number: int) -> None:
            try:
                result = start_request(generator, "app", f"instance-{number}")
                call_context = CallContext(
                    application=result.call_context.application,
                    instance_id=result.call_context.instance_id,
                    execution_id=f"run-{number}",
                    correlation_id=result.call_context.correlation_id,
                )
                depth = random.Random(number).randint(0, 20)
                barrier.wait()
                with use_call_context(call_context):
                    stack = []
                    for _ in range(depth):
                        manager = deeper_indentation()
                        manager.__enter__()
                        stack.append(manager)
                        seen = current_call_context()
                        if seen.execution_id != f"run-{number}" or seen.instance_id != f"instance-{number}":
                            raise AssertionError("another thread's values")
                    seen = current_call_context()
                    if seen.indentation != 1 + depth or seen.correlation_id != call_context.correlation_id:
                        raise AssertionError(f"indentation {seen.indentation}, want {1 + depth}")
                    while stack:
                        stack.pop().__exit__(None, None, None)
                    if current_call_context() is not call_context:
                        raise AssertionError("the context was not restored")
                if current_call_context() is not None:
                    raise AssertionError("the context was not cleared")
            except Exception as error:  # reported to the main thread
                with lock:
                    problems.append(f"thread {number}: {error!r}")

        threads = [threading.Thread(target=run, args=(number,)) for number in range(THREAD_COUNT)]
        for thread in threads:
            thread.start()
        for thread in threads:
            thread.join()
        self.assertEqual(problems, [])

    def test_a_new_thread_does_not_inherit_the_context(self):
        seen = []
        with use_call_context(CallContext(application="app")):
            thread = threading.Thread(target=lambda: seen.append(current_call_context()))
            thread.start()
            thread.join()
        self.assertEqual(seen, [None])


class AsyncioTest(unittest.TestCase):
    def test_tasks_do_not_see_each_other_and_inherit_without_modifying(self):
        parent = CallContext(application="parent", execution_id="parent-run")

        async def worker(number: int) -> tuple:
            inherited = current_call_context()
            with use_call_context(CallContext(application="task", execution_id=f"task-{number}")):
                with deeper_indentation():
                    await asyncio.sleep(random.Random(number).random() / 100)
                    seen = current_call_context()
            return inherited, seen, current_call_context()

        async def main() -> list:
            with use_call_context(parent):
                results = await asyncio.gather(*(worker(number) for number in range(16)))
                after = current_call_context()
            return [results, after]

        results, after = asyncio.run(main())
        self.assertIs(after, parent)
        for number, (inherited, seen, restored) in enumerate(results):
            self.assertIs(inherited, parent)
            self.assertEqual((seen.execution_id, seen.indentation), (f"task-{number}", 2))
            self.assertIs(restored, parent)
        self.assertEqual(parent.indentation, 1)


if __name__ == "__main__":
    unittest.main()
