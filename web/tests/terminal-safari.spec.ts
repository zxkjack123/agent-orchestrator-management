import { test, expect, webkit } from '@playwright/test'

// Test with WebKit (Safari engine) to catch Safari-specific rendering issues
test('terminal renders in WebKit (Safari)', async () => {
  const browser = await webkit.launch({ headless: true })
  const ctx = await browser.newContext({ viewport: { width: 1400, height: 900 } })
  const page = await ctx.newPage()

  const consoleErrors: string[] = []
  const consoleLogs: string[] = []
  page.on('console', msg => {
    if (msg.type() === 'error') consoleErrors.push(msg.text())
    if (msg.type() === 'log') consoleLogs.push(msg.text())
  })

  page.on('pageerror', err => consoleErrors.push('PAGE ERROR: ' + err.message))

  await page.goto('http://localhost:7777')
  await page.waitForSelector('.xterm', { timeout: 10000 })
  await page.waitForTimeout(4000)

  // Check xterm DOM structure
  const xtermInfo = await page.evaluate(() => {
    const xtermDivs = Array.from(document.querySelectorAll('.xterm'))
    return xtermDivs.map(div => {
      const viewport = div.querySelector('.xterm-viewport') as HTMLElement | null
      const screen = div.querySelector('.xterm-screen') as HTMLElement | null
      const canvases = Array.from(div.querySelectorAll('canvas'))
      const helperContainer = div.querySelector('.xterm-helpers') as HTMLElement | null

      return {
        xtermClientH: (div as HTMLElement).clientHeight,
        xtermClientW: (div as HTMLElement).clientWidth,
        viewportClientH: viewport?.clientHeight,
        viewportClientW: viewport?.clientWidth,
        screenClientH: screen?.clientHeight,
        screenClientW: screen?.clientWidth,
        canvasCount: canvases.length,
        canvasSizes: canvases.map(c => `${(c as HTMLCanvasElement).width}x${(c as HTMLCanvasElement).height} style=${c.style.cssText}`),
        hasHelpers: !!helperContainer,
        xtermStyle: (div as HTMLElement).getAttribute('style') ?? '',
      }
    })
  })

  console.log('WebKit xterm info:', JSON.stringify(xtermInfo, null, 2))
  console.log('Console errors:', consoleErrors)
  console.log('Console logs (last 5):', consoleLogs.slice(-5))

  await page.screenshot({ path: '/tmp/safari-war-room.png', fullPage: true })

  // Check WebSocket messages arrive (same as Chrome test)
  await browser.close()
})

test('terminal writes content in WebKit', async () => {
  const browser = await webkit.launch({ headless: true })
  const ctx = await browser.newContext({ viewport: { width: 1400, height: 900 } })
  const page = await ctx.newPage()

  // Intercept WS messages
  const wsMessages: { type: string; dataLen?: number }[] = []
  page.on('websocket', ws => {
    ws.on('framereceived', frame => {
      try {
        const msg = JSON.parse(frame.payload as string)
        wsMessages.push({ type: msg.type, dataLen: msg.data?.length })
      } catch { }
    })
  })

  await page.goto('http://localhost:7777')
  await page.waitForSelector('.xterm', { timeout: 10000 })
  await page.waitForTimeout(4000)

  const outputMsgs = wsMessages.filter(m => m.type === 'output')
  console.log('Output messages received:', outputMsgs.length)
  console.log('First few:', JSON.stringify(outputMsgs.slice(0, 3)))

  // Check accessible text in DOM
  const accessibleText = await page.evaluate(() => {
    return Array.from(document.querySelectorAll('.xterm-rows'))
      .map(r => r.textContent?.trim().substring(0, 100) ?? '')
      .filter(t => t.length > 0)
  })
  console.log('Accessible text:', accessibleText.slice(0, 4))

  expect(outputMsgs.length).toBeGreaterThan(0)
  await browser.close()
})
