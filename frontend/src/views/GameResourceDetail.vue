<template>
  <div class="game-page game-resource-detail">
    <Section v-if="resource" :padding="true" :title="resource.name ?? resource.Name">
      <template #toolbar>
        <router-link :to="{ name: 'resources' }">← Resources</router-link>
        <button type="button" class="good" @click="ctx.openBuyOrderDialogForResource(resourceId)">
          Buy order
        </button>
        <button type="button" class="good" @click="ctx.openSellOrderDialogForResource(resourceId)">
          Sell order
        </button>
      </template>
      <p class="game-resource-detail__meta">
        <span class="game-resource-detail__name">
          <Icon
            v-if="resource.icon"
            :icon="resource.icon"
            width="1.5em"
            height="1.5em"
            aria-hidden="true"
          />
          {{ resource.name ?? resource.Name }}
        </span>
        · You have {{ ctx.resourceAmount(resourceId) }}
        · Rate {{ ctx.resourceRatePerMin(resourceId) }}/min
      </p>
      <p class="game-resource-detail__price">
        Market price: <Money :value="marketPrice" />
      </p>
      <h4>Price history (last 15 ticks)</h4>
      <table class="game-resource-detail__history row-hover">
        <thead>
          <tr>
            <th>Tick</th>
            <th>Price</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="(price, i) in priceHistory" :key="i">
            <td>{{ i + 1 }}</td>
            <td><Money :value="price" /></td>
          </tr>
        </tbody>
      </table>
      <p v-if="priceHistory.length === 0" class="game-resource-detail__empty">No price history yet.</p>
    </Section>
    <Section v-else title="Resource" :padding="true">
      <p>Resource not found.</p>
      <router-link :to="{ name: 'resources' }">← Back to Resources</router-link>
    </Section>
  </div>
</template>

<script setup>
import { inject, computed, ref, onMounted, watch } from 'vue'
import { useRoute } from 'vue-router'
import Section from 'picocrank/vue/components/Section.vue'
import Money from '../components/Money.vue'
import { Icon } from '@iconify/vue'
import { getResourcePriceHistory } from '../api/client'

const route = useRoute()
const ctx = inject('gamePageContext')
if (!ctx) {
  throw new Error('GameResourceDetail requires gamePageContext from Game layout')
}

const resourceId = computed(() => route.params.id ?? '')
const resource = computed(() => {
  const id = resourceId.value
  if (!id) return null
  return ctx.resourceDefinitions.value?.find((r) => (r.id ?? r.Id) === id) ?? null
})
const marketPrice = computed(() => {
  if (!resource.value) return 0
  return ctx.num(resource.value.sell_price ?? resource.value.sellPrice ?? 0)
})

const priceHistory = ref([])

function loadPriceHistory() {
  const id = resourceId.value
  if (!id) {
    priceHistory.value = []
    return
  }
  getResourcePriceHistory(id, 15)
    .then((r) => {
      const hist = r.price_history ?? r.priceHistory ?? []
      priceHistory.value = Array.isArray(hist) ? [...hist] : []
    })
    .catch(() => {
      priceHistory.value = []
    })
}

onMounted(loadPriceHistory)
watch(resourceId, loadPriceHistory)
</script>

<style scoped>
.game-page {
  padding: 1em;
  max-width: 40em;
  margin: 0 auto;
}

.game-resource-detail__meta {
  margin-top: 0;
}

.game-resource-detail__name {
  display: inline-flex;
  align-items: center;
  gap: 0.35em;
}

.game-resource-detail__price {
  margin: 0.5em 0;
}

.game-resource-detail__history {
  width: 100%;
  border-collapse: collapse;
  margin: 0.5em 0;
}

.game-resource-detail__history th,
.game-resource-detail__history td {
  padding: 0.35em 0.5em;
  text-align: left;
  border-bottom: 1px solid var(--border-color, #d7d7d7);
}

.game-resource-detail__history th {
  font-weight: 600;
}

.game-resource-detail__empty {
  color: var(--text-muted, #666);
  margin: 0.5em 0;
}
</style>
