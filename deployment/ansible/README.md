# PolyMaths — деплой через Ansible (вариант 0, одна VM)

Идемпотентный плейбук: первый прогон провижинит чистую Ubuntu-VM, последующие = `git pull` +
пересборка `app`. Поднимает тот же стек, что `deployment/prod/` (см. его README про архитектуру/сайзинг),
но без ручных шагов.

```
deployment/ansible/
├── site.yml              # точка входа: роли common → docker → app → backup
├── requirements.yml      # коллекции (community.docker, community.general)
├── ansible.cfg
├── inventory.ini.example # → inventory.ini (IP/пользователь VM)
├── group_vars/
│   └── all.yml.example   # → all.yml (домен, репо, тюнинг) + секреты (vault)
└── roles/
    ├── common  # пакеты + UFW (наружу только 22/80/443)
    ├── docker  # Docker Engine + compose plugin
    ├── app     # git checkout, секреты, docker compose up
    └── backup  # ysql_dump → Object Storage + cron prune
```

## Что делает плейбук

| Роль | Действия |
|------|----------|
| **common** | apt-пакеты; UFW: deny incoming, allow `22/80/443` (SSH открывается до enable) |
| **docker** | официальный apt-репо Docker, Engine + buildx + compose-plugin, автозапуск |
| **app** | `git` checkout ветки `deploy`; кладёт `backend/.env` (0600); генерит корневой `.env` (DOMAIN/JWT/DOCKER_GID); `docker compose -f docker-compose.yml -f deployment/prod/docker-compose.prod.yml up -d --build` |
| **backup** | aws-cli; `/etc/polymaths-backup.env` (0600); cron дампа 03:30 и `docker system prune` 04:00 |

## Предпосылки (control-машина)

```bash
pip install ansible            # или brew install ansible
cd deployment/ansible
ansible-galaxy collection install -r requirements.yml
```

## Настройка

```bash
cp inventory.ini.example inventory.ini && vi inventory.ini      # IP + ansible_user
cp group_vars/all.yml.example group_vars/all.yml && vi group_vars/all.yml

# Секреты — в отдельный зашифрованный файл (НЕ в all.yml):
mkdir -p group_vars/all && mv group_vars/all.yml group_vars/all/vars.yml
ansible-vault create group_vars/all/vault.yml   # сюда: vault_jwt_secret_key, vault_backup_aws_*

# Прод-секреты приложения (LLM-ключи, git-credential) — файлом, его копирует роль app:
mkdir -p files && cp /path/to/prod/backend.env files/backend.env   # gitignored
```

> `inventory.ini`, `group_vars/all*`, `files/backend.env` уже в `.gitignore` этого каталога.

### Приватный репозиторий
Роль `app` тянет код через `git`. Для приватного репо либо положи на VM deploy-key
(`~/.ssh` пользователя из inventory) с доступом read-only, либо укажи в `repo_url` HTTPS с токеном.
Альтернатива без git на VM — заменить таск на `ansible.posix.synchronize` (push рабочей копии rsync'ом).

## Запуск

```bash
# Проверка связи
ansible -m ping polymaths

# Полная выкатка (с vault-паролем)
ansible-playbook site.yml --ask-vault-pass

# Прогон одной роли (напр. только редеплой app)
ansible-playbook site.yml --ask-vault-pass --tags app   # если добавишь теги ролям
```

Из корня репо есть обёртки: `make deploy-setup` (коллекции) и `make deploy` (плейбук).

## Редеплой новой версии

```bash
git push   # в ветку deploy
ansible-playbook site.yml --ask-vault-pass
```

Роль `app` подтянет код и пересоберёт `app`; миграции накатит one-shot `migrate` ДО старта `app`
(`AUTO_MIGRATE=false`). Даунтайм на пересборку есть (single-node).

## Проверка после деплоя

```bash
curl -fsS https://<domain>/health && echo OK
ansible -m shell -a 'docker compose -f /opt/devteam/docker-compose.yml -f /opt/devteam/deployment/prod/docker-compose.prod.yml ps' polymaths
```

## Автодеплой по push (GitHub Actions)

`.github/workflows/deploy.yml` гоняет этот плейбук на каждый push в ветку `deploy` (и вручную
через workflow_dispatch). В CI `ansible-vault` НЕ используется — секреты приходят из GitHub
Secrets как `--extra-vars`. Нужно завести:

**Secrets:** `SSH_PRIVATE_KEY`, `DEPLOY_SSH_HOST`, `BACKEND_ENV` (полное содержимое прод
`backend/.env`), `JWT_SECRET_KEY`, `BACKUP_AWS_ACCESS_KEY_ID`, `BACKUP_AWS_SECRET_ACCESS_KEY`.

**Variables:** `DEPLOY_SSH_USER`, `DOMAIN`, `ACME_EMAIL`, `REPO_URL`, `BACKUP_S3_BUCKET`,
`BACKUP_S3_ENDPOINT` (+ опц. `ORCHESTRATOR_STEP_WORKERS` / `ORCHESTRATOR_AGENT_WORKERS`).

Предпосылки на VM: SSH-пользователь с **passwordless sudo** и доступ к репозиторию
(`REPO_URL`) — приватный репо тянется deploy-key'ом с самой VM.

## Чего здесь нет (как и в варианте 0)
HA/реплик, managed-БД, секретов в Lockbox. Ручной запуск (`make deploy`) и автодеплой
(Actions) сосуществуют: CI обходит vault через extra-vars, локально работает ansible-vault.
