package domain_test

import (
	"errors"
	"testing"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain"
)

func TestJoinErrorsKeepsEveryErrorAndIgnoresNil(test *testing.T) {
	if domain.JoinErrors() != nil || domain.JoinErrors(nil, nil) != nil {
		test.Fatal("joining nothing must give nil")
	}
	joined := domain.JoinErrors(nil, domain.ErrEventIDInvalid, domain.ErrTimeZoneInvalid)
	if !errors.Is(joined, domain.ErrEventIDInvalid) || !errors.Is(joined, domain.ErrTimeZoneInvalid) {
		test.Fatalf("a joined error lost a cause: %v", joined)
	}
}

func TestTheValueObjectSentinelsAreDistinctAndDescribeTheirRule(test *testing.T) {
	sentinels := []error{
		domain.ErrSeverityInvalid, domain.ErrActorKindInvalid, domain.ErrEventIDInvalid,
		domain.ErrResourceServiceNameInvalid, domain.ErrResourceInstanceIDInvalid, domain.ErrResourceEnvironmentInvalid,
		domain.ErrResourceServiceVersionInvalid, domain.ErrResourceHostInvalid, domain.ErrTimeZoneInvalid,
		domain.ErrActorIDInvalid, domain.ErrActorTypeInvalid, domain.ErrActorVersionInvalid,
		domain.ErrContextTraceIDInvalid, domain.ErrContextSpanIDInvalid, domain.ErrContextExecutionIDInvalid,
		domain.ErrContextCorrelationIDInvalid, domain.ErrAttributeKeyInvalid, domain.ErrAttributeValueInvalid,
	}
	for position, sentinel := range sentinels {
		if sentinel == nil || sentinel.Error() == "" {
			test.Fatalf("the sentinel %d has no message", position)
		}
		for other_position, other := range sentinels {
			if position != other_position && errors.Is(sentinel, other) {
				test.Errorf("%v and %v are the same sentinel", sentinel, other)
			}
		}
	}
}
