const baseURL = '/'
let currentPlayerId = 'player-1'

export function setPlayerId(id) {
  currentPlayerId = id ?? 'player-1'
}

function connectFetch(service, method, body = {}) {
  const path = `/${service}/${method}`
  const url = baseURL ? `${baseURL.replace(/\/$/, '')}${path}` : path
  return fetch(url, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'Accept': 'application/json',
      'Connect-Protocol-Version': '1',
      'X-Player-ID': currentPlayerId,
    },
    body: JSON.stringify(body),
  }).then((r) => {
    if (!r.ok) {
      return r.json().then((err) => {
        throw new Error(err.message || r.statusText)
      }).catch(() => {
        throw new Error(r.statusText)
      })
    }
    return r.json()
  })
}

export function getInit() {
  return connectFetch('game.v1.WorldService', 'Init', {})
}

export function postLogin(username, password) {
  return fetch('/login', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ username, password }),
    credentials: 'include',
  }).then((r) => {
    if (!r.ok) {
      return r.json().then((body) => {
        throw new Error(body.error || r.statusText)
      }).catch((e) => {
        if (e instanceof Error && e.message) throw e
        throw new Error(r.statusText)
      })
    }
    return r.json()
  })
}

export function getWorldState() {
  return connectFetch('game.v1.WorldService', 'GetWorldState', {})
}

export function getPlayerState() {
  return connectFetch('game.v1.GameService', 'GetPlayerState', {})
}

export function placeBuilding(buildingTypeId, cellX, cellY) {
  return connectFetch('game.v1.GameService', 'PlaceBuilding', {
    building_type_id: buildingTypeId,
    cell_x: cellX,
    cell_y: cellY,
  })
}

export function startUpgrade(cellX, cellY) {
  return connectFetch('game.v1.GameService', 'StartUpgrade', { cell_x: cellX, cell_y: cellY })
}

export function sellResource(resourceId, quantity = 0) {
  return connectFetch('game.v1.GameService', 'SellResource', {
    resource_id: resourceId,
    quantity,
  })
}

export function buyResource(resourceId, quantity) {
  return connectFetch('game.v1.GameService', 'BuyResource', {
    resource_id: resourceId,
    quantity,
  })
}

export function cancelUpgrade(cellX, cellY) {
  return connectFetch('game.v1.GameService', 'CancelUpgrade', { cell_x: cellX, cell_y: cellY })
}

export function startSellBuilding(cellX, cellY) {
  return connectFetch('game.v1.GameService', 'StartSellBuilding', { cell_x: cellX, cell_y: cellY })
}

export function getLeaderboard() {
  return connectFetch('game.v1.GameService', 'GetLeaderboard', {})
}

export function angelInvestor() {
  return connectFetch('game.v1.GameService', 'AngelInvestor', {})
}

export function getMarketplace(resourceId = '') {
  return connectFetch('game.v1.GameService', 'GetMarketplace', { resource_id: resourceId })
}

export function getResourcePriceHistory(resourceId, ticks = 15) {
  return connectFetch('game.v1.GameService', 'GetResourcePriceHistory', {
    resource_id: resourceId,
    ticks,
  })
}

export function getMessages() {
  return connectFetch('game.v1.GameService', 'GetMessages', {})
}

export function getOrder(orderId) {
  return connectFetch('game.v1.GameService', 'GetOrder', { order_id: orderId })
}

export function placeSellOrder(resourceId, quantity, pricePerUnit) {
  return connectFetch('game.v1.GameService', 'PlaceSellOrder', {
    resource_id: resourceId,
    quantity,
    price_per_unit: pricePerUnit,
  })
}

export function placeBuyOrder(resourceId, quantity, pricePerUnit) {
  return connectFetch('game.v1.GameService', 'PlaceBuyOrder', {
    resource_id: resourceId,
    quantity,
    price_per_unit: pricePerUnit,
  })
}

export function fulfillSellOrder(orderId) {
  return connectFetch('game.v1.GameService', 'FulfillSellOrder', { order_id: orderId })
}

export function fulfillBuyOrder(orderId) {
  return connectFetch('game.v1.GameService', 'FulfillBuyOrder', { order_id: orderId })
}

export function cancelMarketOrder(orderId) {
  return connectFetch('game.v1.GameService', 'CancelOrder', { order_id: orderId })
}

export function getLoans() {
  return connectFetch('game.v1.GameService', 'GetLoans', {})
}

export function takeLoan(amount) {
  return connectFetch('game.v1.GameService', 'TakeLoan', { amount })
}

export function payOffLoan(loanId, amount) {
  return connectFetch('game.v1.GameService', 'PayOffLoan', { loan_id: loanId, amount })
}
