const puppeteer = require('puppeteer-core');

const browserWSEndpoint = process.argv[2];

if (!browserWSEndpoint) {
  console.error('Usage: node screenshot.js <websocket-url>');
  process.exit(1);
}

(async () => {
  try {
    console.error('=== SCREENSHOT DEBUG ===');
    console.error('Connecting to:', browserWSEndpoint);
    
    // Strip any page-specific path, connect to root browser
    const rootWS = browserWSEndpoint.split('/devtools')[0];
    console.error('Root WebSocket:', rootWS);
    
    const browser = await puppeteer.connect({ 
      browserWSEndpoint: rootWS,
      defaultViewport: { width: 1280, height: 720 }
    });

    console.error('✅ Connected to browser');

    // Get ALL pages (this will see what Puppeteer sees)
    const pages = await browser.pages();
    
    console.error(`\nFound ${pages.length} pages:`);
    
    for (let i = 0; i < pages.length; i++) {
      const p = pages[i];
      const url = p.url();
      console.error(`  Page ${i + 1}: ${url}`);
    }
    
    // Find non-blank page
    let page = pages.find(p => {
      const url = p.url();
      return url !== 'about:blank' && url !== '' && !url.startsWith('chrome://');
    });
    
    if (!page && pages.length > 0) {
      page = pages[pages.length - 1];
      console.error(`\n⚠️ All pages blank, using last page`);
    }
    
    if (!page) {
      console.error('\n❌ No pages found!');
      process.exit(1);
    }

    console.error(`\n📸 Taking screenshot of: ${page.url()}`);
    
    const screenshot = await page.screenshot({ 
      encoding: 'base64',
      type: 'png',
      fullPage: false
    });

    console.error(`✅ Screenshot captured (${screenshot.length} chars)`);
    console.log(screenshot);
    await browser.disconnect();

  } catch (error) {
    console.error('\n❌ Screenshot error:', error.message);
    console.error(error.stack);
    process.exit(1);
  }
})();