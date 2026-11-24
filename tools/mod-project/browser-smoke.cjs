const assert = require("node:assert/strict");
const fs = require("node:fs");
const { chromium } = require("playwright");

(async () => {
  const { url, email, password } = JSON.parse(fs.readFileSync(process.argv[2], "utf8"));
  const origin = new URL(url).origin;
  const browser = await chromium.launch({ headless: true });
  const context = await browser.newContext();
  const external = [];
  const errors = [];
  await context.route("**/*", async (route) => {
    const requestURL = route.request().url();
    if (/^https?:/.test(requestURL) && new URL(requestURL).origin !== origin) {
      external.push(requestURL);
      return route.abort();
    }
    return route.continue();
  });
  const page = await context.newPage();
  page.on("pageerror", (error) => errors.push(error.message));
  try {
    await page.goto(url + "/login", { waitUntil: "networkidle" });
    await page.getByPlaceholder("Email").fill(email);
    await page.getByPlaceholder("Password", { exact: true }).fill(password);
    await page.getByRole("button", { name: "Log in", exact: true }).click();
    await page.waitForURL("**/dashboard**", { timeout: 30000 });
    console.log("PASS real browser login to built dashboard");
    const config = await page.evaluate(async () => {
      const cookie = document.cookie.split("; ").find((part) => part.startsWith("token="));
      const token = decodeURIComponent(cookie.split("=").slice(1).join("="));
      const response = await fetch("/api/v1/fleet/config", { headers: { Authorization: "Bearer " + token } });
      return response.json();
    });
    assert.equal(config.license.tier, "research");
    assert.equal(config.server_settings.enable_analytics, false);
    const routes = [
      "/settings/fleets",
      "/controls/scripts/library",
      "/settings/integrations/sso/fleet-users",
      "/settings/organization/statistics",
      "/account",
      "/software/library/settings/patch-deadline",
      "/settings/integrations/conditional-access",
    ];
    for (const path of routes) {
      await page.goto(url + path, { waitUntil: "networkidle" });
      const text = await page.locator("body").innerText();
      assert.equal(await page.locator(".license-expiry-banner, .license-host-limit-message, .premium-feature-message-container").count(), 0,
        path + " showed a license gate or banner");
      assert(!/Fleet Premium license is about to expire|Your license covers [\d,]+ hosts?|This feature is included in Fleet Premium|Requires Fleet Premium|Page not found/i.test(text),
        path + " showed a gate/error");
      assert.equal(new URL(page.url()).pathname, path, path + " redirected");
      assert(text.length > 60, path + " did not render content");
      console.log("PASS built research dashboard route " + path);
    }
    assert.deepEqual(external, [], "unsolicited external browser requests: " + external.join(", "));
    assert.deepEqual(errors, [], "browser runtime errors: " + errors.join(", "));
    console.log("PASS no external browser requests or uncaught runtime errors");
  } finally {
    await browser.close();
  }
})().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
