import assert from 'node:assert'
import { Builder, By, until } from 'selenium-webdriver'
import chrome from 'selenium-webdriver/chrome.js'

const base = process.env.TYCOONISM_BASE_URL || 'http://127.0.0.1:18080'

describe('Tycoonism smoke', function () {
  this.timeout(30000)
  let driver

  before(async function () {
    const options = new chrome.Options()
    options.addArguments('--headless=new', '--no-sandbox', '--disable-dev-shm-usage')
    driver = await new Builder().forBrowser('chrome').setChromeOptions(options).build()
  })

  after(async function () {
    if (driver) {
      await driver.quit()
    }
  })

  it('serves the game UI with Tycoonism title', async function () {
    await driver.get(`${base}/game`)
    await driver.wait(until.elementLocated(By.css('body')), 10000)
    const title = await driver.getTitle()
    assert.strictEqual(title, 'Tycoonism')
  })

  it('renders the login or game shell', async function () {
    await driver.get(`${base}/game`)
    await driver.wait(until.elementLocated(By.css('#app')), 10000)
    const body = await driver.findElement(By.css('body')).getText()
    assert.match(body, /Tycoonism|Buildings|Sign in/)
  })
})
