# План изменений snowden.system v2 — развёрнутая версия

> Дорожная карта «что / как / почему». Ссылки `файл:строка` проверены по реальному
> коду. Делится на три части:
> - **A. Правильность ядра** — чиним то, что сломано внутри (база, без неё строить дальше нельзя).
> - **B. Адаптивность по теории** — закрываем архитектурные пробелы (`ОБХОД_АДАПТИВНАЯ_ТЕОРИЯ.md`).
> - **C. Порядок, контрольные точки, что НЕ делать, риски**.
>
> Для каждого пункта: **причина → текущее состояние (код) → почему это проблема →
> решение (дизайн + код-эскиз) → граничные случаи → тесты → критерий приёмки.**

Ориентиры из теории, применяемые ниже:
- Адаптивность = петля «**детекти → выбери → примени → проверь → запомни**». Сейчас есть все звенья, кроме **«запомни»** — главный архитектурный пробел.
- «Один конфиг ≠ одна страна/канал»: «не работает» часто = конкретный эндпоинт/узел (DME, torn-down), а не протокол. Нужен **транспортный пул**.
- ТСПУ судит о соединении **по первому пакету** → ротация I1 (AmneziaWG) — дешёвый рычаг, который у нас не задействован.

---

# ЧАСТЬ A. Правильность ядра

## A1. Circuit breaker не подключен (мёртвые ShouldProbe / EnterHalfOpen)

### Причина
Машина состояний `Closed → Open → HalfOpen → Closed` спроектирована, но работает лишь частично: вспомогательные методы никогда не вызываются, поэтому cooldown/backoff не влияют на поведение.

### Текущее состояние (код)
- `circuitBreaker` (`adaptive.go:33-55`): `failThreshold=3`, `halfOpenProbes=2`, `cooldownStart=10s`, `cooldownMax=60s`, `currentCooldown=10s`.
- `ShouldProbe()` (`adaptive.go:99`) и `EnterHalfOpen()` (`adaptive.go:109`) — только определения, **grep по проекту даёт только их объявления**.
- `transition()` (`adaptive.go:137-154`): удваивает `currentCooldown` на каждый Open, сбрасывает на Closed — **но никто его не читает**.
- `loop()` (`adaptive.go:240-298`): фиксированный `ticker` 30 с → `deepProbe` → при ошибке `RecordFailure`; при успехе `onProbeSuccess`.
- Как следствие, восстановление идёт «самотёком»: на 30-с тике `deepProbe` успешен → `RecordSuccess` (`adaptive.go:58`) → в Open переходит в HalfOpen с `consecutiveOK=1` → второй успех → Closed. Задуманный путь Open→(cooldown)→HalfOpen не используется.

### Почему это проблема
1. Cooldown не выполняет роль «дать мёртвому каналу отдохнуть»: после Open система тут же начнёт долбить его каждые 30 c.
2. Интервал зондирования **не адаптируется к состоянию**: в HalfOpen нужна частая проверка, в Closed — редкая, в Open — ждать cooldown. Сейчас везде 30 c.
3. `halfOpenProbes=2` при интервале 30 c означает до минуты на восстановление после первого успеха противоречит заявленному UX.

### Решение (дизайн + код-эскиз)
1. Добавить геттер:
```go
// CurrentCooldown возвращает текущий backoff (для настройки тикера).
func (cb *circuitBreaker) CurrentCooldown() time.Duration {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.currentCooldown
}
```
2. Переписать `loop()` (`adaptive.go:266-297`): управлять интервалом по состоянию.
```go
const (
	healthyInterval = 30 * time.Second // Closed: редкая проверка
	probeInterval   = 3 * time.Second  // HalfOpen: агрессивное подтверждение
	probeTimeout    = 10 * time.Second
)
ticker := time.NewTicker(healthyInterval)
defer ticker.Stop()

for {
	select {
	case <-ae.ctx.Done():
		return
	case <-ticker.C:
	}
	if !ae.engine.Running() {
		continue
	}

	// Интервал тикера — по состоянию circuit breaker.
	switch ae.cb.State() {
	case StateOpen:
		if !ae.cb.ShouldProbe() {
			continue // ждём истечения cooldown
		}
		ae.cb.EnterHalfOpen()
		ae.emit("engine:diag", "[diag] ⏱ cooldown elapsed — HalfOpen, probing")
		ticker.Reset(probeInterval)
	case StateHalfOpen:
		ticker.Reset(probeInterval) // часто, пока подтверждаем
	default: // StateClosed
		ticker.Reset(healthyInterval)
	}

	err := ae.deepProbe(probeTimeout)
	if err == nil {
		ae.onProbeSuccess()
		continue
	}

	cat := ClassifyProbeError(err)
	ae.emit("engine:diag", fmt.Sprintf("[diag] probe failed: %s (%s)", cat.String(), err))

	if tripped := ae.cb.RecordFailure(); tripped {
		ae.onCircuitOpen(cat)
		ticker.Reset(ae.cb.CurrentCooldown()) // не долбим мёртвый канал
	}
}
```
3. `onCircuitOpen` (`adaptive.go:315`) не трогаем: у него уже есть reload + быстрая проверка и ветка `CatNetworkDown` без reload — это остаётся в силе.

### Граничные случаи
- **Open при единственном успехе**: `RecordSuccess` в Open переводит в HalfOpen (`adaptive.go:70-72`) — после нашего тикера это станет корректным «тестовым» пробингом, а не случайным.
- **Cooldown растёт без верхней границы по времени Open**: `currentCooldown` упирается в `cooldownMax=60s` (`adaptive.go:146-149`) — ок; при возврате в Closed сбрасывается (`:152`).
- **Канал реально мёртв**: на каждом Open cooldown удваивается → интервал между попытками растёт 10→20→40→60 c, не флудим.

### Тесты (новые `core/adaptive_test.go`)
- `RecordFailure` триппит Open ровно на `failThreshold`.
- `ShouldProbe` false до истечения cooldown, true после; cooldown удваивается на каждый Open и сбрасывается на Closed.
- `transition` Open→HalfOpen→Closed фиксирует `consecutiveOK`.
- Интеграционный: замокать `deepProbe` (интерфейс) → убедиться, что тикер переключается на `probeInterval` в HalfOpen.

### Критерий приёмки
`go test ./backend/core/` (тесты circuitBreaker) зелёный; логи показывают `cooldown elapsed — HalfOpen, probing` и тикер 3 c в восстановлении; `currentCooldown` реально растёт/сбрасывается.

---

## A2. Утечка горутин в metricsLoop при каждом Reload

### Причина
`go m.metricsLoop()` запускается без управления жизненным циклом; новый цикл на каждый Reload, старые живут до остановки движка.

### Текущее состояние (код)
- `StartVPN` (`manager.go:74`): `go m.metricsLoop()`.
- `ReloadVPN` (`manager.go:118`): `go m.metricsLoop()`.
- `metricsLoop` (`manager.go:79`): `for m.engine.Running() { m.metrics.sample(); m.PollConnections(); time.Sleep(time.Second) }` — без канала отмены и без `WaitGroup`.

### Почему это проблема
- Накапливаются горутины на каждой смене сервера/правила.
- Дублируются `sample()` (искажение скорости) и `PollConnections()` (дубли записей в `domainStats`) — статистика врёт.

### Решение (design + код-эскиз)
Вариант с одним управляемым циклом (предпочтительнее: точное число активных опросов):
```go
// поля Manager:
metricsStop chan struct{}
metricsWG   sync.WaitGroup

// в NewManager:
metricsStop: make(chan struct{}),

// startMetrics — идемпотентно: стоп старого, ровно одна горутина.
func (m *Manager) startMetrics() {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Останавливаем предыдущий цикл, если есть (после Reload).
	select {
	case <-m.metricsStop:
		// уже закрыт/завершён
	default:
		close(m.metricsStop)
	}
	m.metricsStop = make(chan struct{})
	stop := m.metricsStop
	m.metricsWG.Add(1)
	go func() {
		defer m.metricsWG.Done()
		for {
			select {
			case <-stop:
				return
			case <-time.After(time.Second):
				if m.engine.Running() {
					m.metrics.sample()
					m.PollConnections()
				}
			}
		}
	}()
}

func (m *Manager) stopMetrics() {
	m.mu.Lock()
	if m.metricsStop != nil {
		close(m.metricsStop)
		m.metricsStop = nil
	}
	m.mu.Unlock()
	m.metricsWG.Wait()
}
```
Точки вызова:
- `StartVPN` (`manager.go:74`) → `m.metrics.Start(); m.startMetrics()`.
- `ReloadVPN` (`manager.go:118`) → `m.metrics.Start(); m.startMetrics()`.
- `StopVPN` (`manager.go:88`) → `m.stopMetrics()` перед `engine.Close()`.

### Граничные случаи / гонки
- **Двойной вызов `startMetrics`**: закрытие уже закрытого канала — защищено `select/default` (см. выше); закрытие в `stopMetrics` — только под `m.mu`.
- **Вызов `stopMetrics` при никогда не стартованных метриках**: `metricsStop==nil` → идемпотентно.
- **Гонка `close`/`start`**: все обращения к `metricsStop` — под `m.mu`, кроме `stop` в горутине (канал читается только на `<-stop`, не закрывается там).

### Тесты (новые `core/manager_test.go`)
- N подряд `ReloadVPN` при живом движке → число активных metrics-горутин = 1.
- `StopVPN` → `metricsWG.Wait()` завершается без зависания.
- Одновременные `startMetrics`+`stopMetrics` под `-race` не паникуют.

### Критерий приёмки
`go test -race ./backend/core/` зелёный; при 50 `ReloadVPN` нет роста горутин (проверяется счётчиком-двойником `metricsWG`).

---

## A3. Дублирование Start/Reload в engine.go (разный уровень защиты)

### Причина
`Reload` повторяет тело `Start`, но **без** panic-recover и **без** таймаута старта.

### Текущее состояние (код)
- `Start` (`engine.go:158-238`): проверка состояния → `StateStarting` → panic-recover (`:169-174`) → удаление `cache.db` (`:179-187`) → декод → `box.New` → старт с таймаутом 15 c (`:214-231`).
- `Reload` (`engine.go:288-344`): teardown → `StateStarting` → декод → `box.New` → `instance.Start()` **без** panic-recover и **без** таймаута (`:333`).

### Почему это проблема
На битом конфиге `Reload` может: зависнуть на `cache.db` (инициализации) без таймаута, либо упасть в панику без перехода `StateError` → движок остаётся в «вечном `StateStarting`». Расхождение уже реально (см. код выше).

### Решение (design + код-эскиз)
Вынести общий путь в `startLocked` (вызывается только при удержанном `e.mu`):
```go
// startLocked — общий путь Start и Reload (caller держит e.mu).
func (e *Engine) startLocked(configJSON []byte) (errRet error) {
	defer func() {
		if r := recover(); r != nil {
			e.failLocked(fmt.Errorf("sing-box panic: %v", r))
			errRet = fmt.Errorf("sing-box panic: %v", r)
		}
	}()
	// удаление залипшего cache.db
	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exePath)
		for _, f := range []string{"cache.db", "cache.db-journal"} {
			os.Remove(filepath.Join(exeDir, f))
		}
	}
	registryCtx := boxContext(context.Background())
	options, err := json.UnmarshalExtendedContext[option.Options](registryCtx, configJSON)
	if err != nil {
		e.failLocked(fmt.Errorf("decode config: %w", E.Cause(err)))
		return err
	}
	ctx, cancel := context.WithCancel(registryCtx)
	instance, err := box.New(box.Options{
		Context:           ctx,
		Options:           options,
		PlatformLogWriter: e.platformWriter(),
	})
	if err != nil {
		cancel()
		e.failLocked(fmt.Errorf("create sing-box: %w", E.Cause(err)))
		return err
	}
	startDone := make(chan error, 1)
	go func() { startDone <- instance.Start() }()
	select {
	case err := <-startDone:
		if err != nil {
			cancel()
			e.failLocked(fmt.Errorf("start sing-box: %w", E.Cause(err)))
			return err
		}
	case <-time.After(15 * time.Second):
		cancel()
		e.failLocked(fmt.Errorf("start sing-box: timeout (15s) — cache.db hang"))
		return fmt.Errorf("sing-box start timeout")
	}
	e.currentBox = instance
	e.currentCtx = ctx
	e.currentCancel = cancel
	e.setState(StateRunning)
	return nil
}
```
Тонкие обёртки:
```go
func (e *Engine) Start(configJSON []byte) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.State() != StateStopped && e.State() != StateError {
		return ErrAlreadyRunning
	}
	e.setState(StateStarting)
	e.done = make(chan struct{})
	return e.startLocked(configJSON)
}

func (e *Engine) Reload(configJSON []byte) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.State() == StateRunning || e.State() == StateStarting {
		e.setState(StateStopping)
		if e.currentCancel != nil {
			e.currentCancel()
		}
		if e.currentBox != nil {
			_ = e.currentBox.Close()
		}
		e.currentBox = nil
		e.currentCancel = nil
		e.currentCtx = nil
	}
	e.setState(StateStarting)
	e.done = make(chan struct{})
	return e.startLocked(configJSON)
}
```

### Граничные случаи / гонки
- **Состояние не публикуется как `Stopped` в середине Reload**: мьютекс держится на весь своп — наблюдатель всегда видит `Running` или `Starting`.
- **Закрытие `done`**: `failLocked` закрывает канал один раз (защищено `select/default`, `engine.go:363-368`).
- **Двойной `StopVPN` одновременно с `Reload`**: оба берут `e.mu` — сериализуются.

### Тесты
- Юнит: `Reload` с заведомо битым JSON → возвращает ошибку, `State()==StateError` (не зависание).
- Юнит (замокать `box.New` через интерфейс или фабрику): `Reload` в панике → `StateError`.
- `go test -race ./backend/core/`.

### Критерий приёмки
`go build ./...`; `Reload` на битом конфиге не зависит и не оставляет `StateStarting`.

---

## A4. Мёртвый код: injectSplitTunnel / BuildConfig / EnsureCIDRFile

### Причина
RU-CIDR-сплит выпилили из живого пути ради производительности, но функции не удалили; документация описывает несуществующий конфиг-билдер.

### Текущее состояние (код)
- `injectSplitTunnel` (`app.go:606-660`) — **нигде не вызывается**.
- `config.EnsureCIDRFile` (`builder.go:104`) — вызывается только из `injectSplitTunnel` (`app.go:622`) → недостижим.
- `config.BuildConfig` (`builder.go:27`) — вне живого пути вообще (реальный путь — `LoadConfigFile` `app.go:576`, читает шаблон).
- `LoadConfigFile` (`app.go:600-603`): комментарий подтверждает, что CIDR (11 401 правило) убран за производительность.

### Почему это проблема
Дрейф между документацией и кодом. `STRUCTURE.md` подаёт «конфиг-билдер» как активный компонент. Читающий думает, что есть билдер и CIDR-сплит, а их нет.

### Решение (два варианта — выбрать по целям)
**Вариант A (рекомендуемый сейчас):** удалить мёртвый код:
- `injectSplitTunnel` целиком из `app.go`;
- в `builder.go`: `BuildConfig`, `EnsureCIDRFile`, `splitCSV`, `trim` (если больше не нужны);
- поправить `STRUCTURE.md`: конфиг-билдер → «legacy / не в живом пути»; реальный путь — `app.go`/`LoadConfigFile` + `configs/`→`assets/configs/`.

**Вариант B (если CIDR вернуть осознанно):** не раскидывать 11 тыс. правил по `route.rules`, а собрать в **один rule-set** (`type: local`, как `EnsureCIDRFile`) и вернуть вызов под флагом:
```go
// LoadConfigFile: если config.SplitTunnelCIDR (=env/флаг true)
raw, err = injectSplitTunnel(raw, ...) // один rule-set, не плоский список
```
Поведение воспроизводимо и отключаемо.

### Граничные случаи
- Удаление `package config` целиком нельзя: `cfclient` и др. не зависят, но проверь `go vet` на неиспользуемые импорты в `builder.go` после удаления.
- Если `EnsureCIDRFile` вдруг используется тестами `enginetest/e2e.go` — сначала перенести нужную логику.

### Тесты / проверка
- `grep -rn "injectSplitTunnel\|BuildConfig\|EnsureCIDRFile"` по проекту → только в `STRUCTURE.md`, если оставили там упоминание.
- `go build ./...`.

### Критерий приёмки
Зелёная сборка; документация соответствует коду.

---

## A5. Жёсткая завязка на имена тегов outbound

### Причина
Имена каналов захардкожены в Go, хотя конфиг — «точка истины».

### Текущее состояние (код)
`SelectServer` (`app.go:370-409`):
```go
case "auto": finalOutbound = "auto"
case "nl":    finalOutbound = "grpc-nl"  // hardcoded
case "fr":    finalOutbound = "grpc-fr"  // hardcoded
```
и `ReloadVPN` на `manager.ReloadVPN` (`app.go:408`).

### Почему это проблема
Переименуешь тег в `template-vps-reality.json` → `SelectServer` молча направит трафик на несуществующий outbound (уйдёт «в никуда»), без ошибок до факта поломки. Система заявляет: «конфиг — источник истины» — а код её нарушает.

### Решение (design + код-эскиз)
Резолвить тег по фактическому составу `outbounds`:
```go
// resolveOutboundTag находит тег, чьё имя содержит код страны.
func resolveOutboundTag(server string, cfg []byte) (string, error) {
	if server == "auto" {
		return "auto", nil // у нас url-test всегда tag "auto"
	}
	code := strings.ToLower(server)
	var out []struct{ Tag string `json:"tag"` }
	if err := json.Unmarshal(extractOutbounds(cfg), &out); err != nil {
		return "", err
	}
	for _, o := range out {
		if strings.Contains(strings.ToLower(o.Tag), code) {
			return o.Tag, nil
		}
	}
	return "", fmt.Errorf("no outbound matching %q", server)
}
```
Применение: `SelectServer` получает `finalOutbound, err := resolveOutboundTag(server, cfg)`; на ошибку — возвращать её, а не тихо `auto`.
Дополнительно вынести карту `страна → {тег-фильтр, label}` в конфиг (например в `server-params.json`), чтобы добавление страны не требовало перекомпиляции.

### Граничные случаи
- `auto` всегда существует (url-test `tag:"auto"`). Для надёжности — ищем url-test по `type=="urltest"` при отсутствии `tag=="auto"`.
- Неоднозначность (`nl` в `grpc-nl` и `vless-nl`): брать первый совпавший или отдавать приоритет выбранной группе (например сначала искать ровное совпадение `tag`, потом `contains`).

### Тесты (в `core` или `app` уровня)
- `resolveOutboundTag("nl", cfg)` при переименованных тегах (`vless-ne` вместо `vless-nl`) всё равно находит (`vless-nl` по `contains "nl"`).
- `resolveOutboundTag("xx", cfg)` → ошибка (а не тихий `auto`).

### Критерий приёмки `go build ./...`; ручной тест `SelectServer("nl")` с изменённым тегом не теряет трафик.

---

## A6. Медленное детектирование обрыва (3 × 30 c)

### Причина
`failThreshold=3` × `checkInterval=30s` → до 90+ c на срабатывание. `onCircuitOpen` при этом синхронно в горутине цикла делает reload (до 15 c) + sleep + профайл.

### Текущее состояние (код)
- `loop()` (`adaptive.go:259-260`): `checkInterval = 30 * time.Second`, `probeTimeout = 10 * time.Second`.
- `onCircuitOpen` (`adaptive.go:315-350`): вызывается **внутри** `loop()`; `engine.Reload` + `time.Sleep(2s)` + `deepProbe(8s)`.

### Почему это проблема
Реальный обрыв ловится за минуту-полторы — против заявленного «нажал кнопку — работает». Пока `onCircuitOpen` работает, тикер не тикает → мониторинг встаёт. Одиночный блип (потеря пакета) при этом не должен ронять туннель.

### Решение (design + код-эскиз)
Двухфазная детекция в `loop()` (в комбинации с A1):
```go
const (
	failConfirmInterval = 3 * time.Second
	failConfirmNeeded   = 3 // быстрых неудач подряд = реальный обрыв
)
var failsInARow int

// в ветке неудачи:
if err != nil {
	cat := ClassifyProbeError(err)
	failsInARow++
	ae.emit("engine:diag", fmt.Sprintf("[diag] probe failed: %s (%s)", cat.String(), err))
	if failsInARow >= failConfirmNeeded {
		failsInARow = 0
		if tripped := ae.cb.RecordFailure(); tripped {
			ae.onCircuitOpen(cat)
			ticker.Reset(ae.cb.CurrentCooldown())
			continue
		}
	}
	ticker.Reset(failConfirmInterval) // быстро перепроверяем
	continue
}
// успех:
failsInARow = 0
ae.onProbeSuccess()
```
Пересмотр `failThreshold`: при подтверждениях каждые 3 c порог можно снизить до 2-3; итого срабатывание ~6-9 c вместо 90 c. Для `CatNetworkDown` по-прежнему не релоадим, только ждём.

### Граничные случаи / замечания
- **Обрыв посреди `onCircuitOpen`**: он синхронный. Опционально вынести reload+профайл на отдельную горутину, чтобы тикер продолжал работать; но сначала — просто A1+A6 вместе дают корректный тикер (решать после замеров).
- **`failsInARow` между состояниями**: сбрасываем на Open и на успех; между двумя HalfOpen-сессиями не накапливаем.
- **Живой, но медленный канал** (`CatDegraded`): сейчас `deepProbe` возвращает только err/nil; чтобы отличать «медленно» от «упало», нужен тайминг — см. B8.

### Тесты
- Замокать `deepProbe`: 3 подряд фейла за 6-9 c → `onCircuitOpen`; один фейл + успех → туннель жив, `failsInARow` сброшен.
- Лог-прогон имитации обрыва: срабатывание < 10 c.

### Критерий приёмки
Имитация обрыва ловится < 10 c; одиночная потеря пакета не роняет туннель.

---

# ЧАСТЬ B. Адаптивность по теории

> Делать строго после Части A: на нестабильной базе (A2, A3) строить «память» и
> ротацию опасно.

## B1. Независимый слой выхода: WARP в основной конфиг

### Причина
У основного пути один источник выхода — VPS. По теории, барьер «IP/ASN» снимается только сменой точки выхода; нужен независимый канал.

### Текущее состояние (код)
- `template-vps-reality.json`: urltest `auto` содержит **только VPS-каналы** (`grpc-nl`, `grpc-fr`, `httpupgrade-nl/fr`, `vless-nl/fr`, `hysteria2-nl`) + `direct`/`block`.
- WARP живёт отдельным шаблоном `template-warp-awg.json` (один эндпоинт `162.159.192.1:4500`).

### Почему это проблема
Когда все VPS-каналы лежат (VPS упал / IP зарезали), «адаптивному» переключаться не на что — VPN падает, хотя WARP (бесплатный, независимый) доступен.

### Решение (design)
Добавить WARP в urltest-группу `auto` как **независимый резерв**: `[VPS…, warp-awg, direct]`. Порядок в `outbounds` задаёт приоритет; WARP — после VPS, перед `direct` (fallback-цепочка «VPS → WARP → direct», по образцу clash-warp-config).
Источник WARP-параметров — `template-warp-awg.json`: вынести его `endpoints.wireguard` и соответствующий outbound в основной конфиг (или генерировать `config`-ом). Ключевое — `config/sync-to-windows.sh` должен разносить и основной, и WARP-блок, чтобы `assets/configs/` никогда не расходился с источником.

### Тесты / проверка
- `go build ./...`.
- Ручной прогон: отключить VPS-каналы → трафик идёт через WARP, VPN не падает.
- Проверка `sync-to-windows.sh` распространяет оба конфига.

### Риск и критерий
WARP-эндпоинты нестабильны и бывают заблокированы (см. B3) — поэтому WARP **резерв, не главный канал**. Приёмка: VPS-канал жив, WARP в пуле ниже по приоритету.

---

## B2. «Память» — пропущенное звено адаптивной петли

### Причина
Петля «детекти → выбери → примени → проверь → запомни» не имеет «запомни». Теория: память двигает политику.

### Текущее состояние (код)
- `ErrorClassifier` (`classifier.go:82-88`): `current`, `lastError`, события `maxEvents` — **только диагностика, не выбор**.
- `AdaptiveEngine` решает исключительно через `circuitBreaker` + urltest sing-box; выбора «какой канал поднять» по истории нет.
- `Diagnostics()` (`adaptive.go:401`) отдаёт только текущие счётчики в UI.

### Почему это проблема
После каждого обрыва и восстановления система «забывает», что конкретный канал/эндпоинт/I1 в этом контексте падал. Повторный обрыв проверяется с нуля. Нет предпочтения «стратегии, которая реже падала» (byedpi `--auto-mode`), нет `store-selected`-персиста.

### Решение (design + код-эскиз)
Новая структура рядом с `classifier`:
```go
// ChannelRecord — история одной точки (протокол/эндпоинт/I1).
type ChannelRecord struct {
	Key       string    `json:"key"`       // "warp-awg:i1=sip:162.159.192.1:4500"
	OK        int       `json:"ok"`        // успешных проверок подряд
	Fail      int       `json:"fail"`      // фейлов подряд
	LastOK    time.Time `json:"lastOk"`
	LastFail  time.Time `json:"lastFail"`
}

// ChannelMemory — кеш «что работало» (bandit-score по этой цели).
type ChannelMemory struct {
	mu   sync.Mutex
	recs map[string]*ChannelRecord
	path string // для персиста на диск
}

func (m *ChannelMemory) Record(key string, ok bool)
func (m *ChannelMemory) Score(key string) float64 // предпочтение при выборе
func (m *ChannelMemory) Load() / Save()
```
Интеграция:
- `AdaptiveEngine` получает `*ChannelMemory` (или `memory` в `NewAdaptiveEngine`).
- `onProbeSuccess` / `RecordSuccess` → `memory.Record(currentKey, true)`; фейл → `Record(..., false)`.
- Выбор канала для reload/ротации использует `Score`: на равных — предпочитать меньше фейлов.
- Персист: файл рядом с exe (например `channel-memory.json`), `Save()` по изменению + на `Stop()`; `Load()` при старте. Не хранить в памяти только.

### Граничные случаи / безопасность
- **Ключ** обязан быть детерминированным и не содержать секретов (не класть `private_key`).
- **Рост `recs`**: cap (`maxKeys`, LRU), т.к. ключей много (протокол×I1×эндпоинт).
- **Стирание**: если эндпоинт исчез из конфига — чистить его ключи, чтобы не «судить о том, чего нет».
- **Персист не должен биться при параллельных записях**: все мутации под `m.mu`; `Save()` — асинхронно/по таймеру.

### Тесты
- `Record`/`Score`: после 3 фейлов `A` и 3 OK `B` → `Score(B) > Score(A)`.
- Персист `Save`/`Load` из временного файла.
- Размер `recs` не превышает cap при переполнении.

### Критерий приёмки
После восстановления выбор учитывает прошлое: канал, падавший N раз подряд, не выбирается первым; выбор персистится между рестартами и виден в `Diagnostics()` (добавить `bestChannel`/`memory summary`).

---

## B3. Ротация I1 (AmneziaWG) + детекция здоровья WARP-эндпоинтов

### Причина
По теории, «DPI судит о соединении по первому пакету» → ротация I1 (первый пакет хендшейка AWG) — первый рычаг адаптации; junk-параметры — последние. У нас один набор прокси `jc/jmin/jmax/s1-4/h1-4` и заглушка `i1`.

### Текущее состояние (код)
- `template-warp-awg.json`: один `wireguard` endpoint `tag:"warp-awg"`, `i1: YOUR_AMNEZIA_I1_HEX_BLOB` (генерится вручную), `jc:4, jmin:40, jmax:70, s1..s4:0, h1..h4:1..4`.
- `registry.go` регистрирует wireguard-эндпоинт; тег сборки `with_awg` включает AmneziaWG 2.0.

### Почему это проблема
Нет вариантов I1 для ротации: если DPI режет конкретный I1, менять не на что кроме ручной правки. Endpoint на `162.159.192.1:4500` один — негде обойти заблокированный узел/гео.

### Решение (design)
1. **Сгенерировать набор I1**: готовые блобы (DNS/QUIC/SIP/STUN/random) — локально утилитой-генератором (warpscout `-gen-i1` даёт формат; у нас свой генератор по алгоритму AWG). Никаких секретов в коде — только `i1`-блоб и параметры.
2. **Несколько WARP-членов в `auto`** с разными `i1` (+/или страны эндпоинтов `162.159.192.1:4500`, `nl...:4500`, ...).
3. **Политика ротации** (связать с B2): при фейле канала с I1-A перейти на I1-B, потом I1-C; junk-параметры (`jc/jmin/jmax/s1-4/h1-4`) трогать последними.
4. **Детект torn-down/DME** (сенсорная часть):
   - torn-down: вместо одиночного запроса — серия (напр. 10) и смотреть **хвост потерянных пакетов**, а не общий процент; эндпоинт, режущийся на середине, не попадает в `auto`.
   - DME/geo: при неудаче на WARP — пробовать **другой эндпоинт/страну**, а не всю группу помечать мёртвой.

### Тесты / проверка
- Генератор I1 выдаёт валидные, разные блобы (детерминированно при одном seed).
- Конфиг с 3 WARP-членами, разными `i1`; `go build ./...`.
- Мок детектора: эндпоинт с «режущейся серединой сессии» отсеивается; живой остаётся.

### Риски / критерий
I1-блоб чувствителен к версии AWG — генерить под ту же, что в `with_awg`. Приёмка: ротация I1 реально переключает канал без ручной правки; DME/torn-down не ломают здоровый пул.

---

## B4. Подключить mieru как запасной no-TLS протокол

### Причина
Теория: mieru — не-TLS, XChaCha20-Poly1305, ключ от времени, padding/защита от replay, `traffic-pattern` (фрагментация TCP/nonce/low-entropy) — «трудно классифицировать и трудно зондировать». Идеальный запасной критичный канал, когда TLS-обёртки режутся.

### Текущее состояние (код)
- `configs/singbox/mieru-credentials.json` и `mieru-fr-credentials.json` **есть** (сервер/порт/логин/пароль).
- `app.go` `OpenExternalApp("karing")` (`app.go:530-542`) — клиент Mieru.
- В `registry.go` **нет** mieru-аутбаунда; ни один конфиг mieru не использует.

### Почему это проблема
Проект явно метил в mieru (креды + внешний клиент), но канал не интегрирован: нет ни outbound, ни маршрута, ни UI-выбора. Потенциальный резерв на случай, когда VLESS/Reality режутся, недоступен.

### Решение (по шагам, с проверкой на реальном sing-box-lx v1.14.0)
1. **Проверить поддержку**: есть ли mieru outbound в этой версии. Если sing-box-lx умеет — добавить `mieru.RegisterOutbound` в `registry.go` `outbounds()`, и mieru-аутбаунд в пул.
2. **Если sing-box-lx НЕ умеет** mieru нативно — вариант B: mieru как **отдельный процесс/клиент** (`karing` или `mihomo`), а наш движок пробрасывает трафик через `socks`/`mixed`-инбаунд в него, или поднимаем параллельный туннель. Решение зафиксировать в коде/конфиге (не «на словах»).
3. Добавить канал в транспортный пул (B1) как **последнюю линию** (после VPS и WARP).
4. Выйти в UI: `SelectServer`/модуль выбора — понимать `mieru` страну (по `mieru-fr`).

### Граничные случаи / безопасность
- **Секреты**: `mieru-credentials.json` содержит пароль. Сейчас репо untracked — ок; при коммите исключить в `.gitignore` (как `server-params.json`, `warp-keys.json`, `.env`).
- **Синхронизация часов** (ключ mieru зависит от времени сервера) — на десктопе системные часы, убедиться в NTP.
- **Ошибка запуска параллельного клиента**: не должен скрывать основной туннель; `OpenExternalApp` возвращает ошибку (уже есть) — пробрасывать в UI.

### Тесты / критерий
- Поднятие канала `pc → mieru-server` и проброс трафика; фиксация способа запуска.
- `go build ./...`; UI корректно отображает mieru-страну.
- Принято, когда есть рабочий mieru-канал в пуле как последний резерв, а секреты не в VCS.

---

# ЧАСТЬ C. Порядок, контрольные точки, что НЕ делать, риски

## Порядок внедрения (зависимости)

| № | Пункт | Зависит от | Приоритет |
|---|-------|-----------|-----------|
| 1 | A2 (горайтины) | — | Высокий (база) |
| 2 | A3 (Start/Reload) | — | Высокий (база) |
| 3 | A6 (быстрая детекция) + A1 (CB) | — | Высокий (вместе) |
| 4 | A5 (теги) | — | Средний, **до** B1 (иначе новые каналы усилят хрупкость) |
| 5 | A4 (мёртвый код) | — | Средний |
| 6 | B1 (WARP в urltest) | A5 | Средний |
| 7 | B3 (детекция эндпоинтов) | B1 | Средний |
| 8 | B3 (ротация I1) | B1, B2 | Средний |
| 9 | B2 (память) | A1-A6 | Средний (венчает адаптивность) |
| 10 | B4 (mieru) | — | Низкий, можно параллельно |

## Контрольные точки на каждом шаге
- `cd windows/frontend && npm run build` — если тронут фронт.
- `cd windows && go build ./...` — обязательно (требует `frontend/dist`, сначала бандл).
- `cd windows && go test -race ./backend/core/` — после каждого пункта Части A.
- Полная проверка Wails: `wails build -skipbindings -s -tags "with_awg,with_wireguard,with_utls,with_gvisor"`.
- Конфиги править в `configs/singbox/` (источник истины) → `bash configs/sync-to-windows.sh`.
- **Секреты не коммитить**: `mieru-credentials*.json`, `server-params.json`, `warp-keys.json`, `.env` → в `.gitignore`.

## Что осознанно НЕ делаем (по теории — не наша цель сейчас)
- **wdtt / VK-медиарелеи** — заточено под один сервис (VK); наша цель — общий обход.
- **zapret/byedpi (клиентский DPI-байпас без туннеля)** — тащит внешний драйвер (WinDivert) и новый слой; только как «последний ярус» при полностью упавшем туннеле, позже.
- **warp-relay (свой релей DNAT→Cloudflare)** — новая инфраструктура ради WARP-резерва; у нас уже есть VPS как основной выход. Опционально, позже.
- **zieng2/wl (подписки белых списков)** — внешняя подписка; в нашу архитектуру «свои конфиги» не вписывается.

## Риски и оговорки
- **Переделка `loop()` (A1+A6)** — самая чувствительная часть (порядок состояний, тикеры). Делать пошагово, с юнит-тестами на `circuitBreaker` до/после.
- **WARP как резерв нестабилен** (B3): не делать главным каналом, только fallback.
- **I1-блоб чувствителен к версии AWG**: генерить под ту же, что в `with_awg`.
- **B4 зависит от версии sing-box-lx**: сначала проверить нативную поддержку mieru, иначе менять подход (внешний клиент). Это влияет на объём: нативный = меньше кода, внешний = новая интеграция.
- **A4 (удаление `config`-пакета)** — проверить, не используется ли `builder.go` тестами `enginetest`, до удаления.

## Сводный критерий «система стала адаптивной»
1. Обрыв ловится < 10 c и не сваливает от одиночного блипа (A6).
2. Состояния circuit breaker реально проходят Open→cooldown→HalfOpen→Closed (A1).
3. Нет утечки горутин и рассинхрона Start/Reload (A2, A3).
4. Есть независимый выход (WARP) и детект его здоровья (B1, B3).
5. Система запоминает и учитывает прошлое при выборе канала (B2) — «память» замкнула петлю.
6. Запасной no-TLS канал (mieru) доступен пользователю (B4).