package replay

import (
	"context"
	"time"

	"go.keploy.io/server/v2/pkg/models"
)

type Instrumentation interface {
	//Setup prepares the environment for the recording
	Setup(ctx context.Context, cmd string, opts models.SetupOptions) (uint64, error)
	//Hook will load hooks and start the proxy server.
	Hook(ctx context.Context, id uint64, opts models.HookOptions) error
	MockOutgoing(ctx context.Context, id uint64, opts models.OutgoingOptions) error
	// SetMocks Allows for setting mocks between test runs for better filtering and matching
	SetMocks(ctx context.Context, id uint64, filtered []*models.Mock, unFiltered []*models.Mock) error
	// GetConsumedFilteredMocks to log the names of the mocks that were consumed during the test run of failed test cases
	GetConsumedFilteredMocks(ctx context.Context, id uint64) ([]string, error)
	// GetConsumedMocks returns all the names of mock which are used in the test run of a test set
	GetConsumedMocks(ctx context.Context, id uint64) (map[string][]string, error)
	// Run is blocking call and will execute until error
	Run(ctx context.Context, id uint64, opts models.RunOptions) models.AppError

	GetAppIP(ctx context.Context, id uint64) (string, error)
}

type Service interface {
	Start(ctx context.Context) error
	BootReplay(ctx context.Context) (string, uint64, context.CancelFunc, error)
	GetAllTestSetIDs(ctx context.Context) ([]string, error)
	RunTestSet(ctx context.Context, testSetID string, testRunID string, appID uint64, serveTest bool) (models.TestSetStatus, error)
	GetTestSetStatus(ctx context.Context, testRunID string, testSetID string) (models.TestSetStatus, error)
	RunApplication(ctx context.Context, appID uint64, opts models.RunOptions) models.AppError
	ProvideMocks(ctx context.Context) error
}

type TestDB interface {
	GetAllTestSetIDs(ctx context.Context) ([]string, error)
	GetTestCases(ctx context.Context, testSetID string) ([]*models.TestCase, error)
}

type MockDB interface {
	GetFilteredMocks(ctx context.Context, testSetID string, afterTime time.Time, beforeTime time.Time) ([]*models.Mock, error)
	GetUnFilteredMocks(ctx context.Context, testSetID string, afterTime time.Time, beforeTime time.Time) ([]*models.Mock, error)
	DeleteMocks(ctx context.Context, testSetID string, mockNames map[string]bool) error
}

type ReportDB interface {
	GetAllTestRunIDs(ctx context.Context) ([]string, error)
	GetTestCaseResults(ctx context.Context, testRunID string, testSetID string) ([]models.TestResult, error)
	GetReport(ctx context.Context, testRunID string, testSetID string) (*models.TestReport, error)
	InsertTestCaseResult(ctx context.Context, testRunID string, testSetID string, result *models.TestResult) error
	InsertReport(ctx context.Context, testRunID string, testSetID string, testReport *models.TestReport) error
}

type Telemetry interface {
	TestSetRun(success int, failure int, testSet string, runStatus string)
	TestRun(success int, failure int, testSets int, runStatus string)
	MockTestRun(utilizedMocks int)
}
