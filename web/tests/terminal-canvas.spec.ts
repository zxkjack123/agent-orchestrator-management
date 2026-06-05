import { test, expect, chromium } from '@playwright/test'

test('terminal canvas sizing and xterm internal state', async () => {
  const browser = await chromium.launch({ headless: true })
  const ctx = await browser.newContext({ viewport: { width: 1400, height: 900 } })
  const page = await ctx.newPage()

  await page.goto('http://localhost:7777')
  await page.waitForSelector('.xterm-viewport', { timeout: 10000 })
  await page.waitForTimeout(5000)

  const details = await page.evaluate(() => {
    const xtermDivs = Array.from(document.querySelectorAll('.xterm'))
    return xtermDivs.map(div => {
      const viewport = div.querySelector('.xterm-viewport') as HTMLElement | null
      const screen = div.querySelector('.xterm-screen') as HTMLElement | null
      const canvases = Array.from(div.querySelectorAll('canvas'))
      const rowsDiv = div.querySelector('.xterm-rows') as HTMLElement | null

      // Count visible rows and columns from the accessibility DOM
      const rows = Array.from(rowsDiv?.children ?? [])
      const colCount = rows[0]?.textContent?.length ?? 0

      return {
        viewportW: viewport?.clientWidth,
        viewportH: viewport?.clientHeight,
        screenW: screen?.clientWidth,
        screenH: screen?.clientHeight,
        canvasCount: canvases.length,
        canvasSizes: canvases.map(c => `${c.width}x${c.height}`),
        rowCount: rows.length,
        colCount,
        sampleRow0: rows[0]?.textContent?.substring(0, 80),
        sampleRow5: rows[5]?.textContent?.substring(0, 80),
      }
    })
  })

  for (let i = 0; i < details.length; i++) {
    console.log(`Terminal ${i}:`, JSON.stringify(details[i], null, 2))
  }

  for (const d of details) {
    // Screen must not overflow the viewport (no horizontal clipping)
    expect(d.screenW ?? 0).toBeLessThanOrEqual((d.viewportW ?? 0) + 20)
    // Screen must have a reasonable height
    expect(d.screenH ?? 0).toBeGreaterThan(50)
    // Must have rows
    expect(d.rowCount).toBeGreaterThan(5)
  }

  await browser.close()
})
