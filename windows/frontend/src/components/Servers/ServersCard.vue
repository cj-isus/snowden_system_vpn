<script lang="ts" setup>
import { ref, onMounted, onUnmounted, inject } from "vue"
import { GetServers, SelectServer } from "../../../wailsjs/go/main/App"

const props = defineProps<{ configId: string; connected: boolean }>()
const showToast = inject<(text: string, type?: string) => void>("showToast")

interface Server {
  id: string
  name: string
  protocol: string
  server: string
  port: number
  location: string
  active: boolean
  ping: number
}

const servers = ref<Server[]>([])
const loading = ref(true)
const selectedServer = ref<string>("auto")
const switching = ref(false)
let pollTimer: number | undefined

async function refresh() {
  try {
    const list = await GetServers()
    if (list && list.length > 0) {
      servers.value = list as Server[]
    }
  } catch (e) {
    // backend not ready yet
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  refresh()
  pollTimer = window.setInterval(refresh, 10000)
})

onUnmounted(() => {
  if (pollTimer) window.clearInterval(pollTimer)
})

async function switchServer(server: string) {
  if (switching.value) return
  switching.value = true
  const old = selectedServer.value
  selectedServer.value = server
  try {
    await SelectServer(server)
    const labels: Record<string, string> = { auto: "Авто (urltest)", nl: "Нидерланды", fr: "Франция" }
    showToast?.(`Сервер → ${labels[server] || server}`, "success")
  } catch (e: any) {
    selectedServer.value = old
    showToast?.(`Ошибка: ${e?.message || e}`, "error")
  } finally {
    switching.value = false
  }
}

function pingText(p: number): string {
  if (p < 0) return "—"
  return p + "ms"
}

function pingColor(p: number): string {
  if (p < 0) return "var(--text-dim)"
  if (p < 150) return "var(--green)"
  if (p < 400) return "var(--warning)"
  return "var(--danger)"
}
</script>

<template>
  <div class="card servers-card">
    <div class="card-title-row">
      <span class="card-title">СЕРВЕРЫ</span>
      <span class="card-subtitle">// выбери свой путь</span>
    </div>

    <div class="servers-list">
      <div v-if="loading" class="loading">Загрузка серверов…</div>
      <div
        v-for="server in servers"
        :key="server.id"
        class="server-row"
        :class="{ active: server.active, inactive: !server.active }"
      >
        <div class="server-radio">
          <div class="radio-outer" :class="{ on: server.active }">
            <div class="radio-inner" v-if="server.active" />
          </div>
        </div>
        <div class="server-info">
          <div class="server-name">{{ server.name }}</div>
          <div class="server-details">
            <span class="detail ip">{{ server.server }}:{{ server.port }}</span>
            <span class="detail domain">{{ server.location }}<span v-if="server.active"> · активен</span></span>
          </div>
        </div>
        <div class="server-meta">
          <span class="server-ping" :style="{ color: pingColor(server.ping) }">{{ pingText(server.ping) }}</span>
          <span class="server-proto">{{ server.protocol }}</span>
          <div class="check-box" :class="{ checked: server.active }">
            <span v-if="server.active">✓</span>
          </div>
        </div>
      </div>
      <div v-if="!loading && servers.length === 0" class="empty-state">
        Нет серверов. Запустите VPN для загрузки конфигурации.
      </div>
    </div>

    <div class="server-select">
      <span class="mode-label">Сервер:</span>
      <div class="server-buttons">
        <button
          class="srv-btn"
          :class="{ active: selectedServer === 'auto', disabled: switching }"
          @click="switchServer('auto')"
        >🤖 Авто</button>
        <button
          class="srv-btn"
          :class="{ active: selectedServer === 'nl', disabled: switching }"
          @click="switchServer('nl')"
        >🇳🇱 Нидерланды</button>
        <button
          class="srv-btn"
          :class="{ active: selectedServer === 'fr', disabled: switching }"
          @click="switchServer('fr')"
        >🇫🇷 Франция</button>
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
.servers-list {
  display: flex;
  flex-direction: column;
  gap: 6px;
  flex: 1;
}
.server-row {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  padding: 10px;
  border-radius: 10px;
  border: 1px solid transparent;
  transition: all var(--speed) var(--ease);
  cursor: pointer;
}
.server-row:hover {
  background: var(--bg-card-hover);
  border-color: var(--border);
}
.server-row.active {
  border-color: rgba(60, 255, 90, 0.2);
  background: rgba(60, 255, 90, 0.03);
}
.server-row.inactive {
  opacity: 0.7;
}
.server-row.empty {
  opacity: 0.5;
}
.server-radio {
  flex-shrink: 0;
  padding-top: 2px;
}
.radio-outer {
  width: 16px;
  height: 16px;
  border-radius: 50%;
  border: 2px solid var(--text-dim);
  display: flex;
  align-items: center;
  justify-content: center;
  transition: all var(--speed) var(--ease);
}
.radio-outer.on {
  border-color: var(--green);
  background: var(--green);
}
.radio-inner {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--bg-card);
}
.server-info {
  flex: 1;
  min-width: 0;
}
.server-name {
  font-size: 13px;
  font-weight: 600;
  color: var(--text-white);
}
.server-details {
  display: flex;
  flex-direction: column;
  gap: 1px;
  margin-top: 2px;
}
.detail {
  font-size: 10px;
  font-family: "JetBrains Mono", monospace;
}
.detail.domain {
  color: var(--green);
}
.detail.ip {
  color: var(--text-dim);
}
.detail.gray {
  color: var(--text-dim);
}
.detail.hint {
  color: var(--text-muted);
  font-style: italic;
}
.server-meta {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 2px;
  flex-shrink: 0;
}
.server-ping {
  font-size: 12px;
  font-weight: 700;
  color: var(--green);
  font-family: "JetBrains Mono", monospace;
}
.server-proto {
  font-size: 10px;
  color: var(--text-dim);
}
.check-box {
  width: 16px;
  height: 16px;
  border-radius: 4px;
  border: 1px solid var(--border);
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 10px;
  color: var(--green);
  margin-top: 2px;
}
.check-box.checked {
  background: rgba(60, 255, 90, 0.1);
  border-color: var(--green);
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
  letter-spacing: 0.5px;
}
.add-btn:hover {
  border-color: var(--green);
  color: var(--green);
}
.server-select {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-top: 10px;
  padding-top: 8px;
  border-top: 1px solid rgba(255, 255, 255, 0.03);
  flex-wrap: wrap;
}
.mode-label {
  font-size: 11px;
  color: var(--text-dim);
}
.server-buttons {
  display: flex;
  gap: 6px;
  flex-wrap: wrap;
}
.srv-btn {
  padding: 5px 12px;
  border-radius: 8px;
  border: 1px solid var(--border);
  background: transparent;
  color: var(--text-dim);
  font-size: 11px;
  font-weight: 600;
  cursor: pointer;
  transition: all var(--speed) var(--ease);
  font-family: "JetBrains Mono", monospace;
}
.srv-btn:hover:not(.disabled) {
  border-color: var(--green);
  color: var(--green);
}
.srv-btn.active {
  background: rgba(60, 255, 90, 0.1);
  border-color: var(--green);
  color: var(--green);
}
.srv-btn.disabled {
  opacity: 0.5;
  cursor: wait;
}
.mode-row {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-top: 10px;
  padding-top: 8px;
  border-top: 1px solid rgba(255, 255, 255, 0.03);
}
.mode-label {
  font-size: 11px;
  color: var(--text-dim);
}
.mode-options {
  display: flex;
  gap: 12px;
}
.mode-option {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 11px;
  color: var(--text-dim);
  cursor: pointer;
  transition: all var(--speed) var(--ease);
}
.mode-option.active {
  color: var(--green);
}
.mode-option input {
  display: none;
}
.radio-dot {
  width: 12px;
  height: 12px;
  border-radius: 50%;
  border: 2px solid var(--text-dim);
  display: flex;
  align-items: center;
  justify-content: center;
  transition: all var(--speed) var(--ease);
}
.mode-option.active .radio-dot {
  border-color: var(--green);
  background: var(--green);
}
.mode-option.active .radio-dot::after {
  content: "";
  width: 4px;
  height: 4px;
  border-radius: 50%;
  background: var(--bg-card);
}

/* Responsive */
@media (max-width: 1200px) {
  .server-row {
    padding: 8px;
  }
  .server-name {
    font-size: 12px;
  }
  .detail {
    font-size: 9px;
  }
}

@media (max-width: 900px) {
  .server-meta {
    flex-direction: row;
    gap: 8px;
  }
  .mode-options {
    flex-direction: column;
    gap: 6px;
  }
}
</style>
