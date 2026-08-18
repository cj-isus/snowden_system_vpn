<script lang="ts" setup>
import { ref, onMounted, inject } from "vue"
import { SetAutostart, IsAutostartEnabled, ExportConfig, ImportConfig } from "../../../wailsjs/go/main/App"
import pepeTopSecret from "../../assets/memes/pepe_top_secret_fedora.png"

// Wails runtime dialog functions (declared as any — typings don't include them)
declare const window: any

const showToast = inject<(text: string, type?: string) => void>("showToast")

const autostart = ref(false)

onMounted(async () => {
  try { autostart.value = await IsAutostartEnabled() } catch {}
})

async function toggleAutostart() {
  try {
    autostart.value = !autostart.value
    await SetAutostart(autostart.value)
    showToast?.(`Автозапуск ${autostart.value ? 'включён' : 'выключён'}`, autostart.value ? 'success' : 'info')
  } catch {
    autostart.value = !autostart.value
    showToast?.('Ошибка', 'error')
  }
}

async function doExport() {
  try {
    const path = await window.go.main.App.SaveFileDialog ?
      window.go.main.App.SaveFileDialog() :
      await window.runtime.SaveFileDialog({
        title: "Экспорт конфигурации",
        defaultFilename: "snowden-config.json",
        filters: [{ displayName: "JSON (*.json)", pattern: "*.json" }],
      })
    if (!path) return
    const written = await ExportConfig(String(path))
    showToast?.(`Конфиг сохранён: ${written}`, 'success')
  } catch (e: any) {
    showToast?.('Ошибка экспорта: ' + (e?.message || e), 'error')
  }
}

async function doImport() {
  try {
    const path = await window.runtime.OpenFileDialog({
      title: "Импорт конфигурации",
      filters: [{ displayName: "JSON (*.json)", pattern: "*.json" }],
    })
    if (!path) return
    const json = await ImportConfig(String(path))
    if (json) {
      showToast?.(`Конфиг импортирован (${json.length} байт). Запустите VPN заново.`, 'success')
    }
  } catch (e: any) {
    showToast?.('Ошибка импорта: ' + (e?.message || e), 'error')
  }
}

const settings = [
  { id: "autostart", label: "Запускать с Windows", model: autostart, action: toggleAutostart, enabled: true },
]
</script>

<template>
  <div class="card settings-card">
    <div class="card-title-row">
      <div>
        <span class="card-title">НАСТРОЙКИ</span>
        <span class="card-subtitle">// top secret</span>
      </div>
    </div>

    <div class="settings-body">
      <div class="settings-left">
        <div class="info-row">
          <span class="info-label">Прокси-порт:</span>
          <span class="info-val">20808</span>
        </div>
        <div class="info-row">
          <span class="info-label">Домен DNS:</span>
          <span class="info-val">1.1.1.1</span>
        </div>

        <div class="settings-list">
          <label
            v-for="s in settings"
            :key="s.id"
            class="setting-row"
            :class="{ disabled: !s.enabled }"
          >
            <div class="check-box" :class="{ checked: s.model.value }" @click.prevent="s.enabled && s.action?.()">
              <span v-if="s.model.value">✓</span>
            </div>
            <span class="setting-label">{{ s.label }}</span>
          </label>
        </div>

        <div class="cert-row">
          <span class="cert-label">Сертификат:</span>
          <span class="cert-val">
            <span class="cert-icon">🔒</span>
            Let's Encrypt
          </span>
        </div>
        <div class="version-row">
          <span class="version-label">Версия:</span>
          <span class="version-val">snowden.system</span>
        </div>
      </div>

      <div class="pepe-area">
        <img :src="pepeTopSecret" class="pepe-fedora" alt="top secret" />
        <div class="top-secret-badge">TOP<br/>SECRET</div>
      </div>
    </div>

    <div class="actions">
      <button class="action-btn" @click="doImport">
        <span>[↓]</span>
        ИМПОРТ КОНФИГА
      </button>
      <button class="action-btn" @click="doExport">
        <span>[↑]</span>
        ЭКСПОРТ КОНФИГА
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
.card-title-row {
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
  margin-left: 8px;
}
.settings-body {
  display: flex;
  gap: 12px;
  flex: 1;
}
.settings-left {
  flex: 1;
  min-width: 0;
}
.info-row {
  display: flex;
  justify-content: space-between;
  padding: 3px 0;
  font-size: 12px;
}
.info-label {
  color: var(--text-dim);
}
.info-val {
  color: var(--text-white);
  font-family: "JetBrains Mono", monospace;
  font-size: 11px;
}
.settings-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin: 10px 0;
  padding: 10px 0;
  border-top: 1px solid rgba(255, 255, 255, 0.03);
  border-bottom: 1px solid rgba(255, 255, 255, 0.03);
}
.setting-row {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 12px;
  color: var(--text-gray);
  cursor: pointer;
  transition: all var(--speed) var(--ease);
}
.setting-row:hover:not(.disabled) {
  color: var(--text-white);
}
.setting-row.disabled {
  opacity: 0.5;
  cursor: not-allowed;
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
  flex-shrink: 0;
  cursor: pointer;
  transition: all var(--speed) var(--ease);
}
.check-box.checked {
  background: rgba(60, 255, 90, 0.1);
  border-color: var(--green);
}
.setting-label {
  flex: 1;
  line-height: 1.3;
}
.cert-row, .version-row {
  display: flex;
  justify-content: space-between;
  padding: 3px 0;
  font-size: 11px;
}
.cert-label, .version-label {
  color: var(--text-dim);
}
.cert-val {
  color: var(--green);
  display: flex;
  align-items: center;
  gap: 4px;
}
.cert-icon {
  font-size: 10px;
}
.version-val {
  color: var(--text-gray);
  font-family: "JetBrains Mono", monospace;
  font-size: 10px;
}
.pepe-area {
  position: relative;
  width: 100px;
  flex-shrink: 0;
  display: flex;
  align-items: flex-end;
  justify-content: center;
}
.pepe-fedora {
  width: 90px;
  height: auto;
  opacity: 0.9;
}
.top-secret-badge {
  position: absolute;
  bottom: 20px;
  right: 0;
  background: rgba(60, 255, 90, 0.15);
  border: 1px solid var(--green);
  border-radius: 4px;
  padding: 4px 6px;
  font-size: 8px;
  font-weight: 700;
  color: var(--green);
  text-align: center;
  line-height: 1.2;
  letter-spacing: 1px;
  transform: rotate(-5deg);
}
.actions {
  display: flex;
  gap: 8px;
  margin-top: 10px;
}
.action-btn {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  padding: 8px 10px;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: transparent;
  color: var(--text-gray);
  font-size: 10px;
  font-weight: 600;
  cursor: pointer;
  transition: all var(--speed) var(--ease);
  font-family: "JetBrains Mono", monospace;
  letter-spacing: 0.5px;
}
.action-btn:hover {
  border-color: var(--green);
  color: var(--green);
}

/* Responsive */
@media (max-width: 1200px) {
  .settings-body {
    flex-direction: column;
  }
  .pepe-area {
    width: auto;
    height: 80px;
  }
  .pepe-fedora {
    width: 60px;
  }
}

@media (max-width: 900px) {
  .actions {
    flex-direction: column;
  }
}
</style>
