<template>
  <div class="game-page game-resources">
    <Section title="Resources" :padding="true">
      <Table
        :headers="resourceHeaders"
        :data="resourceRows"
        :show-pagination="false"
      >
        <template #cell-name="{ row }">
          <router-link :to="{ name: 'resource', params: { id: row.id } }" class="game-resources-table__name">
            <Icon
              v-if="row.icon"
              :icon="row.icon"
              width="1.2em"
              height="1.2em"
              aria-hidden="true"
            />
            {{ row.name }}
          </router-link>
        </template>
        <template #cell-marketPrice="{ row }">
          <Money :value="row.marketPrice" />
        </template>
        <template #cell-pinAction="{ row }">
          <button
            type="button"
            :aria-pressed="ctx.isPinned(row.id)"
            @click="ctx.togglePinned(row.id)"
          >
            {{ ctx.isPinned(row.id) ? 'Unpin' : 'Pin' }}
          </button>
        </template>
      </Table>
    </Section>
  </div>
</template>

<script setup>
import { inject, computed } from 'vue'
import Section from 'picocrank/vue/components/Section.vue'
import Table from 'picocrank/vue/components/Table.vue'
import Money from '../components/Money.vue'
import { Icon } from '@iconify/vue'

const ctx = inject('gamePageContext')
if (!ctx) {
  throw new Error('GameResources requires gamePageContext from Game layout')
}

const resourceHeaders = [
  { key: 'name', label: 'Resource', sortable: true },
  { key: 'quantity', label: 'Quantity', sortable: true },
  { key: 'rate', label: 'Rate (/min)', sortable: true },
  { key: 'marketPrice', label: 'Market price', sortable: true },
  { key: 'pinAction', label: '', sortable: false },
]

const resourceRows = computed(() => {
  const defs = ctx.resourceDefinitions.value ?? []
  return defs.map((res) => {
    const id = res.id ?? res.Id
    return {
      id,
      name: res.name ?? res.Name,
      icon: res.icon,
      quantity: ctx.resourceAmount(id),
      rate: ctx.resourceRatePerMin(id),
      marketPrice: ctx.num(res.sell_price ?? res.sellPrice ?? 0),
      pinAction: null,
    }
  })
})
</script>

<style scoped>
.game-page {
  padding: 1em;
  margin: 0 auto;
}

.game-resources-table__name {
  display: inline-flex;
  align-items: center;
  gap: 0.35em;
}
</style>
