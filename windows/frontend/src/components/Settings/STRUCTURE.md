# STRUCTURE.md — components/Settings

> Настройки приложения: автозапуск, экспорт/импорт конфига.

## Файлы
| Файл | Роль | Wails-вызовы |
|------|------|--------------|
| `SettingsCard.vue` | Один экран настроек | `SetAutostart`, `IsAutostartEnabled`, `ExportConfig`, `ImportConfig` |

## Поведение
- **Автозапуск**: при mount `IsAutostartEnabled()`; переключатель → `SetAutostart(bool)`,
  оптимистичное обновление + `showToast`, откат при ошибке.
- **Экспорт конфига**: вызывает диалог сохранения (`window.go.main.App.SaveFileDialog`
  или `window.runtime.SaveFileDialog`), затем `ExportConfig(path)` → `showToast`.
- **Импорт конфига**: диалог открытия → `ImportConfig(path)`.

## Нюансы
- Wails-диалоги объявлены как `declare const window: any` (типов нет).
- Использует `inject("showToast")`; мем-ассет `pepe_top_secret_fedora.png` как декор.

## Связи
`App.vue` рендерит карточку; прямые вызовы Go через `../../../wailsjs/go/main/App`.