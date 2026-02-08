<template>
  <div class="game-page game-messages">
    <Section title="Messages" :padding="true">
      <div
        ref="messagesContainerRef"
        class="game-messages__stream"
        role="log"
        aria-live="polite"
        aria-label="Activity and messages"
      >
        <div
          v-for="(msg, index) in messages"
          :key="msg.id ?? index"
          class="game-messages__item"
        >
          <span class="game-messages__time" aria-hidden="true">{{ formatTime(msg.at) }}</span>
          <span class="game-messages__text">{{ msg.text }}</span>
        </div>
        <p v-if="messages.length === 0" class="game-messages__empty">No messages yet.</p>
      </div>
    </Section>
  </div>
</template>

<script setup>
import { inject, ref, onMounted, watch, nextTick } from 'vue'
import Section from 'picocrank/vue/components/Section.vue'
import { getMessages } from '../api/client'

const ctx = inject('gamePageContext')
if (!ctx) {
  throw new Error('GameMessages requires gamePageContext from Game layout')
}

const messagesContainerRef = ref(null)
const messages = ref([])

function formatTime(at) {
  if (at == null) return ''
  const d = new Date(typeof at === 'number' ? at * 1000 : at)
  return d.toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit', second: '2-digit' })
}

function scrollToBottom() {
  nextTick(() => {
    const el = messagesContainerRef.value
    if (el) el.scrollTop = el.scrollHeight
  })
}

onMounted(() => {
  loadMessages()
  scrollToBottom()
})

watch(messages, () => scrollToBottom(), { deep: true })

function loadMessages() {
  getMessages()
    .then((r) => {
      const list = r.messages ?? r.message_list ?? []
      const arr = Array.isArray(list) ? list : []
      messages.value = arr.map((m) => ({
        id: m.id ?? m.Id,
        text: m.text ?? m.Text ?? '',
        at: m.at ?? m.At,
      }))
    })
    .catch(() => {
      messages.value = []
    })
}
</script>

<style scoped>
.game-page {
  padding: 1em;
  max-width: 40em;
  margin: 0 auto;
}

.game-messages__stream {
  overflow-y: auto;
  max-height: 60vh;
  min-height: 12em;
  padding: 0.5em 0;
  border: 1px solid var(--border-color, #d7d7d7);
  border-radius: 4px;
  background: var(--standout-bg-color, #f8f9fa);
}

.game-messages__item {
  display: flex;
  gap: 0.75em;
  padding: 0.25em 0.75em;
  font-size: 0.95em;
  border-bottom: 1px solid var(--border-color, #e9e9e9);
}

.game-messages__item:last-child {
  border-bottom: none;
}

.game-messages__time {
  flex-shrink: 0;
  color: var(--text-muted, #666);
  font-variant-numeric: tabular-nums;
}

.game-messages__text {
  flex: 1;
  word-break: break-word;
}

.game-messages__empty {
  padding: 0.75em 1em;
  margin: 0;
  color: var(--text-muted, #666);
}
</style>
