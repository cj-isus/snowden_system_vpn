<script lang="ts" setup>
const props = defineProps<{ connected: boolean; busy: boolean }>()
const emit = defineEmits<{ toggle: [] }>()
</script>

<template>
  <header class="topbar">
    <div class="left">
      <div class="status-indicator" :class="{ on: connected, connecting: busy && !connected }">
        <span class="conn-dot" />
        <span class="conn-text">{{ connected ? "СОЕДИНЕНИЕ АКТИВНО" : busy ? "ПОДКЛЮЧЕНИЕ…" : "ОТКЛЮЧЕНО" }}</span>
      </div>
      <span class="term-cmd">root@snowden:~# whoami → freedom</span>
    </div>
    <div class="right">
      <button
        class="power-btn"
        :class="{ on: connected, busy, connecting: busy && !connected }"
        :disabled="busy"
        @click="emit('toggle')"
        data-tooltip="Включить/выключить VPN"
      >
        <span class="btn-glow" :class="{ active: connected }" />
        <span class="btn-dot" :class="{ on: connected, pulse: busy }" />
        <span class="btn-text">{{ connected ? "ВЫКЛ" : "ВКЛ" }}</span>
      </button>
    </div>
  </header>
</template>

<style scoped>
.topbar {
  height: 56px;
  min-height: 56px;
  background: var(--bg-sidebar);
  border-bottom: 1px solid var(--border);
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 20px;
  position: relative;
  z-index: 10;
}
.left {
  display: flex;
  align-items: center;
  gap: 16px;
}
.status-indicator {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 6px 12px;
  border-radius: 8px;
  background: transparent;
  border: 1px solid transparent;
  transition: all var(--speed) var(--ease);
}
.status-indicator.on {
  background: rgba(60, 255, 90, 0.05);
  border-color: rgba(60, 255, 90, 0.15);
}
.status-indicator.connecting {
  background: rgba(255, 197, 51, 0.05);
  border-color: rgba(255, 197, 51, 0.15);
}
.conn-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--text-dim);
  transition: all var(--speed) var(--ease);
  flex-shrink: 0;
}
.status-indicator.on .conn-dot {
  background: var(--green-success);
  animation: pulseGlow 2s ease-in-out infinite;
}
.status-indicator.connecting .conn-dot {
  background: var(--yellow-warn);
  animation: pulseGlowYellow 1.5s ease-in-out infinite;
}
.conn-text {
  font-size: 12px;
  font-weight: 700;
  color: var(--text-gray);
  letter-spacing: 0.5px;
  transition: all var(--speed) var(--ease);
}
.status-indicator.on .conn-text {
  color: var(--green-success);
}
.status-indicator.connecting .conn-text {
  color: var(--yellow-warn);
}
.term-cmd {
  font-family: "JetBrains Mono", monospace;
  font-size: 11px;
  color: var(--text-dim);
  opacity: 0.7;
}
.right {
  display: flex;
  align-items: center;
  gap: 12px;
}
.power-btn {
  position: relative;
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 20px;
  border-radius: 10px;
  border: 1px solid var(--border);
  background: var(--bg-card);
  color: var(--text-gray);
  font-size: 13px;
  font-weight: 700;
  cursor: pointer;
  transition: all var(--speed) var(--ease);
  overflow: hidden;
  letter-spacing: 1px;
}
.power-btn:hover:not(:disabled) {
  border-color: var(--border-hover);
  transform: translateY(-1px);
  box-shadow: 0 4px 15px rgba(0,0,0,0.3);
}
.power-btn:active:not(:disabled) {
  transform: translateY(0) scale(0.98);
}
.power-btn.on {
  background: rgba(60, 255, 90, 0.08);
  border-color: var(--green);
  color: var(--green);
  box-shadow: 0 0 20px rgba(60, 255, 90, 0.1);
}
.power-btn.on:hover:not(:disabled) {
  box-shadow: 0 0 30px rgba(60, 255, 90, 0.2);
}
.power-btn.connecting {
  background: rgba(255, 197, 51, 0.08);
  border-color: var(--yellow-warn);
  color: var(--yellow-warn);
}
.power-btn:disabled {
  opacity: 0.7;
  cursor: wait;
}
.btn-glow {
  position: absolute;
  inset: 0;
  border-radius: 10px;
  opacity: 0;
  transition: opacity var(--speed) var(--ease);
  pointer-events: none;
}
.btn-glow.active {
  opacity: 1;
  box-shadow: inset 0 0 20px rgba(60, 255, 90, 0.1);
}
.btn-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--text-dim);
  transition: all var(--speed) var(--ease);
  flex-shrink: 0;
}
.btn-dot.on {
  background: var(--green-success);
  box-shadow: 0 0 8px var(--green-success);
}
.btn-dot.pulse {
  background: var(--yellow-warn);
  animation: pulseGlowYellow 1s ease-in-out infinite;
}
.btn-text {
  position: relative;
  z-index: 1;
}
</style>