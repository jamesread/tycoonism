<template>
  <div class="game-buildings-view">
    <div class="game-grid" role="grid" :aria-label="`${ctx.GRID_SIZE} by ${ctx.GRID_SIZE} building grid`">
      <button
        v-for="cell in ctx.cells.value"
        :key="`${cell.x},${cell.y}`"
        type="button"
        class="game-cell"
        :class="{
          'game-cell--has-building': !!cell.building,
          'game-cell--colored': !!cell.building && ctx.cellButtonStyle(cell),
        }"
        :style="cell.building ? ctx.cellButtonStyle(cell) : undefined"
        :aria-label="ctx.cellLabel(cell)"
        @click="ctx.onCellClick(cell)"
      >
        <template v-if="cell.building">
          <Icon
            v-if="ctx.buildingDef(cell.building.typeId)?.icon"
            :icon="ctx.buildingDef(cell.building.typeId).icon"
            width="3em"
            height="3em"
            class="game-cell__icon"
            aria-hidden="true"
          />
          <span class="game-cell__type">{{ ctx.buildingLabel(cell.building.typeId) }}</span>
          <span class="game-cell__level">Lv{{ ctx.buildingLevel(cell.building) }}</span>
          <span
            v-if="ctx.upgradeFinishesAt(cell.building)"
            class="game-cell__upgrade"
          >
            ↑ {{ ctx.upgradeCountdown(cell.building) }}
          </span>
          <span
            v-else-if="ctx.sellingFinishesAt(cell.building)"
            class="game-cell__sell"
          >
            Selling {{ ctx.sellingCountdown(cell.building) }}
          </span>
        </template>
        <span v-else class="game-cell__empty">·</span>
      </button>
    </div>

    <p v-if="ctx.error.value" class="game-error" role="alert">{{ ctx.error.value }}</p>

    <dialog v-if="ctx.buildMenuCell.value" ref="buildMenuDialogRef" class="game-dialog" aria-label="Build menu">
      <h3>Build at ({{ ctx.buildMenuCell.value?.x }}, {{ ctx.buildMenuCell.value?.y }})</h3>
      <p class="game-dialog__hint">
        Cost shown per building
      </p>
      <ul class="game-build-menu">
        <li
          v-for="b in ctx.buildingDefinitions.value"
          :key="b.id ?? b.Id"
        >
          <button
            type="button"
            class="good game-build-menu__building-btn"
            :class="{ 'game-build-menu__building-btn--requirements-not-met': !ctx.requirementsMetForBuilding(b) }"
            :disabled="ctx.num(ctx.state.value.coins) < ctx.placementCostForBuilding(b)"
            :style="ctx.requirementsMetForBuilding(b) ? ctx.buildingButtonStyle(b) : undefined"
            @click="ctx.requirementsMetForBuilding(b) ? ctx.placeFromBuildMenu(b.id ?? b.Id) : ctx.openRequirementsDialog(b.id ?? b.Id)"
          >
            {{ b.name ?? b.Name }} (<Icon icon="game-icons:two-coins" width="1em" height="1em" class="game-coins-icon" aria-hidden="true" /> {{ ctx.placementCostForBuilding(b) }})
          </button>
        </li>
      </ul>
      <button type="button" @click="ctx.clearBuildMenuCell()">Cancel</button>
    </dialog>

    <dialog v-if="ctx.selectedBuilding.value?.building" ref="dialogRef" class="game-dialog">
      <h3>{{ ctx.buildingLabel(ctx.selectedBuilding.value.building.typeId) }} · Lv{{ ctx.buildingLevel(ctx.selectedBuilding.value.building) }}</h3>
      <Icon
        v-if="ctx.buildingDef(ctx.selectedBuilding.value.building.typeId ?? ctx.selectedBuilding.value.building.type_id)?.icon"
        :icon="ctx.buildingDef(ctx.selectedBuilding.value.building.typeId ?? ctx.selectedBuilding.value.building.type_id).icon"
        width="4em"
        height="4em"
        class="game-dialog__building-icon"
        aria-hidden="true"
      />
      <p>{{ ctx.dialogRate.value }}</p>
      <p v-if="ctx.dialogCountdown.value">{{ ctx.dialogCountdown.value }}</p>
      <div role="toolbar" class="game-dialog__actions">
        <button
          v-if="ctx.canUpgrade(ctx.selectedBuilding.value)"
          type="button"
          class="good"
          :disabled="ctx.num(ctx.state.value.coins) < ctx.nextUpgradeCost(ctx.selectedBuilding.value.building)"
          @click="ctx.doUpgradeFromDialog(ctx.selectedBuilding.value)"
        >
          Upgrade (<Icon icon="game-icons:two-coins" width="1em" height="1em" class="game-coins-icon" aria-hidden="true" /> {{ ctx.nextUpgradeCost(ctx.selectedBuilding.value.building) }})
        </button>
        <button
          v-if="ctx.selectedBuilding.value?.building && !ctx.upgradeFinishesAt(ctx.selectedBuilding.value.building) && !ctx.sellingFinishesAt(ctx.selectedBuilding.value.building)"
          type="button"
          class="danger"
          @click="ctx.sellConfirmOpen.value = true"
        >
          Sell
        </button>
        <button type="button" @click="ctx.clearSelectedBuilding()">Close</button>
      </div>
    </dialog>

    <dialog v-if="ctx.sellConfirmOpen.value" ref="sellConfirmDialogRef" class="game-dialog" aria-label="Confirm sell">
      <h3>Sell building?</h3>
      <p>You will receive a coin refund based on this building's level. Selling takes time for upgraded buildings.</p>
      <div role="toolbar" class="game-dialog__actions">
        <button type="button" @click="ctx.confirmSell()">Sell</button>
        <button type="button" @click="ctx.cancelSellConfirm()">Cancel</button>
      </div>
    </dialog>

    <dialog v-if="ctx.requirementsDialogOpen.value" ref="requirementsDialogRef" class="game-dialog" aria-label="Building requirements">
      <h3>Building requirements not met</h3>
      <p>You need to place the following buildings before placing {{ ctx.requirementsDialogBuildingName.value }}:</p>
      <ul v-if="ctx.requirementsDialogRequirements.value.length" class="game-requirements-list">
        <li v-for="(req, i) in ctx.requirementsDialogRequirements.value" :key="i">
          {{ req.count }}× {{ req.name }}
        </li>
      </ul>
      <button type="button" @click="ctx.closeRequirementsDialog()">OK</button>
    </dialog>
  </div>
</template>

<script setup>
import { inject, watch, nextTick, ref } from 'vue'
import { Icon } from '@iconify/vue'

const ctx = inject('gameBuildingsContext')
if (!ctx) {
  throw new Error('Buildings view requires gameBuildingsContext from Game layout')
}

const buildMenuDialogRef = ref(null)
const dialogRef = ref(null)
const sellConfirmDialogRef = ref(null)
const requirementsDialogRef = ref(null)

watch(() => ctx.buildMenuCell.value, (cell) => {
  if (cell) {
    nextTick(() => buildMenuDialogRef.value?.showModal())
  }
})

watch(() => ctx.selectedBuilding.value, (sel) => {
  if (sel?.building) {
    nextTick(() => dialogRef.value?.showModal())
  }
})

watch(() => ctx.sellConfirmOpen.value, (open) => {
  if (open) {
    nextTick(() => sellConfirmDialogRef.value?.showModal())
  }
})

watch(() => ctx.requirementsDialogOpen.value, (open) => {
  if (open) {
    nextTick(() => requirementsDialogRef.value?.showModal())
  }
})
</script>

<style scoped>
.game-buildings-view {
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

.game-dialog__hint {
  margin-top: 0;
  font-size: 0.9em;
  color: var(--text-color);
}

.game-requirements-list {
  margin: 0.75em 0;
  padding-left: 1.5em;
}

.game-build-menu {
  list-style: none;
  padding: 0;
  margin: 0.5em 0;
}

.game-build-menu li + li {
  margin-top: 0.25em;
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

.game-coins-icon {
  vertical-align: middle;
  flex-shrink: 0;
}
</style>
