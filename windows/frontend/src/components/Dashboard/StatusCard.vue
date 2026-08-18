<script lang="ts" setup>
import { ref, computed, onMounted, onUnmounted } from "vue"
import { GetServers, GetLatency } from "../../../wailsjs/go/main/App"

const props = defineProps<{
  state: string
  configId: string
  diag: any
  busy: boolean
}>()
const emit = defineEmits<{ toggle: [] }>()

const activeServer = ref<string>("")
const latency = ref<number>(-1)
let pollTimer: number | undefined

async function refreshServer() {
  if (props.state !== "running") {
    activeServer.value = ""
    latency.value = -1
    return
  }
  try {
    const servers = await GetServers()
    const active = (servers as any[])?.find((s: any) => s.active)
    if (active) {
      activeServer.value = active.name + " · " + active.location
    }
    latency.value = await GetLatency()
  } catch {}
}

onMounted(() => {
  refreshServer()
  pollTimer = window.setInterval(refreshServer, 5000)
})
onUnmounted(() => { if (pollTimer) window.clearInterval(pollTimer) })

type StatusConfig = {
  color: string
  glow: string
  label: string
  desc: string
  dotAnim: string
}

const statusInfo = computed((): StatusConfig => {
  const s = props.state
  const d = props.diag
  const cat = d?.category || ""
  const expl = d?.explanation || ""

  if (s === "running" && cat === "healthy") {
    return { color: "var(--green-success)", glow: "var(--green-glow)", label: "ВСЁ ХОРОШО", desc: "Туннель работает нормально", dotAnim: "pulseGlow" }
  }
  if (s === "running" && cat === "degraded") {
    return { color: "var(--yellow-warn)", glow: "var(--yellow-glow)", label: "ЗАМЕДЛЕНО", desc: expl || "Туннель работает медленно", dotAnim: "pulseGlowYellow" }
  }
  if (s === "running" && d?.state === "RECOVERING") {
    return { color: "var(--blue-info)", glow: "var(--blue-glow)", label: "ВОССТАНОВЛЕНИЕ", desc: "Переподключение…", dotAnim: "pulseGlowYellow" }
  }
  if (s === "running" && (cat === "server_down" || cat === "server_blocked" || cat === "tls_failure")) {
    return { color: "var(--red-danger)", glow: "var(--red-glow)", label: "СБОЙ", desc: expl || "Проблема с сервером", dotAnim: "pulseGlowRed" }
  }
  if (s === "starting") {
    return { color: "var(--yellow-warn)", glow: "var(--yellow-glow)", label: "ПОДКЛЮЧЕНИЕ…", desc: "Устанавливаем соединение", dotAnim: "pulseGlowYellow" }
  }
  if (s === "stopping") {
    return { color: "var(--yellow-warn)", glow: "var(--yellow-glow)", label: "ОТКЛЮЧЕНИЕ…", desc: "Завершаем соединение", dotAnim: "pulseGlowYellow" }
  }
  if (s === "error") {
    return { color: "var(--red-danger)", glow: "var(--red-glow)", label: "ОШИБКА", desc: expl || "Что-то пошло не так", dotAnim: "pulseGlowRed" }
  }
  return { color: "var(--text-dim)", glow: "none", label: "ОТКЛЮЧЕНО", desc: "VPN выключен", dotAnim: "none" }
})

const isConnected = computed(() => props.state === "running")
const isConnecting = computed(() => props.state === "starting" || props.busy)

function handleClick() {
  if (!props.busy) {
    emit("toggle")
  }
}
</script>

<template>
  <div class="card status-card">
    <div class="card-title">СТАТУС ПОДКЛЮЧЕНИЯ</div>

    <div class="status-body">
      <!-- Big power button -->
      <button
        class="power-circle"
        :class="{ on: isConnected, connecting: isConnecting }"
        :disabled="busy"
        @click="handleClick"
      >
        <div class="power-ring" :class="{ on: isConnected, connecting: isConnecting }">
          <div class="power-inner">
            <span class="power-icon">⏻</span>
          </div>
        </div>
      </button>

      <div class="status-text-block">
        <div class="status-label-big" :style="{ color: statusInfo.color }">
          {{ statusInfo.label }}
        </div>
        <div class="status-desc">{{ statusInfo.desc }}</div>
        <div class="server-info" v-if="isConnected">
          <span class="server-name">{{ activeServer || 'VPS' }}</span>
          <span class="server-sep" v-if="latency >= 0">·</span>
          <span class="server-ping" v-if="latency >= 0">{{ latency }}ms</span>
        </div>
      </div>
    </div>

    <div class="status-footer">
      <span class="footer-text">{{ isConnected ? 'Обновляется каждые 5 сек' : 'VPN выключен' }}</span>
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
.card-title {
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 1px;
  color: var(--text-dim);
  margin-bottom: 12px;
}
.status-body {
  display: flex;
  align-items: center;
  gap: 16px;
  flex: 1;
}
.power-circle {
  background: none;
  border: none;
  cursor: pointer;
  padding: 0;
  flex-shrink: 0;
  transition: all var(--speed) var(--ease);
}
.power-circle:hover:not(:disabled) {
  transform: scale(1.05);
}
.power-circle:active:not(:disabled) {
  transform: scale(0.95);
}
.power-circle:disabled {
  cursor: wait;
  opacity: 0.7;
}
.power-ring {
  width: 64px;
  height: 64px;
  border-radius: 50%;
  border: 2px solid var(--border);
  display: flex;
  align-items: center;
  justify-content: center;
  transition: all var(--speed) var(--ease);
  background: var(--bg-card-hover);
}
.power-ring.on {
  border-color: var(--green);
  background: rgba(60, 255, 90, 0.1);
  box-shadow: 0 0 20px rgba(60, 255, 90, 0.2), inset 0 0 10px rgba(60, 255, 90, 0.1);
  animation: pulseGlow 2s ease-in-out infinite;
}
.power-ring.connecting {
  border-color: var(--yellow-warn);
  background: rgba(255, 197, 51, 0.1);
  animation: pulseGlowYellow 1.5s ease-in-out infinite;
}
.power-inner {
  width: 48px;
  height: 48px;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--bg-card);
  transition: all var(--speed) var(--ease);
}
.power-ring.on .power-inner {
  background: rgba(60, 255, 90, 0.15);
}
.power-icon {
  font-size: 24px;
  color: var(--text-dim);
  transition: all var(--speed) var(--ease);
}
.power-ring.on .power-icon {
  color: var(--green-success);
  filter: drop-shadow(0 0 4px var(--green-success));
}
.power-ring.connecting .power-icon {
  color: var(--yellow-warn);
}
.status-text-block {
  flex: 1;
  min-width: 0;
}
.status-label-big {
  font-size: 20px;
  font-weight: 700;
  letter-spacing: 1px;
  transition: all var(--speed) var(--ease);
}
.status-desc {
  font-size: 12px;
  color: var(--text-gray);
  margin-top: 2px;
}
.server-info {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-top: 6px;
  font-size: 12px;
}
.server-name {
  color: var(--text-white);
  font-weight: 500;
}
.server-sep {
  color: var(--text-dim);
}
.server-ping {
  color: var(--green);
  font-family: "JetBrains Mono", monospace;
}
.status-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-top: 10px;
  padding-top: 8px;
  border-top: 1px solid rgba(255, 255, 255, 0.03);
}
.footer-text {
  font-size: 10px;
  color: var(--text-dim);
}
.detail-btn {
  padding: 3px 10px;
  border: 1px solid var(--border);
  border-radius: 6px;
  background: transparent;
  color: var(--text-dim);
  font-size: 10px;
  cursor: pointer;
  transition: all var(--speed) var(--ease);
  font-family: "JetBrains Mono", monospace;
}
.detail-btn:hover {
  border-color: var(--green);
  color: var(--green);
}
</style>
