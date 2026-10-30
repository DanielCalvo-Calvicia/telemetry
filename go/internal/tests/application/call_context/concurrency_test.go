package call_context_test

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"testing"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/call_context"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/tests/fakes"
)

func Test_concurrent_requests_never_see_each_others_values(t *testing.T) {
	const goroutine_count = 64
	id_generator := fakes.NewFakeIDGenerator()
	var wait_group sync.WaitGroup
	failures := make(chan string, goroutine_count)

	for goroutine_number := 0; goroutine_number < goroutine_count; goroutine_number++ {
		wait_group.Add(1)
		go func() {
			defer wait_group.Done()
			execution_id := fmt.Sprintf("run-%d", goroutine_number)
			request_context, request_error := call_context.NewRequestContext(context.Background(), id_generator, "app", "instance")
			if request_error != nil {
				failures <- request_error.Error()
				return
			}
			base_call_context, _ := call_context.FromContext(request_context)
			own_call_context, build_error := call_context.NewCallContext(
				call_context.WithApplication(base_call_context.Application()),
				call_context.WithInstanceID(base_call_context.InstanceID()),
				call_context.WithCorrelationID(base_call_context.CorrelationID()),
				call_context.WithExecutionID(execution_id),
			)
			if build_error != nil {
				failures <- build_error.Error()
				return
			}
			ctx := call_context.WithCallContext(request_context, own_call_context)

			depth_steps := rand.Intn(50)
			for step := 0; step < depth_steps; step++ {
				ctx, _ = call_context.NextIndentation(ctx)
				if step%7 == 0 {
					check_call_context, _ := call_context.FromContext(ctx)
					if check_call_context.ExecutionID() != execution_id {
						failures <- fmt.Sprintf("goroutine %d saw execution id %q", goroutine_number, check_call_context.ExecutionID())
						return
					}
				}
			}

			final_call_context, found := call_context.FromContext(ctx)
			if !found || final_call_context.ExecutionID() != execution_id ||
				final_call_context.CorrelationID() != base_call_context.CorrelationID() ||
				final_call_context.Indentation() != call_context.StartingIndentation+depth_steps {
				failures <- fmt.Sprintf("goroutine %d read back %+v after %d steps", goroutine_number, final_call_context, depth_steps)
			}
		}()
	}
	wait_group.Wait()
	close(failures)

	for failure := range failures {
		t.Error(failure)
	}
}
