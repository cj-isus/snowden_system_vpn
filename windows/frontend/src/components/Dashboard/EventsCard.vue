<script lang="ts" setup>
interface Event {
  time: string
  icon: string
  title: string
  desc: string
  type?: string
}

const props = defineProps<{ events: Event[] }>()

const eventColors: Record<string, string> = {
  success: "var(--green-success)",
  error: "var(--red-danger)",
  warn: "var(--yellow-warn)",
  info: "var(--blue-info)"
}

const displayEvents = computed(() => {
  return props.events.slice(0, 8)
})

function getEventColor(type?: string): string {
  return eventColors[type || "info"] || "var(--text-dim)"
}
</script>

<script lang="ts">
import { computed } from "vue"
</script>

<template>
  <div class="card events-card">
    <div class="card-header">
      <span class="card-title">СОБЫТИЯ</span>
      <span class="expand-icon">▼</span>
    </div>

    <div class="timeline">
      <div
        v-for="(e, i) in displayEvents"
        :key="i"
        class="event"
        :style="{ animationDelay: `${i * 40}ms` }"
      >
        <span class="ev-time">{{ e.time }}</span>
        <span class="ev-dot" :style="{ color: getEventColor(e.type) }">●</span>
        <span class="ev-title">{{ e.title }}</span>
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
  max-height: 280px;
}
.card:hover {
  border-color: var(--border-hover);
}
.card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 10px;
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
  cursor: pointer;
}
.timeline {
  display: flex;
  flex-direction: column;
  gap: 4px;
  overflow-y: auto;
  flex: 1;
}
.event {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 3px 0;
  transition: all var(--speed) var(--ease);
  animation: slideUp var(--speed) var(--ease) forwards;
  opacity: 0;
}
.event:hover {
  background: var(--bg-card-hover);
  border-radius: 4px;
  padding: 3px 6px;
  margin: 0 -6px;
}
.ev-time {
  font-size: 10px;
  color: var(--text-dim);
  font-family: "JetBrains Mono", monospace;
  flex-shrink: 0;
  width: 36px;
}
.ev-dot {
  font-size: 8px;
  flex-shrink: 0;
  transition: transform var(--speed) var(--ease);
}
.event:hover .ev-dot {
  transform: scale(1.5);
}
.ev-title {
  font-size: 11px;
  color: var(--text-gray);
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  transition: color var(--speed) var(--ease);
}
.event:hover .ev-title {
  color: var(--text-white);
}
</style>
