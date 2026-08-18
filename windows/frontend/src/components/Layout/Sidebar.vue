<script lang="ts" setup>
import pepeFckDpi from "../../assets/memes/pepe_fck_dpi_vpn_tears.png"
import pepeShhh from "../../assets/memes/pepe_shhh_silence.png"

const props = defineProps<{
  active: string
  connected: boolean
  state: string
  uptime: string
}>()

const emit = defineEmits<{ nav: [page: string] }>()

const menu = [
  { id: "dashboard", label: "СЕРВЕРЫ", icon: "🌐" },
  { id: "routing", label: "МАРШРУТИЗАЦИЯ", icon: "⚡" },
  { id: "traffic", label: "ТРАФИК", icon: "📊" },
  { id: "settings", label: "НАСТРОЙКИ", icon: "⚙" },
  { id: "diagnostics", label: "ДИАГНОСТИКА", icon: "🔍" },
  { id: "events", label: "СОБЫТИЯ", icon: "📋" },
  { id: "logs", label: "ЛОГИ", icon: "📝" },
  { id: "about", label: "О СИСТЕМЕ", icon: "ⓘ" },
]

const statusLabel = computed(() => {
  if (props.connected) return "ЗАЩИЩЁН"
  if (props.state === "starting") return "ПОДКЛЮЧЕНИЕ…"
  if (props.state === "stopping") return "ОТКЛЮЧЕНИЕ…"
  if (props.state === "error") return "ОШИБКА"
  return "ОТКЛЮЧЕН"
})

const statusColor = computed(() => {
  if (props.connected) return "var(--green-success)"
  if (props.state === "starting" || props.state === "stopping") return "var(--yellow-warn)"
  if (props.state === "error") return "var(--red-danger)"
  return "var(--text-dim)"
})
</script>

<script lang="ts">
import { computed } from "vue"
</script>

<template>
  <aside class="sidebar">
    <!-- Brand -->
    <div class="brand-area">
      <div class="brand">snowden.system</div>
      <div class="tagline">> privacy is a human right</div>
    </div>

    <!-- Big Pepe -->
    <div class="pepe-hero">
      <img :src="pepeFckDpi" class="pepe-big" alt="Pepe" />
    </div>

    <!-- Terminal status block -->
    <div class="status-block">
      <div class="term-line">user@snowden:~$ system_status</div>
      <div class="term-output">
        <div class="status-line">
          <span class="status-arrow">></span>
          <span class="status-label" :style="{ color: statusColor }">Статус: {{ statusLabel }}</span>
        </div>
        <div class="status-line">
          <span class="status-arrow">></span>
          <span class="status-text">{{ props.connected ? 'Туннель активен' : 'Ожидание подключения' }}</span>
        </div>
        <div class="uptime-line">
          <span class="status-arrow">></span>
          <span class="uptime-label">UPTIME:</span>
          <span class="uptime-val">{{ props.uptime || '—' }}</span>
        </div>
      </div>
    </div>

    <!-- Navigation -->
    <nav class="nav">
      <button
        v-for="item in menu"
        :key="item.id"
        :class="['nav-item', { active: active === item.id }]"
        @click="emit('nav', item.id)"
      >
        <span class="nav-icon">{{ item.icon }}</span>
        <span class="nav-label">{{ item.label }}</span>
      </button>
    </nav>

    <!-- Bottom Pepe shhh -->
    <div class="meme-bottom">
      <img :src="pepeShhh" class="meme-shhh" alt="shhh" />
      <div class="meme-text">
        <div class="meme-line">ДАННЫЕ ЛЮБЯТ</div>
        <div class="meme-line green">ТИШИНУ</div>
        <div class="meme-sub">shh...</div>
      </div>
    </div>
  </aside>
</template>

<style scoped>
.sidebar {
  width: 240px;
  min-width: 200px;
  max-width: 280px;
  background: var(--bg-sidebar);
  border-right: 1px solid var(--border);
  display: flex;
  flex-direction: column;
  padding: 16px 14px;
  overflow: hidden;
  flex-shrink: 0;
  resize: horizontal;
}

/* === BRAND === */
.brand-area {
  margin-bottom: 12px;
  padding: 0 4px;
}
.brand {
  font-family: "JetBrains Mono", "Consolas", monospace;
  font-size: 18px;
  font-weight: 700;
  color: var(--green);
  letter-spacing: -0.5px;
}
.tagline {
  font-size: 11px;
  color: var(--text-dim);
  margin-top: 4px;
  font-family: "JetBrains Mono", monospace;
}

/* === PEPE HERO === */
.pepe-hero {
  position: relative;
  margin-bottom: 12px;
  border-radius: 14px;
  overflow: hidden;
  border: 1px solid var(--border);
  flex-shrink: 0;
}
.pepe-big {
  width: 100%;
  height: auto;
  display: block;
  opacity: 0.9;
}
.pepe-overlay {
  position: absolute;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  font-family: "JetBrains Mono", monospace;
  font-size: 22px;
  font-weight: 900;
  color: var(--green);
  text-shadow: 0 0 10px rgba(0,0,0,0.9), 0 0 30px var(--green-glow), 0 0 60px rgba(60,255,90,0.3);
  text-align: center;
  line-height: 1.2;
  letter-spacing: 3px;
  pointer-events: none;
}

/* === STATUS BLOCK === */
.status-block {
  background: var(--bg-card);
  border: 1px solid var(--border);
  border-radius: 12px;
  padding: 12px;
  margin-bottom: 12px;
  font-family: "JetBrains Mono", "Consolas", monospace;
  font-size: 12px;
  flex-shrink: 0;
}
.term-line {
  color: var(--text-gray);
  margin-bottom: 8px;
  font-size: 11px;
}
.term-output {
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.status-line {
  display: flex;
  align-items: center;
  gap: 6px;
}
.status-arrow {
  color: var(--green);
  font-size: 11px;
}
.status-label {
  font-weight: 700;
  font-size: 12px;
  transition: all var(--speed) var(--ease);
}
.status-text {
  color: var(--text-gray);
  font-size: 12px;
}
.uptime-line {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-top: 4px;
}
.uptime-label {
  color: var(--text-dim);
  font-size: 11px;
}
.uptime-val {
  color: var(--green);
  font-size: 11px;
}

/* === NAVIGATION === */
.nav {
  display: flex;
  flex-direction: column;
  gap: 2px;
  flex: 1;
  min-height: 0;
  overflow-y: auto;
}
.nav-item {
  height: 40px;
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 0 12px;
  border-radius: 10px;
  background: transparent;
  border: none;
  color: var(--text-gray);
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  transition: all var(--speed) var(--ease);
  text-align: left;
  font-family: "JetBrains Mono", monospace;
  letter-spacing: 0.5px;
  flex-shrink: 0;
}
.nav-item:hover {
  background: var(--bg-card-hover);
  color: var(--text-white);
}
.nav-item.active {
  background: rgba(60, 255, 90, 0.08);
  color: var(--green);
}
.nav-icon {
  font-size: 14px;
  width: 20px;
  text-align: center;
}

/* === MEME BOTTOM === */
.meme-bottom {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 4px;
  margin-top: auto;
  border-top: 1px solid var(--border);
  flex-shrink: 0;
}
.meme-shhh {
  width: 56px;
  height: 56px;
  opacity: 0.8;
  flex-shrink: 0;
}
.meme-text {
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.meme-line {
  font-size: 13px;
  color: var(--text-dim);
  font-weight: 700;
  letter-spacing: 1px;
  line-height: 1.2;
}
.meme-line.green {
  color: var(--green);
  font-size: 16px;
  font-weight: 800;
  letter-spacing: 2px;
}
.meme-sub {
  font-size: 12px;
  color: var(--text-dim);
  margin-top: 4px;
  font-style: italic;
}

/* === RESPONSIVE === */
@media (max-width: 1400px) {
  .sidebar {
    width: 200px;
    padding: 12px 10px;
  }
  .brand {
    font-size: 15px;
  }
  .pepe-overlay {
    font-size: 18px;
  }
  .nav-item {
    font-size: 11px;
    height: 34px;
  }
  .meme-shhh {
    width: 44px;
    height: 44px;
  }
  .meme-line {
    font-size: 11px;
  }
  .meme-line.green {
    font-size: 13px;
  }
}

@media (max-width: 1200px) {
  .sidebar {
    width: 60px;
    min-width: 60px;
    padding: 12px 8px;
  }
  .brand-area,
  .pepe-hero,
  .status-block,
  .meme-bottom {
    display: none;
  }
  .nav-item {
    justify-content: center;
    padding: 0;
  }
  .nav-label {
    display: none;
  }
  .nav-icon {
    font-size: 18px;
    width: auto;
  }
}

@media (max-width: 900px) {
  .sidebar {
    display: none;
  }
}
</style>
