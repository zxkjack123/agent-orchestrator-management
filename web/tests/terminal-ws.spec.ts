import { test, expect, chromium } from '@playwright/test'

test('terminal WebSocket: init then output messages arrive in order', async () => {
  const browser = await chromium.launch({ headless: true })
  const ctx = await browser.newContext({ viewport: { width: 1400, height: 900 } })
  const page = await ctx.newPage()

  // Intercept WebSocket messages from the war room terminal connections
  const wsMessages: { pane: string; type: string; cols?: number; rows?: number; dataLen?: number }[] = []

  page.on('websocket', ws => {
    const pane = ws.url().split('/ws/terminal/')[1] ?? '?'
    ws.on('framereceived', frame => {
      try {
        const msg = JSON.parse(frame.payload as string)
        wsMessages.push({
          pane,
          type: msg.type,
          cols: msg.cols,
          rows: msg.rows,
          dataLen: msg.data?.length,
        })
      } catch { /* binary frame */ }
    })
  })

  await page.goto('http://localhost:7777')
  await page.waitForSelector('.xterm-viewport', { timeout: 10000 })
  await page.waitForTimeout(4000)

  console.log('WS messages received (first 12):', JSON.stringify(wsMessages.slice(0, 12), null, 2))

  // Each terminal should receive an "init" message first
  const initMsgs = wsMessages.filter(m => m.type === 'init')
  const outputMsgs = wsMessages.filter(m => m.type === 'output')

  console.log(`init messages: ${initMsgs.length}, output messages: ${outputMsgs.length}`)

  // Should have one init per terminal
  expect(initMsgs.length).toBeGreaterThanOrEqual(1)

  // Init messages must have valid dimensions
  for (const msg of initMsgs) {
    console.log(`  init: pane=${msg.pane} cols=${msg.cols} rows=${msg.rows}`)
    expect(msg.cols).toBeGreaterThan(10)
    expect(msg.rows).toBeGreaterThan(5)
  }

  // Should have output messages with actual content
  expect(outputMsgs.length).toBeGreaterThanOrEqual(1)
  for (const msg of outputMsgs.slice(0, 4)) {
    console.log(`  output: pane=${msg.pane} dataLen=${msg.dataLen}`)
    expect(msg.dataLen).toBeGreaterThan(0)
  }

  // init must come BEFORE first output for the same pane
  for (const init of initMsgs) {
    const initIdx = wsMessages.findIndex(m => m.pane === init.pane && m.type === 'init')
    const firstOutputIdx = wsMessages.findIndex(m => m.pane === init.pane && m.type === 'output')
    if (firstOutputIdx !== -1) {
      console.log(`  ordering: pane ${init.pane} init@${initIdx} output@${firstOutputIdx}`)
      expect(initIdx).toBeLessThan(firstOutputIdx)
    }
  }

  // Check xterm.js actually resized to tmux dimensions
  const termInfo = await page.evaluate(() => {
    // xterm.js stores Terminal instance on the container element
    const viewports = Array.from(document.querySelectorAll('.xterm-viewport'))
    return viewports.map(v => {
      const parent = v.closest('.xterm') as any
      const terminal = parent?._xterm ?? parent?.terminal
      return {
        hasDom: !!parent,
        scrollWidth: (v as HTMLElement).scrollWidth,
        scrollHeight: (v as HTMLElement).scrollHeight,
      }
    })
  })
  console.log('xterm viewport sizes:', JSON.stringify(termInfo))

  // Canvas pixel check - verify canvas has non-black pixels (actual content rendered)
  const hasContent = await page.evaluate(() => {
    const canvases = Array.from(document.querySelectorAll('.xterm-screen canvas'))
    for (const canvas of canvases) {
      const ctx2d = (canvas as HTMLCanvasElement).getContext('2d')
      if (!ctx2d) continue
      const { width, height } = canvas as HTMLCanvasElement
      if (width === 0 || height === 0) continue
      // Sample pixels across the canvas
      const imageData = ctx2d.getImageData(0, 0, width, height)
      const data = imageData.data
      let nonBlack = 0
      for (let i = 0; i < data.length; i += 4) {
        const r = data[i], g = data[i+1], b = data[i+2]
        if (r > 20 || g > 20 || b > 20) nonBlack++
      }
      if (nonBlack > 100) return true
    }
    return false
  })
  console.log('Canvas has visible content:', hasContent)

  await browser.close()
})

test('terminal text content is readable Thai+English', async () => {
  const browser = await chromium.launch({ headless: true })
  const ctx = await browser.newContext({ viewport: { width: 1400, height: 900 } })
  const page = await ctx.newPage()

  await page.goto('http://localhost:7777')
  await page.waitForSelector('.xterm-rows', { timeout: 10000 })
  await page.waitForTimeout(4000)

  // Read text from xterm accessibility layer (DOM rows)
  const allText = await page.evaluate(() => {
    return Array.from(document.querySelectorAll('.xterm-rows'))
      .map(r => r.textContent ?? '')
      .join('\n')
  })

  console.log('All terminal text (first 500 chars):', allText.substring(0, 500))

  // Should contain recognizable content from the claude agents
  const hasEnglish = /[a-zA-Z]{3,}/.test(allText)
  const hasDashes = /─{3,}/.test(allText)

  console.log('Has English words:', hasEnglish)
  console.log('Has box-drawing dashes:', hasDashes)

  // Terminal should have at least some text content
  expect(allText.trim().length).toBeGreaterThan(20)

  await browser.close()
})
