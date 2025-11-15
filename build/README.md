# Build Directory

This directory contains **all** build artifacts and is excluded from git (except this file).

## Structure

```
build/
├── bin/                    # Compiled binaries
│   ├── hermes                  # Main binary
│   ├── hermes-linux            # Linux binary
│   ├── hermes-migrate          # Database migration binary
│   └── hermes-notifier         # Notification service binary
├── coverage/               # Test coverage reports
│   ├── coverage.out            # General coverage data
│   ├── coverage.html           # HTML coverage report
│   ├── unit/                   # Unit test coverage
│   ├── integration/            # Integration test coverage
│   └── e2e/                    # E2E test coverage
├── reports/                # Test reports and artifacts
│   ├── e2e/                    # End-to-end test reports
│   │   ├── e2e-test-report-*.html
│   │   └── e2e-test-*.log
│   ├── integration/            # Integration test reports
│   │   ├── edge-sync-*.log
│   │   └── notifications-*.log
│   └── unit/                   # Unit test reports
│       └── test-results.xml
├── test/                   # Test outputs and temporary files
│   ├── integration.log         # Integration test output
│   ├── api_integration.log     # API integration test output
│   └── fixtures/               # Test fixtures and data
├── logs/                   # Runtime and debug logs
│   ├── hermes.log
│   ├── indexer.log
│   └── notifier.log
└── tmp/                    # Temporary build artifacts
    ├── *.md                    # Temporary test documents
    └── *.txt                   # Temporary tokens
```

## Usage

All build artifacts are automatically placed here by the Makefile and test scripts:

### Building
- `make build` - Builds all binaries to `build/bin/`
- `make clean` - Removes all build artifacts

### Testing
- `make test` - Runs tests
- `make test-coverage` - Runs tests with coverage to `build/coverage/`
- `make test-integration` - Runs integration tests with output to `build/test/`
- `./testing/test-comprehensive-e2e.sh` - Generates reports in `build/reports/e2e/`

### Viewing Results
- `open build/coverage/coverage.html` - View code coverage
- `open build/reports/e2e/e2e-test-report-*.html` - View E2E test report

## Git Exclusions

All contents of this directory (except README.md and .gitkeep) are excluded from git via `.gitignore`.

## Directory Purposes

- **bin/** - Compiled executable binaries ready for deployment or testing
- **coverage/** - Code coverage data and reports from unit/integration/e2e tests
- **reports/** - Structured test reports (HTML, XML, JSON) organized by test type
- **test/** - Test execution logs and temporary test data
- **logs/** - Runtime application logs for debugging
- **tmp/** - Ephemeral files created during builds/tests, safe to delete anytime
