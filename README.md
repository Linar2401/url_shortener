# go-musthave-shortener-tpl

Шаблон репозитория для трека «Сервис сокращения URL».

## Начало работы

1. Склонируйте репозиторий в любую подходящую директорию на вашем компьютере.
2. В корне репозитория выполните команду `go mod init <name>` (где `<name>` — адрес вашего репозитория на GitHub без префикса `https://`) для создания модуля.

## Обновление шаблона

Чтобы иметь возможность получать обновления автотестов и других частей шаблона, выполните команду:

```
git remote add -m v2 template https://github.com/Yandex-Practicum/go-musthave-shortener-tpl.git
```

Для обновления кода автотестов выполните команду:

```
git fetch template && git checkout template/v2 .github
```

Затем добавьте полученные изменения в свой репозиторий.

## Запуск автотестов

Для успешного запуска автотестов называйте ветки `iter<number>`, где `<number>` — порядковый номер инкремента. Например, в ветке с названием `iter4` запустятся автотесты для инкрементов с первого по четвёртый.

При мёрже ветки с инкрементом в основную ветку `main` будут запускаться все автотесты.

Подробнее про локальный и автоматический запуск читайте в [README автотестов](https://github.com/Yandex-Practicum/go-autotests).

## Структура проекта

Приведённая в этом репозитории структура проекта является рекомендуемой, но не обязательной.

Это лишь пример организации кода, который поможет вам в реализации сервиса.

При необходимости можно вносить изменения в структуру проекта, использовать любые библиотеки и предпочитаемые структурные паттерны организации кода приложения, например:
- **DDD** (Domain-Driven Design)
- **Clean Architecture**
- **Hexagonal Architecture**
- **Layered Architecture**

## Бенчмарки и профилирование памяти

Бенчмарки покрывают горячие хэндлеры: `BenchmarkCreateHandle`, `BenchmarkShortenJSONHandle`, `BenchmarkGetHandle`, `BenchmarkBatchHandle` (см. `internal/handler/benchmark_test.go`).

Снятие профилей:

```
go test -run=^$ -bench=. -benchmem -benchtime=3s -count=1 -memprofile=profiles/base.pprof   ./internal/handler/
go test -run=^$ -bench=. -benchmem -benchtime=3s -count=1 -memprofile=profiles/result.pprof ./internal/handler/
```

### Что было оптимизировано

- `url.JoinPath` заменён на простую конкатенацию `base + "/" + code` в `joinResultURL`: `ResultAddress` уже нормализован в `config.Load`, поэтому `url.Parse` не нужен.
- `http.Redirect` в `GetHandle` заменён на прямую установку заголовка `Location` и `WriteHeader(307)`, что устраняет внутренний `url.Parse` цели редиректа.
- Запись plain-text ответа в `CreateHandle` переведена с `w.Write([]byte(resultURL))` на `io.WriteString(w, resultURL)`, что снимает лишнюю аллокацию `[]byte`.

### Результат `pprof -top -diff_base=profiles/base.pprof profiles/result.pprof`

Отрицательные значения — уменьшение аллокаций после оптимизации. Положительные значения относятся к коду httptest/bufio и растут лишь потому, что после оптимизаций за то же `benchtime` успевает выполниться больше итераций.

```
File: handler.test.exe
Type: alloc_space
Showing nodes accounting for 1875.49MB, 3.30% of 56798.05MB total
Dropped 44 nodes (cum <= 283.99MB)
      flat  flat%   sum%        cum   cum%
 3331.85MB  5.87%  5.87%  3331.85MB  5.87%  bufio.NewReaderSize (inline)
-1520.18MB  2.68%  3.19% -1702.68MB  3.00%  net/url.(*URL).JoinPath
-1465.70MB  2.58%  0.61% -1465.70MB  2.58%  net/url.parse
  543.67MB  0.96%  1.57%   543.67MB  0.96%  net/http.(*Request).WithContext (inline)
 -445.02MB  0.78%  0.78%  -445.02MB  0.78%  strings.(*Builder).grow
  321.01MB  0.57%  1.35%   321.01MB  0.57%  github.com/Linar2401/url_shortener/internal/handler.joinResultURL (inline)
  240.57MB  0.42%  1.77%   402.10MB  0.71%  net/http.readRequest
  224.07MB  0.39%  2.17%   224.07MB  0.39%  net/http.Header.Clone (inline)
 -180.51MB  0.32%  1.85% -2667.15MB  4.70%  net/http.Redirect
  171.74MB   0.3%  2.15%   171.74MB   0.3%  encoding/json.(*Decoder).refill
  154.63MB  0.27%  2.42%   154.63MB  0.27%  reflect.growslice
  137.01MB  0.24%  2.66%   137.01MB  0.24%  net/http/httptest.NewRecorder (inline)
  130.56MB  0.23%  2.89%   130.56MB  0.23%  net/textproto.MIMEHeader.Set (inline)
  103.02MB  0.18%  3.08%   103.02MB  0.18%  net/http.(*Request).SetPathValue (inline)
     -95MB  0.17%  2.91%  -948.58MB  1.67%  github.com/Linar2401/url_shortener/internal/handler.(*Handlers).CreateHandle
     -90MB  0.16%  2.75%      -90MB  0.16%  path.(*lazybuf).append (inline)
   69.02MB  0.12%  2.87%    69.02MB  0.12%  encoding/json.NewDecoder (inline)
  -67.88MB  0.12%  2.75%   -67.88MB  0.12%  bytes.growSlice
   63.54MB  0.11%  2.86%  -606.54MB  1.07%  github.com/Linar2401/url_shortener/internal/handler.(*Handlers).BatchHandle
   62.03MB  0.11%  2.97%    62.03MB  0.11%  io.ReadAll
  -48.50MB 0.085%  3.18%   -48.50MB 0.085%  path.(*lazybuf).string (inline)
     -44MB 0.077%  3.18%  -182.50MB  0.32%  path.Join
     -11MB 0.019%  3.32%      -11MB 0.019%  github.com/Linar2401/url_shortener/internal/handler.generateCode
   -5.50MB 0.0097%  3.31%  -730.06MB  1.29%  github.com/Linar2401/url_shortener/internal/handler.(*Handlers).ShortenJSONHandle
         0     0%  3.30%     -510MB   0.9%  github.com/Linar2401/url_shortener/internal/handler.(*Handlers).GetHandle
         0     0%  3.30% -3381.87MB  5.95%  net/url.JoinPath
         0     0%  3.30% -1579.72MB  2.78%  net/url.Parse
         0     0%  3.30%  -138.50MB  0.24%  path.Clean
```

Сравнение `B/op` и `allocs/op` (benchstat-стиль, до → после):

| Хэндлер              | B/op до | B/op после | allocs/op до | allocs/op после |
|----------------------|---------|------------|--------------|-----------------|
| CreateHandle         | 6855    | 6463       | 32           | 25              |
| ShortenJSONHandle    | 7972    | 7612       | 41           | 35              |
| GetHandle            | 7097    | 6758       | 26           | 21              |
| BatchHandle (20 URL) | 22577   | 15355      | 241          | 121             |
