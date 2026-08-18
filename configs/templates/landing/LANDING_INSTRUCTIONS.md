# Лендинг — инструкция

## Cloudflare Pages (бесплатно)

### 1. Установить wrangler
```bash
npm install -g wrangler
wrangler login
```

### 2. Создать проект
```bash
mkdir my-landing
cp index.html my-landing/
cd my-landing
```

### 3. Заполнить шаблон
В `index.html` заменить:
```javascript
const VPS = "ТВОЙ_VPS_IP:8090";
const VERSION = "1.0";
```

### 4. Деплой
```bash
wrangler pages deploy . --project-name my-vpn
```

### 5. Результат
Получишь URL: `https://my-vpn.pages.dev`

### 6. Обновление
Заменить файлы → снова `wrangler pages deploy`

## Что на лендинге
- 5 карточек скачивания (ПК, Android APK, iOS конфиг, sing-box конфиг, Amnezia)
- Раскрывающаяся документация (ПК, Android, iOS, Amnezia)
- Тёмная cyberpunk тема
- Авто-подстановка ссылок из VPS:8090

## version.json (для автообновления)
```json
{
  "version": "1.0",
  "versionCode": 100,
  "pc_url": "http://IP:8090/snowden-portable.zip",
  "android_url": "http://IP:8090/snowden-android.apk",
  "changelog": "Initial release"
}
```
Приложения могут проверять этот файл для автообновления.
