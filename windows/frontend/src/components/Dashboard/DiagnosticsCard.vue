<script lang="ts" setup>
import { ref, onMounted, onUnmounted, computed } from "vue"
import { Diagnostics, GetLatency } from "../../../wailsjs/go/main/App"

const diag = ref<any>({})
const expanded = ref(false)
const latency = ref<number>(-1)

let timer: number | undefined
async function poll() {
  try {
    diag.value = await Diagnostics()
    latency.value = await GetLatency()
  } catch {}
}

onMounted(() => { poll(); timer = window.setInterval(poll, 3000) })
onUnmounted(() => { if (timer) clearInterval(timer) })

interface CheckItem {
  label: string
  target: string
  ok: boolean
  lat: string
}

const checks = computed((): CheckItem[] => {
  const d = diag.value
  const isHealthy = d?.state === "HEALTHY"
  const cat = d?.category || ""
  const lat = latency.value
  const latStr = lat >= 0 ? lat + "ms" : "—"
  return [
    { label: "Локальный интернет", target: "8.8.8.8", ok: cat !== "network_down", lat: "ok" },
    { label: "VPS сервер", target: "VPS:443", ok: cat !== "server_down", lat: "ok" },
    { label: "Туннель", target: "generate_204", ok: isHealthy, lat: latStr },
    { label: "DNS", target: "1.1.1.1", ok: cat !== "dns_failure", lat: "ok" },
    { label: "TLS handshake", target: "", ok: cat !== "tls_failure", lat: cat !== "tls_failure" ? "OK" : "FAIL" }
  ]
})

const detailInfo = computed(() => {
  const d = diag.value
  return {
    server: "VPS Netherlands",
    protocol: "VLESS+TLS",
    activeSessions: "—",
    reconnects: d?.failCount || 0,
    reason: d?.lastError ? (d.lastError as string).substring(0, 60) : (d?.explanation || "—")
  }
})
</script>

<template>
  <div class="card diagnostics-card">
    <div class="card-header" @click="expanded = !expanded">
      <div class="title-group">
        <span class="card-title">ДИАГНОСТИКА</span>
        <span class="expand-icon" :class="{ expanded }">▼</span>
      </div>
    </div>

    <div class="checks-list">
      <div v-for="c in checks" :key="c.label" class="check" :class="{ ok: c.ok }">
        <span class="check-icon">{{ c.ok ? '✓' : '✕' }}</span>
        <div class="check-info">
          <div class="check-name">{{ c.label }}</div>
          <div class="check-target" v-if="c.target">{{ c.target }}</div>
        </div>
        <span class="check-lat" :class="{ ok: c.ok }">{{ c.lat }}</span>
      </div>
    </div>

    <!-- Expanded details panel -->
    <div class="details-panel" v-show="expanded">
      <div class="detail-row">
        <span class="detail-label">Активный сервер:</span>
        <span class="detail-val">{{ detailInfo.server }}</span>
      </div>
      <div class="detail-row">
        <span class="detail-label">Протокол:</span>
        <span class="detail-val">{{ detailInfo.protocol }}</span>
      </div>
      <div class="detail-row">
        <span class="detail-label">Активные сессии:</span>
        <span class="detail-val">{{ detailInfo.activeSessions }}</span>
      </div>
      <div class="detail-row">
        <span class="detail-label">Переподключений за сессию:</span>
        <span class="detail-val">{{ detailInfo.reconnects }}</span>
      </div>
      <div class="detail-row error" v-if="!checks.every(c => c.ok)">
        <span class="detail-label">Причина:</span>
        <span class="detail-val">{{ detailInfo.reason }}</span>
      </div>
    </div>

    <div class="actions">
      <button class="action-btn" @click="poll">
        <span>↻</span>
        ПЕРЕПРОВЕРИТЬ
      </button>
      <button class="action-btn" @click="expanded = !expanded">
        <span>{{ expanded ? '▲' : '▼' }}</span>
        {{ expanded ? 'СКРЫТЬ' : 'ПОДРОБНЕЕ' }}
      </button>
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
.card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 10px;
  cursor: pointer;
}
.title-group {
  display: flex;
  align-items: center;
  gap: 8px;
}
.card-title {
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 1px;
  color: var(--text-dim);
}
.expand-icon {
  font-size: 10px;
  color: var(--text-dim);
  transition: transform var(--speed) var(--ease);
}
.expand-icon.expanded {
  transform: rotate(180deg);
}
.checks-list {
  display: flex;
  flex-direction: column;
  gap: 6px;
  flex: 1;
}
.check {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 4px 0;
  transition: all var(--speed) var(--ease);
}
.check-icon {
  width: 16px;
  height: 16px;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 9px;
  font-weight: 700;
  flex-shrink: 0;
  background: rgba(255, 77, 77, 0.1);
  color: var(--red-danger);
}
.check.ok .check-icon {
  background: rgba(60, 255, 90, 0.1);
  color: var(--green-success);
}
.check-info {
  flex: 1;
  min-width: 0;
}
.check-name {
  font-size: 12px;
  font-weight: 500;
}
.check-target {
  font-size: 10px;
  color: var(--text-dim);
  font-family: "JetBrains Mono", monospace;
}
.check-lat {
  font-size: 11px;
  font-family: "JetBrains Mono", monospace;
  color: var(--red-danger);
}
.check-lat.ok {
  color: var(--green);
}
.details-panel {
  margin-top: 10px;
  padding: 10px;
  background: var(--bg-terminal);
  border: 1px solid rgba(0, 255, 80, 0.05);
  border-radius: 10px;
  animation: slideDown var(--speed) var(--ease) forwards;
}
.detail-row {
  display: flex;
  justify-content: space-between;
  padding: 3px 0;
  font-size: 11px;
}
.detail-label {
  color: var(--text-dim);
}
.detail-val {
  color: var(--text-white);
  font-family: "JetBrains Mono", monospace;
  font-size: 10px;
}
.detail-row.error .detail-val {
  color: var(--red-danger);
}
.actions {
  display: flex;
  gap: 6px;
  margin-top: 10px;
  padding-top: 8px;
  border-top: 1px solid rgba(255, 255, 255, 0.03);
}
.action-btn {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 5px 10px;
  border: 1px solid var(--border);
  border-radius: 6px;
  background: transparent;
  color: var(--text-gray);
  font-size: 10px;
  font-weight: 600;
  cursor: pointer;
  transition: all var(--speed) var(--ease);
  font-family: "JetBrains Mono", monospace;
}
.action-btn:hover {
  border-color: var(--green);
  color: var(--green);
}
.action-btn.copy {
  padding: 5px 8px;
}
</style>
