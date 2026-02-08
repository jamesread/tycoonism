<template>
  <template v-if="showLoginForm">
    <Section
      title="Tycoonism"
      subtitle="Sign in to play"
      :padding="true"
      classes="game-section"
    >
      <Login
        ref="loginRef"
        :show-default-tabs="localLoginEnabled"
        :custom-tabs="localLoginEnabled ? [] : [{ id: 'oauth', label: 'Sign in' }]"
        :oauth-providers="oauthProvidersForLogin"
        @local-login="handleLocalLogin"
        @oauth-login="handleOAuthLogin"
      >
        <template #tab-oauth>
          <div class="login-section">
            <div
              v-if="oauthProvidersForLogin.length > 0"
              class="oauth-providers"
            >
              <button
                v-for="p in oauthProvidersForLogin"
                :key="p.id"
                type="button"
                class="oauth-button good"
                :title="`Sign in with ${p.name}`"
                @click="handleOAuthLogin(p)"
              >
                <span>{{ p.name }}</span>
              </button>
            </div>
            <div
              v-else
              class="no-providers"
            >
              <p>No OAuth providers configured.</p>
            </div>
          </div>
        </template>
      </Login>
    </Section>
  </template>
  <div v-else class="game-layout">
    <Header
      title="Tycoonism"
      logo-url="/logo.svg"
      :sidebar-enabled="false"
      :top-bar-enabled="false"
      :show-branding="true"
      class="game-header"
    >
      <template #toolbar>
        <div
          class="game-tick-progress"
          role="progressbar"
          :aria-valuenow="Math.round(tickProgress * 100)"
          aria-valuemin="0"
          aria-valuemax="100"
          :title="`Next tick in ${tickCountdown}s`"
        >
          <div class="game-tick-progress__fill" :style="{ width: `${tickProgress * 100}%` }" />
        </div>
        <span class="game-header-resources">
          <Money :value="state.coins" />
          <span v-if="headerPinnedText">{{ headerPinnedText }}</span>
        </span>
        <router-link to="/game/resources">Resources</router-link>
        <router-link to="/game/bank">Bank</router-link>
        <router-link to="/game/market">Market</router-link>
        <router-link to="/game/buildings">Buildings</router-link>
        <router-link to="/game/messages">Messages</router-link>
      </template>
    </Header>
    <main class="game-content">
      <router-view />
    </main>
    <footer class="game-footer">
      <span>{{ worldCountdownText }}</span>
    </footer>
  </div>

  <dialog v-if="sellOrderDialogOpen" ref="sellOrderDialogRef" class="game-dialog" aria-label="Sell order">
    <h3>Sell order</h3>
    <template v-if="!sellOrderDialogResource">
      <p class="game-dialog__hint">Choose a resource to offer for sale. You set the price in the next step.</p>
      <label :for="sellOrderResourceSelectId">Resource</label>
      <select :id="sellOrderResourceSelectId" v-model="sellOrderDialogResourceId">
        <option value="">—</option>
        <option
          v-for="res in resourcesWithStock"
          :key="res.id ?? res.Id"
          :value="res.id ?? res.Id"
        >
          {{ res.name ?? res.Name }} (have {{ resourceAmount(res.id ?? res.Id) }})
        </option>
      </select>
      <div role="toolbar" class="game-dialog__actions">
        <button type="button" class="good" :disabled="!sellOrderDialogResourceId" @click="selectSellOrderResource">Next</button>
        <button type="button" @click="closeSellOrderDialog">Cancel</button>
      </div>
    </template>
    <template v-else>
      <h4>{{ sellOrderDialogResource.name ?? sellOrderDialogResource.Name }}</h4>
      <p class="game-dialog__hint">Available: {{ resourceAmount(sellOrderDialogResource.id ?? sellOrderDialogResource.Id) }}</p>
      <p class="game-dialog__hint">Market: <Money :value="sellOrderDialogMarketPrice" /></p>
      <label :for="sellOrderPriceId">Price per unit</label>
      <div class="game-export-slider">
        <input
          :id="sellOrderPriceSliderId"
          v-model.number="sellOrderDialogPrice"
          type="range"
          :min="sellOrderPriceSliderMin"
          :max="sellOrderPriceSliderMax"
          step="1"
          class="game-export-slider__input"
          aria-valuetext="Price"
        />
        <span class="game-export-slider__value"><Money :value="sellOrderDialogPrice" /></span>
      </div>
      <input :id="sellOrderPriceId" v-model.number="sellOrderDialogPrice" type="number" min="1" class="game-order-price-input" aria-label="Price per unit (exact)" />
      <div class="game-export-slider">
        <label :for="sellOrderSliderId">Quantity to sell</label>
        <input
          :id="sellOrderSliderId"
          v-model.number="sellOrderDialogQuantity"
          type="range"
          min="0"
          :max="sellOrderDialogMax"
          step="1"
          class="game-export-slider__input"
        />
        <span class="game-export-slider__value">{{ sellOrderDialogQuantity }}</span>
      </div>
      <p class="game-export-preview">
        Total: <Icon icon="game-icons:two-coins" width="1em" height="1em" class="game-coins-icon" aria-hidden="true" /> {{ sellOrderPreviewTotal }}
      </p>
      <div role="toolbar" class="game-dialog__actions">
        <button type="button" class="good" :disabled="sellOrderDialogQuantity <= 0 || sellOrderDialogPrice < 1" @click="confirmSellOrder">
          Place sell order
        </button>
        <button type="button" @click="closeSellOrderDialog">Cancel</button>
      </div>
    </template>
  </dialog>

  <dialog v-if="buyOrderDialogOpen" ref="buyOrderDialogRef" class="game-dialog" aria-label="Buy order">
    <h3>Buy order</h3>
    <template v-if="!buyOrderDialogResource">
      <p class="game-dialog__hint">Choose a resource and set your price in the next step.</p>
      <label :for="buyOrderResourceSelectId">Resource</label>
      <select :id="buyOrderResourceSelectId" v-model="buyOrderDialogResourceId">
        <option value="">—</option>
        <option
          v-for="res in resourceDefinitions"
          :key="res.id ?? res.Id"
          :value="res.id ?? res.Id"
        >
          {{ res.name ?? res.Name }}
        </option>
      </select>
      <div role="toolbar" class="game-dialog__actions">
        <button type="button" class="good" :disabled="!buyOrderDialogResourceId" @click="selectBuyOrderResource">Next</button>
        <button type="button" @click="closeBuyOrderDialog">Cancel</button>
      </div>
    </template>
    <template v-else>
      <h4>{{ buyOrderDialogResource.name ?? buyOrderDialogResource.Name }}</h4>
      <p class="game-dialog__hint">Coins: <Icon icon="game-icons:two-coins" width="1em" height="1em" class="game-coins-icon" aria-hidden="true" /> {{ num(state.coins) }}</p>
      <label :for="buyOrderPriceId">Price per unit</label>
      <input :id="buyOrderPriceId" v-model.number="buyOrderDialogPrice" type="number" min="1" class="game-order-price-input" />
      <div class="game-export-slider">
        <label :for="buyOrderSliderId">Quantity to buy</label>
        <input
          :id="buyOrderSliderId"
          v-model.number="buyOrderDialogQuantity"
          type="range"
          min="0"
          :max="buyOrderDialogMax"
          step="1"
          class="game-export-slider__input"
        />
        <span class="game-export-slider__value">{{ buyOrderDialogQuantity }}</span>
      </div>
      <p class="game-export-preview">
        Cost: <Icon icon="game-icons:two-coins" width="1em" height="1em" class="game-coins-icon" aria-hidden="true" /> {{ buyOrderPreviewCost }}
      </p>
      <div role="toolbar" class="game-dialog__actions">
        <button type="button" class="good" :disabled="buyOrderDialogQuantity <= 0 || buyOrderDialogPrice < 1 || num(state.coins) < buyOrderPreviewCost" @click="confirmBuyOrder">
          Place buy order
        </button>
        <button type="button" @click="closeBuyOrderDialog">Cancel</button>
      </div>
    </template>
  </dialog>

</template>

<script setup>
import { ref, computed, watch, nextTick, onMounted, onUnmounted, provide } from 'vue'
import Section from 'picocrank/vue/components/Section.vue'
import Header from 'picocrank/vue/components/Header.vue'
import Login from 'picocrank/vue/components/Login.vue'
import Money from '../components/Money.vue'
import { Icon } from '@iconify/vue'
import {
  getInit,
  getWorldState,
  getPlayerState,
  getLeaderboard,
  getMarketplace,
  placeBuilding,
  startUpgrade,
  startSellBuilding,
  sellResource as sellResourceApi,
  buyResource as buyResourceApi,
  placeSellOrder,
  placeBuyOrder,
  fulfillSellOrder,
  fulfillBuyOrder,
  cancelMarketOrder,
  getLoans,
  takeLoan,
  payOffLoan,
  angelInvestor,
  postLogin,
  setPlayerId,
} from '../api/client'

const state = ref({
  playerId: '',
  coins: 0,
  resources: {},
  resourcesPerMin: {},
  grid: [],
})
const world = ref({
  currentDay: 1,
  cycleId: 0,
  nextTickAt: 0,
  placementCost: 0,
  resourceDefinitions: [],
  buildingDefinitions: [],
})
const buildMenuCell = ref(null)
const selectedBuilding = ref(null)
const leaderboardEntries = ref([])
const sellConfirmOpen = ref(false)
const authenticationProviders = ref([])
const localLoginEnabled = ref(false)
const username = ref('')
const initDone = ref(false)
const pinnedResourceIds = ref([])
const error = ref('')
const nowSeconds = ref(Math.floor(Date.now() / 1000))
let lastActionAppliedAt = 0
let refreshRequestedAt = 0
const loansList = ref([])
const offeredAmounts = ref([])
const loanPayAmounts = ref({})
const requirementsDialogOpen = ref(false)
const requirementsDialogBuildingTypeId = ref('')
const sellOrderDialogOpen = ref(false)
const sellOrderDialogResource = ref(null)
const sellOrderDialogResourceId = ref('')
const sellOrderDialogQuantity = ref(0)
const sellOrderDialogPrice = ref(0)
const sellOrderDialogRef = ref(null)
const sellOrderPriceId = 'sell-order-price'
const buyOrderDialogOpen = ref(false)
const buyOrderDialogResource = ref(null)
const buyOrderDialogResourceId = ref('')
const buyOrderDialogQuantity = ref(0)
const buyOrderDialogPrice = ref(0)
const buyOrderDialogRef = ref(null)
const sellOrderResourceSelectId = 'sell-order-resource'
const buyOrderResourceSelectId = 'buy-order-resource'
const buyOrderPriceId = 'buy-order-price'
const loginRef = ref(null)
const marketplaceSellOrders = ref([])
const marketplaceBuyOrders = ref([])

const GRID_SIZE = 5

function num(v) {
  const n = Number(v)
  return Number.isFinite(n) ? n : 0
}

function responseState(r) {
  if (!r || typeof r !== 'object') return null
  return r.state ?? r
}

function normState(r) {
  const s = responseState(r)
  if (!s || typeof s !== 'object') return state.value
  const resources = s.resources ?? {}
  const resourcesPerMin = s.resourcesPerMin ?? s.resources_per_min ?? {}
  const resMap = {}
  for (const [k, v] of Object.entries(resources)) {
    resMap[k] = num(v)
  }
  const rateMap = {}
  for (const [k, v] of Object.entries(resourcesPerMin)) {
    rateMap[k] = num(v)
  }
  const sellOrderIds = s.sellOrderIds ?? s.sell_order_ids
  const buyOrderIds = s.buyOrderIds ?? s.buy_order_ids
  return {
    playerId: String(s.playerId ?? s.player_id ?? ''),
    coins: num(s.coins),
    resources: resMap,
    resourcesPerMin: rateMap,
    grid: Array.isArray(s.grid) ? s.grid : [],
    sellOrderIds: Array.isArray(sellOrderIds) ? sellOrderIds : [],
    buyOrderIds: Array.isArray(buyOrderIds) ? buyOrderIds : [],
  }
}

const resourceDefinitions = computed(() => world.value.resourceDefinitions ?? world.value.resource_definitions ?? [])
const buildingDefinitions = computed(() => world.value.buildingDefinitions ?? world.value.building_definitions ?? [])

const authProvidersList = computed(() => authenticationProviders.value ?? [])
const oauthProvidersForLogin = computed(() => {
  const list = authProvidersList.value
  if (!Array.isArray(list)) return []
  return list.map((p) => ({
    id: p.id ?? p.Id ?? '',
    name: p.name ?? p.Name ?? p.id ?? p.Id ?? 'Provider',
    class: 'good',
  }))
})
const showLoginForm = computed(() => initDone.value && !username.value)

const requirementsDialogBuildingName = computed(() => {
  const def = buildingDef(requirementsDialogBuildingTypeId.value)
  return def ? (def.name ?? def.Name ?? requirementsDialogBuildingTypeId.value) : requirementsDialogBuildingTypeId.value
})
const requirementsDialogRequirements = computed(() => {
  const def = buildingDef(requirementsDialogBuildingTypeId.value)
  if (!def) return []
  const reqs = def.requirements ?? def.Requirements ?? []
  return reqs.map((r) => {
    const bid = r.building_id ?? r.buildingId ?? ''
    const b = buildingDef(bid)
    return { count: r.count ?? 0, name: b ? (b.name ?? b.Name ?? bid) : bid }
  })
})

const resourcesWithStock = computed(() =>
  (resourceDefinitions.value ?? []).filter((r) => resourceAmount(r.id ?? r.Id) > 0)
)

const sellOrderSliderId = computed(() => {
  const res = sellOrderDialogResource.value
  const id = res?.id ?? res?.Id ?? 'sell-order'
  return `sell-order-slider-${id}`
})
const sellOrderDialogMarketPrice = computed(() => {
  const res = sellOrderDialogResource.value
  if (!res) return 0
  return num(res.sell_price ?? res.sellPrice ?? 0)
})
const sellOrderPriceSliderMin = computed(() => {
  const market = sellOrderDialogMarketPrice.value
  if (market <= 0) return 1
  return Math.max(1, Math.floor(market * 0.5))
})
const sellOrderPriceSliderMax = computed(() => {
  const market = sellOrderDialogMarketPrice.value
  if (market <= 0) return 100
  return Math.max(sellOrderPriceSliderMin.value + 1, Math.ceil(market * 1.5))
})
const sellOrderPriceSliderId = computed(() => {
  const res = sellOrderDialogResource.value
  const id = res?.id ?? res?.Id ?? 'sell-price'
  return `sell-order-price-slider-${id}`
})
const sellOrderDialogMax = computed(() => {
  const res = sellOrderDialogResource.value
  if (!res) return 0
  return Math.max(0, resourceAmount(res.id ?? res.Id))
})
const sellOrderPreviewTotal = computed(() => {
  const price = num(sellOrderDialogPrice.value)
  const qty = Math.max(0, Math.min(num(sellOrderDialogQuantity.value), sellOrderDialogMax.value))
  return qty * price
})

const buyOrderSliderId = computed(() => {
  const res = buyOrderDialogResource.value
  const id = res?.id ?? res?.Id ?? 'buy-order'
  return `buy-order-slider-${id}`
})
const buyOrderDialogMax = computed(() => {
  const price = num(buyOrderDialogPrice.value)
  if (price <= 0) return 0
  const coins = num(state.value.coins)
  return Math.max(0, Math.floor(coins / price))
})
const buyOrderPreviewCost = computed(() => {
  const price = num(buyOrderDialogPrice.value)
  const qty = Math.max(0, Math.min(num(buyOrderDialogQuantity.value), buyOrderDialogMax.value))
  return qty * price
})

async function handleLocalLogin(credentials) {
  const u = credentials?.username ?? ''
  const p = credentials?.password ?? ''
  if (!u || !p) return
  try {
    await postLogin(u, p)
    const r = await getInit()
    username.value = r.username ?? r.Username ?? ''
    authenticationProviders.value = r.authentication_providers ?? r.authenticationProviders ?? []
    localLoginEnabled.value = !!(r.local_login_enabled ?? r.localLoginEnabled)
    setPlayerId(username.value || 'player-1')
  } catch (e) {
    loginRef.value?.setLocalLoginError?.(e?.message ?? 'Login failed')
  }
}

function handleOAuthLogin(provider) {
  const id = provider?.id ?? provider?.Id ?? ''
  if (id) {
    window.location.href = `/oauth/login?provider=${encodeURIComponent(id)}`
  }
}

function resourceAmount(resourceId) {
  return num(state.value.resources?.[resourceId])
}

function buildingDef(typeId) {
  return buildingDefinitions.value.find((b) => (b.id ?? b.Id) === typeId)
}

function placementCostForBuilding(def) {
  if (!def) return 0
  return num(def.placement_cost ?? def.placementCost ?? 0)
}

function placementCostForBuildingType(typeId) {
  return placementCostForBuilding(buildingDef(typeId))
}

function requirementsMetForBuilding(def) {
  if (!def) return true
  const reqs = def.requirements ?? def.Requirements ?? []
  if (reqs.length === 0) return true
  const grid = state.value.grid ?? []
  const countByType = new Map()
  for (const b of grid) {
    const tid = b.typeId ?? b.type_id ?? ''
    countByType.set(tid, (countByType.get(tid) ?? 0) + 1)
  }
  for (const r of reqs) {
    const bid = r.building_id ?? r.buildingId ?? ''
    const need = r.count ?? 0
    const have = countByType.get(bid) ?? 0
    if (have < need) return false
  }
  return true
}

function resourceDef(resourceId) {
  return resourceDefinitions.value.find((r) => (r.id ?? r.Id) === resourceId)
}

function darkenColor(hex, factor = 0.6) {
  if (!hex || typeof hex !== 'string') return null
  const s = hex.trim().replace(/^#/, '')
  let r, g, b
  if (s.length === 6) {
    r = parseInt(s.slice(0, 2), 16)
    g = parseInt(s.slice(2, 4), 16)
    b = parseInt(s.slice(4, 6), 16)
  } else if (s.length === 3) {
    r = parseInt(s[0] + s[0], 16)
    g = parseInt(s[1] + s[1], 16)
    b = parseInt(s[2] + s[2], 16)
  } else {
    return null
  }
  if (Number.isNaN(r) || Number.isNaN(g) || Number.isNaN(b)) return null
  r = Math.round(r * factor)
  g = Math.round(g * factor)
  b = Math.round(b * factor)
  return `#${r.toString(16).padStart(2, '0')}${g.toString(16).padStart(2, '0')}${b.toString(16).padStart(2, '0')}`
}

function relativeLuminance(hex) {
  if (!hex || typeof hex !== 'string') return null
  const s = hex.trim().replace(/^#/, '')
  let r, g, b
  if (s.length === 6) {
    r = parseInt(s.slice(0, 2), 16) / 255
    g = parseInt(s.slice(2, 4), 16) / 255
    b = parseInt(s.slice(4, 6), 16) / 255
  } else if (s.length === 3) {
    r = parseInt(s[0] + s[0], 16) / 255
    g = parseInt(s[1] + s[1], 16) / 255
    b = parseInt(s[2] + s[2], 16) / 255
  } else {
    return null
  }
  if (Number.isNaN(r) || Number.isNaN(g) || Number.isNaN(b)) return null
  const toLinear = (c) => (c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4)
  r = toLinear(r)
  g = toLinear(g)
  b = toLinear(b)
  return 0.2126 * r + 0.7152 * g + 0.0722 * b
}

function textColorForBackground(hex) {
  const L = relativeLuminance(hex)
  if (L == null) return undefined
  return L > 0.179 ? '#000000' : '#ffffff'
}

function buildingDefToButtonStyle(buildingDef) {
  if (!buildingDef) return undefined
  const tickRes = buildingDef.tickResources ?? buildingDef.tick_resources
  const firstResourceId = tickRes && typeof tickRes === 'object' ? Object.keys(tickRes)[0] : null
  if (!firstResourceId) return undefined
  const res = resourceDef(firstResourceId)
  if (!res) return undefined
  const base = res.baseColor ?? res.base_color
  if (!base) return undefined
  const border = darkenColor(base)
  const color = textColorForBackground(base)
  const style = { backgroundColor: base }
  if (border) style.borderColor = border
  if (color) style.color = color
  return style
}

function buildingButtonStyle(building) {
  return buildingDefToButtonStyle(building)
}

function cellButtonStyle(cell) {
  if (!cell?.building) return undefined
  const def = buildingDef(cell.building.typeId ?? cell.building.type_id)
  return buildingDefToButtonStyle(def)
}

const canUseAngelInvestor = computed(() => {
  const coins = num(state.value.coins)
  const buildings = state.value.grid ?? []
  return coins === 0 && buildings.length === 0
})

function buildingCellX(b) {
  const v = b?.cellX ?? b?.cell_x
  return Number.isFinite(Number(v)) ? Number(v) : 0
}
function buildingCellY(b) {
  const v = b?.cellY ?? b?.cell_y
  return Number.isFinite(Number(v)) ? Number(v) : 0
}

const cells = computed(() => {
  const out = []
  const gridMap = new Map(
    (state.value.grid || []).map((b) => [`${buildingCellX(b)},${buildingCellY(b)}`, b])
  )
  for (let y = 0; y < GRID_SIZE; y++) {
    for (let x = 0; x < GRID_SIZE; x++) {
      out.push({
        x,
        y,
        building: gridMap.get(`${x},${y}`) || null,
      })
    }
  }
  return out
})

const tickProgress = computed(() => {
  const nextAt = num(world.value.nextTickAt ?? world.value.next_tick_at)
  const interval = tickIntervalSeconds()
  if (!nextAt || interval <= 0) return 0
  const lastTickAt = nextAt - interval
  const elapsed = nowSeconds.value - lastTickAt
  return Math.min(1, Math.max(0, elapsed / interval))
})

const tickCountdown = computed(() => {
  const nextAt = num(world.value.nextTickAt ?? world.value.next_tick_at)
  if (!nextAt) return 0
  const remaining = nextAt - nowSeconds.value
  return Math.max(0, remaining)
})

function resourceRatePerMin(resourceId) {
  return num(state.value.resourcesPerMin?.[resourceId])
}

function isPinned(resourceId) {
  return pinnedResourceIds.value.includes(resourceId)
}

function togglePinned(resourceId) {
  const idx = pinnedResourceIds.value.indexOf(resourceId)
  if (idx >= 0) {
    pinnedResourceIds.value = pinnedResourceIds.value.filter((id) => id !== resourceId)
  } else {
    pinnedResourceIds.value = [...pinnedResourceIds.value, resourceId]
  }
}

const pinnedResourcesForHeader = computed(() =>
  pinnedResourceIds.value
    .map((id) => resourceDef(id))
    .filter(Boolean)
)

// Server timestamps (world_ends_at, server_time_unix) are always Unix time in seconds.
// They may arrive as string (e.g. "1739491200") due to int64 JSON serialization.
// Normalize to integer seconds: values > 1e12 are treated as ms and divided by 1000.
function toUnixSeconds(v) {
  if (v === undefined || v === null) return null
  const n = typeof v === 'string' ? parseInt(v, 10) : Number(v)
  if (!Number.isFinite(n) || Number.isNaN(n)) return null
  const secs = n > 1e12 ? Math.floor(n / 1000) : Math.floor(n)
  return secs
}

function formatCountdown(endsAtUnix, nowForCountdown) {
  const end = toUnixSeconds(endsAtUnix)
  if (end == null || end <= 0) return ''
  const now = nowForCountdown != null ? toUnixSeconds(nowForCountdown) : nowSeconds.value
  if (now == null) return ''
  const secs = Math.max(0, end - now)
  if (secs === 0) return 'World ended  |  '
  const d = Math.floor(secs / 86400)
  const h = Math.floor((secs % 86400) / 3600)
  const m = Math.floor((secs % 3600) / 60)
  const s = secs % 60
  const parts = []
  if (d > 0) parts.push(`${d}d`)
  parts.push(`${h}h`)
  parts.push(`${m}m`)
  parts.push(`${s}s`)
  return `World ends in ${parts.join(' ')}`
}

const worldCountdownText = computed(() => {
  // world_ends_at from API is Unix timestamp in seconds (may be string from int64 JSON)
  const endsAt = world.value.worldEndsAt ?? world.value.world_ends_at
  const serverUnix = world.value.serverTimeUnix ?? world.value.server_time_unix
  const receivedAt = world.value.worldStateReceivedAtClientSeconds
  const serverSecs = toUnixSeconds(serverUnix)
  const receivedSecs = toUnixSeconds(receivedAt)
  const nowForCountdown =
    serverSecs != null && receivedSecs != null
      ? serverSecs + (nowSeconds.value - receivedSecs)
      : nowSeconds.value
  return formatCountdown(endsAt, nowForCountdown)
})

const headerPinnedText = computed(() => {
  const parts = []
  for (const r of pinnedResourcesForHeader.value) {
    const id = r.id ?? r.Id
    const amount = resourceAmount(id)
    const perMin = resourceRatePerMin(id)
    let str = `${r.name ?? r.Name}: ${amount}`
    if (perMin > 0) str += ` (+${perMin})`
    parts.push(str)
  }
  return parts.length ? '  ' + parts.join('  ') : ''
})

const headerSubtitle = computed(() => {
  const s = state.value
  return `Coins: ${num(s.coins)}${headerPinnedText.value}`
})

function fetchLeaderboard() {
  return getLeaderboard()
    .then((r) => {
      leaderboardEntries.value = r.entries ?? []
    })
    .catch((e) => {
      error.value = e.message
    })
}

function fetchLoansForDialog() {
  return getLoans().then((r) => {
    loansList.value = r.loans ?? []
    offeredAmounts.value = r.offered_amounts ?? r.offeredAmounts ?? []
    loanPayAmounts.value = {}
  })
}

function takeLoanClick(amount) {
  takeLoan(amount)
    .then((r) => {
      state.value = normState(r.state ?? r)
      lastActionAppliedAt = Date.now()
      error.value = ''
      return fetchLoansForDialog()
    })
    .catch((e) => {
      error.value = e?.message ?? 'Take loan failed'
    })
}

function payOffLoanClick(loan) {
  const loanId = loan.loan_id ?? loan.loanId
  const amount = loanPayAmounts.value[loanId]
  if (!amount || amount <= 0) return
  payOffLoan(loanId, amount)
    .then((r) => {
      state.value = normState(r.state ?? r)
      lastActionAppliedAt = Date.now()
      error.value = ''
      loanPayAmounts.value[loanId] = 0
      return fetchLoansForDialog()
    })
    .catch((e) => {
      error.value = e?.message ?? 'Pay off failed'
    })
}

function resourceNameById(id) {
  if (!id) return '—'
  const def = resourceDefinitions.value.find((r) => (r.id ?? r.Id) === id)
  return def ? (def.name ?? def.Name ?? id) : id
}

function isMyMarketOrder(order) {
  const pid = order.player_id ?? order.playerId
  return pid === state.value.playerId
}

function fetchMarketplace() {
  return getMarketplace()
    .then((r) => {
      marketplaceSellOrders.value = r.sell_orders ?? r.sellOrders ?? []
      marketplaceBuyOrders.value = r.buy_orders ?? r.buyOrders ?? []
    })
    .catch((e) => {
      error.value = e?.message ?? 'Failed to load marketplace'
    })
}

function fulfillSellOrderClick(order) {
  const orderId = order.order_id ?? order.orderId
  fulfillSellOrder(orderId)
    .then((r) => {
      state.value = normState(r)
      lastActionAppliedAt = Date.now()
      error.value = ''
      return fetchMarketplace()
    })
    .catch((e) => {
      error.value = e?.message ?? 'Failed to fulfill sell order'
    })
}

function fulfillBuyOrderClick(order) {
  const orderId = order.order_id ?? order.orderId
  fulfillBuyOrder(orderId)
    .then((r) => {
      state.value = normState(r)
      lastActionAppliedAt = Date.now()
      error.value = ''
      return fetchMarketplace()
    })
    .catch((e) => {
      error.value = e?.message ?? 'Failed to fulfill buy order'
    })
}

function cancelOrder(order) {
  const orderId = order.order_id ?? order.orderId
  cancelMarketOrder(orderId)
    .then((r) => {
      state.value = normState(r)
      lastActionAppliedAt = Date.now()
      error.value = ''
      return fetchMarketplace()
    })
    .catch((e) => {
      error.value = e?.message ?? 'Failed to cancel order'
    })
}

function openRequirementsDialog(buildingTypeId) {
  requirementsDialogBuildingTypeId.value = buildingTypeId ?? ''
  requirementsDialogOpen.value = true
}

function closeRequirementsDialog() {
  requirementsDialogOpen.value = false
}

function openSellOrderDialogFromMarketplace() {
  sellOrderDialogResource.value = null
  sellOrderDialogResourceId.value = ''
  sellOrderDialogQuantity.value = 0
  sellOrderDialogPrice.value = 0
  sellOrderDialogOpen.value = true
  nextTick(() => sellOrderDialogRef.value?.showModal())
}

function openBuyOrderDialogFromMarketplace() {
  buyOrderDialogResource.value = null
  buyOrderDialogResourceId.value = ''
  buyOrderDialogQuantity.value = 0
  buyOrderDialogPrice.value = 0
  buyOrderDialogOpen.value = true
  nextTick(() => buyOrderDialogRef.value?.showModal())
}

function openSellOrderDialogForResource(resourceId) {
  if (!resourceId) return
  sellOrderDialogResource.value = null
  sellOrderDialogResourceId.value = resourceId
  sellOrderDialogQuantity.value = 0
  sellOrderDialogPrice.value = 0
  sellOrderDialogOpen.value = true
  nextTick(() => {
    selectSellOrderResource()
    sellOrderDialogRef.value?.showModal()
  })
}

function openBuyOrderDialogForResource(resourceId) {
  if (!resourceId) return
  buyOrderDialogResource.value = null
  buyOrderDialogResourceId.value = resourceId
  buyOrderDialogQuantity.value = 0
  buyOrderDialogPrice.value = 0
  buyOrderDialogOpen.value = true
  nextTick(() => {
    selectBuyOrderResource()
    buyOrderDialogRef.value?.showModal()
  })
}

function selectSellOrderResource() {
  const id = sellOrderDialogResourceId.value
  if (!id) return
  const res = resourceDefinitions.value?.find((r) => (r.id ?? r.Id) === id)
  if (res) {
    sellOrderDialogResource.value = res
    sellOrderDialogQuantity.value = 0
    const market = num(res.sell_price ?? res.sellPrice ?? 0)
    sellOrderDialogPrice.value = Math.max(1, market)
  }
}

function selectBuyOrderResource() {
  const id = buyOrderDialogResourceId.value
  if (!id) return
  const res = resourceDefinitions.value?.find((r) => (r.id ?? r.Id) === id)
  if (res) {
    buyOrderDialogResource.value = res
    buyOrderDialogQuantity.value = 0
    buyOrderDialogPrice.value = 1
  }
}

function closeSellOrderDialog() {
  sellOrderDialogOpen.value = false
  sellOrderDialogResource.value = null
  sellOrderDialogResourceId.value = ''
  sellOrderDialogQuantity.value = 0
  sellOrderDialogPrice.value = 0
  sellOrderDialogRef.value?.close()
}

function closeBuyOrderDialog() {
  buyOrderDialogOpen.value = false
  buyOrderDialogResource.value = null
  buyOrderDialogResourceId.value = ''
  buyOrderDialogQuantity.value = 0
  buyOrderDialogPrice.value = 0
  buyOrderDialogRef.value?.close()
}

function confirmSellOrder() {
  const res = sellOrderDialogResource.value
  const qty = Math.max(0, Math.min(num(sellOrderDialogQuantity.value), sellOrderDialogMax.value))
  const pricePerUnit = Math.max(1, num(sellOrderDialogPrice.value))
  if (!res || qty <= 0 || pricePerUnit < 1) return
  const resourceId = res.id ?? res.Id
  placeSellOrder(resourceId, qty, pricePerUnit)
    .then((r) => {
      state.value = normState(r)
      lastActionAppliedAt = Date.now()
      error.value = ''
      closeSellOrderDialog()
      return fetchMarketplace()
    })
    .catch((e) => {
      error.value = e?.message ?? 'Sell order failed'
    })
}

function confirmBuyOrder() {
  const res = buyOrderDialogResource.value
  const qty = Math.max(0, Math.min(num(buyOrderDialogQuantity.value), buyOrderDialogMax.value))
  const pricePerUnit = Math.max(1, num(buyOrderDialogPrice.value))
  if (!res || qty <= 0 || pricePerUnit < 1) return
  const resourceId = res.id ?? res.Id
  placeBuyOrder(resourceId, qty, pricePerUnit)
    .then((r) => {
      state.value = normState(r)
      lastActionAppliedAt = Date.now()
      error.value = ''
      closeBuyOrderDialog()
      return fetchMarketplace()
    })
    .catch((e) => {
      error.value = e?.message ?? 'Buy order failed'
    })
}

const dialogRate = computed(() => {
  const sel = selectedBuilding.value
  if (!sel?.building) return ''
  const b = sel.building
  const def = buildingDef(b.typeId ?? b.type_id)
  if (!def?.tickResources) return ''
  const parts = []
  for (const [resId, baseQty] of Object.entries(def.tickResources ?? {})) {
    const res = resourceDef(resId)
    const name = res?.name ?? resId
    const perMin = num(state.value.resourcesPerMin?.[resId])
    parts.push(`${name}: ${perMin}/min`)
  }
  return parts.join(', ')
})

const dialogCountdown = computed(() => {
  const sel = selectedBuilding.value
  if (!sel?.building) return ''
  const upgradeEnd = upgradeFinishesAt(sel.building)
  if (upgradeEnd) return `Upgrade completes in ${upgradeCountdown(sel.building)}`
  const sellEnd = sellingFinishesAt(sel.building)
  if (sellEnd) return `Selling completes in ${sellingCountdown(sel.building)}`
  return ''
})

function buildingLabel(typeId) {
  const def = buildingDef(typeId)
  return def?.name ?? def?.Name ?? typeId ?? '?'
}

function buildingLevel(building) {
  const v = building?.level ?? building?.Level
  const n = Number(v)
  return Number.isFinite(n) && n >= 1 ? n : 1
}

function cellLabel(cell) {
  if (cell.building) {
    return `${buildingLabel(cell.building.typeId ?? cell.building.type_id)} level ${buildingLevel(cell.building)}`
  }
  return 'Empty cell (click to build)'
}

function upgradeFinishesAt(building) {
  if (!building) return 0
  const t = building.upgradeFinishesAt ?? building.upgrade_finishes_at
  return Number(t) || 0
}

function sellingFinishesAt(building) {
  if (!building) return 0
  const t = building.sellingFinishesAt ?? building.selling_finishes_at
  return Number(t) || 0
}

function upgradeCountdown(building) {
  const end = upgradeFinishesAt(building)
  if (!end) return ''
  const secs = Math.max(0, end - nowSeconds.value)
  const m = Math.floor(secs / 60)
  const s = secs % 60
  return `${m}:${String(s).padStart(2, '0')}`
}

function sellingCountdown(building) {
  const end = sellingFinishesAt(building)
  if (!end) return ''
  const secs = Math.max(0, end - nowSeconds.value)
  const m = Math.floor(secs / 60)
  const s = secs % 60
  return `${m}:${String(s).padStart(2, '0')}`
}

function canUpgrade(cell) {
  const b = cell?.building
  if (!b || buildingLevel(b) >= 10 || upgradeFinishesAt(b) || sellingFinishesAt(b)) return false
  return true
}

function tickIntervalSeconds() {
  const w = world.value
  const s = w.tickIntervalSeconds ?? w.tick_interval_seconds
  return Math.max(1, num(s))
}

function nextUpgradeCost(building) {
  if (!building || buildingLevel(building) >= 10) return 0
  const placementCost = placementCostForBuildingType(building.typeId ?? building.type_id)
  const toLevel = buildingLevel(building) + 1
  const base = placementCost * 0.7
  return Math.round(base * Math.pow(1.3, toLevel - 2))
}

function onCellClick(cell) {
  if (cell.building) {
    selectedBuilding.value = { ...cell }
    return
  }
  buildMenuCell.value = { x: cell.x, y: cell.y }
}

function doUpgradeFromDialog({ x, y, building }) {
  const cost = nextUpgradeCost(building)
  state.value = { ...state.value, coins: Math.max(0, num(state.value.coins) - cost) }
  const cellX = buildingCellX(building) ?? x
  const cellY = buildingCellY(building) ?? y
  startUpgrade(cellX, cellY)
    .then((r) => {
      state.value = normState(r)
      lastActionAppliedAt = Date.now()
      error.value = ''
      selectedBuilding.value = null
    })
    .catch((e) => {
      error.value = e.message
      state.value = { ...state.value, coins: num(state.value.coins) + cost }
    })
}

function cancelSellConfirm() {
  sellConfirmOpen.value = false
}

function confirmSell() {
  const sel = selectedBuilding.value
  if (!sel?.building) {
    sellConfirmOpen.value = false
    return
  }
  sellConfirmOpen.value = false
  const cellX = buildingCellX(sel.building) ?? sel.x
  const cellY = buildingCellY(sel.building) ?? sel.y
  startSellBuilding(cellX, cellY)
    .then((r) => {
      state.value = normState(r)
      lastActionAppliedAt = Date.now()
      error.value = ''
      selectedBuilding.value = null
    })
    .catch((e) => {
      error.value = e.message
    })
}

function placeFromBuildMenu(buildingTypeId) {
  const cell = buildMenuCell.value
  if (!cell) return
  const cost = placementCostForBuildingType(buildingTypeId)
  state.value = { ...state.value, coins: Math.max(0, num(state.value.coins) - cost) }
  buildMenuCell.value = null
  placeBuilding(buildingTypeId, cell.x, cell.y)
    .then((r) => {
      state.value = normState(r)
      lastActionAppliedAt = Date.now()
      error.value = ''
    })
    .catch((e) => {
      state.value = { ...state.value, coins: num(state.value.coins) + cost }
      const msg = e?.message ?? ''
      if (msg.includes('requirements') || msg.includes('Building requirements')) {
        openRequirementsDialog(buildingTypeId)
      } else {
        error.value = msg
      }
    })
}


function doAngelInvestor() {
  angelInvestor()
    .then((r) => {
      state.value = normState(r)
      lastActionAppliedAt = Date.now()
      error.value = ''
    })
    .catch((e) => {
      error.value = e.message
    })
}

function refresh() {
  getWorldState().then((w) => {
    world.value = {
      currentDay: w.currentDay ?? w.current_day,
      cycleId: w.cycleId ?? w.cycle_id,
      nextTickAt: w.nextTickAt ?? w.next_tick_at,
      worldEndsAt: w.worldEndsAt ?? w.world_ends_at,
      serverTimeUnix: w.serverTimeUnix ?? w.server_time_unix,
      worldStateReceivedAtClientSeconds: Math.floor(Date.now() / 1000),
      placementCost: w.placementCost ?? w.placement_cost,
      resourceDefinitions: w.resourceDefinitions ?? w.resource_definitions ?? [],
      buildingDefinitions: w.buildingDefinitions ?? w.building_definitions ?? [],
      ...w,
    }
  }).catch(() => {})
  refreshRequestedAt = Date.now()
  getPlayerState()
    .then((r) => {
      if (lastActionAppliedAt <= refreshRequestedAt) {
        state.value = normState(r)
      }
    })
    .catch((e) => {
      error.value = e.message
    })
}

let refreshIntervalId
let nowIntervalId

function tickPause() {
  if (refreshIntervalId) {
    clearInterval(refreshIntervalId)
    refreshIntervalId = null
  }
}

function tickResume() {
  if (!refreshIntervalId) {
    refreshIntervalId = setInterval(refresh, 1000)
  }
}

if (typeof window !== 'undefined') {
  window.tickPause = tickPause
  window.tickResume = tickResume
}

provide('gameBuildingsContext', {
  state,
  world,
  buildMenuCell,
  selectedBuilding,
  sellConfirmOpen,
  requirementsDialogOpen,
  requirementsDialogBuildingTypeId,
  error,
  buildingDefinitions,
  resourceDefinitions,
  cells,
  nowSeconds,
  requirementsDialogBuildingName,
  requirementsDialogRequirements,
  dialogRate,
  dialogCountdown,
  buildingDef,
  buildingLabel,
  buildingLevel,
  cellLabel,
  cellButtonStyle,
  placementCostForBuilding,
  placementCostForBuildingType,
  requirementsMetForBuilding,
  buildingButtonStyle,
  openRequirementsDialog,
  placeFromBuildMenu,
  onCellClick,
  upgradeFinishesAt,
  sellingFinishesAt,
  upgradeCountdown,
  sellingCountdown,
  canUpgrade,
  nextUpgradeCost,
  doUpgradeFromDialog,
  confirmSell,
  cancelSellConfirm,
  closeRequirementsDialog,
  num,
  GRID_SIZE,
  clearBuildMenuCell: () => { buildMenuCell.value = null },
  clearSelectedBuilding: () => { selectedBuilding.value = null },
})

provide('gamePageContext', {
  state,
  world,
  resourceDefinitions,
  resourceAmount,
  resourceRatePerMin,
  isPinned,
  togglePinned,
  leaderboardEntries,
  loansList,
  offeredAmounts,
  loanPayAmounts,
  fetchLeaderboard,
  fetchLoansForDialog,
  takeLoanClick,
  payOffLoanClick,
  canUseAngelInvestor,
  doAngelInvestor,
  marketplaceSellOrders,
  marketplaceBuyOrders,
  fetchMarketplace,
  openSellOrderDialogFromMarketplace,
  openBuyOrderDialogFromMarketplace,
  openSellOrderDialogForResource,
  openBuyOrderDialogForResource,
  fulfillSellOrderClick,
  fulfillBuyOrderClick,
  cancelOrder,
  resourceNameById,
  isMyMarketOrder,
  num,
})

watch(sellOrderDialogOpen, (open) => {
  if (open) {
    nextTick(() => sellOrderDialogRef.value?.showModal())
  }
})

watch(sellOrderDialogMax, (max) => {
  if (sellOrderDialogQuantity.value > max) {
    sellOrderDialogQuantity.value = max
  }
})

watch(buyOrderDialogOpen, (open) => {
  if (open) {
    nextTick(() => buyOrderDialogRef.value?.showModal())
  }
})

watch(buyOrderDialogMax, (max) => {
  if (buyOrderDialogQuantity.value > max) {
    buyOrderDialogQuantity.value = max
  }
})

onMounted(() => {
  getInit()
    .then((r) => {
      authenticationProviders.value = r.authentication_providers ?? r.authenticationProviders ?? []
      username.value = r.username ?? r.Username ?? ''
      localLoginEnabled.value = !!(r.local_login_enabled ?? r.localLoginEnabled)
      setPlayerId(username.value || 'player-1')
      initDone.value = true
    })
    .catch(() => {
      initDone.value = true
    })
    .finally(() => {
      refresh()
      refreshIntervalId = setInterval(refresh, 1000)
      nowIntervalId = setInterval(() => {
        nowSeconds.value = Math.floor(Date.now() / 1000)
      }, 1000)
    })
})

onUnmounted(() => {
  if (refreshIntervalId) clearInterval(refreshIntervalId)
  if (nowIntervalId) clearInterval(nowIntervalId)
})
</script>

<style scoped>
.game-layout {
  display: flex;
  flex-direction: column;
  min-height: 0;
}

.game-section :deep(.section-header h2) {
  display: inline-flex;
  align-items: center;
}

.game-section :deep(.section-header h2::before) {
  content: '';
  display: inline-block;
  width: 1.5em;
  height: 1.5em;
  margin-right: 0.35em;
  background: url(/logo.svg) center/contain no-repeat;
  flex-shrink: 0;
}

.game-tick-progress {
  width: 6em;
  height: 0.75em;
  margin-right: 0.75em;
  background: var(--standout-bg-color, #f8f9fa);
  border: 1px solid var(--border-color, #d7d7d7);
  border-radius: 0.25em;
  overflow: hidden;
}

.game-tick-progress__fill {
  height: 100%;
  background: var(--karma-good, lightgreen);
  transition: width 1s linear;
}

.game-header-resources {
  margin-right: 1em;
  white-space: nowrap;
}

.game-footer {
  margin-top: 0.5em;
  font-size: 0.9em;
}

.game-coins-icon {
  vertical-align: middle;
  flex-shrink: 0;
}

.game-content {
  flex: 1;
  padding: 1em;
  display: flex;
  flex-direction: column;
  align-items: center;
}

.game-grid {
  display: grid;
  grid-template-columns: repeat(5, 1fr);
  gap: 0.5em;
  max-width: 24em;
}

.game-cell {
  aspect-ratio: 1;
  min-height: 3em;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  border-radius: 0.4em;
  border: 2px solid var(--border-color, #d7d7d7);
  background-color: var(--standout-bg-color, #f8f9fa);
  cursor: pointer;
  font-size: 0.85em;
}

.game-cell:hover {
  background-color: var(--hover-background-color, #e9e9e9);
}

.game-cell--has-building {
  position: relative;
}

.game-cell--colored {
  border-width: 2px;
  border-style: solid;
}

.game-cell--colored:hover {
  filter: brightness(1.1);
}

.game-cell__icon {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  opacity: 0.4;
}

.game-cell__type,
.game-cell__level {
  display: block;
  position: relative;
  z-index: 1;
}

.game-cell__upgrade {
  position: relative;
  z-index: 1;
  font-size: 0.9em;
  margin-top: 0.2em;
}

.game-cell__sell {
  position: relative;
  z-index: 1;
  font-size: 0.9em;
  margin-top: 0.2em;
}

.game-cell__empty {
  opacity: 0.6;
}

.game-error {
  color: var(--karma-bad-fg, #ce3636);
  margin-top: 0.5em;
  margin-bottom: 0;
}

.game-dialog {
  padding: 1em;
  max-width: 20em;
}

.game-requirements-list {
  margin: 0.75em 0;
  padding-left: 1.5em;
}

.game-export-slider {
  display: flex;
  align-items: center;
  gap: 0.75em;
  margin: 1em 0;
}

.game-export-slider label {
  flex-shrink: 0;
}

.game-export-slider__input {
  flex: 1;
  min-width: 0;
}

.game-order-price-input {
  width: 6em;
  margin-bottom: 0.5em;
}

.game-export-slider__value {
  flex-shrink: 0;
  min-width: 2.5em;
  font-variant-numeric: tabular-nums;
}

.game-export-preview {
  margin: 0.5em 0 1em;
  font-weight: 500;
}

@media (min-width: 768px) {
  .game-dialog--resources,
  .game-dialog--marketplace {
    max-width: 80vw;
  }
}

.game-dialog h3 {
  margin-top: 0;
}

.game-dialog__building-icon {
  display: block;
  margin: 0.5em 0;
}

.game-dialog__actions {
  display: flex;
  gap: 0.5em;
  margin-top: 1em;
}

.game-dialog__actions--inline {
  margin-bottom: 0.5em;
}

.game-dialog__hint {
  margin-top: 0;
  font-size: 0.9em;
  color: var(--text-color);
}

.game-resources-table {
  width: 100%;
  border-collapse: collapse;
  margin: 0.5em 0;
}

.game-resources-table th,
.game-resources-table td {
  padding: 0.35em 0.5em;
  text-align: left;
  border-bottom: 1px solid var(--border-color, #d7d7d7);
}

.game-resources-table th {
  font-weight: 600;
}

.game-resources-table__name {
  display: inline-flex;
  align-items: center;
  gap: 0.35em;
}

.game-leaderboard-table {
  width: 100%;
  border-collapse: collapse;
  margin: 0.5em 0;
}

.game-leaderboard-table th,
.game-leaderboard-table td {
  padding: 0.35em 0.5em;
  text-align: left;
  border-bottom: 1px solid var(--border-color, #d7d7d7);
}

.game-leaderboard-table th {
  font-weight: 600;
}

.game-leaderboard-empty {
  margin: 0.5em 0;
  color: var(--text-muted, #666);
}

.game-loans-list {
  list-style: none;
  padding: 0;
  margin: 0.5em 0;
}

.game-loans-item {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.5em;
  margin-bottom: 0.5em;
  padding: 0.35em 0;
  border-bottom: 1px solid var(--border-color, #d7d7d7);
}

.game-loans-pay {
  display: inline-flex;
  align-items: center;
  gap: 0.35em;
}

.game-loans-input {
  width: 6em;
}

.game-loans-empty {
  margin: 0.5em 0;
  color: var(--text-muted, #666);
}

.game-build-menu__building-btn {
  border-width: 2px;
  border-style: solid;
}

.game-build-menu__building-btn--requirements-not-met {
  background: transparent !important;
  border-color: var(--border-color, #d7d7d7);
  color: #000;
  opacity: 0.5;
}

.game-build-menu__building-btn--requirements-not-met:hover {
  opacity: 1;
}

.game-build-menu__building-btn:not(:disabled):hover {
  filter: brightness(1.1);
}

.game-build-menu {
  list-style: none;
  padding: 0;
  margin: 0.5em 0;
}

.game-build-menu li + li {
  margin-top: 0.25em;
}

.login-section {
  display: flex;
  flex-direction: column;
  gap: 1rem;
  align-items: center;
}

.oauth-providers {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
  width: 100%;
}

.oauth-button {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 0.75rem;
  padding: 0.75rem 1.5rem;
  font-size: 1em;
  cursor: pointer;
  border: none;
  border-radius: 4px;
  transition: opacity 0.2s, transform 0.1s;
  width: 100%;
}

.oauth-button:hover {
  opacity: 0.9;
  transform: translateY(-1px);
}

.no-providers {
  padding: 2rem;
  text-align: center;
  color: var(--text-muted, #666);
}
</style>
