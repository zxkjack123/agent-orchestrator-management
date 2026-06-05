import { test, chromium } from '@playwright/test'

test('screenshot war room', async () => {
  const browser = await chromium.launch({ headless: true })
  const ctx = await browser.newContext({ viewport: { width: 1400, height: 900 } })
  const page = await ctx.newPage()
  await page.goto('http://localhost:7777')
  await page.waitForSelector('.xterm-rows', { timeout: 10000 })
  await page.waitForTimeout(4000)
  await page.screenshot({ path: '/tmp/warroom.png' })
  await browser.close()
})
