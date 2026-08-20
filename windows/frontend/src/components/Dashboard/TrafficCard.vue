<script lang="ts" setup>
import { ref, onMounted, onUnmounted, computed } from "vue"
import { Status, GetTraffic } from "../../../wailsjs/go/main/App"
import pepeRkn from "../../assets/memes/pepe_rkn_jail_crying.png"

const connected = ref(false)
const state = ref("stopped")
const dlSpeed = ref(0)   // bytes/sec
const ulSpeed = ref(0)
const dlTotal = ref(0)   // bytes
const ulTotal = ref(0)
const uptime = ref(0)
const available = ref(false)
const chartData = ref<number[]>(Array(30).fill(0))

let timer: number | undefined

async function poll() {
  try {
    const s = await Status()
    connected.value = s.connected
    state.value = s.state
    if (s.connected) {
      const t = await GetTraffic()
      available.value = Boolean((t as any).available)
      dlSpeed.value = t.downloadSpeed || 0
      ulSpeed.value = t.uploadSpeed || 0
      dlTotal.value = t.downloadTotal || 0
      ulTotal.value = t.uploadTotal || 0
      uptime.value = t.uptime || 0
      chartData.value.push(available.value ? Math.round((t.downloadSpeed || 0) / 1024) : 0)
      if (chartData.value.length > 30) chartData.value.shift()
    } else {
      available.value = false
      dlSpeed.value = 0
      ulSpeed.value = 0
      chartData.value.push(0)
      if (chartData.value.length > 30) chartData.value.shift()
    }
  } catch {}
}

function fmtSpeed(bytesPerSec: number): string {
  if (!available.value) return "нет данных"
  if (bytesPerSec < 1024) return bytesPerSec.toFixed(0) + " B/s"
  if (bytesPerSec < 1048576) return (bytesPerSec / 1024).toFixed(1) + " KB/s"
  return (bytesPerSec / 1048576).toFixed(2) + " MB/s"
}

function fmtBytes(b: number): string {
  if (!available.value) return "нет данных"
  if (b < 1024) return b.toFixed(0) + " B"
  if (b < 1048576) return (b / 1024).toFixed(1) + " KB"
  if (b < 1073741824) return (b / 1048576).toFixed(2) + " MB"
  return (b / 1073741824).toFixed(2) + " GB"
}

function fmtUptime(sec: number): string {
  const h = Math.floor(sec / 3600)
  const m = Math.floor((sec % 3600) / 60)
  const s = Math.floor(sec % 60)
  if (h > 0) return `${h}:${String(m).padStart(2, "0")}:${String(s).padStart(2, "0")}`
  return `${m}:${String(s).padStart(2, "0")}`
}

onMounted(() => { poll(); timer = window.setInterval(poll, 1000) })
onUnmounted(() => { if (timer) clearInterval(timer) })
</script>

<template>
  <div class="card traffic-card">
    <div class="card-title-row">
      <div>
        <span class="card-title">ТРАФИК</span>
        <span class="card-subtitle">// РКН не дремлет</span>
      </div>
      <img :src="pepeRkn" class="pepe-corner" alt="РКН" />
    </div>

    <div class="speeds-row">
      <div class="speed-box">
        <div class="speed-label">
          <span class="arrow">↓</span>
          <span>DOWNLOAD</span>
        </div>
        <div class="speed-value">
          <span class="val">{{ fmtSpeed(dlSpeed) }}</span>
        </div>
      </div>
      <div class="speed-box">
        <div class="speed-label">
          <span class="arrow">↑</span>
          <span>UPLOAD</span>
        </div>
        <div class="speed-value">
          <span class="val">{{ fmtSpeed(ulSpeed) }}</span>
        </div>
      </div>
    </div>

    <div class="chart-area">
      <div class="chart">
        <div
          v-for="(v, i) in chartData"
          :key="i"
          class="bar"
          :style="{ height: Math.min(100, v) + '%', opacity: 0.3 + (Math.min(100, v)/100) * 0.7 }"
        />
      </div>
    </div>

    <div class="traffic-footer">
      <div class="session-info">
        <span class="session-label">СЕССИЯ:</span>
        <template v-if="connected && available">
          <span class="session-val">↓ {{ fmtBytes(dlTotal) }}</span>
          <span class="session-val">↑ {{ fmtBytes(ulTotal) }}</span>
          <span class="session-val uptime">{{ fmtUptime(uptime) }}</span>
        </template>
        <span class="session-val muted" v-else-if="connected">— нет данных</span>
        <span class="session-val muted" v-else>— отключено</span>
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
  min-height: 220px;
}
.card:hover {
  border-color: var(--border-hover);
}
.card-title-row {
  margin-bottom: 10px;
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
.speeds-row {
  display: flex;
  gap: 20px;
  margin-bottom: 8px;
}
.speed-box {
  flex: 1;
}
.speed-label {
  display: flex;
  align-items: center;
  gap: 4px;
  font-size: 10px;
  color: var(--text-dim);
  letter-spacing: 0.5px;
  margin-bottom: 2px;
}
.arrow {
  font-size: 14px;
  font-weight: 700;
}
.speed-box:first-child .arrow { color: var(--green); }
.speed-box:last-child .arrow { color: var(--blue-info); }
.speed-value {
  display: flex;
  align-items: baseline;
  gap: 4px;
}
.val {
  font-size: 28px;
  font-weight: 700;
  color: var(--green);
  font-family: "JetBrains Mono", monospace;
  line-height: 1;
}
.speed-box:last-child .val {
  color: var(--blue-info);
}
.unit {
  font-size: 11px;
  color: var(--text-dim);
}
.chart-area {
  position: relative;
  flex: 1;
  min-height: 60px;
  margin: 8px 0;
}
.chart {
  display: flex;
  align-items: flex-end;
  gap: 2px;
  height: 60px;
  position: absolute;
  bottom: 0;
  left: 0;
  right: 0;
}
.bar {
  flex: 1;
  min-height: 2px;
  background: var(--green);
  border-radius: 1px;
  transition: height 0.4s var(--ease);
}
.pepe-corner {
  width: 36px;
  height: 36px;
  object-fit: contain;
  opacity: 0.7;
  margin-left: auto;
  flex-shrink: 0;
}
.card-title-row {
  display: flex;
  align-items: center;
  margin-bottom: 10px;
}
.traffic-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-top: auto;
  padding-top: 8px;
  border-top: 1px solid rgba(255, 255, 255, 0.03);
}
.session-info {
  display: flex;
  align-items: center;
  gap: 8px;
}
.session-label {
  font-size: 10px;
  color: var(--text-dim);
  font-weight: 600;
}
.session-val {
  font-size: 10px;
  color: var(--text-gray);
  font-family: "JetBrains Mono", monospace;
}
.reset-btn {
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
.reset-btn:hover {
  border-color: var(--red-danger);
  color: var(--red-danger);
}

/* Responsive */
@media (max-width: 1200px) {
  .val {
    font-size: 22px;
  }
  .pepe-corner {
    width: 28px;
    height: 28px;
  }
}

@media (max-width: 900px) {
  .speeds-row {
    gap: 12px;
  }
  .val {
    font-size: 20px;
  }
  .session-info {
    flex-wrap: wrap;
    gap: 4px;
  }
}
</style>
