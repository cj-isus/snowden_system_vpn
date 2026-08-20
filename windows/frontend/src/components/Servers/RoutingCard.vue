<script lang="ts" setup>
import { ref, onMounted, inject } from "vue"
import { GetRouteRules, ToggleRouteRule } from "../../../wailsjs/go/main/App"

const showToast = inject<(text: string, type?: string) => void>("showToast")

interface Rule {
  id: string
  icon: string
  title: string
  sub: string
  sub2: string
  route: string
  on: boolean
  ruleIndex: number  // backend rule index for ToggleRouteRule
}

const rules = ref<Rule[]>([])
const loading = ref(true)
const toggling = ref<string | null>(null)

async function refresh() {
  try {
    const list = await GetRouteRules()
    const actualRules = (list || []).filter(rule => /^rule-\d+$/.test(String(rule.id || "")))
    rules.value = actualRules.map((r, i) => ({
      id: r.id,
      icon: r.icon,
      title: r.title,
      sub: r.sub,
      sub2: "",
      route: r.route,
      on: r.on,
      // Backend IDs preserve the original route.rules index even when
      // service rules (sniff/hijack-dns) are omitted from the UI.
      ruleIndex: backendRuleIndex(String(r.id || ""), i),
    }))
    
  } catch {} finally {
    loading.value = false
  }
}

onMounted(() => { refresh() })

function backendRuleIndex(id: string, fallback: number): number {
  const match = /^rule-(\d+)$/.exec(id)
  return match ? Number(match[1]) : fallback
}

async function toggleRule(rule: Rule) {
  if (toggling.value) return
  toggling.value = rule.id
  const oldState = rule.on
  rule.on = !rule.on
  rule.route = rule.on ? "Через VPN" : "Напрямую"
  try {
    await ToggleRouteRule(rule.ruleIndex, rule.on)
    showToast?.(`Правило "${rule.title}" → ${rule.route}`, "success")
  } catch (e: any) {
    // Revert on error
    rule.on = oldState
    rule.route = oldState ? "Через VPN" : "Напрямую"
    showToast?.(`Ошибка: ${e?.message || e}`, "error")
  } finally {
    toggling.value = null
  }
}
</script>

<template>
  <div class="card routing-card">
    <div class="card-title-row">
      <span class="card-title">МАРШРУТИЗАЦИЯ</span>
      <span class="card-subtitle">// правила решают</span>
    </div>

    <div class="rules-list">
      <div
        v-for="rule in rules"
        :key="rule.id"
        class="rule"
        :class="{ active: rule.on }"
      >
        <span class="rule-icon">{{ rule.icon }}</span>
        <div class="rule-info">
          <div class="rule-name">{{ rule.title }}</div>
          <div class="rule-sub">{{ rule.sub }}</div>
          <div class="rule-sub2" v-if="rule.sub2">{{ rule.sub2 }}</div>
          <div class="rule-edit" v-if="rule.sub2">[Изменить список]</div>
        </div>
        <div class="rule-route">
          <span class="route-label">{{ rule.route }}</span>
          <div class="toggle" :class="{ on: rule.on, disabled: toggling === rule.id }" @click="toggleRule(rule)">
            <div class="toggle-dot" />
          </div>
        </div>
      </div>
    </div>

    <div class="other-rule">
      <span class="other-icon">🌐</span>
      <span class="other-name">Всё остальное</span>
      <span class="other-route">Через VPN (final)</span>
    </div>
  </div>
</template>

<style scoped>
.card {
  background: var(--bg-card);
  border: 1px solid var(--border);
  border-radius: 18px;
  padding: 16px;
  transition: all var(--speed) var(--ease);
  animation: fadeIn var(--speed) var(--ease) forwards;
  display: flex;
  flex-direction: column;
}
.card:hover {
  border-color: var(--border-hover);
}
.card-title-row {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 12px;
}
.card-title {
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 1px;
  color: var(--text-dim);
}
.card-subtitle {
  font-size: 11px;
  color: var(--text-muted);
  font-style: italic;
}
.rules-list {
  display: flex;
  flex-direction: column;
  gap: 4px;
  flex: 1;
}
.rule {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  padding: 8px;
  border-radius: 10px;
  border: 1px solid transparent;
  transition: all var(--speed) var(--ease);
}
.rule:hover {
  background: var(--bg-card-hover);
  border-color: var(--border);
}
.rule-icon {
  font-size: 20px;
  flex-shrink: 0;
  margin-top: 2px;
}
.rule-info {
  flex: 1;
  min-width: 0;
}
.rule-name {
  font-size: 13px;
  font-weight: 500;
  color: var(--text-white);
}
.rule-sub, .rule-sub2 {
  font-size: 10px;
  color: var(--text-dim);
  font-family: "JetBrains Mono", monospace;
  margin-top: 1px;
}
.rule-edit {
  font-size: 10px;
  color: var(--text-muted);
  margin-top: 2px;
  cursor: pointer;
}
.rule-edit:hover {
  color: var(--green);
}
.rule-route {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 4px;
  flex-shrink: 0;
}
.route-label {
  font-size: 10px;
  color: var(--text-dim);
}
.toggle {
  width: 36px;
  height: 20px;
  border-radius: 10px;
  background: var(--bg-card-hover);
  border: 1px solid var(--border);
  display: flex;
  align-items: center;
  padding: 2px;
  cursor: pointer;
  transition: all var(--speed) var(--ease);
}
.toggle.on {
  background: rgba(60, 255, 90, 0.2);
  border-color: var(--green);
}
.toggle-dot {
  width: 14px;
  height: 14px;
  border-radius: 50%;
  background: var(--text-gray);
  transition: all var(--speed) var(--ease-bounce);
}
.toggle.on .toggle-dot {
  background: var(--green-success);
  transform: translateX(16px);
  box-shadow: 0 0 6px var(--green-success);
}
.add-btn {
  width: 100%;
  padding: 8px;
  margin-top: 8px;
  background: transparent;
  border: 1px dashed var(--border);
  border-radius: 8px;
  color: var(--text-gray);
  font-size: 11px;
  font-weight: 600;
  cursor: pointer;
  transition: all var(--speed) var(--ease);
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
}
.add-btn:hover {
  border-color: var(--green);
  color: var(--green);
}
.other-rule {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px;
  margin-top: 6px;
  border-top: 1px solid rgba(255, 255, 255, 0.03);
}
.other-icon {
  font-size: 16px;
}
.other-name {
  flex: 1;
  font-size: 12px;
  color: var(--text-white);
}
.other-route {
  font-size: 10px;
  color: var(--text-dim);
}

/* Responsive */
@media (max-width: 1200px) {
  .rule {
    padding: 6px;
  }
  .rule-name {
    font-size: 12px;
  }
  .rule-sub, .rule-sub2, .rule-edit {
    font-size: 9px;
  }
}

@media (max-width: 900px) {
  .rule-route {
    flex-direction: row;
    gap: 8px;
  }
}
</style>
