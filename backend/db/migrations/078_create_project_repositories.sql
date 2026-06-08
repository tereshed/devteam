-- +goose Up
-- +goose StatementBegin

-- project_repositories — несколько git-репозиториев на один проект (например, отдельный
-- репозиторий под UI и под высоконагруженную часть + вспомогательные). Каждый репо несёт
-- role_description (свободный текст), который читает decomposer, чтобы самостоятельно
-- (LLM-driven, без хардкода в Go) раскладывать подзадачи по нужному репо через repo_slug.
-- Бэк-компат: для каждого существующего проекта создаётся одна primary-строка slug='main'
-- из текущих полей projects.git_url/...; старые поля на projects остаются deprecated.
CREATE TABLE IF NOT EXISTS project_repositories (
    id                  UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id          UUID          NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    slug                VARCHAR(64)   NOT NULL,
    display_name        VARCHAR(255)  NOT NULL,
    role_description    TEXT          NOT NULL DEFAULT '',
    git_provider        VARCHAR(50)   NOT NULL DEFAULT 'local',
    git_url             VARCHAR(1024) NOT NULL,
    git_default_branch  VARCHAR(255)  NOT NULL DEFAULT 'main',
    git_credentials_id  UUID          REFERENCES git_credentials(id) ON DELETE SET NULL,
    vector_collection   VARCHAR(255),
    last_indexed_commit VARCHAR(255)  NOT NULL DEFAULT '',
    status              VARCHAR(50)   NOT NULL DEFAULT 'active',
    is_primary          BOOLEAN       NOT NULL DEFAULT false,
    sort_order          INT           NOT NULL DEFAULT 0,
    created_at          TIMESTAMPTZ   NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ   NOT NULL DEFAULT now(),
    CONSTRAINT chk_project_repositories_git_provider
        CHECK (git_provider IN ('github', 'gitlab', 'bitbucket', 'local')),
    UNIQUE (project_id, slug)
);

CREATE INDEX IF NOT EXISTS idx_project_repositories_project ON project_repositories(project_id);
-- Ровно один primary-репозиторий на проект.
CREATE UNIQUE INDEX IF NOT EXISTS uq_project_primary_repo
    ON project_repositories(project_id) WHERE is_primary;

-- Data-миграция: каждый существующий проект с непустым git_url -> одна primary-строка.
INSERT INTO project_repositories
    (project_id, slug, display_name, role_description, git_provider, git_url,
     git_default_branch, git_credentials_id, vector_collection, last_indexed_commit,
     status, is_primary, sort_order)
SELECT id, 'main', name, '', git_provider, COALESCE(git_url, ''),
       git_default_branch, git_credentials_id, vector_collection, last_indexed_commit,
       'active', true, 0
FROM projects
WHERE git_url IS NOT NULL AND git_url <> '';

-- task_pull_requests — несколько PR на одну задачу (по одному на каждый затронутый репо).
-- Done-гейт оркестратора считает задачу done только когда по всем затронутым репо открыт PR.
CREATE TABLE IF NOT EXISTS task_pull_requests (
    id          UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id     UUID          NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    repo_slug   VARCHAR(64)   NOT NULL,
    pr_number   INT           NOT NULL,
    pr_url      VARCHAR(1024) NOT NULL,
    created_at  TIMESTAMPTZ   NOT NULL DEFAULT now(),
    UNIQUE (task_id, repo_slug)
);

CREATE INDEX IF NOT EXISTS idx_task_pull_requests_task ON task_pull_requests(task_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS task_pull_requests;
DROP TABLE IF EXISTS project_repositories;

-- +goose StatementEnd
