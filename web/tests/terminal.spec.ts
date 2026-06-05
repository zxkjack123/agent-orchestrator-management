import { test, expect, chromium } from '@playwright/test'

test('terminal renders pane content correctly', async () => {
  const browser = await chromium.launch({ headless: true })
  const ctx = await browser.newContext({ viewport: { width: 1400, height: 900 } })
  const page = await ctx.newPage()

  // Capture console errors
  const consoleErrors: string[] = []
  page.on('console', msg => {
    if (msg.type() === 'error') consoleErrors.push(msg.text())
  })

  await page.goto('http://localhost:7777')
  await page.waitForLoadState('networkidle')

  // Wait for war room terminals to appear
  await page.waitForSelector('.xterm-viewport', { timeout: 10000 })

  // Wait a bit for WS to connect and render content
  await page.waitForTimeout(3000)

  // Screenshot the whole page
  await page.screenshot({ path: '/tmp/war-room-full.png', fullPage: true })

  // Screenshot just the first terminal pane
  const firstPane = page.locator('.xterm-screen').first()
  await firstPane.screenshot({ path: '/tmp/terminal-pane-0.png' })

  // Check xterm canvas dimensions match what we expect
  const xtermRows = await page.evaluate(() => {
    const screens = document.querySelectorAll('.xterm-screen')
    const results: { width: number; height: number; cols: number; rows: number }[] = []
    screens.forEach(s => {
      const r = s.getBoundingClientRect()
      results.push({ width: Math.round(r.width), height: Math.round(r.height), cols: 0, rows: 0 })
    })
    return results
  })
  console.log('xterm screen dimensions:', JSON.stringify(xtermRows))

  // Check terminal text content
  const terminalTexts = await page.evaluate(() => {
    const rows = document.querySelectorAll('.xterm-rows')
    const results: string[] = []
    rows.forEach(r => {
      const text = r.textContent ?? ''
      if (text.trim()) results.push(text.trim().substring(0, 200))
    })
    return results
  })
  console.log('Terminal text content:', JSON.stringify(terminalTexts.slice(0, 4)))

  // Check WebSocket connection log
  const wsLogs = await page.evaluate(() => {
    return (window as any).__wsLogs ?? []
  })

  console.log('Console errors:', consoleErrors)

  // Verify terminals are showing content (not empty)
  const hasContent = terminalTexts.some(t => t.length > 5)
  expect(hasContent).toBe(true)

  await browser.close()
})

test('terminal dimensions match tmux pane', async () => {
  const browser = await chromium.launch({ headless: true })
  const ctx = await browser.newContext({ viewport: { width: 1400, height: 900 } })
  const page = await ctx.newPage()

  await page.goto('http://localhost:7777')
  await page.waitForSelector('.xterm-viewport', { timeout: 10000 })
  await page.waitForTimeout(4000)

  // Get the terminal cols/rows from xterm internals
  const termDims = await page.evaluate(() => {
    const terminals = (window as any).__xtermInstances ?? []
    // Try to find xterm instances via the DOM
    const viewports = document.querySelectorAll('.xterm-viewport')
    const dims: { cols: number; rows: number; scrollWidth: number; scrollHeight: number }[] = []
    viewports.forEach(v => {
      const screen = v.previousElementSibling as HTMLElement
      if (screen) {
        const rows = screen.querySelectorAll('.xterm-rows > div')
        const cols = rows[0]?.querySelectorAll('span').length ?? 0
        dims.push({
          cols,
          rows: rows.length,
          scrollWidth: v.scrollWidth,
          scrollHeight: v.scrollHeight,
        })
      }
    })
    return dims
  })
  console.log('Terminal DOM dimensions:', JSON.stringify(termDims))

  await page.screenshot({ path: '/tmp/war-room-dims.png', fullPage: true })
  await browser.close()
})
