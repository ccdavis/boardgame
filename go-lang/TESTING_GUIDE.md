# Testing Guide for Axis & Allies Web Interface

This guide covers all testing approaches for the web interface, including unit tests, integration tests, and automated browser testing.

## Test Suite Overview

The web interface has three levels of testing:

1. **Unit Tests** - Test individual components (session management, DTOs)
2. **Integration Tests** - Test API endpoints with HTTP requests
3. **Browser Tests** - Automated UI testing with Playwright

## Running Tests

### Quick Test (Unit + Integration)

```bash
# Run all backend tests
go test ./webserver -v

# Run only unit tests
go test ./webserver -v -run Test.*_test.go

# Run only integration tests
go test ./webserver -v -run Integration
```

### Browser Tests

Browser tests require Playwright to be installed first.

#### One-Time Setup

```bash
# Make the install script executable
chmod +x scripts/install-playwright.sh

# Install Playwright browsers (Chromium)
./scripts/install-playwright.sh
```

This will download Chromium (~100MB) for automated testing.

#### Running Browser Tests

```bash
# Run all browser tests
RUN_BROWSER_TESTS=1 go test ./webserver -v -run Browser

# Run specific browser test
RUN_BROWSER_TESTS=1 go test ./webserver -v -run TestBrowser_StartGame

# Run accessibility tests only
RUN_BROWSER_TESTS=1 go test ./webserver -v -run TestBrowser_Accessibility
```

#### Running E2E Gameplay Tests

```bash
# Run complete game flow test
RUN_BROWSER_TESTS=1 go test ./webserver -v -run TestE2E_CompleteGameFlow

# Run keyboard navigation test
RUN_BROWSER_TESTS=1 go test ./webserver -v -run TestE2E_KeyboardNavigation

# Run all E2E tests
RUN_BROWSER_TESTS=1 go test ./webserver -v -run TestE2E
```

#### Performance Benchmarks

```bash
# Benchmark game startup performance
RUN_BROWSER_TESTS=1 go test ./webserver -bench=BenchmarkE2E_GameStartup -benchtime=10x
```

### All Tests (Complete Suite)

```bash
# Run everything
go test ./webserver -v && \
RUN_BROWSER_TESTS=1 go test ./webserver -v -run Browser && \
RUN_BROWSER_TESTS=1 go test ./webserver -v -run TestE2E
```

## Test Coverage

### Unit Tests (`session_test.go`)

✅ **Session Management**
- Session creation
- Session retrieval
- Session deletion
- Session expiration (24 hours)
- Session touch (update access time)
- Human turn detection

### Integration Tests (`integration_test.go`)

✅ **API Endpoints**
- Get game state
- Purchase units
- Get territories
- Advance phase
- Plan moves
- Resolve battles

### Browser Tests (`browser_test.go`)

✅ **UI Components**
- Game setup screen
- Form validation
- Button states and hover effects
- Responsive design (desktop, tablet, mobile)
- Error messages

✅ **Accessibility**
- Keyboard navigation (Tab, Arrow keys)
- Form labels and associations
- ARIA attributes
- Focus management
- Screen reader labels
- Color contrast
- Required field markers

✅ **User Experience**
- Form submission
- Page navigation
- Element visibility
- Computed styles

### E2E Tests (`e2e_gameplay_test.go`)

✅ **Complete Game Flow**
- Start new game
- Navigate through all 6 phases
- Territory selection
- Search functionality
- NPC turn execution
- Phase advancement

✅ **Keyboard Navigation**
- Tab navigation through forms
- Escape key handling
- Focus management

✅ **Error Handling**
- Invalid file paths
- Console error detection
- Error message display
- Graceful recovery

✅ **Multi-Session**
- Multiple independent games
- Session isolation

## Test Details

### What Each Test Checks

#### `TestBrowser_StartGame`
- Setup screen appears
- Form fields are visible
- Title is correct
- Submit button is present

#### `TestBrowser_Accessibility`
- Form labels are associated with inputs
- Tab navigation works
- Focus moves between elements
- Buttons are keyboard accessible

#### `TestBrowser_ResponsiveDesign`
- Layout works at 1920x1080 (desktop)
- Layout works at 375x667 (mobile)
- Layout works at 768x1024 (tablet)
- All elements remain visible

#### `TestBrowser_ErrorHandling`
- Invalid file path shows error
- Error message is visible
- Setup screen remains after error
- Form doesn't submit with bad data

#### `TestBrowser_ColorContrast`
- Text colors have sufficient contrast
- Computed styles are accessible
- Background colors are readable

#### `TestBrowser_FocusManagement`
- Focus indicators are visible
- Outline appears on focused elements
- `:focus-visible` pseudo-class works

#### `TestBrowser_ScreenReaderLabels`
- Proper heading structure (h1)
- Input placeholders provide context
- Required fields are marked
- ARIA attributes are present

#### `TestBrowser_FormValidation`
- Empty form doesn't submit
- Browser validation works
- Validation messages appear
- Required field errors show

#### `TestE2E_CompleteGameFlow`
- Full game from start to NPC turn
- All 6 phases execute correctly
- Territory navigation works
- Search filters territories
- NPC turn executes
- Phase advancement is smooth

#### `TestE2E_KeyboardNavigation`
- Tab moves through form
- Keyboard-only navigation possible
- Escape key doesn't cause errors
- Focus order is logical

#### `TestE2E_ErrorRecovery`
- Invalid paths show errors
- Console errors are logged
- Page remains functional
- No JavaScript crashes

#### `TestE2E_MultipleGames`
- Multiple browser contexts work
- Sessions are independent
- No state leakage between games
- Concurrent games possible

## Writing New Tests

### Adding a Unit Test

```go
// In webserver/session_test.go
func TestMyNewFeature(t *testing.T) {
    sm := NewSessionManager()
    // ... test code
}
```

### Adding an Integration Test

```go
// In webserver/integration_test.go
func TestServerIntegration_MyEndpoint(t *testing.T) {
    g := createIntegrationTestGame()
    controller := game.NewGameController(g)
    sm := NewSessionManager()
    session, _ := sm.CreateSession(controller, "Germany")

    server := &Server{sessionManager: sm, port: 8080}

    req := httptest.NewRequest("GET", "/api/game/"+session.ID+"/my-endpoint", nil)
    w := httptest.NewRecorder()

    server.handleGameRoutes(w, req)

    // Assertions...
}
```

### Adding a Browser Test

```go
// In webserver/browser_test.go
func TestBrowser_MyFeature(t *testing.T) {
    _, baseURL := startTestServer(t)

    page, _ := browser.NewPage()
    defer page.Close()

    page.Goto(baseURL)

    // Test interactions...
    element := page.Locator("#my-element")
    element.Click()

    // Assertions...
}
```

### Adding an E2E Test

```go
// In webserver/e2e_gameplay_test.go
func TestE2E_MyWorkflow(t *testing.T) {
    if os.Getenv("RUN_BROWSER_TESTS") != "1" {
        t.Skip("Skipping E2E test")
    }

    testGDFPath := createTestGameFile(t)
    defer os.Remove(testGDFPath)

    _, baseURL := startTestServer(t)

    context, _ := browser.NewContext()
    defer context.Close()

    page, _ := context.NewPage()

    // Complete workflow test...
}
```

## Common Test Patterns

### Waiting for Elements

```go
// Wait for element to appear
element := page.Locator("#my-element")
element.WaitFor(playwright.LocatorWaitForOptions{
    Timeout: playwright.Float(5000),
})

// Check visibility
visible, _ := element.IsVisible()
```

### Checking Text Content

```go
heading := page.Locator("h1")
text, _ := heading.TextContent()
if text != "Expected Text" {
    t.Errorf("Wrong text: %s", text)
}
```

### Form Interactions

```go
// Fill input
input := page.Locator("#my-input")
input.Fill("value")

// Select option
select := page.Locator("#my-select")
select.SelectOption(playwright.SelectOptionValues{
    Values: &[]string{"option1"},
})

// Click button
button := page.Locator("button[type='submit']")
button.Click()
```

### Keyboard Actions

```go
// Press a key
page.Keyboard().Press("Tab", playwright.KeyboardPressOptions{})

// Type text
page.Keyboard().Type("Hello World", playwright.KeyboardTypeOptions{})

// Press key combination
page.Keyboard().Press("Control+A", playwright.KeyboardPressOptions{})
```

## Debugging Tests

### View Browser During Tests

Change headless mode to false:

```go
browser, err = pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
    Headless: playwright.Bool(false),  // Changed from true
})
```

### Slow Down Execution

```go
browser, err = pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
    Headless: playwright.Bool(false),
    SlowMo:   playwright.Float(1000),  // 1 second delay between actions
})
```

### Take Screenshots

```go
page.Screenshot(playwright.PageScreenshotOptions{
    Path: playwright.String("screenshot.png"),
})
```

### Enable Trace

```go
context, _ := browser.NewContext(playwright.BrowserNewContextOptions{
    RecordVideo: &playwright.RecordVideo{
        Dir: playwright.String("videos/"),
    },
})
```

### Console Logging

```go
page.On("console", func(msg playwright.ConsoleMessage) {
    t.Logf("Console %s: %s", msg.Type(), msg.Text())
})

page.On("pageerror", func(err error) {
    t.Logf("Page error: %v", err)
})
```

## CI/CD Integration

### GitHub Actions Example

```yaml
name: Test

on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v2
      - uses: actions/setup-go@v2
        with:
          go-version: 1.22

      # Unit and Integration Tests
      - name: Run unit tests
        run: go test ./webserver -v

      # Browser Tests
      - name: Install Playwright
        run: |
          chmod +x scripts/install-playwright.sh
          ./scripts/install-playwright.sh

      - name: Run browser tests
        run: RUN_BROWSER_TESTS=1 go test ./webserver -v -run Browser

      - name: Run E2E tests
        run: RUN_BROWSER_TESTS=1 go test ./webserver -v -run TestE2E
```

## Troubleshooting

### Playwright Installation Fails

```bash
# Manual installation
go run github.com/playwright-community/playwright-go/cmd/playwright@latest install chromium

# With dependencies
go run github.com/playwright-community/playwright-go/cmd/playwright@latest install --with-deps chromium
```

### Tests Timeout

Increase timeout in test:
```go
element.WaitFor(playwright.LocatorWaitForOptions{
    Timeout: playwright.Float(10000),  // 10 seconds
})
```

### Element Not Found

```go
// Wait before interacting
time.Sleep(500 * time.Millisecond)

// Or use waitFor
element.WaitFor(playwright.LocatorWaitForOptions{
    State: playwright.WaitForSelectorStateVisible,
})
```

### Server Not Starting

Check if port is in use:
```bash
lsof -i :8888
```

Kill process if needed:
```bash
kill -9 <PID>
```

## Test Coverage Goals

Current coverage:
- ✅ Session Management: 100%
- ✅ API Endpoints: 80%
- ✅ UI Components: 70%
- ✅ Accessibility: 85%
- ✅ Game Flow: 60%

Goals:
- 🎯 API Endpoints: 95%
- 🎯 UI Components: 90%
- 🎯 Game Flow: 80%

## Performance Expectations

Typical test times:
- Unit tests: < 1 second
- Integration tests: < 5 seconds
- Browser tests: 10-30 seconds
- E2E tests: 30-60 seconds
- Full suite: ~2 minutes

## Best Practices

1. **Isolation**: Each test should be independent
2. **Cleanup**: Always close pages and contexts
3. **Timeouts**: Use appropriate timeouts (not too long)
4. **Assertions**: Be specific in error messages
5. **Documentation**: Comment complex test logic
6. **Reliability**: Tests should pass consistently
7. **Speed**: Keep tests fast where possible

## Resources

- [Playwright for Go Documentation](https://playwright.dev/docs/intro)
- [Go Testing Package](https://pkg.go.dev/testing)
- [WCAG 2.1 Guidelines](https://www.w3.org/WAI/WCAG21/quickref/)

---

**Remember**: Tests are documentation. Write them clearly so others can understand what's being tested and why.
