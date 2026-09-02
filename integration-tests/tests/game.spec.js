import assert from 'node:assert'
import { Builder, By, until } from 'selenium-webdriver'
import chrome from 'selenium-webdriver/chrome.js'

const base = process.env.TYCOONISM_BASE_URL || 'http://127.0.0.1:18080'

async function login(driver) {
  await driver.get(`${base}/game`)
  const buildingsLinks = await driver.findElements(By.linkText('Buildings'))
  if (buildingsLinks.length > 0) {
    return
  }
  await driver.wait(until.elementLocated(By.id('username')), 10000)
  await driver.findElement(By.id('username')).sendKeys('testuser')
  await driver.findElement(By.id('password')).sendKeys('testpass')
  await driver.findElement(By.css('button[type="submit"].good')).click()
  await driver.wait(until.elementLocated(By.linkText('Buildings')), 10000)
}

describe('Tycoonism game flow', function () {
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

  it('logs in with local credentials', async function () {
    await login(driver)
    const header = await driver.findElement(By.css('.game-header, .game-layout, main')).getText()
    assert.match(header, /Buildings|Resources|Bank/)
  })

  it('places a building on the grid', async function () {
    await login(driver)
    await driver.findElement(By.linkText('Buildings')).click()
    await driver.wait(until.elementLocated(By.css('.game-grid')), 10000)

    const emptyCell = await driver.findElement(By.css('.game-cell:not(.game-cell--has-building)'))
    await emptyCell.click()

    await driver.wait(until.elementLocated(By.css('.game-build-menu')), 10000)
    const buildBtn = await driver.findElement(By.css('.game-build-menu__building-btn'))
    await buildBtn.click()

    await driver.wait(until.elementLocated(By.css('.game-cell--has-building')), 10000)
    const buildingCells = await driver.findElements(By.css('.game-cell--has-building'))
    assert.ok(buildingCells.length >= 1)
  })
})
