# PolyMaths — деплой «вариант 0» (одна VM)

Минимальная боевая выкатка: **весь стек на одной машине** через `docker compose`. Без Swarm/k8s,
без managed-БД. Подходит, пока экономим. HA нет — рестарт = даунтайм; защита данных = внешний бэкап.

Что добавляет этот каталог поверх базового `docker-compose.yml`:

| Файл | Назначение |
|------|------------|
| `docker-compose.prod.yml` | overlay: Caddy наружу, `restart: unless-stopped`, прод-env |
| `Caddyfile` | reverse proxy + авто-TLS (Let's Encrypt) |
| `backup.sh` | дамп YugabyteDB → gzip → Object Storage |

> Базовый `docker-compose.yml` уже забиндил порты БД/Weaviate/Redis/app на `127.0.0.1`.
> Наружу смотрит **только Caddy** (`:80/:443`). Остальное доступно лишь внутри docker-сети.

---

## 1. Требования к VM

- **RAM 16–32 GB, ≥4 vCPU, ≥100 GB SSD.** На коробке одновременно: Yugabyte, Weaviate + t2v-модель,
  Redis, app и **sandbox-контейнеры** (Flutter/Go сборки — тяжёлые). Песочницы лимитированы
  per-container, но хост должен вынести несколько параллельных. Мало RAM → снижай
  `ORCHESTRATOR_AGENT_WORKERS`.
- ОС с Docker Engine + Compose plugin (`docker compose`, не `docker-compose`).
- Публичный IP + DNS A-record `DOMAIN` → IP (и `mcp.DOMAIN`, если MCP нужен снаружи).

## 2. Сеть / firewall (security-group)

Наружу открыть **только**: `80`, `443`, `22` (SSH, лучше по ключу + ограниченный source).
Всё остальное (`5433`, `15000`, `8082`, `6379`, `8080`, `8081`) уже на loopback — наружу не торчит.

## 3. Секреты

```bash
sudo mkdir -p /opt/devteam && cd /opt/devteam
git clone <repo> . && git checkout deploy
# backend/.env — НЕ в репозитории. Положи руками, root-only:
sudo install -m 600 /dev/null backend/.env
sudo vi backend/.env   # LLM_API_KEY/ANTHROPIC_API_KEY, JWT_SECRET_KEY, git-credential и т.д.
```

`JWT_SECRET_KEY` обязательно смени с дефолта. Ключи LLM, git-credential — только в `backend/.env`.

## 4. Запуск

```bash
export DOMAIN=poly.example.com ACME_EMAIL=ops@example.com
docker compose -f docker-compose.yml -f deployment/prod/docker-compose.prod.yml up -d --build
```

Порядок старта обеспечен зависимостями: `yugabytedb`(healthy) → `migrate`(one-shot, накат goose) →
`app`. Caddy ждёт `app` healthy, затем берёт сертификат. Первый старт Yugabyte ~30–60 c,
t2v-модель грузится ещё ~30–90 c.

Проверка:
```bash
docker compose -f docker-compose.yml -f deployment/prod/docker-compose.prod.yml ps
curl -fsS https://$DOMAIN/health && echo OK
```

## 5. Бэкапы (обязательно)

```bash
sudo install -m 600 /dev/null /etc/polymaths-backup.env
sudo vi /etc/polymaths-backup.env   # S3_BUCKET, S3_ENDPOINT, AWS_ACCESS_KEY_ID/SECRET, RETENTION_DAYS
```

Yandex Object Storage: `S3_ENDPOINT=https://storage.yandexcloud.net`. Ключ — от сервис-аккаунта
с ролью `storage.editor`. Cron:

```cron
30 3 * * *  set -a; . /etc/polymaths-backup.env; set +a; /opt/devteam/deployment/prod/backup.sh >> /var/log/polymaths-backup.log 2>&1
```

Раз в квартал проверяй **восстановление** дампа на отдельной коробке — бэкап без проверки restore
бэкапом не считается.

## 6. Гигиена диска

Sandbox-сборки и образы быстро забивают диск. Чистка по cron:

```cron
0 4 * * *  docker system prune -af --filter "until=48h" >> /var/log/docker-prune.log 2>&1
```

`SANDBOX_KEEP_ON_FAILURE=0` (в prod-overlay) не даёт копиться контейнерам упавших прогонов.

## 7. Обновление / откат

```bash
cd /opt/devteam && git pull
docker compose -f docker-compose.yml -f deployment/prod/docker-compose.prod.yml up -d --build
# миграции накатит one-shot `migrate` ДО старта app; app стартует с AUTO_MIGRATE=false.
```

Даунтайм на пересборку app есть (single-node). Откат: `git checkout <prev>` + повторный `up -d --build`.
Перед рискованным апдейтом — свежий `backup.sh` вручную.

## Чего здесь СОЗНАТЕЛЬНО нет (отложено как дорогое)

- HA / несколько реплик (есть `docker-compose.roles.yml` + `scale.yml` на будущее).
- Managed PostgreSQL вместо self-host Yugabyte.
- Секреты в Lockbox (на single-VM хватает root-only `backend/.env`).
- Централизованные метрики/алерты (`internal/metrics/` готов под Prometheus, когда дорастём).
