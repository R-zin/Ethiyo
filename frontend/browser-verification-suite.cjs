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
const MOCK_URL = process.env.MOCK_URL || 'http://localhost:9099';
// Optional: a production build (vite preview) to verify the installable PWA.
const PWA_URL = process.env.PWA_URL || '';

const sleep = ms => new Promise(r => setTimeout(r, ms));

// Toggles the mock upstream's simulated Chalo outage.
async function setMockOutage(on) {
  const res = await fetch(`${MOCK_URL}/__mock/fail?on=${on ? 1 : 0}`);
  if (!res.ok) throw new Error(`mock outage switch failed: ${res.status}`);
}

// Polls fn() until it returns a truthy value or the timeout passes.
async function waitUntil(fn, timeoutMs, intervalMs = 500) {
  const deadline = Date.now() + timeoutMs;
  for (;;) {
    const value = await fn();
    if (value) return value;
    if (Date.now() > deadline) return value;
    await sleep(intervalMs);
  }
}

const SCREENSHOT_DIR = process.env.SCREENSHOT_DIR || path.join(__dirname, 'screenshots');

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
      // Third-party map tiles may be unreachable from sandboxed runners;
      // that is an environment limitation, not an application error.
      const sourceUrl = (msg.location && msg.location().url) || '';
      if (sourceUrl.includes('tile.openstreetmap.org')) {
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

    // Grant geolocation for the "How far is my bus?" feature, placing the
    // viewer ~3 km south of the mock KS602 route.
    await browser.defaultBrowserContext().overridePermissions(APP_URL, ['geolocation', 'notifications']);
    await page.setGeolocation({ latitude: 11.1789, longitude: 75.8115, accuracy: 10 });

    // Record system notifications the app raises (arrival alerts).
    await page.evaluateOnNewDocument(() => {
      window.__notifications = [];
      const Native = window.Notification;
      if (!Native) return;
      function Recording(title, opts) {
        window.__notifications.push({ title, body: opts && opts.body });
        return new Native(title, opts);
      }
      Object.defineProperty(Recording, 'permission', { get: () => Native.permission });
      Recording.requestPermission = (...args) => Native.requestPermission(...args);
      Recording.prototype = Native.prototype;
      window.Notification = Recording;
    });

    // Force initial light theme via evaluateOnNewDocument
    await page.evaluateOnNewDocument(() => {
      // The browser profile persists between runs; start without saved buses.
      localStorage.removeItem('ethiyo_favorites');
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
      console.log('Clicked preset chip KS602. Waiting for the live tracking card...');
      // Wait for the tracking card specifically: an error alert must NOT
      // count as "rendered" (it used to, which hid a broken API contract).
      await page.waitForSelector('[data-testid="connection-mode"], [role="alert"]', { timeout: 20000 });

      const liveCard = await page.evaluate(() => {
        const tiles = Array.from(document.querySelectorAll('.tabular-nums')).map(n => n.textContent.trim());
        return {
          hasCard: !!document.querySelector('[data-testid="connection-mode"]'),
          hasError: !!document.querySelector('[data-testid="error-state"]'),
          tiles,
          body: document.body.innerText,
        };
      });
      // The mock reports 28.5 km/h; seeing "29 km/h" proves the UI reads the
      // API's real field names end to end.
      const speedRendered = liveCard.tiles.includes('29 km/h');
      const stopsRendered = liveCard.body.includes('From: HSR Layout');
      console.log('Live tracker card:', { hasCard: liveCard.hasCard, hasError: liveCard.hasError, tiles: liveCard.tiles });
      console.log('Speed from API rendered (29 km/h):', speedRendered, '| previous stop rendered:', stopsRendered);

      // Server-sent events: the connection pill should reach "live".
      let streamLive = false;
      try {
        await page.waitForSelector('[data-testid="connection-mode"][data-mode="live"]', { timeout: 15000 });
        streamLive = true;
      } catch (e) {
        const mode = await page.evaluate(() => document.querySelector('[data-testid="connection-mode"]')?.getAttribute('data-mode'));
        console.warn('SSE stream did not reach live mode; current mode:', mode);
      }
      console.log('SSE live stream connected:', streamLive);

      // Motion intelligence: as the mock bus moves, a heading arrow and a
      // breadcrumb trail must appear on the map.
      let motionRendered = false;
      try {
        await page.waitForSelector('.custom-bus-marker .bus-heading', { timeout: 25000 });
        await page.waitForSelector('path.bus-trail', { timeout: 10000 });
        motionRendered = true;
      } catch (e) {
        console.warn('Heading arrow / trail did not appear in time');
      }
      const heading = await page.evaluate(() => document.querySelector('.bus-heading')?.getAttribute('data-heading'));
      console.log('Heading arrow + trail rendered:', motionRendered, '| heading:', heading);

      report.interactions.presetSearch = {
        busCode: 'KS602',
        rendered: liveCard.hasCard && !liveCard.hasError && speedRendered && stopsRendered,
        speedRendered,
        stopsRendered,
      };
      report.interactions.liveStream = { connected: streamLive };
      report.interactions.motion = { rendered: motionRendered, heading };

      const shot3 = path.join(SCREENSHOT_DIR, '03-desktop-tracked-ks602.png');
      await page.screenshot({ path: shot3, fullPage: true });
      report.screenshots.push({ name: '03-desktop-tracked-ks602.png', path: shot3, label: 'Desktop Live Tracking Card & Leaflet Map (KS602)' });
      console.log('Captured:', shot3);

      // "How far is my bus?" — geolocation-based distance and ETA.
      console.log('\n[Step 3b] Testing "How far is my bus?" proximity feature...');
      const locateClicked = await page.evaluate(() => {
        const btn = Array.from(document.querySelectorAll('button')).find(b => b.textContent.includes('Use my location'));
        if (btn) btn.click();
        return !!btn;
      });
      let proximityText = null;
      if (locateClicked) {
        try {
          await page.waitForSelector('[data-testid="proximity-result"]', { timeout: 10000 });
          proximityText = await page.$eval('[data-testid="proximity-result"]', el => el.textContent);
        } catch (e) {
          console.warn('Proximity result did not render');
        }
      }
      console.log('Proximity result:', proximityText);
      report.interactions.proximity = {
        rendered: !!proximityText && /km away/.test(proximityText) && /min at current speed/.test(proximityText),
        text: proximityText,
      };

      // Arrival alert: the bus is ~3-4 km away at 28.5 km/h (ETA ≈ 6-9 min),
      // so arming "≤ 15 min" must fire at once — as an in-page banner and as
      // a browser notification.
      console.log('\n[Step 3c] Testing arrival alerts...');
      let alertTitle = null;
      let notifications = [];
      try {
        await page.select('#approach-alert', '15');
        await page.waitForSelector('[data-testid="alert-banner"][data-kind="approach"]', { timeout: 10000 });
        alertTitle = await page.$eval('[data-testid="alert-banner"][data-kind="approach"]', el => el.textContent);
        notifications = await page.evaluate(() => window.__notifications || []);
      } catch (e) {
        console.warn('Arrival alert did not fire:', e.message);
      }
      console.log('Alert banner:', alertTitle, '| notifications raised:', notifications.map(n => n.title));
      const shotAlert = path.join(SCREENSHOT_DIR, '03c-arrival-alert.png');
      await page.screenshot({ path: shotAlert, fullPage: false });
      report.screenshots.push({ name: '03c-arrival-alert.png', path: shotAlert, label: 'Arrival alert fired' });
      report.interactions.alerts = {
        bannerShown: !!alertTitle && alertTitle.includes('almost here'),
        notified: notifications.some(n => n.title.includes('KS602 is almost here')),
      };
      // Dismiss so later steps' role="alert" checks only see what they expect.
      await page.evaluate(() => document.querySelectorAll('[aria-label="Dismiss alert"]').forEach(b => b.click()));

      // My buses: star KS602, then DL1PC0001; both must show live status
      // delivered over ONE multi-bus stream.
      console.log('\n[Step 3d] Testing My buses (favorites + multi-bus stream)...');
      const favorites = { cards: 0, allLive: false, fleetLive: false };
      try {
        const saveCurrentBus = async () => {
          const pressed = await page.$eval('[data-testid="favorite-toggle"]', el => el.getAttribute('aria-pressed'));
          if (pressed !== 'true') await page.click('[data-testid="favorite-toggle"]');
        };
        await saveCurrentBus();
        await page.waitForSelector('[data-testid="favorite-card"][data-code="KS602"]', { timeout: 10000 });

        // Replace the search text via the app's own clear button (a
        // triple-click does not reliably select a controlled input's value).
        await page.click('button[aria-label="Clear input"]');
        await page.type('#bus-search-input', 'DL1PC0001');
        await page.keyboard.press('Enter');
        await page.waitForFunction(
          () => document.querySelector('[data-testid="connection-mode"]') && document.querySelector('h2.font-mono')?.textContent === 'DL1PC0001',
          { timeout: 15000 }
        );
        await saveCurrentBus();

        const settled = await waitUntil(
          () =>
            page.evaluate(() => {
              const cards = Array.from(document.querySelectorAll('[data-testid="favorite-card"]'));
              const mode = document.querySelector('[data-testid="fleet-mode"]')?.getAttribute('data-mode');
              return cards.length === 2 && cards.every(c => c.getAttribute('data-status') === 'Live') && mode === 'live'
                ? { cards: cards.length, allLive: true, fleetLive: true }
                : null;
            }),
          15000
        );
        Object.assign(favorites, settled || {});
        if (!settled) {
          favorites.cards = await page.$$eval('[data-testid="favorite-card"]', els => els.length);
        }

        // Back to KS602 from its saved card.
        await page.click('[data-testid="favorite-card"][data-code="KS602"] button[aria-label^="Track bus KS602"]');
        await page.waitForFunction(() => document.querySelector('h2.font-mono')?.textContent === 'KS602', { timeout: 15000 });
      } catch (e) {
        console.warn('My buses flow failed:', e.message);
      }
      console.log('My buses:', favorites);
      report.interactions.favorites = favorites;
      const shotFav = path.join(SCREENSHOT_DIR, '03d-my-buses.png');
      await page.screenshot({ path: shotFav, fullPage: true });
      report.screenshots.push({ name: '03d-my-buses.png', path: shotFav, label: 'My buses with live status' });
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
      console.log('Clicked Route Endpoints tab. Waiting for headless-browser route discovery...');
      let routeUrl = null;
      try {
        await page.waitForSelector('[data-testid="route-endpoints"], [data-testid="route-error"]', { timeout: 45000 });
        routeUrl = await page.evaluate(() => {
          const input = document.querySelector('input[aria-label="Scheduler Live Route URL"]');
          return input ? input.value : null;
        });
      } catch (e) {
        console.warn('Route panel did not settle in time');
      }
      console.log('Discovered route URL:', routeUrl);
      report.interactions.routeDiscovery = {
        discovered: !!routeUrl && routeUrl.includes('/routedetailslive'),
        routeUrl,
      };

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
    // STEP 4b: CHALO OUTAGE -> DEGRADED MODE -> RECOVERY
    // ==========================================
    console.log('\n[Step 4b] Testing graceful degradation during a simulated Chalo outage...');
    const degraded = { banner: false, offlineAlert: false, fleetLastKnown: false, headerDegraded: false, recovered: false };
    try {
      await page.click('#offline-alert'); // "Notify me if it stops reporting"
      await setMockOutage(true);

      await page.waitForSelector('[data-testid="degraded-banner"]', { timeout: 15000 });
      degraded.banner = true;
      degraded.offlineAlert = !!(await waitUntil(
        () => page.$('[data-testid="alert-banner"][data-kind="offline"]'),
        5000
      ));
      degraded.fleetLastKnown = !!(await waitUntil(
        () =>
          page.evaluate(() =>
            Array.from(document.querySelectorAll('[data-testid="favorite-card"]')).every(
              c => c.getAttribute('data-status') === 'Last known'
            )
          ),
        15000
      ));
      // The breaker opens after repeated failures; refresh the header status.
      degraded.headerDegraded = !!(await waitUntil(async () => {
        await page.click('[data-testid="api-status"]');
        await sleep(300);
        return page.$eval('[data-testid="api-status"]', el => el.getAttribute('data-state') === 'degraded');
      }, 20000, 1000));

      const shotDeg = path.join(SCREENSHOT_DIR, '04b-degraded-mode.png');
      await page.screenshot({ path: shotDeg, fullPage: true });
      report.screenshots.push({ name: '04b-degraded-mode.png', path: shotDeg, label: 'Chalo outage: last known positions' });
    } catch (e) {
      console.warn('Degraded mode checks failed:', e.message);
    } finally {
      await setMockOutage(false);
    }
    try {
      // Recovery only counts if degradation was actually observed first.
      degraded.recovered =
        degraded.banner &&
        !!(await waitUntil(async () => !(await page.$('[data-testid="degraded-banner"]')), 25000));
    } catch (e) {
      console.warn('Recovery check failed:', e.message);
    }
    await page.evaluate(() => document.querySelectorAll('[aria-label="Dismiss alert"]').forEach(b => b.click()));
    console.log('Degraded mode:', degraded);
    report.interactions.degraded = degraded;

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
      await page.waitForSelector('[data-testid="error-state"]', { timeout: 15000 });
      await new Promise(r => setTimeout(r, 800));

      const errorHeading = await page.evaluate(() => {
        const heading = document.querySelector('[data-testid="error-state"] h2');
        return heading ? heading.textContent : null;
      });
      console.log('Error state alert heading:', errorHeading);
      // Chalo answers unknown buses with HTTP 202 + an error body; the API
      // must still translate that into a clear "not found".
      report.interactions.errorState = {
        renderedHeading: errorHeading,
        isNotFound: !!errorHeading && errorHeading.includes('Not Found'),
      };

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

    // ==========================================
    // STEP 8: INSTALLABLE APP (PWA) ON THE PRODUCTION BUILD
    // ==========================================
    if (PWA_URL) {
      console.log(`\n[Step 8] Verifying the installable app against ${PWA_URL}...`);
      const pwa = { manifest: false, serviceWorker: false, noLiveDataCached: false, offlineShell: false };
      const pwaPage = await browser.newPage();
      try {
        await pwaPage.goto(PWA_URL, { waitUntil: 'networkidle0', timeout: 20000 });
        const manifest = await pwaPage.evaluate(async () => {
          const link = document.querySelector('link[rel="manifest"]');
          if (!link) return null;
          return (await fetch(link.href)).json();
        });
        pwa.manifest =
          !!manifest &&
          manifest.display === 'standalone' &&
          manifest.icons.some(i => i.sizes === '512x512') &&
          manifest.icons.some(i => i.purpose === 'maskable');

        await pwaPage.evaluate(() => navigator.serviceWorker.ready);
        await pwaPage.reload({ waitUntil: 'networkidle0' });
        pwa.serviceWorker = await pwaPage.evaluate(() => !!navigator.serviceWorker.controller);

        // Exercise live endpoints, then confirm none of them were cached.
        await pwaPage.evaluate(() => fetch('/api/v1/bus/KS602/track').then(r => r.text()));
        const cached = await pwaPage.evaluate(async () => {
          const paths = [];
          for (const key of await caches.keys()) {
            const cache = await caches.open(key);
            for (const req of await cache.keys()) paths.push(new URL(req.url).pathname);
          }
          return paths;
        });
        console.log('Cached paths:', cached);
        pwa.noLiveDataCached =
          cached.includes('/') && !cached.some(p => p.startsWith('/api/') || p.startsWith('/health') || p.startsWith('/metrics'));

        await pwaPage.setOfflineMode(true);
        await pwaPage.reload({ waitUntil: 'domcontentloaded' });
        await pwaPage.waitForSelector('#bus-search-input', { timeout: 10000 });
        pwa.offlineShell = true;
        const shotPwa = path.join(SCREENSHOT_DIR, '08-pwa-offline-shell.png');
        await pwaPage.screenshot({ path: shotPwa });
        report.screenshots.push({ name: '08-pwa-offline-shell.png', path: shotPwa, label: 'Installed app shell while offline' });
      } catch (e) {
        console.warn('PWA checks failed:', e.message);
      } finally {
        await pwaPage.setOfflineMode(false).catch(() => {});
        await pwaPage.close();
      }
      console.log('PWA:', pwa);
      report.interactions.pwa = pwa;
    } else {
      console.log('\n[Step 8] PWA checks skipped (set PWA_URL to a production build, e.g. vite preview).');
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
    const checks = {
      cleanConsole: report.cleanConsole,
      liveDataRendered: !!report.interactions.presetSearch?.rendered,
      sseStreamConnected: !!report.interactions.liveStream?.connected,
      headingAndTrail: !!report.interactions.motion?.rendered,
      proximity: !!report.interactions.proximity?.rendered,
      routeDiscovery: !!report.interactions.routeDiscovery?.discovered,
      notFoundError: !!report.interactions.errorState?.isNotFound,
      arrivalAlert: !!report.interactions.alerts?.bannerShown && !!report.interactions.alerts?.notified,
      myBuses: report.interactions.favorites?.cards === 2 && !!report.interactions.favorites?.allLive && !!report.interactions.favorites?.fleetLive,
      degradedMode: !!report.interactions.degraded?.banner && !!report.interactions.degraded?.offlineAlert && !!report.interactions.degraded?.fleetLastKnown,
      degradedHeader: !!report.interactions.degraded?.headerDegraded,
      recovery: !!report.interactions.degraded?.recovered,
      noHorizontalScroll: !Object.values(report.responsiveChecks).some(c => c.hasHorizontalScroll),
    };
    if (PWA_URL) {
      const pwa = report.interactions.pwa || {};
      checks.pwaManifest = !!pwa.manifest;
      checks.pwaServiceWorker = !!pwa.serviceWorker;
      checks.pwaNoLiveDataCached = !!pwa.noLiveDataCached;
      checks.pwaOfflineShell = !!pwa.offlineShell;
    }
    report.checks = checks;
    fs.writeFileSync(reportPath, JSON.stringify(report, null, 2));
    console.log('\nCriteria:');
    for (const [name, ok] of Object.entries(checks)) {
      console.log(`  ${ok ? 'PASS' : 'FAIL'}  ${name}`);
    }
    const hasFailures = Object.values(checks).some(ok => !ok);

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
