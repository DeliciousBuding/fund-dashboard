import { expect, type Page, test } from "@playwright/test";

const password = process.env.E2E_PASSWORD ?? "ci-smoke-password-1";

function captureApplicationErrors(page: Page) {
  const errors: string[] = [];
  page.on("console", (message) => {
    if (message.type() === "error") errors.push(`console: ${message.text()}`);
  });
  page.on("pageerror", (error) => errors.push(`pageerror: ${error.message}`));
  return () => expect(errors, errors.join("\n")).toEqual([]);
}

async function expectRouteSettled(page: Page) {
  await expect(page.getByTestId("route-progress")).toHaveAttribute("aria-hidden", "true");
  await expect
    .poll(() =>
      page.getByTestId("route-content").evaluate((element) => getComputedStyle(element).opacity),
    )
    .toBe("1");
}

test.describe("authentication", () => {
  test.use({ storageState: { cookies: [], origins: [] } });

  test("successful login redirects into the protected shell", async ({ page }) => {
    const assertNoErrors = captureApplicationErrors(page);
    await page.goto("/login");
    await page.getByLabel("密码").fill(password);
    await page.getByRole("button", { name: "登录", exact: true }).click();

    await expect(page).toHaveURL(/\/$/);
    await expect(page.locator("aside")).toBeVisible();
    await expect(
      page.locator("aside").getByRole("link", { name: "总览", exact: true }),
    ).toBeVisible();
    await expectRouteSettled(page);
    assertNoErrors();
  });
});

test.describe("desktop navigation", () => {
  test("direct click shows immediate loading feedback and never leaves transparent content", async ({
    page,
  }) => {
    const assertNoErrors = captureApplicationErrors(page);
    let delayedChunk = false;
    await page.route(/\/assets\/analysis-[^/]+\.js$/, async (route) => {
      delayedChunk = true;
      await new Promise((resolve) => setTimeout(resolve, 700));
      await route.continue();
    });
    await page.goto("/");

    const analysisLink = page.locator("aside").getByRole("link", { name: "分析", exact: true });
    await analysisLink.dispatchEvent("click");
    await expect(page.getByTestId("route-progress")).toHaveAttribute("aria-hidden", "false");
    await expect(page.getByTestId("route-progress")).toBeVisible();
    await expect(page).toHaveURL(/\/analysis$/);
    expect(delayedChunk).toBe(true);
    await expectRouteSettled(page);
    assertNoErrors();
  });

  test("hover preloads a route chunk and click reuses it", async ({ page }) => {
    const assertNoErrors = captureApplicationErrors(page);
    const routeResponses: string[] = [];
    page.on("response", (response) => {
      if (/\/assets\/market-[^/]+\.js$/.test(response.url())) routeResponses.push(response.url());
    });
    await page.goto("/");

    const marketLink = page.locator("aside").getByRole("link", { name: "市场", exact: true });
    await marketLink.hover();
    await expect.poll(() => routeResponses.length).toBe(1);
    await marketLink.click();
    await expect(page).toHaveURL(/\/market$/);
    await expectRouteSettled(page);
    await page.waitForTimeout(100);
    expect(routeResponses).toHaveLength(1);
    assertNoErrors();
  });

  test("all core sidebar routes settle with visible content", async ({ page }) => {
    const assertNoErrors = captureApplicationErrors(page);
    await page.goto("/");
    const routes = [
      ["持仓", "/holdings"],
      ["交易", "/transactions"],
      ["定投", "/dca"],
      ["分析", "/analysis"],
      ["市场", "/market"],
      ["信号", "/insights"],
      ["报告", "/reports"],
      ["工作台", "/system"],
      ["审计", "/system/audit"],
      ["设置", "/settings"],
      ["总览", "/"],
    ] as const;

    for (const [label, path] of routes) {
      await page
        .locator("aside")
        .getByRole("link", { name: label, exact: true })
        .dispatchEvent("click");
      await expect(page).toHaveURL(new RegExp(`${path === "/" ? "/" : path}$`));
      await expectRouteSettled(page);
    }
    assertNoErrors();
  });
});

test.describe("mobile navigation", () => {
  test.use({ viewport: { width: 390, height: 844 } });

  test("touch-style direct click gets the same route feedback", async ({ page }) => {
    const assertNoErrors = captureApplicationErrors(page);
    let delayedChunk = false;
    await page.route(/\/assets\/holdings-[^/]+\.js$/, async (route) => {
      delayedChunk = true;
      await new Promise((resolve) => setTimeout(resolve, 700));
      await route.continue();
    });
    await page.goto("/");
    await expect(page.locator("aside")).toBeHidden();

    await page.getByRole("link", { name: "持仓", exact: true }).last().dispatchEvent("click");
    await expect(page.getByTestId("route-progress")).toHaveAttribute("aria-hidden", "false");
    await expect(page).toHaveURL(/\/holdings$/);
    expect(delayedChunk).toBe(true);
    await expectRouteSettled(page);
    assertNoErrors();
  });
});
