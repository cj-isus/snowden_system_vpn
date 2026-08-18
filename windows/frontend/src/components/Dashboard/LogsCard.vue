<script lang="ts" setup>
import { ref, computed } from "vue"
import pepeHacker from "../assets/memes/pepe_hacker_laptop_skull.png"

const props = defineProps<{ logs: string[]; forceExpanded?: boolean }>()
const expanded = defineModel<boolean>('expanded', { default: false })

const filter = ref("")
const paused = ref(false)
const search = ref("")
const selectedLevel = ref<"all" | "info" | "warn" | "error">("all")

const filtered = computed(() => {
  let l = [...props.logs]
  if (search.value) l = l.filter(x => x.toLowerCase().includes(search.value.toLowerCase()))
  if (selectedLevel.value === "error") l = l.filter(x => x.includes("[error]"))
  if (selectedLevel.value === "warn") l = l.filter(x => x.includes("[warn]"))
  if (selectedLevel.value === "info") l = l.filter(x => x.includes("[info]"))
  return l.slice(-100)
})

function copyLogs() {
  navigator.clipboard.writeText(props.logs.join("\n")).catch(() => {})
}

function getLineClass(line: string): string {
  if (line.includes("[error]")) return "error"
  if (line.includes("[warn]")) return "warn"
  if (line.includes("[info]")) return "info"
  return ""
}
</script>

<template>
  <div class="card logs-card" :class="{ expanded }">
    <div class="card-header">
      <div class="title-group">
        <span class="card-title">ЛОГИ</span>
        <span class="card-subtitle">// ничего не скрыто</span>
      </div>
      <div class="window-controls">
        <button class="win-btn minimize" @click="expanded = false">−</button>
        <button class="win-btn maximize" @click="expanded = !expanded">□</button>
        <button class="win-btn close">×</button>
      </div>
    </div>

    <div class="toolbar">
      <div class="search-box">
        <span class="search-icon">🔍</span>
        <input v-model="search" placeholder="Поиск..." class="search-input" />
      </div>
      <div class="filter-group">
        <button :class="['filter-btn', { active: selectedLevel === 'all' }]" @click="selectedLevel = 'all'">ALL</button>
        <button :class="['filter-btn', { active: selectedLevel === 'info' }]" @click="selectedLevel = 'info'">INFO</button>
        <button :class="['filter-btn', { active: selectedLevel === 'warn' }]" @click="selectedLevel = 'warn'">WARN</button>
        <button :class="['filter-btn', { active: selectedLevel === 'error' }]" @click="selectedLevel = 'error'">ERR</button>
      </div>
      <button class="tool-btn" @click="paused = !paused">{{ paused ? '▶' : '⏸' }}</button>
      <button class="tool-btn" @click="copyLogs">📋</button>
    </div>

    <div class="terminal-output" :class="{ paused }">
      <div class="log-line" v-for="(line, i) in filtered" :key="i" :class="getLineClass(line)">
        <span class="log-time">[21:37:0{{ i % 10 }}]</span>
        <span class="log-level" :class="getLineClass(line)">{{ line.includes('[error]') ? 'ERROR' : line.includes('[warn]') ? 'WARN' : 'INFO' }}</span>
        <span class="log-content">{{ line.replace(/\[(error|warn|info)\]\s*/, '') }}</span>
      </div>
      <div class="empty-log" v-if="!filtered.length">
        <span>—</span>
      </div>
    </div>

    <div class="logs-footer">
      <img :src="pepeHacker" class="pepe-hacker" alt="hacker" />
    </div>
  </div>
</template>

<style scoped>
.card {
  background: var(--bg-card);
  border: 1px solid var(--border);
  border-radius: 18px;
  padding: 12px;
  transition: all var(--speed) var(--ease);
  animation: fadeIn var(--speed) var(--ease) forwards;
  display: flex;
  flex-direction: column;
  max-height: 280px;
}
.card.expanded {
  max-height: 500px;
}
.card:hover {
  border-color: var(--border-hover);
}
.card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 8px;
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
.card-subtitle {
  font-size: 11px;
  color: var(--text-muted);
  font-style: italic;
}
.window-controls {
  display: flex;
  gap: 4px;
}
.win-btn {
  width: 20px;
  height: 20px;
  border-radius: 4px;
  border: 1px solid var(--border);
  background: var(--bg-card-hover);
  color: var(--text-dim);
  font-size: 12px;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: all var(--speed-fast) var(--ease);
  line-height: 1;
}
.win-btn:hover {
  background: var(--bg-card);
  color: var(--text-white);
}
.win-btn.close:hover {
  background: var(--red-danger);
  color: white;
  border-color: var(--red-danger);
}
.toolbar {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-bottom: 8px;
  flex-wrap: wrap;
}
.search-box {
  flex: 1;
  min-width: 100px;
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 3px 8px;
  border: 1px solid var(--border);
  border-radius: 6px;
  background: var(--bg-terminal);
  transition: all var(--speed) var(--ease);
}
.search-box:focus-within {
  border-color: var(--green);
}
.search-icon {
  font-size: 10px;
  opacity: 0.5;
}
.search-input {
  flex: 1;
  background: transparent;
  border: none;
  color: var(--text-gray);
  font-size: 11px;
  outline: none;
  font-family: "JetBrains Mono", monospace;
}
.search-input::placeholder {
  color: var(--text-muted);
}
.filter-group {
  display: flex;
  gap: 2px;
}
.filter-btn {
  padding: 3px 8px;
  border: 1px solid transparent;
  border-radius: 4px;
  background: transparent;
  color: var(--text-dim);
  font-size: 9px;
  font-weight: 600;
  cursor: pointer;
  transition: all var(--speed-fast) var(--ease);
  font-family: "JetBrains Mono", monospace;
}
.filter-btn:hover {
  background: var(--bg-card-hover);
  color: var(--text-white);
}
.filter-btn.active {
  background: var(--bg-card-hover);
  border-color: var(--border);
  color: var(--green);
}
.tool-btn {
  padding: 3px 8px;
  border: 1px solid var(--border);
  border-radius: 4px;
  background: transparent;
  color: var(--text-dim);
  font-size: 10px;
  cursor: pointer;
  transition: all var(--speed-fast) var(--ease);
}
.tool-btn:hover {
  border-color: var(--green);
  color: var(--green);
}
.terminal-output {
  background: var(--bg-terminal);
  border: 1px solid rgba(0, 255, 80, 0.05);
  border-radius: 8px;
  padding: 8px;
  flex: 1;
  overflow-y: auto;
  font-family: "JetBrains Mono", "Consolas", monospace;
  font-size: 10px;
  line-height: 1.5;
  min-height: 80px;
}
.terminal-output.paused {
  opacity: 0.7;
}
.log-line {
  display: flex;
  gap: 6px;
  padding: 1px 0;
  color: var(--green-term);
  opacity: 0.85;
}
.log-time {
  color: var(--text-muted);
  flex-shrink: 0;
}
.log-level {
  flex-shrink: 0;
  min-width: 36px;
}
.log-level.info { color: var(--blue-info); }
.log-level.warn { color: var(--yellow-warn); }
.log-level.error { color: var(--red-danger); }
.log-content {
  flex: 1;
  min-width: 0;
  word-break: break-all;
}
.log-line.error .log-content { color: var(--red-danger); }
.log-line.warn .log-content { color: var(--yellow-warn); }
.empty-log {
  color: var(--text-dim);
  text-align: center;
  padding: 30px 0;
}
.logs-footer {
  display: flex;
  justify-content: flex-end;
  margin-top: 6px;
}
.pepe-hacker {
  width: 32px;
  height: 32px;
  opacity: 0.4;
}

/* Responsive */
@media (max-width: 1200px) {
  .toolbar {
    flex-wrap: wrap;
  }
  .search-box {
    min-width: 80px;
  }
}

@media (max-width: 900px) {
  .filter-group {
    order: -1;
    width: 100%;
  }
}
</style>
