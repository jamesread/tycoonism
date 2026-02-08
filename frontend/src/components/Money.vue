<template>
  <span class="game-money" :data-value="numericValue">
    <Icon
      icon="game-icons:two-coins"
      width="1em"
      height="1em"
      class="game-coins-icon"
      aria-hidden="true"
    />
    {{ formatted }}
  </span>
</template>

<script setup>
import { computed } from 'vue'
import { Icon } from '@iconify/vue'

const props = defineProps({
  value: {
    type: [Number, String],
    default: 0,
  },
})

const numericValue = computed(() => {
  const n = Number(props.value)
  return Number.isFinite(n) ? Math.floor(n) : 0
})

function formatRoundedDown(n) {
  if (n >= 1e9) {
    const amount = Math.floor((n / 1e9) * 10) / 10
    return `${amount}b`
  }
  if (n >= 1e6) {
    const amount = Math.floor((n / 1e6) * 10) / 10
    return `${amount}m`
  }
  if (n >= 1e3) {
    const amount = Math.floor((n / 1e3) * 10) / 10
    return `${amount}k`
  }
  return String(n)
}

const formatted = computed(() => formatRoundedDown(numericValue.value))
</script>

<style scoped>
.game-money {
  display: inline-flex;
  align-items: center;
  gap: 0.25em;
}

.game-coins-icon {
  flex-shrink: 0;
  vertical-align: middle;
}
</style>
