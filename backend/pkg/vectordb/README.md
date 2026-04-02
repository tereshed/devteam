# Векторная База Данных (Weaviate)

## 🎯 Назначение

Weaviate используется для **семантического поиска** лингвистического контента:
- Леммы (vocabulary lemmas)
- Фразы (phrases)
- Топики (topics)
- Тексты (texts)

## 🚀 Запуск

### Запуск всех сервисов (включая Weaviate)
```bash
make up
```

### Только Weaviate
```bash
make weaviate-up
```

### Остановка Weaviate
```bash
make weaviate-down
```

### Проверка здоровья
```bash
make weaviate-health
```

### Логи
```bash
make weaviate-logs          # Логи Weaviate
make weaviate-logs-transformers  # Логи модели векторизации
```

### Полная очистка (удаление данных)
```bash
make weaviate-clean
```

## 🔧 Конфигурация

### Переменные окружения

В `backend/.env`:
```env
WEAVIATE_HOST=weaviate:8080
WEAVIATE_SCHEME=http
```

**Важно:** При локальной разработке (вне Docker):
```env
WEAVIATE_HOST=localhost:8081
```

## 📊 Архитектура

### Компоненты

1. **Weaviate** (порт 8081) - векторная база данных
2. **t2v-transformers** - модель векторизации (MiniLM)

### Модель: paraphrase-multilingual-MiniLM-L12-v2

- **Языки:** Более 50 языков (включая es, en, ru, de, fr, pt)
- **Размер:** ~500 MB
- **Векторы:** 384 измерения
- **Скорость:** Быстрая (подходит для CPU)

## 📐 Схема данных

### Класс: LinguisticContent

| Поле | Тип | Индекс | Описание |
|------|-----|--------|----------|
| `contentId` | string | ✅ | ID записи в PostgreSQL |
| `content` | text | ✅ (vector) | Текст для векторизации |
| `contentType` | string | ✅ | Тип: lemma, phrase, topic, text |
| `language` | string | ✅ | ISO код языка (es, en, ru) |
| `metadata` | object | ❌ | Дополнительные данные (JSON) |
| `createdAt` | date | ❌ | Время создания |
| `updatedAt` | date | ❌ | Время обновления |

### Связь с PostgreSQL

Каждый документ в Weaviate содержит `contentId` - ссылку на запись в PostgreSQL:
- `lemma` → `vocabulary_lemmas.id`
- `topic` → `topics.id`
- `phrase` → `phrases.id`

## 🔍 Поиск

### Типы поиска

1. **Semantic Search** (Alpha = 1.0) - только векторный поиск
2. **Keyword Search** (Alpha = 0.0) - только BM25
3. **Hybrid Search** (Alpha = 0.5) - комбинированный

### Фильтрация

Поддерживается фильтрация по:
- `contentType` (lemma, phrase, topic)
- `language` (es, en, ru, etc.)
- `contentId` (конкретные ID)

## 📦 Структура пакетов

```
backend/
├── pkg/vectordb/
│   ├── client.go           # Клиент Weaviate
│   ├── types.go            # Типы и константы
│   └── schema/
│       └── schema.go       # Схема LinguisticContent
└── internal/models/
    └── vector_document.go  # Модель документа
```

## ⚠️ Важные заметки

1. **Первый запуск:** Модель векторизации загружается ~30-60 секунд
2. **Память:** Требуется ~1-2 GB RAM для Weaviate + Transformers
3. **Порт 8081:** Weaviate доступен на 8081 (8080 занят Go API)
4. **Персистентность:** Данные хранятся в Docker volume `weaviate_data`

## 🧪 Проверка работы

```bash
# Проверить meta информацию
curl http://localhost:8081/v1/meta | jq

# Проверить схему
curl http://localhost:8081/v1/schema | jq

# Проверить количество объектов
curl http://localhost:8081/v1/objects?limit=1 | jq
```

## 📚 Следующие шаги

- [x] Фаза 1: Инфраструктура (Docker + Schema)
- [ ] Фаза 2: Repository Layer
- [ ] Фаза 3: Hybrid Search Service
- [ ] Фаза 4: Data Loader
- [ ] Фаза 5: API Endpoints
- [ ] Фаза 6: Tests

