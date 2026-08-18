# iOS — инструкция

## Приложение: sing-box или Karing

### Вариант 1: sing-box (App Store, бесплатно)
1. App Store → найти **sing-box** → установить
2. Скачать `sing-box-config.json`
3. Заменить `YOUR_VPS_IP`, `YOUR_UUID`, `YOUR_DOMAIN`
4. Открыть файл через sing-box (Files → выбрать JSON)
5. Разрешить VPN → Connect

### Вариант 2: Karing (App Store, бесплатно)
1. App Store → найти **Karing** → установить
2. Karing поддерживает импорт sing-box JSON конфигов
3. Тот же файл `sing-box-config.json`
4. Импорт → Connect

### Вариант 3: собственное iOS приложение (нужен Mac)
Требует macOS + Xcode + Apple Developer $99/год.
См. `snowden_ios/README_IOS.md` из основного проекта для исходного кода.

## Особенности iOS
- Нет per-app split-tunnel → только доменные правила
- Нет Telegram-репортера (используйте ПК-бота)
- Нет кнопки в шторке (используйте виджет sing-box)
- TUN работает через NEPacketTunnelProvider (если своё приложение)
- При использовании готового sing-box — всё работает из коробки
