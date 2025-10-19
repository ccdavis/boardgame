package webserver

import (
	"fmt"
	"testing"
	"time"

	"github.com/playwright-community/playwright-go"
)

// TestMapAlignmentDiagnostic checks if clickable circles align with actual territory locations
func TestMapAlignmentDiagnostic(t *testing.T) {
	// Start the web server
	server := NewServer(8889)
	go func() {
		if err := server.Start(); err != nil {
			t.Logf("Server error: %v", err)
		}
	}()
	time.Sleep(500 * time.Millisecond)

	pw, err := playwright.Run()
	if err != nil {
		t.Fatalf("Could not start playwright: %v", err)
	}
	defer pw.Stop()

	browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(false), // Show browser for visual debugging
	})
	if err != nil {
		t.Fatalf("Could not launch browser: %v", err)
	}
	defer browser.Close()

	// Test at multiple viewport sizes to check for drift
	viewportSizes := []struct {
		width  int
		height int
		name   string
	}{
		{1920, 1080, "1920x1080"},
		// Uncomment these after fixing initial issues
		// {1366, 768, "1366x768"},
		// {1024, 768, "1024x768"},
	}

	for _, vp := range viewportSizes {
		t.Run(vp.name, func(t *testing.T) {
			context, err := browser.NewContext(playwright.BrowserNewContextOptions{
				Viewport: &playwright.Size{
					Width:  vp.width,
					Height: vp.height,
				},
			})
			if err != nil {
				t.Fatalf("Could not create context: %v", err)
			}
			defer context.Close()

			page, err := context.NewPage()
			if err != nil {
				t.Fatalf("Could not create page: %v", err)
			}

			// Navigate to the game
			if _, err := page.Goto("http://localhost:8889"); err != nil {
				t.Fatalf("Could not navigate: %v", err)
			}

			// Wait for page to load
			page.WaitForSelector("#gdfPath", playwright.PageWaitForSelectorOptions{
				Timeout: playwright.Float(5000),
			})

			// Fill in GDF path (relative to webserver directory)
			gdfInput := page.Locator("#gdfPath")
			if err := gdfInput.Fill("../../aaa.gdf"); err != nil {
				t.Fatalf("Failed to fill GDF path: %v", err)
			}

			// Select country
			playerSelect := page.Locator("#playerName")
			if _, err := playerSelect.SelectOption(playwright.SelectOptionValues{
				Values: &[]string{"Germany"},
			}); err != nil {
				t.Fatalf("Failed to select player: %v", err)
			}

			// Click Start Game button
			startBtn := page.Locator("button[type='submit']")
			if err := startBtn.Click(); err != nil {
				t.Fatalf("Failed to click start button: %v", err)
			}

			// Wait for game screen and map to load
			page.WaitForSelector("#gameMap", playwright.PageWaitForSelectorOptions{
				Timeout: playwright.Float(10000),
			})
			time.Sleep(2 * time.Second)

			// Make circles highly visible for diagnostic purposes
			page.Evaluate(`() => {
				const style = document.createElement('style');
				style.textContent = '.map-overlay circle { fill: rgba(255, 0, 0, 0.5) !important; stroke: yellow !important; stroke-width: 0.5 !important; opacity: 1 !important; } .map-overlay circle:hover { fill: rgba(0, 255, 0, 0.7) !important; }';
				document.head.appendChild(style);
			}`, nil)

			// Get detailed alignment information
			alignmentInfo, _ := page.Evaluate(`() => {
				const img = document.getElementById('gameMap');
				const overlay = document.getElementById('mapOverlay');
				const wrapper = document.querySelector('.map-wrapper');

				if (!img || !overlay) return null;

				const imgRect = img.getBoundingClientRect();
				const overlayRect = overlay.getBoundingClientRect();
				const wrapperRect = wrapper.getBoundingClientRect();

				// Get some sample circles to check their positions
				const circles = overlay.querySelectorAll('circle[data-territory]');
				const sampleCircles = [];

				// Get a few well-known territories to check
				const testTerritories = ['Berlin', 'Moscow', 'London', 'Tokyo', 'Washington', 'Rome'];
				testTerritories.forEach(name => {
					const circle = overlay.querySelector('circle[data-territory="' + name + '"]');
					if (circle) {
						const cx = parseFloat(circle.getAttribute('cx'));
						const cy = parseFloat(circle.getAttribute('cy'));
						const r = parseFloat(circle.getAttribute('r'));
						const circleRect = circle.getBoundingClientRect();

						sampleCircles.push({
							territory: name,
							svgCoords: { cx, cy, r },
							screenCoords: {
								centerX: circleRect.x + circleRect.width / 2,
								centerY: circleRect.y + circleRect.height / 2,
								width: circleRect.width,
								height: circleRect.height
							}
						});
					}
				});

				return {
					viewport: {
						width: window.innerWidth,
						height: window.innerHeight
					},
					image: {
						naturalWidth: img.naturalWidth,
						naturalHeight: img.naturalHeight,
						displayWidth: img.offsetWidth,
						displayHeight: img.offsetHeight,
						boundingBox: {
							x: imgRect.x,
							y: imgRect.y,
							width: imgRect.width,
							height: imgRect.height
						}
					},
					overlay: {
						viewBox: overlay.getAttribute('viewBox'),
						preserveAspectRatio: overlay.getAttribute('preserveAspectRatio'),
						boundingBox: {
							x: overlayRect.x,
							y: overlayRect.y,
							width: overlayRect.width,
							height: overlayRect.height
						}
					},
					wrapper: {
						boundingBox: {
							x: wrapperRect.x,
							y: wrapperRect.y,
							width: wrapperRect.width,
							height: wrapperRect.height
						}
					},
					alignment: {
						xOffset: overlayRect.x - imgRect.x,
						yOffset: overlayRect.y - imgRect.y,
						widthDiff: overlayRect.width - imgRect.width,
						heightDiff: overlayRect.height - imgRect.height
					},
					sampleCircles: sampleCircles,
					totalCircles: circles.length
				};
			}`, nil)

			if alignmentInfo != nil {
				info := alignmentInfo.(map[string]interface{})
				t.Logf("\n=== Viewport: %s ===", vp.name)

				if viewport, ok := info["viewport"].(map[string]interface{}); ok {
					t.Logf("Viewport: %vx%v", viewport["width"], viewport["height"])
				}

				if image, ok := info["image"].(map[string]interface{}); ok {
					t.Logf("\nImage:")
					t.Logf("  Natural size: %vx%v", image["naturalWidth"], image["naturalHeight"])
					t.Logf("  Display size: %vx%v", image["displayWidth"], image["displayHeight"])
					if bbox, ok := image["boundingBox"].(map[string]interface{}); ok {
						t.Logf("  BoundingBox: x=%.2f, y=%.2f, w=%.2f, h=%.2f",
							bbox["x"], bbox["y"], bbox["width"], bbox["height"])
					}
				}

				if overlay, ok := info["overlay"].(map[string]interface{}); ok {
					t.Logf("\nOverlay:")
					t.Logf("  ViewBox: %v", overlay["viewBox"])
					t.Logf("  PreserveAspectRatio: %v", overlay["preserveAspectRatio"])
					if bbox, ok := overlay["boundingBox"].(map[string]interface{}); ok {
						t.Logf("  BoundingBox: x=%.2f, y=%.2f, w=%.2f, h=%.2f",
							bbox["x"], bbox["y"], bbox["width"], bbox["height"])
					}
				}

				if alignment, ok := info["alignment"].(map[string]interface{}); ok {
					// Helper to convert int or float64 to float64
					toFloat := func(v interface{}) float64 {
						switch val := v.(type) {
						case float64:
							return val
						case int:
							return float64(val)
						default:
							return 0
						}
					}

					xOff := toFloat(alignment["xOffset"])
					yOff := toFloat(alignment["yOffset"])
					wDiff := toFloat(alignment["widthDiff"])
					hDiff := toFloat(alignment["heightDiff"])

					t.Logf("\nAlignment Check:")
					t.Logf("  X offset: %.2f px (should be 0)", xOff)
					t.Logf("  Y offset: %.2f px (should be 0)", yOff)
					t.Logf("  Width diff: %.2f px (should be 0)", wDiff)
					t.Logf("  Height diff: %.2f px (should be 0)", hDiff)

					if xOff > 1 || xOff < -1 || yOff > 1 || yOff < -1 ||
						wDiff > 1 || wDiff < -1 || hDiff > 1 || hDiff < -1 {
						t.Errorf("❌ Overlay not perfectly aligned with image!")
					} else {
						t.Logf("✓ Overlay aligned with image")
					}
				}

				if circles, ok := info["sampleCircles"].([]interface{}); ok && len(circles) > 0 {
					t.Logf("\nSample Territories:")
					for _, circleData := range circles {
						circle := circleData.(map[string]interface{})
						territory := circle["territory"].(string)
						svgCoords := circle["svgCoords"].(map[string]interface{})
						screenCoords := circle["screenCoords"].(map[string]interface{})

						t.Logf("  %s:", territory)
						t.Logf("    SVG coords: cx=%.2f%%, cy=%.2f%%, r=%.2f%%",
							svgCoords["cx"], svgCoords["cy"], svgCoords["r"])
						t.Logf("    Screen position: x=%.2f, y=%.2f (diameter: %.2fx%.2f px)",
							screenCoords["centerX"], screenCoords["centerY"],
							screenCoords["width"], screenCoords["height"])
					}
				}

				t.Logf("\nTotal clickable circles: %v", info["totalCircles"])
			}

			// Take a screenshot showing the alignment
			screenshotPath := fmt.Sprintf("screenshots/alignment-%s.png", vp.name)
			_, err = page.Screenshot(playwright.PageScreenshotOptions{
				Path: playwright.String(screenshotPath),
			})
			if err != nil {
				t.Errorf("Could not take screenshot: %v", err)
			} else {
				t.Logf("\n✓ Screenshot saved to: %s", screenshotPath)
			}

			// Wait a bit so we can visually inspect if headless=false
			time.Sleep(2 * time.Second)
		})
	}
}
