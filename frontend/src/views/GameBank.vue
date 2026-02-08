<template>
  <div class="game-page game-bank">
    <h2>Bank</h2>

    <section class="game-bank-section">
      <h3>Leaderboard</h3>
      <table class="game-leaderboard-table row-hover">
        <thead>
          <tr>
            <th>Rank</th>
            <th>Player</th>
            <th>Coins</th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="(entry, index) in ctx.leaderboardEntries.value"
            :key="entry.player_id ?? entry.playerId ?? index"
          >
            <td>{{ index + 1 }}</td>
            <td>{{ entry.player_id ?? entry.playerId ?? '—' }}</td>
            <td>
              <Money :value="entry.coins" />
            </td>
          </tr>
        </tbody>
      </table>
      <p v-if="ctx.leaderboardEntries.value.length === 0" class="game-leaderboard-empty">No players yet.</p>
      <div role="toolbar" class="game-dialog__actions">
        <button
          v-if="ctx.canUseAngelInvestor.value"
          type="button"
          class="good"
          @click="ctx.doAngelInvestor()"
        >
          Angel Investor
        </button>
      </div>
    </section>

    <section class="game-bank-section">
      <h3>Loans</h3>
      <p class="game-dialog__hint">Interest is added to your balance each tick (not taken from your coins). Pay off loans to reduce debt.</p>
      <h4>Your loans</h4>
      <table v-if="ctx.loansList.value.length > 0" class="game-loans-table row-hover">
        <thead>
          <tr>
            <th>Original amount</th>
            <th>Balance</th>
            <th>Interest rate</th>
            <th>Interest per tick</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="loan in ctx.loansList.value" :key="loan.loan_id ?? loan.loanId" class="game-loans-item">
            <td>
              <Icon icon="game-icons:two-coins" width="1em" height="1em" class="game-coins-icon" aria-hidden="true" />
              {{ ctx.num(loan.original_amount ?? loan.originalAmount ?? loan.balance) }}
            </td>
            <td>
              <Icon icon="game-icons:two-coins" width="1em" height="1em" class="game-coins-icon" aria-hidden="true" />
              {{ ctx.num(loan.balance) }}
            </td>
            <td>{{ interestRatePercent(loan) }}</td>
            <td>
              <Icon icon="game-icons:two-coins" width="1em" height="1em" class="game-coins-icon" aria-hidden="true" />
              {{ interestPerTickCoins(loan) }} per tick
            </td>
            <td class="game-loans-pay">
              <input
                v-model.number="ctx.loanPayAmounts.value[loan.loan_id ?? loan.loanId]"
                type="number"
                min="0"
                :max="Math.min(ctx.num(loan.balance), ctx.num(ctx.state.value.coins))"
                placeholder="Amount"
                class="game-loans-input"
              />
              <button
                type="button"
                class="good"
                :disabled="!ctx.loanPayAmounts.value[loan.loan_id ?? loan.loanId] || ctx.loanPayAmounts.value[loan.loan_id ?? loan.loanId] <= 0 || ctx.loanPayAmounts.value[loan.loan_id ?? loan.loanId] > ctx.num(ctx.state.value.coins) || ctx.loanPayAmounts.value[loan.loan_id ?? loan.loanId] > ctx.num(loan.balance)"
                @click="ctx.payOffLoanClick(loan)"
              >
                Pay off
              </button>
            </td>
          </tr>
        </tbody>
      </table>
      <p v-else class="game-loans-empty">No active loans.</p>
      <h4>Take a loan</h4>
      <p class="game-dialog__hint">Amounts are based on your current coins, rounded to 1000.</p>
      <div role="toolbar" class="game-dialog__actions game-dialog__actions--inline">
        <button
          v-for="amt in ctx.offeredAmounts.value"
          :key="amt"
          type="button"
          class="good"
          @click="ctx.takeLoanClick(amt)"
        >
          Borrow <Icon icon="game-icons:two-coins" width="1em" height="1em" class="game-coins-icon" aria-hidden="true" /> {{ ctx.num(amt) }}
        </button>
      </div>
      <p v-if="ctx.offeredAmounts.value.length === 0" class="game-dialog__hint">No loan offers available (need at least 1000 coins proportion, or max loans reached).</p>
    </section>
  </div>
</template>

<script setup>
import { inject, onMounted } from 'vue'
import { Icon } from '@iconify/vue'
import Money from '../components/Money.vue'

const ctx = inject('gamePageContext')
if (!ctx) {
  throw new Error('GameBank requires gamePageContext from Game layout')
}

function interestRatePercent(loan) {
  const rate = Number(loan.interest_rate ?? loan.interestRate ?? 0)
  if (!Number.isFinite(rate)) return '—'
  const pct = rate * 100
  return pct === Math.round(pct) ? `${pct}%` : `${pct.toFixed(1)}%`
}

function interestPerTickCoins(loan) {
  const rate = Number(loan.interest_rate ?? loan.interestRate ?? 0)
  if (!Number.isFinite(rate) || rate <= 0) return 0
  const balance = ctx.num(loan.balance)
  return Math.floor(balance * rate)
}

onMounted(() => {
  ctx.fetchLeaderboard().catch(() => {})
  ctx.fetchLoansForDialog().catch(() => {})
})
</script>

<style scoped>
.game-page {
  padding: 1em;
  max-width: 40em;
  margin: 0 auto;
}

.game-bank-section {
  margin-bottom: 2em;
}

.game-bank-section h3 {
  margin-top: 0;
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

.game-dialog__hint {
  margin-top: 0;
  font-size: 0.9em;
  color: var(--text-color);
}

.game-dialog__actions {
  display: flex;
  gap: 0.5em;
  margin-top: 1em;
}

.game-dialog__actions--inline {
  margin-bottom: 0.5em;
}

.game-loans-table {
  width: 100%;
  border-collapse: collapse;
  margin: 0.5em 0;
}

.game-loans-table th,
.game-loans-table td {
  padding: 0.35em 0.5em;
  text-align: left;
  border-bottom: 1px solid var(--border-color, #d7d7d7);
}

.game-loans-table th {
  font-weight: 600;
}

.game-loans-item td {
  vertical-align: middle;
}

.game-loans-pay {
  display: flex;
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

.game-coins-icon {
  vertical-align: middle;
  flex-shrink: 0;
}
</style>
