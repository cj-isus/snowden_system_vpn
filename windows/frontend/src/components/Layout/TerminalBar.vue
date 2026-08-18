<script lang="ts" setup>
import { ref, onMounted, onUnmounted, watch, nextTick } from "vue"
import MatrixRain from "./ui/MatrixRain.vue"

const props = defineProps<{
  logs: string[]
  state: string
  connected: boolean
}>()

const displayLines = ref<string[]>([])
const scrollContainer = ref<HTMLElement | null>(null)
const paused = ref(false)
const maxLines = 8
const copied = ref(false)

// Keep the last N log lines for the marquee display.
watch(() => props.logs, async () => {
  if (paused.value) return
  const recent = props.logs.slice(-maxLines)
  displayLines.value = recent
  await nextTick()
  if (scrollContainer.value) {
    scrollContainer.value.scrollTop = scrollContainer.value.scrollHeight
  }
}, { deep: true })

// Seed with current logs immediately
onMounted(() => {
  displayLines.value = props.logs.slice(-maxLines)
})

function togglePause() {
  paused.value = !paused.value
}

function levelClass(line: string): string {
  const l = line.toLowerCase()
  if (l.includes("[error]") || l.includes("error") || l.includes("fail")) return "lvl-err"
  if (l.includes("[warn]") || l.includes("warn")) return "lvl-warn"
  if (l.includes("[info]") || l.includes("started") || l.includes("connected")) return "lvl-info"
  if (l.includes("[debug]") || l.includes("[trace]")) return "lvl-dbg"
  return "lvl-normal"
}

function formatTime(): string {
  return new Date().toLocaleTimeString("ru-RU", { hour: "2-digit", minute: "2-digit", second: "2-digit" })
}

// Copy all visible logs to clipboard
async function copyAll() {
  try {
    const text = displayLines.value.join("\n")
    await navigator.clipboard.writeText(text)
    copied.value = true
    setTimeout(() => { copied.value = false }, 1500)
  } catch {
    // Fallback: select the text in the terminal body so the user can Ctrl+C
    const body = scrollContainer.value
    if (body) {
      const range = document.createRange()
      range.selectNodeContents(body)
      const sel = window.getSelection()
      sel?.removeAllRanges()
      sel?.addRange(range)
    }
  }
}

// Enable text selection without interfering with the scroll/pause buttons.
// The terminal body has `user-select: text` so mouse drag selects log lines,
// then Ctrl+C copies. We stop auto-scroll while the user is selecting so the
// selection doesn't jump.
function onSelectionStart() {
  paused.value = true
}
</script>

<template>
  <div class="terminal-bar" :class="{ paused }">
    <MatrixRain />
    <div class="terminal-content">
      <div class="terminal-header">
        <div class="header-left">
          <span class="dot red" />
          <span class="dot yellow" />
          <span class="dot green" />
          <span class="term-title">snowden@system ~ live logs</span>
        </div>
        <div class="header-right">
          <span class="term-count">{{ displayLines.length }} lines</span>
          <button class="copy-btn" @click="copyAll" :title="copied ? 'Скопировано!' : 'Копировать все'">
            {{ copied ? "✓" : "⧉" }}
          </button>
          <button class="pause-btn" @click="togglePause" :title="paused ? 'Продолжить' : 'Пауза'">
            {{ paused ? "▶" : "⏸" }}
          </button>
        </div>
      </div>
      <div
        class="terminal-body"
        ref="scrollContainer"
        @mousedown="onSelectionStart"
      >
        <div v-if="displayLines.length === 0" class="term-empty">
          <span class="t-prompt">$</span>
          <span class="t-wait">ожидание логов…</span>
          <span class="cursor blink">█</span>
        </div>
        <div v-for="(l, i) in displayLines" :key="i" class="term-line" :class="levelClass(l)">
          <span class="t-time">{{ formatTime() }}</span>
          <span class="t-text">{{ l }}</span>
        </div>
        <div class="term-cursor-line" v-if="!paused && connected">
          <span class="t-prompt">$</span>
          <span class="cursor blink">█</span>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.terminal-bar {
  height: 120px;
  min-height: 120px;
  background: var(--bg-terminal);
  border-top: 1px solid rgba(0, 255, 80, 0.08);
  position: relative;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}
.terminal-bar.paused {
  border-top-color: rgba(255, 197, 51, 0.2);
}
.terminal-content {
  position: relative;
  z-index: 1;
  flex: 1;
  display: flex;
  flex-direction: column;
  padding: 6px 16px 4px;
  font-family: "JetBrains Mono", "Consolas", monospace;
  font-size: 11px;
  line-height: 1.5;
}
.terminal-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding-bottom: 4px;
  margin-bottom: 2px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.03);
  flex-shrink: 0;
}
.header-left {
  display: flex;
  align-items: center;
  gap: 6px;
}
.header-right {
  display: flex;
  align-items: center;
  gap: 10px;
}
.dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  display: inline-block;
}
.dot.red { background: #ff5f56; }
.dot.yellow { background: #ffbd2e; }
.dot.green { background: #27c93f; }
.term-title {
  color: var(--text-dim);
  font-size: 10px;
  margin-left: 6px;
  letter-spacing: 0.5px;
}
.term-count {
  color: var(--text-dim);
  font-size: 10px;
  opacity: 0.6;
}
.pause-btn {
  background: transparent;
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 4px;
  color: var(--text-dim);
  font-size: 10px;
  cursor: pointer;
  padding: 1px 6px;
  transition: all var(--speed) var(--ease);
}
.pause-btn:hover {
  border-color: var(--green);
  color: var(--green);
}
.copy-btn {
  background: transparent;
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 4px;
  color: var(--text-dim);
  font-size: 10px;
  cursor: pointer;
  padding: 1px 6px;
  transition: all var(--speed) var(--ease);
}
.copy-btn:hover {
  border-color: var(--green);
  color: var(--green);
}
.terminal-body {
  flex: 1;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 0;
  scrollbar-width: thin;
  scrollbar-color: rgba(60, 255, 90, 0.15) transparent;
  /* Allow text selection so the user can highlight lines with the mouse and
     Ctrl+C to copy. Auto-scroll pauses on mousedown (see onSelectionStart). */
  user-select: text;
  -webkit-user-select: text;
  cursor: text;
}
.terminal-body::-webkit-scrollbar {
  width: 4px;
}
.terminal-body::-webkit-scrollbar-thumb {
  background: rgba(60, 255, 90, 0.15);
  border-radius: 2px;
}
.term-line {
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  display: flex;
  gap: 8px;
  align-items: baseline;
  opacity: 0.9;
  animation: slideIn 0.2s ease-out;
}
.t-time {
  color: var(--text-dim);
  font-size: 10px;
  opacity: 0.5;
  flex-shrink: 0;
}
.t-text {
  color: var(--green-term);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.lvl-err .t-text { color: var(--red-danger); }
.lvl-warn .t-text { color: var(--yellow-warn); }
.lvl-info .t-text { color: var(--green-term); }
.lvl-dbg .t-text { color: var(--text-dim); opacity: 0.6; }

.term-empty {
  display: flex;
  gap: 6px;
  align-items: center;
  color: var(--text-dim);
}
.t-wait {
  color: var(--text-dim);
  font-style: italic;
}
.t-prompt {
  color: var(--green);
  font-weight: 700;
}
.term-cursor-line {
  display: flex;
  gap: 4px;
  align-items: center;
}
.cursor {
  color: var(--green);
  font-weight: 700;
}
.cursor.blink {
  animation: blink 1s step-end infinite;
}

@keyframes slideIn {
  from { opacity: 0; transform: translateX(-4px); }
  to { opacity: 0.9; transform: translateX(0); }
}
@keyframes blink {
  0%, 50% { opacity: 1; }
  51%, 100% { opacity: 0; }
}
</style>
