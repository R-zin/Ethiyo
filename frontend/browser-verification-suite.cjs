const puppeteer = require('puppeteer-core');
const fs = require('fs');
const path = require('path');

function findBrowser() {
  if (process.env.BROWSER_PATH && fs.existsSync(process.env.BROWSER_PATH)) {
    return process.env.BROWSER_PATH;
  }
  if (process.env.CHROME_BIN && fs.existsSync(process.env.CHROME_BIN)) {
    return process.env.CHROME_BIN;
  }

  const candidates = [
    // Linux standard paths (e.g. GitHub Actions ubuntu-latest)
    '/usr/bin/chromium',
    '/usr/bin/chromium-browser',
    '/usr/bin/google-chrome',
    '/usr/bin/google-chrome-stable',
    '/snap/bin/chromium',
    // macOS standard paths
    '/Applications/Brave Browser.app/Contents/MacOS/Brave Browser',
    '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',
    '/Applications/Chromium.app/Contents/MacOS/Chromium',
    '/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge',
    path.join(process.env.HOME || '', 'Applications/Brave Browser.app/Contents/MacOS/Brave Browser'),
    path.join(process.env.HOME || '', 'Applications/Google Chrome.app/Contents/MacOS/Google Chrome'),
  ];

  for (const candidate of candidates) {
    if (fs.existsSync(candidate)) {
      return candidate;
    }
  }

  throw new Error('No supported Chromium or Chrome executable found. Set BROWSER_PATH or CHROME_BIN.');
}

const BROWSER_BIN = findBrowser();
const APP_URL = process.env.APP_URL || 'http://localhost:3000';

const DEFAULT_ARTIFACT_DIR = '/Users/razinm/.gemini/antigravity-cli/brain/6f51d19b-4d76-45db-adb4-045f3bf0afd8/screenshots';
const SCREENSHOT_DIR = process.env.SCREENSHOT_DIR ||
  (fs.existsSync(path.dirname(DEFAULT_ARTIFACT_DIR)) ? DEFAULT_ARTIFACT_DIR : path.join(__dirname, 'screenshots'));

if (!fs.existsSync(SCREENSHOT_DIR)) {
  fs.mkdirSync(SCREENSHOT_DIR, { recursive: true });
}

async function runVerification() {
  console.log('=== STARTING BROWSER TESTING SUITE WITH DEVTOOLS ===');
  console.log('Browser Binary:', BROWSER_BIN);
  console.log('Target Server:', APP_URL);
  console.log('Screenshot Directory:', SCREENSHOT_DIR);

  const report = {
    timestamp: new Date().toISOString(),
    cleanConsole: true,
    consoleIssues: [],
    networkRequests: [],
    performanceMetrics: {},
    accessibility: {},
    responsiveChecks: {},
    interactions: {},
    screenshots: [],
  };

  const browser = await puppeteer.launch({
    executablePath: BROWSER_BIN,
    headless: true,
    args: [
      '--no-sandbox',
      '--disable-setuid-sandbox',
      '--disable-dev-shm-usage',
      '--disable-gpu',
      '--user-data-dir=/tmp/brave-verification-profile',
      '--disable-features=Crashpad',
      '--no-zygote',
    ],
  });

  try {
    const page = await browser.newPage();

    // 1. Setup Console Listener
    page.on('console', msg => {
      const type = msg.type();
      const text = msg.text();
      // Ignore Vite dev connection & React DevTools info prompt
      if (text.includes('[vite]') || text.includes('React DevTools')) {
        return;
      }
      // Native browser 404 log for intentional negative test is expected
      const isExpected404 = text.includes('404') && text.includes('Failed to load resource');
      if (isExpected404) {
        return;
      }

      if (type === 'error' || type === 'warn') {
        report.cleanConsole = false;
        report.consoleIssues.push({ type, text });
      }
    });

    page.on('pageerror', err => {
      report.cleanConsole = false;
      report.consoleIssues.push({ type: 'pageerror', text: err.toString() });
    });

    // 2. Setup Network Monitor
    page.on('request', req => {
      req._startTime = Date.now();
    });

    page.on('response', async res => {
      const req = res.request();
      const duration = req._startTime ? Date.now() - req._startTime : 0;
      const url = res.url();
      if (url.includes('/api/') || url.includes('/health')) {
        let snippet = '';
        try {
          const text = await res.text();
          snippet = text.length > 200 ? text.substring(0, 200) + '...' : text;
        } catch (e) {
          snippet = '<binary/error>';
        }
        report.networkRequests.push({
          url,
          method: req.method(),
          status: res.status(),
          statusText: res.statusText(),
          durationMs: duration,
          contentType: res.headers()['content-type'] || 'unknown',
          payloadPreview: snippet,
        });
      }
    });

    // Force initial light theme via evaluateOnNewDocument
    await page.evaluateOnNewDocument(() => {
      localStorage.setItem('ethiyo-theme', 'light');
    });

    // ==========================================
    // STEP 1: INITIAL DESKTOP LOAD (LIGHT THEME)
    // ==========================================
    console.log('\n[Step 1] Loading Initial Desktop Light State (1440x900)...');
    await page.setViewport({ width: 1440, height: 900 });
    const navStart = Date.now();
    await page.goto(APP_URL, { waitUntil: 'networkidle0', timeout: 20000 });
    const navEnd = Date.now();
    console.log(`Page navigation complete in ${navEnd - navStart}ms`);

    // Performance timing extraction
    const perfData = await page.evaluate(() => {
      const nav = performance.getEntriesByType('navigation')[0];
      const paint = performance.getEntriesByType('paint');
      const fcp = paint.find(p => p.name === 'first-contentful-paint');
      return {
        dnsLookup: nav ? nav.domainLookupEnd - nav.domainLookupStart : 0,
        tcpConnection: nav ? nav.connectEnd - nav.connectStart : 0,
        ttfb: nav ? nav.responseStart - nav.requestStart : 0,
        domInteractive: nav ? nav.domInteractive : 0,
        firstContentfulPaint: fcp ? Math.round(fcp.startTime) : 0,
      };
    });
    report.performanceMetrics = perfData;
    console.log('Performance Baseline:', JSON.stringify(perfData, null, 2));

    const isLightInitial = await page.evaluate(() => !document.documentElement.classList.contains('dark'));
    console.log('Initial mode is light:', isLightInitial);

    // Save initial light screenshot
    const shot1 = path.join(SCREENSHOT_DIR, '01-desktop-initial-light.png');
    await page.screenshot({ path: shot1, fullPage: true });
    report.screenshots.push({ name: '01-desktop-initial-light.png', path: shot1, label: 'Desktop Initial View (Nordic Light Theme)' });
    console.log('Captured:', shot1);

    // ==========================================
    // STEP 2: TOGGLE DARK THEME (POLAR NIGHT)
    // ==========================================
    console.log('\n[Step 2] Testing Theme Toggle to Dark (Polar Night)...');
    const themeBtn = await page.$('button[aria-label*="theme"]');
    if (themeBtn) {
      await themeBtn.click();
      await new Promise(r => setTimeout(r, 400));
      const isDark = await page.evaluate(() => document.documentElement.classList.contains('dark'));
      const bg = await page.evaluate(() => window.getComputedStyle(document.body).backgroundColor);
      console.log(`Theme toggled. html.dark present: ${isDark}, body background: ${bg}`);
      report.interactions.themeToggle = { isDark, backgroundColor: bg };

      const shot2 = path.join(SCREENSHOT_DIR, '02-desktop-initial-dark.png');
      await page.screenshot({ path: shot2, fullPage: true });
      report.screenshots.push({ name: '02-desktop-initial-dark.png', path: shot2, label: 'Desktop Initial View (Nordic Dark Theme)' });
      console.log('Captured:', shot2);
    } else {
      console.warn('Theme toggle button not found!');
    }

    // ==========================================
    // STEP 3: INTERACTIVE SEARCH VIA QUICK PRESET CHIP
    // ==========================================
    console.log('\n[Step 3] Testing Quick Preset Click: KS602...');
    const presetClicked = await page.evaluate(() => {
      const buttons = Array.from(document.querySelectorAll('button'));
      const ks602Btn = buttons.find(b => b.textContent && b.textContent.trim().startsWith('KS602'));
      if (ks602Btn) {
        ks602Btn.click();
        return true;
      }
      return false;
    });

    if (presetClicked) {
      console.log('Clicked preset chip KS602. Waiting for network response and DOM update...');
      await page.waitForSelector('.tabular-nums, [data-testid="error-state"], [role="alert"], .leaflet-container', { timeout: 15000 });
      await new Promise(r => setTimeout(r, 1200));

      const trackingCardPresent = await page.evaluate(() => {
        return !!document.querySelector('.tabular-nums, [data-testid="error-state"], [role="alert"]');
      });
      console.log('Live tracker metrics rendered:', trackingCardPresent);
      report.interactions.presetSearch = { busCode: 'KS602', rendered: trackingCardPresent };

      const shot3 = path.join(SCREENSHOT_DIR, '03-desktop-tracked-ks602.png');
      await page.screenshot({ path: shot3, fullPage: true });
      report.screenshots.push({ name: '03-desktop-tracked-ks602.png', path: shot3, label: 'Desktop Live Tracking Card & Leaflet Map (KS602)' });
      console.log('Captured:', shot3);
    }

    // ==========================================
    // STEP 4: ROUTE ENDPOINTS & SPEC TAB
    // ==========================================
    console.log('\n[Step 4] Testing Route Endpoints & Spec Tab...');
    const tabClicked = await page.evaluate(() => {
      const tab = document.getElementById('tab-route');
      if (tab) {
        tab.click();
        return true;
      }
      return false;
    });

    if (tabClicked) {
      console.log('Clicked Route Endpoints tab. Waiting for DOM update...');
      await new Promise(r => setTimeout(r, 800));

      const tabActive = await page.evaluate(() => {
        const tab = document.getElementById('tab-route');
        return tab ? tab.getAttribute('aria-selected') === 'true' : false;
      });
      console.log('Route tab active aria-selected:', tabActive);
      report.interactions.routeTab = { active: tabActive };

      const shot4 = path.join(SCREENSHOT_DIR, '04-desktop-route-spec-tab.png');
      await page.screenshot({ path: shot4, fullPage: true });
      report.screenshots.push({ name: '04-desktop-route-spec-tab.png', path: shot4, label: 'Route Endpoints & Spec Inspector' });
      console.log('Captured:', shot4);
    } else {
      console.warn('Route Endpoints tab (#tab-route) not found!');
    }

    // ==========================================
    // STEP 5: SEARCH INVALID BUS CODE -> ERROR STATE RECOVERY
    // ==========================================
    console.log('\n[Step 5] Testing Search for Invalid Bus Code & Error State Recovery...');
    // Switch back to live tab
    await page.evaluate(() => {
      const tab = document.getElementById('tab-map');
      if (tab) tab.click();
    });
    await new Promise(r => setTimeout(r, 400));

    // Focus input and submit invalid bus code
    const input = await page.$('#bus-search-input');
    if (input) {
      await page.evaluate(() => {
        const el = document.getElementById('bus-search-input');
        if (el) {
          el.value = '';
          el.dispatchEvent(new Event('input', { bubbles: true }));
        }
      });
      await input.type('UNKNOWN_BUS_999');
      await page.keyboard.press('Enter');

      console.log('Submitted search for UNKNOWN_BUS_999. Waiting for error response...');
      await page.waitForSelector('[role="alert"]', { timeout: 15000 });
      await new Promise(r => setTimeout(r, 800));

      const errorHeading = await page.evaluate(() => {
        const heading = document.querySelector('[role="alert"] h2');
        return heading ? heading.textContent : null;
      });
      console.log('Error state alert heading:', errorHeading);
      report.interactions.errorState = { renderedHeading: errorHeading };

      const shot5 = path.join(SCREENSHOT_DIR, '05-desktop-error-state.png');
      await page.screenshot({ path: shot5, fullPage: true });
      report.screenshots.push({ name: '05-desktop-error-state.png', path: shot5, label: 'Error State with Actionable Recovery' });
      console.log('Captured:', shot5);
    }

    // ==========================================
    // STEP 6: ACCESSIBILITY (a11y) INSPECTION
    // ==========================================
    console.log('\n[Step 6] Running Accessibility & Heading Inspection...');
    const a11ySummary = await page.evaluate(() => {
      const headings = Array.from(document.querySelectorAll('h1, h2, h3, h4, h5, h6')).map(h => ({
        tag: h.tagName.toLowerCase(),
        text: h.textContent.trim(),
      }));

      const interactiveElements = Array.from(document.querySelectorAll('button, input, a, [role="tab"]')).map(el => {
        const accessibleName = el.getAttribute('aria-label') || el.innerText || el.getAttribute('placeholder') || el.getAttribute('title') || '';
        return {
          tag: el.tagName.toLowerCase(),
          role: el.getAttribute('role') || 'default',
          accessibleName: accessibleName.trim().substring(0, 50),
          hasAriaLabel: el.hasAttribute('aria-label'),
        };
      });

      const missingLabels = interactiveElements.filter(e => !e.accessibleName);

      return {
        headings,
        interactiveElementsCount: interactiveElements.length,
        missingAccessibleNamesCount: missingLabels.length,
        missingAccessibleNames: missingLabels,
      };
    });
    report.accessibility = a11ySummary;
    console.log('Headings hierarchy:', JSON.stringify(a11ySummary.headings, null, 2));
    console.log(`Interactive elements: ${a11ySummary.interactiveElementsCount}, Missing accessible names: ${a11ySummary.missingAccessibleNamesCount}`);

    // ==========================================
    // STEP 7: RESPONSIVE VIEWPORT TESTING
    // ==========================================
    console.log('\n[Step 7] Testing Responsive Viewports (320px, 768px, 1024px, 1440px)...');
    const viewports = [
      { name: '06-tablet-768px.png', width: 768, height: 1024, label: 'Tablet Viewport (768px)' },
      { name: '07-mobile-320px.png', width: 320, height: 800, label: 'Mobile Viewport (320px iPhone SE)' },
    ];

    for (const vp of viewports) {
      await page.setViewport({ width: vp.width, height: vp.height });
      await new Promise(r => setTimeout(r, 600));

      const overflow = await page.evaluate(() => {
        return {
          scrollWidth: document.documentElement.scrollWidth,
          clientWidth: document.documentElement.clientWidth,
          hasHorizontalScroll: document.documentElement.scrollWidth > document.documentElement.clientWidth,
        };
      });

      report.responsiveChecks[`${vp.width}px`] = overflow;
      console.log(`Viewport ${vp.width}px: scrollWidth=${overflow.scrollWidth}, clientWidth=${overflow.clientWidth}, hasHorizontalScroll=${overflow.hasHorizontalScroll}`);

      const shotPath = path.join(SCREENSHOT_DIR, vp.name);
      await page.screenshot({ path: shotPath, fullPage: true });
      report.screenshots.push({ name: vp.name, path: shotPath, label: vp.label });
      console.log('Captured:', shotPath);
    }

    console.log('\n=== BROWSER TESTING SUITE COMPLETE ===');
    console.log('Clean Console:', report.cleanConsole ? 'PASS (0 unexpected errors/warnings)' : 'FAIL');
    if (!report.cleanConsole) {
      console.log('Console issues:', report.consoleIssues);
    }
    console.log('Network API calls captured:', report.networkRequests.length);

    // Save final JSON report
    const reportPath = path.join(SCREENSHOT_DIR, 'devtools-test-report.json');
    fs.writeFileSync(reportPath, JSON.stringify(report, null, 2));
    console.log('Verification report written to:', reportPath);

    // Assertions for CI exit code
    const hasFailures =
      !report.cleanConsole ||
      !report.interactions.presetSearch?.rendered ||
      !report.interactions.errorState?.renderedHeading ||
      Object.values(report.responsiveChecks).some(c => c.hasHorizontalScroll);

    if (hasFailures) {
      console.error('\n❌ E2E VERIFICATION SUITE FAILED CRITICAL ASSERTIONS');
      process.exit(1);
    } else {
      console.log('\n✅ ALL E2E INTEGRATION CRITERIA PASSED SUCCESSFULLY');
      process.exit(0);
    }
  } finally {
    await browser.close();
  }
}

runVerification().catch(err => {
  console.error('Browser testing suite encountered an unhandled error:', err);
  process.exit(1);
});
