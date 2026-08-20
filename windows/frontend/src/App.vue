<script lang="ts" setup>
import { ref, onMounted, onUnmounted, computed, provide, nextTick } from "vue"
import { StartVPN, StopVPN, Status, LoadConfigFile, Diagnostics, GetTraffic } from "../wailsjs/go/main/App"
import { EventsOn, EventsOff } from "../wailsjs/runtime/runtime"
import Sidebar from "./components/Layout/Sidebar.vue"
import TopBar from "./components/Layout/TopBar.vue"
import TerminalBar from "./components/Layout/TerminalBar.vue"
import ServersCard from "./components/Servers/ServersCard.vue"
import RoutingCard from "./components/Servers/RoutingCard.vue"
import TrafficCard from "./components/Dashboard/TrafficCard.vue"
import StatusCard from "./components/Dashboard/StatusCard.vue"
import DiagnosticsCard from "./components/Dashboard/DiagnosticsCard.vue"
import EventsCard from "./components/Dashboard/EventsCard.vue"
import DomainStatsCard from "./components/Dashboard/DomainStatsCard.vue"
import SettingsCard from "./components/Settings/SettingsCard.vue"

const activePage = ref("dashboard")
const state = ref("stopped")
const configId = ref("")
const message = ref("")
const busy = ref(false)
const logs = ref<string[]>([])
const events = ref<{time:string;icon:string;title:string;desc:string;type?:string}[]>([])
const diag = ref<any>({})
const toasts = ref<{id:number;text:string;type:string;time:number}[]>([])
const sidebarUptime = ref("—")

const connected = computed(() => state.value === "running")
const CONFIG_ID = "vps-reality"
const CONFIG_FILE = "template-vps-reality.json"

let toastId = 0
function showToast(text: string, type: "success" | "error" | "warn" | "info" = "info") {
  const id = ++toastId
  toasts.value.push({ id, text, type, time: Date.now() })
  setTimeout(() => { toasts.value = toasts.value.filter(t => t.id !== id) }, 3000)
}
provide("showToast", showToast)

async function openAmnezia() {
  try {
    const w = window as any
    await w.go.main.App.OpenExternalApp("amnezia")
    showToast("Amnezia VPN запущен — Snowden VLESS будет отключён", "success")
    await stopIfRunning()
  } catch (e: any) {
    showToast("Amnezia не установлена: " + (e?.message || e), "warn")
  }
}

async function openKaring() {
  try {
    const w = window as any
    await w.go.main.App.OpenExternalApp("karing")
    showToast("Karing запущен — Snowden VLESS будет отключён", "success")
    await stopIfRunning()
  } catch (e: any) {
    showToast("Karing не установлен: " + (e?.message || e), "warn")
  }
}

// Stop the running Snowden VLESS engine so it doesn't conflict with
// the externally-launched client (both claim the system proxy / TUN).
async function stopIfRunning() {
  try {
    if (state.value === "running" || state.value === "starting" || state.value === "stopping") {
      const w = window as any
      await w.go.main.App.StopVPN()
    }
  } catch {
    /* engine may already be stopped */
  }
}

function navTo(page: string) {
  activePage.value = page
  if (page === "logs") {
    // "Логи" scrolls the terminal bar at the bottom into view
    activePage.value = "dashboard"
    nextTick(() => {
      const el = document.querySelector(".terminal-bar") as HTMLElement | null
      if (el) el.scrollIntoView({ behavior: "smooth", block: "end" })
    })
    return
  }
  if (page !== "about") {
    // Map nav id → active dashboard section, scroll + flash
    activePage.value = "dashboard"
    nextTick(() => {
      const el = document.getElementById("section-" + page)
      if (el) {
        el.scrollIntoView({ behavior: "smooth", block: "center" })
        el.classList.add("flash")
        setTimeout(() => el.classList.remove("flash"), 1500)
      }
    })
  }
}

async function refreshStatus() {
  try {
    const s = await Status()
    state.value = s.state
    configId.value = s.configId
    if (s.state === "running" && s.connected) {
      message.value = ""
      state.value = "running"
      const t = await GetTraffic()
      if (t.uptime > 0) {
        const h = Math.floor(t.uptime / 3600)
        const m = Math.floor((t.uptime % 3600) / 60)
        const sec = Math.floor(t.uptime % 60)
        sidebarUptime.value = `${String(h).padStart(2,"0")}:${String(m).padStart(2,"0")}:${String(sec).padStart(2,"0")}`
      }
    } else {
      sidebarUptime.value = "—"
    }
    diag.value = await Diagnostics()
  } catch {}
}

async function toggle() {
  if (busy.value) return
  busy.value = true
  message.value = ""
  try {
    if (connected.value) {
      state.value = "stopping"
      await StopVPN()
      showToast("VPN отключён", "info")
      events.value.unshift({time: now(), icon: "⏹", title: "Отключено", desc: "VPN выключен", type: "info"})
    } else {
      state.value = "starting"
      message.value = "читаю конфиг…"
      const cfg = await LoadConfigFile(CONFIG_FILE)
      await StartVPN(CONFIG_ID, cfg)
      const st = await Status()
      if (st.connected) {
        showToast("VPN подключён — VPS Netherlands", "success")
        events.value.unshift({time: now(), icon: "✅", title: "Подключено", desc: "VPS Netherlands", type: "success"})
        message.value = ""
      } else {
        const errMsg = st.message || "Engine failed to reach Running state"
        message.value = errMsg
        state.value = "error"
        showToast(errMsg, "error")
        events.value.unshift({time: now(), icon: "⛔", title: "Ошибка запуска", desc: errMsg, type: "error"})
      }
    }
  } catch (e: any) {
    const err = describeError(e)
    message.value = err
    state.value = "error"
    showToast(err, "error")
    events.value.unshift({time: now(), icon: "⛔", title: "Ошибка", desc: err, type: "error"})
  } finally {
    busy.value = false
    await refreshStatus()
  }
}

function describeError(e: any): string {
  if (!e) return "неизвестная ошибка"
  if (typeof e === "string") return e
  if (e instanceof Error) return e.message
  const m = (e && (e.message || e.error || e.msg)) as string | undefined
  return m ? String(m) : String(e)
}

function now(): string {
  return new Date().toLocaleTimeString("ru-RU", {hour:"2-digit",minute:"2-digit",second:"2-digit"})
}

let pollTimer: number | undefined
onMounted(() => {
  refreshStatus()
  pollTimer = window.setInterval(refreshStatus, 2000)
  EventsOn("engine:log", (line: string) => {
    logs.value.push(line)
    if (logs.value.length > 500) logs.value.shift()
  })
  EventsOn("engine:diag", (line: string) => {
    events.value.unshift({time: now(), icon: "🔍", title: line, desc: "", type: "info"})
    if (events.value.length > 50) events.value.pop()
  })
})
onUnmounted(() => {
  if (pollTimer) clearInterval(pollTimer)
  EventsOff("engine:log")
  EventsOff("engine:diag")
})
</script>

<template>
  <div class="app-layout">
    <Sidebar :active="activePage" @nav="navTo" :connected="connected" :state="state" :uptime="sidebarUptime" />
    <div class="main-area">
      <TopBar :connected="connected" :busy="busy" />

      <div class="content-scroll">
        <div class="dashboard-grid" v-show="activePage === 'dashboard'">
          <!-- Row 1: Servers | Routing -->
          <div class="row row-2">
            <div id="section-dashboard" class="section-anchor"><ServersCard :configId="configId" :connected="connected" /></div>
            <div id="section-routing" class="section-anchor"><RoutingCard /></div>
          </div>

          <!-- Row 2: Traffic | Settings -->
          <div class="row row-2">
            <div id="section-traffic" class="section-anchor"><TrafficCard /></div>
            <div id="section-settings" class="section-anchor"><SettingsCard /></div>
          </div>

          <!-- Row 3: Status | Diagnostics | Events -->
          <div class="row row-3">
            <StatusCard :state="state" :configId="configId" :diag="diag" @toggle="toggle" :busy="busy" />
            <div id="section-diagnostics" class="section-anchor"><DiagnosticsCard :diag="diag" /></div>
            <div id="section-events" class="section-anchor"><EventsCard :events="events" /></div>
          </div>

          <!-- Row 4: Domain Intelligence -->
          <div class="row row-1">
            <DomainStatsCard />
          </div>

          <!-- Row 5: Alternative Protocols -->
          <div class="row row-2">
            <div class="protocol-card" @click="openAmnezia">
              <div class="protocol-icon amnezia">🛡</div>
              <div class="protocol-info">
                <div class="protocol-name">Amnezia VPN</div>
                <div class="protocol-desc">WARP bypass · AmneziaWG · отдельный туннель</div>
              </div>
              <div class="protocol-action">
                <span class="protocol-arrow">Открыть →</span>
              </div>
            </div>
            <div class="protocol-card" @click="openKaring">
              <div class="protocol-icon karing">⚡</div>
              <div class="protocol-info">
                <div class="protocol-name">Karing (Mieru)</div>
                <div class="protocol-desc">Без TLS · высокая скорость · XChaCha20</div>
              </div>
              <div class="protocol-action">
                <span class="protocol-arrow">Открыть →</span>
              </div>
            </div>
          </div>
        </div>

        <!-- Full-page: About -->
        <div v-show="activePage === 'about'" class="about-page">
          <div class="about-card">
            <h1 class="about-title">snowden.system</h1>
            <p class="about-tagline">privacy is a human right</p>
            <div class="about-grid">
              <div class="about-row"><span class="ab-label">Версия</span><span class="ab-val">snowden.system</span></div>
              <div class="about-row"><span class="ab-label">Ядро</span><span class="ab-val">sing-box (embedded)</span></div>
              <div class="about-row"><span class="ab-label">Протоколы</span><span class="ab-val">Hysteria2 · selector · fail-closed</span></div>
              <div class="about-row"><span class="ab-label">Каналы</span><span class="ab-val">из фактического конфига · adaptive recovery</span></div>
              <div class="about-row"><span class="ab-label">Split-tunnel</span><span class="ab-val">RU→direct · заблокированные→VPN</span></div>
              <div class="about-row"><span class="ab-label">Диагностика</span><span class="ab-val">probe · circuit breaker · channel memory</span></div>
              <div class="about-row"><span class="ab-label">Telegram бот</span><span class="ab-val">админ-панель + логи</span></div>
            </div>
            <div class="about-footer">
              <span>© 2026 · сделано для свободы интернета</span>
            </div>
          </div>
        </div>
      </div>

      <TerminalBar :logs="logs" :state="state" :connected="connected" />
    </div>

    <!-- Toast notifications -->
    <div class="toast-container">
      <transition-group name="toast">
        <div v-for="toast in toasts" :key="toast.id" class="toast" :class="toast.type">
          <span class="toast-icon">
            {{ toast.type === 'success' ? '✓' : toast.type === 'error' ? '✕' : toast.type === 'warn' ? '▲' : 'ℹ' }}
          </span>
          <span class="toast-text">{{ toast.text }}</span>
        </div>
      </transition-group>
    </div>
  </div>
</template>

<style scoped>
.app-layout {
  display: flex;
  width: 100vw;
  height: 100vh;
  overflow: hidden;
  min-width: 900px;
}
.main-area {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-width: 0;
  overflow: hidden;
}
.content-scroll {
  flex: 1;
  overflow-y: auto;
  overflow-x: hidden;
  padding: 12px 16px;
}
.dashboard-grid {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.row {
  display: grid;
  gap: 12px;
}
.row-2 {
  grid-template-columns: 1fr 1fr;
}
.row-3 {
  grid-template-columns: 1fr 1fr 1fr;
}
.row-1 {
  grid-template-columns: 1fr;
}
.row-3 > *,
.row-2 > * {
  min-width: 0;
}

/* Responsive breakpoints */
@media (max-width: 1400px) {
  .row-3 {
    grid-template-columns: 1fr 1fr;
  }
  .row-3 > :last-child:nth-child(odd) {
    grid-column: span 2;
  }
}

@media (max-width: 1200px) {
  .row-2,
  .row-3 {
    grid-template-columns: 1fr;
  }
  .row-3 > :last-child:nth-child(odd) {
    grid-column: span 1;
  }
}

@media (max-width: 900px) {
  .app-layout {
    min-width: 600px;
  }
  .content-scroll {
    padding: 8px;
  }
}

/* Toast container */
.toast-container {
  position: fixed;
  top: 16px;
  right: 16px;
  z-index: var(--z-toast);
  display: flex;
  flex-direction: column;
  gap: 8px;
  pointer-events: none;
}
.toast {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 16px;
  background: var(--bg-card);
  border: 1px solid var(--border);
  border-radius: 10px;
  box-shadow: 0 4px 20px rgba(0,0,0,0.4);
  pointer-events: auto;
  animation: slideDown var(--speed) var(--ease) forwards;
}
.toast.success { border-color: rgba(60,255,90,0.3); }
.toast.error { border-color: rgba(255,77,77,0.3); }
.toast.warn { border-color: rgba(255,197,51,0.3); }
.toast.info { border-color: var(--border); }
.toast-icon { font-size: 14px; font-weight: 700; }
.toast.success .toast-icon { color: var(--green-success); }
.toast.error .toast-icon { color: var(--red-danger); }
.toast.warn .toast-icon { color: var(--yellow-warn); }
.toast.info .toast-icon { color: var(--blue-info); }
.toast-text { font-size: 13px; color: var(--text-white); }

/* Protocol cards */
.protocol-card {
  background: var(--bg-card);
  border: 1px solid var(--border);
  border-radius: 14px;
  padding: 16px 20px;
  display: flex;
  align-items: center;
  gap: 14px;
  cursor: pointer;
  transition: all 0.2s ease;
  min-height: 64px;
}
.protocol-card:hover {
  border-color: rgba(60,255,90,0.15);
  background: var(--bg-card-hover);
  transform: translateY(-1px);
}
.protocol-icon {
  width: 40px; height: 40px;
  border-radius: 10px;
  display: flex; align-items: center; justify-content: center;
  font-size: 20px; flex-shrink: 0;
}
.protocol-icon.amnezia { background: rgba(255,152,0,0.1); }
.protocol-icon.karing { background: rgba(45,184,255,0.1); }
.protocol-info { flex: 1; min-width: 0; }
.protocol-name { font-size: 14px; font-weight: 700; color: var(--text-white); }
.protocol-desc { font-size: 11px; color: var(--text-dim); margin-top: 2px; }
.protocol-arrow { font-size: 12px; color: var(--text-dim); font-weight: 600; }
.protocol-card:hover .protocol-arrow { color: var(--green); }

/* Toast transitions */
.toast-enter-active, .toast-leave-active { transition: all var(--speed) var(--ease); }
.toast-enter-from { opacity: 0; transform: translateX(20px) scale(0.95); }
.toast-leave-to { opacity: 0; transform: translateX(20px) scale(0.95); }

/* Section anchors for nav scrolling */
.section-anchor {
  min-width: 0;
}
.section-anchor :deep(.card) {
  transition: border-color 0.3s, box-shadow 0.3s;
}
.section-anchor.flash :deep(.card) {
  border-color: var(--green) !important;
  box-shadow: 0 0 30px rgba(60, 255, 90, 0.2) !important;
}

/* About page */
.about-page {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 40px;
  height: 100%;
}
.about-card {
  background: var(--bg-card);
  border: 1px solid var(--border);
  border-radius: 20px;
  padding: 40px;
  max-width: 600px;
  width: 100%;
}
.about-title {
  font-family: "JetBrains Mono", monospace;
  font-size: 32px;
  color: var(--green);
  font-weight: 700;
  margin: 0;
  letter-spacing: -1px;
}
.about-tagline {
  font-size: 14px;
  color: var(--text-dim);
  margin: 4px 0 28px;
  font-family: "JetBrains Mono", monospace;
}
.about-grid {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.about-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 10px 0;
  border-bottom: 1px solid rgba(255,255,255,0.03);
}
.ab-label {
  font-size: 12px;
  color: var(--text-dim);
  font-weight: 600;
  letter-spacing: 0.5px;
}
.ab-val {
  font-size: 13px;
  color: var(--text-white);
  font-family: "JetBrains Mono", monospace;
}
.about-footer {
  margin-top: 28px;
  text-align: center;
  font-size: 11px;
  color: var(--text-dim);
  opacity: 0.6;
}
</style>
