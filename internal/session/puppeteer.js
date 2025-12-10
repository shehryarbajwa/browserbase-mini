const puppeteer = require('puppeteer-core');
const readline = require('readline');

let browser, page;
let screenshotLock = false;

(async () => {
    try {
        const browserWSEndpoint = process.argv[2];
        
        console.error("WebSocket endpoint:", browserWSEndpoint);
        
        if (!browserWSEndpoint) {
            console.error("ERROR: No WebSocket URL provided");
            process.exit(1);
        }

        console.error("Connecting to browser...");
        browser = await puppeteer.connect({
            browserWSEndpoint: browserWSEndpoint,
            defaultViewport: { width: 1280, height: 720 }
        });
        console.error("✅ Connected to browser");

        // Close all existing pages to avoid stale references
        const existingPages = await browser.pages();
        console.error("Found", existingPages.length, "existing pages");
        for (const p of existingPages) {
            try {
                await p.close();
                console.error("Closed existing page:", p.url());
            } catch (err) {
                console.error("Error closing page:", err.message);
            }
        }

        // Create a fresh page
        page = await browser.newPage();
        console.error("Created new page:", page.url());

        console.log(JSON.stringify({ status: "ready" }));
        console.error("Sent 'ready' signal");

        const rl = readline.createInterface({
            input: process.stdin,
            output: process.stdout,
            terminal: false
        });

        console.error("Readline interface created, listening for commands...");

        rl.on("line", async (line) => {
            console.error("📥 Received command:", line);
            
            try {
                const cmd = JSON.parse(line);
                console.error("📋 Parsed command:", cmd.action);

                if (cmd.action === "navigate") {
    console.error("🚀 Starting navigation to:", cmd.url);

    try {
        console.error("Calling page.goto...");
        await page.goto(cmd.url, {
            waitUntil: "domcontentloaded",  // ← CHANGED from networkidle0
            timeout: 15000  // ← REDUCED from 30000
        });
        console.error("✅ page.goto completed");

        // Shorter wait
        console.error("Waiting 500ms...");
        await new Promise(resolve => setTimeout(resolve, 500));  // ← REDUCED from 1000
        console.error("✅ Wait complete");

        const currentUrl = page.url();
        console.error("Current URL:", currentUrl);

        console.log(JSON.stringify({
            status: "navigated",
            url: currentUrl
        }));
        console.error("✅ Sent navigated response");

    } catch (navErr) {
        console.error("❌ Navigation error:", navErr.message);
        console.log(JSON.stringify({
            status: "navigated",
            url: page.url()
        }));
        console.error("Sent navigated response (with error)");
    }
}
                else if (cmd.action === 'screenshot') {
                    console.error("📸 Screenshot command received");
                    
                    if (screenshotLock) {
                        console.error("⚠️ Screenshot lock is active, skipping");
                        console.log(JSON.stringify({ status: "busy" }));
                        return;
                    }

                    console.error("🔒 Acquiring screenshot lock");
                    screenshotLock = true;

                    try {
                        // Check if page is still connected
                        if (!page || page.isClosed()) {
                            console.error("⚠️ Page was closed, creating new page...");
                            page = await browser.newPage();
                            console.error("✅ Created new page");
                        }

                        const currentUrl = page.url();
                        console.error("Taking screenshot of:", currentUrl);

                        console.error("Creating screenshot task...");
                        const screenshotTask = (async () => {
                            console.error("Inside screenshot task - calling page.screenshot()");
                            const result = await page.screenshot({ 
                                encoding: 'base64',
                                type: 'png',
                                fullPage: false
                            });
                            console.error("page.screenshot() returned, length:", result.length);
                            return result;
                        })();

                        console.error("Creating timeout task (5 seconds)...");
                        const timeoutTask = new Promise((_, reject) => {
                            setTimeout(() => {
                                console.error("⏰ TIMEOUT TRIGGERED");
                                reject(new Error('timeout'));
                            }, 5000);
                        });

                        console.error("Starting Promise.race...");
                        const screenshot = await Promise.race([screenshotTask, timeoutTask]);
                        console.error("✅ Promise.race completed, screenshot length:", screenshot.length);

                        screenshotLock = false;
                        console.error("🔓 Released screenshot lock");

                        console.log(JSON.stringify({
                            status: "screenshot",
                            data: screenshot
                        }));
                        console.error("✅ Sent screenshot response");

                    } catch (err) {
                        console.error("❌ Screenshot failed:", err.message);

                        // If page was closed during screenshot, recreate it
                        if (err.message.includes('Session closed') || err.message.includes('Target closed')) {
                            console.error("🔄 Page session lost, recreating page...");
                            try {
                                page = await browser.newPage();
                                console.error("✅ Recreated page");
                            } catch (recreateErr) {
                                console.error("❌ Failed to recreate page:", recreateErr.message);
                            }
                        }

                        screenshotLock = false;
                        console.error("🔓 Released screenshot lock (after error)");

                        console.error("Returning blank PNG...");
                        const blankPng = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==";

                        console.log(JSON.stringify({
                            status: "screenshot",
                            data: blankPng
                        }));
                        console.error("✅ Sent blank PNG response");
                    }
                }
                else if (cmd.action === "close") {
                    console.error("Closing browser...");
                    await browser.disconnect();
                    console.error("Browser disconnected, exiting");
                    process.exit(0);
                }

            } catch (err) {
                console.error("❌ Command processing error:", err.message);
                console.log(JSON.stringify({
                    status: "error",
                    message: err.message
                }));
            }
        });

    } catch (err) {
        console.error("❌ Fatal error:", err.message);
        console.log(JSON.stringify({
            status: "error",
            message: err.message
        }));
        process.exit(1);
    }
})();