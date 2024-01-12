package infraoperator

import (
	"context"
	"testing"
)

type TestingServiceImplementsNothing struct{}

type TestingServiceImplementsStarter struct {
	startCalled bool
}

type TestingServiceImplementsStopper struct {
	stopCalled bool
}

type TestingServiceImplementsChecker struct {
	checkCalled bool
}

type TestingServiceImplementsEverything struct {
	TestingServiceImplementsStarter
	TestingServiceImplementsStopper
	TestingServiceImplementsChecker
}

func (ts *TestingServiceImplementsStarter) Start(ctx context.Context) error {
	ts.startCalled = true
	return nil
}
func (ts *TestingServiceImplementsStopper) Stop(ctx context.Context) error {
	ts.stopCalled = true
	return nil
}
func (ts *TestingServiceImplementsChecker) Check(ctx context.Context) error {
	ts.checkCalled = true
	return nil
}

func Test_operator_AddService(t *testing.T) {
	type testcase struct {
		name          string
		service       interface{}
		stoppersCount int
		checkersCount int
	}

	testcases := []testcase{
		{"nothing", &TestingServiceImplementsNothing{}, 0, 0},
		{"starter", &TestingServiceImplementsStarter{}, 0, 0},
		{"stopper", &TestingServiceImplementsStopper{}, 1, 0},
		{"checker", &TestingServiceImplementsChecker{}, 0, 1},
		{"everything", &TestingServiceImplementsEverything{}, 1, 1},
	}

	for _, tc := range testcases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			op := Operator{}

			if err := op.AddService(ctx, tc.service); err != nil {
				t.Error(err)
			}

			if len(op.stoppers) != tc.stoppersCount {
				t.Errorf("expected %d stoppers, got %d", tc.stoppersCount, len(op.stoppers))
			}

			if len(op.checkers) != tc.checkersCount {
				t.Errorf("expected %d checkers, got %d", tc.checkersCount, len(op.checkers))
			}
		})
	}
}
