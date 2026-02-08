<template>
  <div class="game-page game-market">
    <Section title="Marketplace" :padding="true">
      <p class="game-dialog__hint">Fulfill another player's order to buy or sell at their price. Cancel your own orders to get resources or coins back.</p>
    </Section>

    <Section
      title="Sell orders (others are selling — you buy)"
      :padding="true"
    >
      <template #toolbar>
        <button type="button" class="good" @click="ctx.openSellOrderDialogFromMarketplace()">
          Sell order
        </button>
      </template>
      <table class="game-marketplace-table row-hover">
        <thead>
          <tr>
            <th>Resource</th>
            <th>Qty</th>
            <th>Unit Price</th>
            <th>Market</th>
            <th>%</th>
            <th>Cost</th>
            <th>Seller</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="order in ctx.marketplaceSellOrders.value"
            :key="order.order_id ?? order.orderId"
          >
            <td class="game-marketplace-table__resource">
              <router-link
                :to="{ name: 'resource', params: { id: order.resource_id ?? order.resourceId } }"
                class="game-marketplace-table__resource-link"
              >
                <Icon
                  v-if="getResourceDef(order.resource_id ?? order.resourceId)?.icon"
                  :icon="getResourceDef(order.resource_id ?? order.resourceId).icon"
                  width="1.2em"
                  height="1.2em"
                  class="game-marketplace-table__resource-icon"
                  aria-hidden="true"
                />
                {{ ctx.resourceNameById(order.resource_id ?? order.resourceId) }}
              </router-link>
            </td>
          <td>{{ ctx.num(order.quantity) }}</td>
          <td><Money :value="ctx.num(order.price_per_unit ?? order.pricePerUnit)" /></td>
          <td><Money :value="getMarketPrice(order.resource_id ?? order.resourceId)" /></td>
          <td :class="orderPricePctClass(order)" :title="orderPricePctTitle(order)">{{ orderPricePct(order) }}</td>
          <td><Money :value="ctx.num(order.quantity) * ctx.num(order.price_per_unit ?? order.pricePerUnit)" /></td>
          <td class="game-marketplace-table__player">
            <template v-if="ctx.isMyMarketOrder(order)">You</template>
            <template v-else>
              <Icon v-if="orderPlayerDisplay(order.player_id ?? order.playerId).isNpc" icon="game-icons:robot-antennas" width="1em" height="1em" class="game-npc-icon" title="NPC" aria-hidden="true" />
              {{ orderPlayerDisplay(order.player_id ?? order.playerId).displayName }}
            </template>
          </td>
          <td>
            <template v-if="ctx.isMyMarketOrder(order)">
              <button type="button" class="danger" @click="ctx.cancelOrder(order)">Cancel</button>
            </template>
            <template v-else>
              <button type="button" class="good" :disabled="ctx.num(ctx.state.value.coins) < ctx.num(order.quantity) * ctx.num(order.price_per_unit ?? order.pricePerUnit)" @click="ctx.fulfillSellOrderClick(order)">Buy</button>
            </template>
          </td>
          </tr>
        </tbody>
      </table>
      <p v-if="ctx.marketplaceSellOrders.value.length === 0" class="game-marketplace-empty">No sell orders.</p>
    </Section>

    <Section
      title="Buy orders (others are buying — you sell)"
      :padding="true"
    >
      <template #toolbar>
        <button type="button" class="good" @click="ctx.openBuyOrderDialogFromMarketplace()">
          Buy order
        </button>
      </template>
      <table class="game-marketplace-table row-hover">
        <thead>
          <tr>
            <th>Resource</th>
            <th>Qty</th>
            <th>Unit Price</th>
            <th>Market</th>
            <th>%</th>
            <th>Profit</th>
            <th>Buyer</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="order in ctx.marketplaceBuyOrders.value"
            :key="order.order_id ?? order.orderId"
          >
            <td class="game-marketplace-table__resource">
              <router-link
                :to="{ name: 'resource', params: { id: order.resource_id ?? order.resourceId } }"
                class="game-marketplace-table__resource-link"
              >
                <Icon
                  v-if="getResourceDef(order.resource_id ?? order.resourceId)?.icon"
                  :icon="getResourceDef(order.resource_id ?? order.resourceId).icon"
                  width="1.2em"
                  height="1.2em"
                  class="game-marketplace-table__resource-icon"
                  aria-hidden="true"
                />
                {{ ctx.resourceNameById(order.resource_id ?? order.resourceId) }}
              </router-link>
            </td>
          <td>{{ ctx.num(order.quantity) }}</td>
          <td><Money :value="ctx.num(order.price_per_unit ?? order.pricePerUnit)" /></td>
          <td><Money :value="getMarketPrice(order.resource_id ?? order.resourceId)" /></td>
          <td :class="orderPricePctClass(order, true)" :title="orderPricePctTitle(order)">{{ orderPricePct(order) }}</td>
          <td><Money :value="ctx.num(order.quantity) * ctx.num(order.price_per_unit ?? order.pricePerUnit)" /></td>
          <td class="game-marketplace-table__player">
            <template v-if="ctx.isMyMarketOrder(order)">You</template>
            <template v-else>
              <Icon v-if="orderPlayerDisplay(order.player_id ?? order.playerId).isNpc" icon="game-icons:robot-antennas" width="1em" height="1em" class="game-npc-icon" title="NPC" aria-hidden="true" />
              {{ orderPlayerDisplay(order.player_id ?? order.playerId).displayName }}
            </template>
          </td>
          <td>
            <template v-if="ctx.isMyMarketOrder(order)">
              <button type="button" class="danger" @click="ctx.cancelOrder(order)">Cancel</button>
            </template>
            <template v-else>
              <button type="button" class="good" :disabled="ctx.resourceAmount(order.resource_id ?? order.resourceId) < ctx.num(order.quantity)" @click="ctx.fulfillBuyOrderClick(order)">Sell</button>
            </template>
          </td>
          </tr>
        </tbody>
      </table>
      <p v-if="ctx.marketplaceBuyOrders.value.length === 0" class="game-marketplace-empty">No buy orders.</p>
    </Section>
  </div>
</template>

<script setup>
import { inject, onMounted } from 'vue'
import Section from 'picocrank/vue/components/Section.vue'
import Money from '../components/Money.vue'
import { Icon } from '@iconify/vue'

const ctx = inject('gamePageContext')
if (!ctx) {
  throw new Error('GameMarket requires gamePageContext from Game layout')
}

const NPC_PREFIX = 'npc:'

function orderPlayerDisplay(playerId) {
  if (!playerId) return { displayName: '—', isNpc: false }
  if (playerId.startsWith(NPC_PREFIX)) {
    return { displayName: playerId.slice(NPC_PREFIX.length), isNpc: true }
  }
  return { displayName: playerId, isNpc: false }
}

function getResourceDef(resourceId) {
  return ctx.resourceDefinitions.value?.find((r) => (r.id ?? r.Id) === resourceId)
}

function getMarketPrice(resourceId) {
  const def = getResourceDef(resourceId)
  const p = def?.sell_price ?? def?.sellPrice ?? 0
  return ctx.num(p)
}

function orderPricePct(order) {
  const resourceId = order.resource_id ?? order.resourceId
  const market = getMarketPrice(resourceId)
  if (market <= 0) return '—'
  const orderPrice = ctx.num(order.price_per_unit ?? order.pricePerUnit)
  const pct = ((orderPrice - market) / market) * 100
  if (pct > 0) return `+${pct.toFixed(0)}%`
  if (pct < 0) return `${pct.toFixed(0)}%`
  return '0%'
}

function orderPricePctTitle(order) {
  const resourceId = order.resource_id ?? order.resourceId
  const market = getMarketPrice(resourceId)
  if (market <= 0) return 'No market price'
  const orderPrice = ctx.num(order.price_per_unit ?? order.pricePerUnit)
  const pct = ((orderPrice - market) / market) * 100
  return `Order price vs market: ${pct >= 0 ? '+' : ''}${pct.toFixed(1)}%`
}

function orderPricePctClass(order, isBuyOrder = false) {
  const resourceId = order.resource_id ?? order.resourceId
  const market = getMarketPrice(resourceId)
  if (market <= 0) return ''
  const orderPrice = ctx.num(order.price_per_unit ?? order.pricePerUnit)
  const pct = (orderPrice - market) / market
  if (pct > 0) return isBuyOrder ? 'game-marketplace-pct--below' : 'game-marketplace-pct--above'
  if (pct < 0) return isBuyOrder ? 'game-marketplace-pct--above' : 'game-marketplace-pct--below'
  return ''
}

onMounted(() => {
  ctx.fetchMarketplace().catch(() => {})
})
</script>

<style scoped>
.game-page {
  padding: 1em;
  max-width: 90%;
  margin: 0 auto;
}

.game-dialog__hint {
  margin-top: 0;
  font-size: 0.9em;
  color: var(--text-color);
}

.game-dialog__actions--inline {
  margin-bottom: 1em;
}

.game-marketplace-empty {
  margin: 0.5em 0;
  color: var(--text-muted, #666);
}

.game-marketplace-table__resource-link {
  display: inline-flex;
  align-items: center;
  gap: 0.35em;
}

.game-marketplace-table__resource-icon {
  flex-shrink: 0;
}

.game-coins-icon {
  vertical-align: middle;
  flex-shrink: 0;
}

.game-npc-icon {
  flex-shrink: 0;
  opacity: 0.85;
}

.game-marketplace-pct--above {
  color: var(--karma-bad-fg, #b94);
  font-weight: bold;
}

.game-marketplace-pct--below {
  color: #4a4;
  font-weight: bold;
}

@media (min-width: 768px) {
  .game-market {
    max-width: 80vw;
  }
}
</style>
