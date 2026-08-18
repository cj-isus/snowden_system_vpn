<script lang="ts" setup>
import { ref, onMounted, onUnmounted, computed } from "vue"
import { GetDomainStats } from "../../../wailsjs/go/main/App"

interface DomainScore {
  domain: string
  bestOutbound: string
  score: number
  requests: number
  avgLatencyMs: number
  successRate: number
}

const domains = ref<DomainScore[]>([])
const loading = ref(true)
let pollTimer: number | undefined

async function refresh() {
  try {
    const list = await GetDomainStats()
    if (list && list.length > 0) {
      domains.value = list as DomainScore[]
    }
  } catch {} finally {
    loading.value = false
  }
}

onMounted(() => {
  refresh()
  pollTimer = window.setInterval(refresh, 5000)
})
onUnmounted(() => {
  if (pollTimer) window.clearInterval(pollTimer)
})

function scoreColor(score: number): string {
  if (score >= 80) return "var(--green)"
  if (score >= 50) return "var(--yellow-warn)"
  return "var(--red-danger)"
}

function latencyText(ms: number): string {
  if (ms <= 0) return "—"
  return ms + "ms"
}

function outboundLabel(ob: string): string {
  if (ob === "auto") return "авто"
  if (ob === "vless-tls" || ob === "vless") return "VLESS"
  if (ob === "hysteria2") return "Hy2"
  if (ob === "direct") return "напрямую"
  return ob
}
</script>

<template>
  <div class="card domain-card">
    <div class="card-title-row">
      <div>
        <span class="card-title">ИНТЕЛЛЕКТ СИСТЕМЫ</span>
        <span class="card-subtitle">// что где работает лучше</span>
      </div>
    </div>

    <div class="domains-list">
      <div v-if="loading" class="loading">Анализирую…</div>
      <div
        v-for="d in domains"
        :key="d.domain"
        class="domain-row"
      >
        <div class="domain-name">{{ d.domain }}</div>
        <div class="domain-meta">
          <span class="meta-pill" :style="{ color: scoreColor(d.score) }">{{ outboundLabel(d.bestOutbound) }}</span>
          <span class="meta-text">{{ latencyText(d.avgLatencyMs) }}</span>
          <span class="meta-text">×{{ d.requests }}</span>
          <div class="score-bar">
            <div class="score-fill" :style="{ width: d.score + '%', background: scoreColor(d.score) }" />
          </div>
        </div>
      </div>
      <div v-if="!loading && domains.length === 0" class="empty">
        Запустите VPN — система запомнит, какой канал лучше для каждого сайта
      </div>
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
  min-height: 180px;
}
.card:hover {
  border-color: var(--border-hover);
}
.card-title-row {
  display: flex;
  align-items: center;
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
  margin-left: 8px;
}
.domains-list {
  display: flex;
  flex-direction: column;
  gap: 6px;
  flex: 1;
  overflow-y: auto;
  max-height: 200px;
}
.loading, .empty {
  font-size: 11px;
  color: var(--text-dim);
  text-align: center;
  padding: 20px 8px;
  font-style: italic;
}
.domain-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 6px 8px;
  border-radius: 8px;
  border: 1px solid transparent;
  transition: all var(--speed) var(--ease);
}
.domain-row:hover {
  background: var(--bg-card-hover);
  border-color: var(--border);
}
.domain-name {
  font-size: 12px;
  font-weight: 600;
  color: var(--text-white);
  font-family: "JetBrains Mono", monospace;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  max-width: 140px;
}
.domain-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
}
.meta-pill {
  font-size: 10px;
  font-weight: 700;
  padding: 1px 6px;
  border-radius: 4px;
  border: 1px solid currentColor;
}
.meta-text {
  font-size: 10px;
  color: var(--text-dim);
  font-family: "JetBrains Mono", monospace;
  min-width: 30px;
  text-align: right;
}
.score-bar {
  width: 40px;
  height: 4px;
  border-radius: 2px;
  background: rgba(255,255,255,0.05);
  overflow: hidden;
}
.score-fill {
  height: 100%;
  border-radius: 2px;
  transition: width 0.4s var(--ease);
}
</style>
