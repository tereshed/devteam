// ignore: unused_import
import 'package:intl/intl.dart' as intl;
import 'app_localizations.dart';

// ignore_for_file: type=lint

/// The translations for Russian (`ru`).
class AppLocalizationsRu extends AppLocalizations {
  AppLocalizationsRu([String locale = 'ru']) : super(locale);

  @override
  String get appTitle => 'Wibe Flutter Gin Template';

  @override
  String get login => 'Войти';

  @override
  String get logout => 'Выйти';

  @override
  String get register => 'Регистрация';

  @override
  String get email => 'Email';

  @override
  String get password => 'Пароль';

  @override
  String get emailHint => 'example@mail.com';

  @override
  String get enterEmail => 'Введите email';

  @override
  String get enterValidEmail => 'Введите корректный email';

  @override
  String get enterPassword => 'Введите пароль';

  @override
  String passwordTooShort(int minLength) {
    return 'Пароль должен содержать минимум $minLength символов';
  }

  @override
  String get passwordsDoNotMatch => 'Пароли не совпадают';

  @override
  String get passwordMinLength => 'Пароль должен быть не менее 8 символов';

  @override
  String get confirmPasswordPlaceholder => 'Подтвердите пароль';

  @override
  String get noAccountRegister => 'Нет аккаунта? Зарегистрироваться';

  @override
  String get haveAccountLogin => 'Уже есть аккаунт? Войти';

  @override
  String get welcomeBack => 'Добро пожаловать';

  @override
  String get loginTitle => 'Вход';

  @override
  String get registerTitle => 'Регистрация';

  @override
  String get createAccount => 'Создать аккаунт';

  @override
  String get dashboard => 'Dashboard';

  @override
  String get dashboardAdminManagePrompts => 'Управление промптами (Админ)';

  @override
  String get dashboardAdminManageWorkflows => 'Управление воркфлоу (Админ)';

  @override
  String get dashboardAdminViewLlmLogs => 'Логи LLM (Админ)';

  @override
  String get profile => 'Профиль';

  @override
  String get userInfo => 'Информация о пользователе';

  @override
  String get role => 'Роль';

  @override
  String get emailVerified => 'Email подтвержден';

  @override
  String get yes => 'Да';

  @override
  String get no => 'Нет';

  @override
  String get goToProfile => 'Перейти в профиль';

  @override
  String get information => 'Информация';

  @override
  String get refreshData => 'Обновить данные';

  @override
  String get dataLoadError => 'Ошибка загрузки данных';

  @override
  String get retry => 'Повторить';

  @override
  String get userNotAuthorized => 'Пользователь не авторизован';

  @override
  String get logoutConfirmTitle => 'Выход';

  @override
  String get logoutConfirmMessage => 'Вы уверены, что хотите выйти?';

  @override
  String get cancel => 'Отмена';

  @override
  String logoutError(String error) {
    return 'Ошибка при выходе: $error';
  }

  @override
  String get errorInvalidCredentials => 'Неверный email или пароль';

  @override
  String get errorUserNotFound => 'Пользователь не найден';

  @override
  String get errorUserAlreadyExists => 'Пользователь уже существует';

  @override
  String get errorAccessDenied => 'Доступ запрещен';

  @override
  String get errorNetwork => 'Ошибка сети. Проверьте подключение к интернету.';

  @override
  String get errorRequestCancelled => 'Запрос отменён.';

  @override
  String get errorServer => 'Ошибка сервера. Попробуйте позже.';

  @override
  String get errorUnknown => 'Произошла неизвестная ошибка.';

  @override
  String get routerNavigationError => 'Не удалось открыть эту страницу.';

  @override
  String get landingTitle => 'Создавайте быстрее с Wibe';

  @override
  String get landingSubtitle =>
      'Идеальный шаблон Flutter + Gin для вашей следующей идеи.\nГотов к продакшену, масштабируемый и красивый.';

  @override
  String get startForFree => 'Начать бесплатно';

  @override
  String get learnMore => 'Узнать больше';

  @override
  String get whyWibe => 'Почему Wibe?';

  @override
  String get featurePerformanceTitle => 'Высокая производительность';

  @override
  String get featurePerformanceDesc =>
      'Создан на Go (Gin) и Flutter для максимальной скорости.';

  @override
  String get featureSecurityTitle => 'Безопасность по умолчанию';

  @override
  String get featureSecurityDesc =>
      'JWT Auth, RBAC и лучшие практики безопасности включены.';

  @override
  String get featureCrossPlatformTitle => 'Кроссплатформенность';

  @override
  String get featureCrossPlatformDesc =>
      'Отлично работает на Web, iOS, Android и Desktop.';

  @override
  String get getStarted => 'Начать';

  @override
  String get goToDashboard => 'Перейти в Dashboard';

  @override
  String get promptsTitle => 'Управление промптами';

  @override
  String get promptsList => 'Список промптов';

  @override
  String get createPrompt => 'Создать промпт';

  @override
  String get editPrompt => 'Редактировать промпт';

  @override
  String get deletePrompt => 'Удалить промпт';

  @override
  String get deletePromptConfirmation =>
      'Вы уверены, что хотите удалить этот промпт?';

  @override
  String get promptName => 'Имя (Уникальный ID)';

  @override
  String get promptDescription => 'Описание';

  @override
  String get promptTemplate => 'Шаблон';

  @override
  String get promptJsonSchema => 'JSON Схема (Опционально)';

  @override
  String get promptIsActive => 'Активен';

  @override
  String get promptNameRequired => 'Имя обязательно';

  @override
  String get promptTemplateRequired => 'Шаблон обязателен';

  @override
  String get invalidJson => 'Неверный формат JSON';

  @override
  String get save => 'Сохранить';

  @override
  String get update => 'Обновить';

  @override
  String get create => 'Создать';

  @override
  String get delete => 'Удалить';

  @override
  String get managePrompts => 'Управление промптами (Админ)';

  @override
  String get templatePlaceholderHelper =>
      'Используйте <.Variable> для переменных';

  @override
  String get apiKeysTitle => 'API-ключи';

  @override
  String get apiKeyDescription =>
      'API-ключи позволяют вашим приложениям обращаться к API без пароля. Каждый ключ действует от вашего имени.';

  @override
  String get apiKeyCreate => 'Создать ключ';

  @override
  String get apiKeyName => 'Название ключа';

  @override
  String get apiKeyNameHint => 'Например: Мой скрипт, CI/CD';

  @override
  String get apiKeyExpiry => 'Срок действия';

  @override
  String get apiKeyNoExpiry => 'Бессрочный';

  @override
  String get apiKeyExpiry30Days => '30 дней';

  @override
  String get apiKeyExpiry90Days => '90 дней';

  @override
  String get apiKeyExpiry1Year => '1 год';

  @override
  String get apiKeyCreated => 'Ключ создан';

  @override
  String get apiKeyCreatedWarning =>
      'Скопируйте ключ сейчас! Он больше не будет показан.';

  @override
  String get apiKeyCopy => 'Скопировать ключ';

  @override
  String get apiKeyCopied => 'Ключ скопирован в буфер обмена';

  @override
  String get apiKeyUnderstood => 'Понятно, я сохранил ключ';

  @override
  String get apiKeyRevoke => 'Отозвать';

  @override
  String get apiKeyRevokeTitle => 'Отзыв ключа';

  @override
  String get apiKeyRevokeConfirm =>
      'Ключ перестанет работать. Это действие необратимо. Продолжить?';

  @override
  String get apiKeyDeleteTitle => 'Удаление ключа';

  @override
  String get apiKeyDeleteConfirm => 'Ключ будет полностью удалён. Продолжить?';

  @override
  String get apiKeyExpired => 'Истёк';

  @override
  String get apiKeyCreatedAt => 'Создан';

  @override
  String get apiKeyExpiresAt => 'Истекает';

  @override
  String get apiKeyLastUsed => 'Использован';

  @override
  String get apiKeyEmpty => 'Нет API-ключей';

  @override
  String get apiKeyEmptyHint =>
      'Создайте ключ, чтобы использовать API из своих приложений';

  @override
  String get apiKeysManage => 'API-ключи';

  @override
  String get globalSettingsScreenTitle => 'Глобальные настройки LLM';

  @override
  String get globalSettingsStubIntro =>
      'Ключи LLM-провайдеров (OpenAI, Anthropic, Gemini и др.) для агентов пока настраиваются на сервере. Полный экран с сохранением появится после готовности API.';

  @override
  String get globalSettingsBlockedByLabel => 'Задача backend в репозитории:';

  @override
  String get globalSettingsStubApiKeysNote =>
      'Ниже — ключи доступа к приложению DevTeam (MCP). Это не ключи LLM-провайдеров.';

  @override
  String get globalSettingsOpenDevTeamApiKeys => 'Ключи API приложения';

  @override
  String get mcpConfigTitle => 'Конфигурация MCP';

  @override
  String get mcpConfigDescription =>
      'Используйте эту конфигурацию для подключения вашего LLM-клиента (Cursor, Claude Desktop, VS Code Copilot) к этому серверу';

  @override
  String get mcpConfigCopy => 'Скопировать конфиг';

  @override
  String get mcpConfigCopied => 'Конфигурация скопирована в буфер обмена';

  @override
  String get mcpConfigInstructions => 'Инструкция:';

  @override
  String get mcpConfigStep1 => '1. Скопируйте конфигурацию ниже';

  @override
  String get mcpConfigStep2 => '2. Откройте настройки вашего LLM-клиента';

  @override
  String get mcpConfigStep3Cursor => '   - Cursor: .cursor/config.json';

  @override
  String get mcpConfigStep3Claude =>
      '   - Claude Desktop: ~/Library/Application Support/Claude/claude_desktop_config.json';

  @override
  String get mcpConfigStep4 =>
      '3. Вставьте конфигурацию и перезапустите клиент';

  @override
  String get mcpConfigLoadError => 'Не удалось загрузить конфигурацию MCP';

  @override
  String get mcpConfigDisabled => 'MCP-сервер выключен';

  @override
  String get projectsTitle => 'Проекты';

  @override
  String get createProject => 'Создать проект';

  @override
  String get searchProjectsHint => 'Поиск проектов...';

  @override
  String get filterAll => 'Все';

  @override
  String get statusActive => 'Активный';

  @override
  String get statusPaused => 'Приостановлен';

  @override
  String get statusArchived => 'Архив';

  @override
  String get statusIndexing => 'Индексация';

  @override
  String get statusIndexingFailed => 'Ошибка индексации';

  @override
  String get statusReady => 'Готов';

  @override
  String get statusUnknown => 'Неизвестно';

  @override
  String get noProjectsYet => 'Проектов пока нет';

  @override
  String get noProjectsMatchFilter => 'Ничего не найдено';

  @override
  String get clearFilters => 'Очистить фильтры';

  @override
  String get errorLoadingProjects => 'Не удалось загрузить проекты';

  @override
  String get errorUnauthorized => 'Сессия истекла. Войдите снова';

  @override
  String get errorForbidden => 'Нет доступа к проектам';

  @override
  String get gitProviderGithub => 'GitHub';

  @override
  String get gitProviderGitlab => 'GitLab';

  @override
  String get gitProviderBitbucket => 'Bitbucket';

  @override
  String get gitProviderLocal => 'Локально';

  @override
  String get gitProviderUnknown => 'Git';

  @override
  String get createProjectScreenTitle => 'Новый проект';

  @override
  String get projectNameFieldLabel => 'Название';

  @override
  String get projectNameFieldHint => 'Мой проект';

  @override
  String get projectNameRequired => 'Введите название';

  @override
  String projectNameMaxLength(int max) {
    return 'Не более $max символов';
  }

  @override
  String get projectDescriptionFieldLabel => 'Описание';

  @override
  String get projectDescriptionFieldHint => 'Для чего этот проект?';

  @override
  String get gitUrlFieldLabel => 'URL репозитория';

  @override
  String get gitUrlFieldHint => 'https://...';

  @override
  String get gitUrlRequiredForRemote => 'Укажите URL репозитория';

  @override
  String get gitUrlInvalid => 'Введите корректный http(s) URL';

  @override
  String get gitProviderFieldLabel => 'Провайдер Git';

  @override
  String get createProjectErrorConflict => 'Такое имя уже занято';

  @override
  String get createProjectErrorGeneric => 'Не удалось создать проект';

  @override
  String get projectDashboardFallbackTitle => 'Проект';

  @override
  String get projectDashboardChat => 'Чат';

  @override
  String get projectDashboardTasks => 'Задачи';

  @override
  String get projectDashboardTeam => 'Команда';

  @override
  String get projectDashboardSettings => 'Настройки';

  @override
  String get projectDashboardNotFoundTitle => 'Проект не найден';

  @override
  String get projectDashboardNotFoundBackToList => 'К списку проектов';

  @override
  String get projectSettingsSectionGit => 'Git-репозиторий';

  @override
  String get projectSettingsSectionVector => 'Векторный индекс';

  @override
  String get projectSettingsSectionTechStack => 'Технологический стек';

  @override
  String get projectSettingsGitDefaultBranchLabel => 'Ветка по умолчанию';

  @override
  String get projectSettingsGitCredentialCardTitle =>
      'Привязанный Git credential';

  @override
  String get projectSettingsUnlinkCredential => 'Отвязать credential';

  @override
  String get projectSettingsUnlinkPendingHint =>
      'Отвязка выполнится после сохранения.';

  @override
  String get projectSettingsVectorCollectionLabel => 'Имя коллекции Weaviate';

  @override
  String get projectSettingsVectorCollectionHint => 'например ProjectCode';

  @override
  String get projectSettingsVectorCollectionInvalid =>
      'Сначала заглавная латинская буква, далее буквы, цифры или подчёркивание.';

  @override
  String get projectSettingsVectorCollectionRenamed =>
      'Имя коллекции изменилось. Запустите переиндексацию — векторы не переносятся в новую коллекцию автоматически.';

  @override
  String get projectSettingsReindex => 'Переиндексировать';

  @override
  String get projectSettingsReindexInProgress => 'Идёт индексация…';

  @override
  String get projectSettingsReindexUnavailable =>
      'Переиндексация недоступна для локального проекта или при пустом URL репозитория.';

  @override
  String get projectSettingsReindexStarted => 'Запущена переиндексация';

  @override
  String get projectSettingsReindexConflict =>
      'Индексация уже выполняется или возник конфликт.';

  @override
  String get projectSettingsReindexGenericError =>
      'Не удалось запустить переиндексацию';

  @override
  String get projectSettingsReindexValidationError =>
      'Запрос переиндексации отклонён';

  @override
  String get projectSettingsTechStackAddRow => 'Добавить строку';

  @override
  String get projectSettingsTechStackClear => 'Очистить tech stack';

  @override
  String get projectSettingsTechStackKeyLabel => 'Ключ';

  @override
  String get projectSettingsTechStackValueLabel => 'Значение';

  @override
  String get projectSettingsSave => 'Сохранить';

  @override
  String get projectSettingsSaved => 'Настройки сохранены';

  @override
  String get projectSettingsNoChanges => 'Нет изменений для сохранения';

  @override
  String get projectSettingsGitRemoteAccessFailed =>
      'Не удалось обратиться к Git remote (ошибка клонирования или проверки).';

  @override
  String get projectSettingsActionForbidden =>
      'Действие запрещено для вашей учётной записи.';

  @override
  String get projectSettingsSaveConflict =>
      'Сохранение отклонено из‑за конфликта.';

  @override
  String get projectSettingsSaveGenericError =>
      'Не удалось сохранить настройки';

  @override
  String get projectSettingsSaveValidationError =>
      'Некорректные данные — проверьте форму и попробуйте снова.';

  @override
  String get chatErrorGeneric => 'Не удалось загрузить чат';

  @override
  String get chatErrorConversationNotFound => 'Чат не найден';

  @override
  String get chatErrorRateLimited => 'Слишком много запросов. Попробуйте позже';

  @override
  String get chatScreenAppBarFallbackTitle => 'Чат';

  @override
  String get chatScreenSelectConversationHint =>
      'Выберите беседу или откройте её по прямой ссылке с идентификатором.';

  @override
  String chatScreenMessageSemanticUser(String text) {
    return 'Вы: $text';
  }

  @override
  String chatScreenMessageSemanticAssistant(String text) {
    return 'Ассистент: $text';
  }

  @override
  String chatScreenMessageSemanticSystem(String text) {
    return 'Система: $text';
  }

  @override
  String get chatScreenInputHint => 'Сообщение…';

  @override
  String get chatScreenLoadingOlder => 'Загрузка истории…';

  @override
  String get chatScreenPendingSending => 'Отправка…';

  @override
  String get chatScreenPendingRetry => 'Повторить отправку';

  @override
  String get chatScreenNotFoundBack => 'К списку проектов';

  @override
  String get chatInputHint => 'Сообщение…';

  @override
  String get chatInputSendTooltip => 'Отправить';

  @override
  String get chatInputStopTooltip => 'Отменить отправку';

  @override
  String get chatInputAttachTooltip => 'Вложение';

  @override
  String get chatInputAttachDisabledHint => 'Вложения недоступны';

  @override
  String get taskStatusPending => 'В ожидании';

  @override
  String get taskStatusPlanning => 'Планирование';

  @override
  String get taskStatusInProgress => 'В работе';

  @override
  String get taskStatusReview => 'Ревью';

  @override
  String get taskStatusTesting => 'Тестирование';

  @override
  String get taskStatusChangesRequested => 'Нужны правки';

  @override
  String get taskStatusCompleted => 'Готово';

  @override
  String get taskStatusFailed => 'Ошибка';

  @override
  String get taskStatusCancelled => 'Отменена';

  @override
  String get taskStatusPaused => 'На паузе';

  @override
  String get taskStatusUnknownStatus => 'Неизвестный статус';

  @override
  String get tasksSearchHint => 'Поиск задач';

  @override
  String get tasksEmpty => 'Задач пока нет';

  @override
  String get tasksEmptyFiltered => 'Нет задач по выбранным фильтрам';

  @override
  String get tasksEmptyFilteredClear => 'Сбросить фильтры';

  @override
  String get taskPriorityCritical => 'Критический';

  @override
  String get taskPriorityHigh => 'Высокий';

  @override
  String get taskPriorityMedium => 'Средний';

  @override
  String get taskPriorityLow => 'Низкий';

  @override
  String get taskPriorityUnknown => 'Неизвестный приоритет';

  @override
  String get taskCardUnassigned => 'Не назначен';

  @override
  String taskCardAgentLine(String name, String role) {
    return '$name · $role';
  }

  @override
  String taskCardUpdatedAt(String time) {
    return 'Обновлено: $time';
  }

  @override
  String taskStatusCardFallbackTitle(String shortId) {
    return 'Задача $shortId';
  }

  @override
  String get taskCardAgentRoleWorker => 'Исполнитель';

  @override
  String get taskCardAgentRoleSupervisor => 'Супервизор';

  @override
  String get taskCardAgentRoleOrchestrator => 'Оркестратор';

  @override
  String get taskCardAgentRolePlanner => 'Планировщик';

  @override
  String get taskCardAgentRoleDeveloper => 'Разработчик';

  @override
  String get taskCardAgentRoleReviewer => 'Ревьюер';

  @override
  String get taskCardAgentRoleTester => 'Тестировщик';

  @override
  String get taskCardAgentRoleDevops => 'DevOps';

  @override
  String get taskErrorGeneric => 'Не удалось выполнить операцию с задачами';

  @override
  String get taskListErrorProjectNotFound => 'Проект не найден';

  @override
  String get taskDetailErrorTaskNotFound => 'Задача не найдена';

  @override
  String get taskSendMessageNoIdempotencyHint =>
      'Повторное нажатие «Отправить» создаёт второе сообщение на сервере (идемпотентность — отдельная задача).';

  @override
  String get taskDetailAppBarLoading => 'Загрузка…';

  @override
  String get taskDetailRefreshTimedOut =>
      'Обновление занимает слишком много времени. Попробуйте снова.';

  @override
  String get taskDetailDeletedTitle => 'Задача удалена';

  @override
  String get taskDetailDeletedBody =>
      'Эта задача удалена на сервере. Откройте список задач, чтобы продолжить работу с другими карточками.';

  @override
  String get taskDetailSectionDescription => 'Описание';

  @override
  String get taskDetailSectionResult => 'Результат';

  @override
  String get taskDetailSectionDiff => 'Изменения (diff)';

  @override
  String get taskDetailSectionMessages => 'Лог сообщений';

  @override
  String get taskDetailSectionErrorMessage => 'Ошибка задачи';

  @override
  String get taskDetailSectionSubtasks => 'Подзадачи';

  @override
  String get taskDetailNoDiff => 'Нет diff';

  @override
  String get taskDetailNoDescription => 'Нет описания';

  @override
  String get taskDetailNoResult => 'Нет результата';

  @override
  String get taskDetailNoMessages => 'Сообщений пока нет';

  @override
  String get taskDetailBackToList => 'К списку задач';

  @override
  String get taskDetailProjectMismatch => 'Задача из другого проекта';

  @override
  String get taskDetailRealtimeMutationBlocked =>
      'Обновление задачи по сети временно недоступно';

  @override
  String get taskDetailRealtimeSessionFailure => 'Проблема сессии realtime';

  @override
  String get taskDetailRealtimeServiceFailure => 'Сбой realtime-сервиса';

  @override
  String get taskActionPause => 'Пауза';

  @override
  String get taskActionResume => 'Возобновить';

  @override
  String get taskActionCancel => 'Отменить задачу';

  @override
  String get taskActionCancelConfirmTitle => 'Отменить задачу?';

  @override
  String get taskActionCancelConfirmBody =>
      'Задача перейдёт в статус «отменена». Это действие нельзя отменить.';

  @override
  String get taskActionConfirm => 'Да, отменить задачу';

  @override
  String get taskActionBlockedByRealtimeSnack =>
      'Сейчас нельзя изменить задачу: обновления по сети временно недоступны.';

  @override
  String get taskMessageTypeUnknown => 'Неизвестный тип сообщения';

  @override
  String get taskSenderTypeUnknown => 'Неизвестный отправитель';

  @override
  String get taskMessageTypeInstruction => 'Инструкция';

  @override
  String get taskMessageTypeResult => 'Результат';

  @override
  String get taskMessageTypeQuestion => 'Вопрос';

  @override
  String get taskMessageTypeFeedback => 'Обратная связь';

  @override
  String get taskMessageTypeError => 'Ошибка';

  @override
  String get taskMessageTypeComment => 'Комментарий';

  @override
  String get taskMessageTypeSummary => 'Сводка';

  @override
  String get taskSenderTypeUser => 'Пользователь';

  @override
  String get taskSenderTypeAgent => 'Агент';

  @override
  String get agentRoleUnknown => 'Неизвестная роль';

  @override
  String get agentRoleWorker => 'Исполнитель';

  @override
  String get agentRoleSupervisor => 'Супервизор';

  @override
  String get agentRoleOrchestrator => 'Оркестратор';

  @override
  String get agentRolePlanner => 'Планировщик';

  @override
  String get agentRoleDeveloper => 'Разработчик';

  @override
  String get agentRoleReviewer => 'Ревьюер';

  @override
  String get agentRoleTester => 'Тестировщик';

  @override
  String get agentRoleDevops => 'DevOps';

  @override
  String get teamEmptyAgents => 'В команде пока нет агентов.';

  @override
  String get teamAgentModelUnset => 'Модель по умолчанию';

  @override
  String get teamAgentNameUnset => 'Агент без имени';

  @override
  String get teamAgentActive => 'Активен';

  @override
  String get teamAgentInactive => 'Неактивен';

  @override
  String get teamAgentEditTitle => 'Редактирование агента';

  @override
  String get teamAgentEditFieldModel => 'Модель LLM';

  @override
  String get teamAgentEditFieldPrompt => 'Промпт';

  @override
  String get teamAgentEditFieldCodeBackend => 'Code backend';

  @override
  String get teamAgentEditFieldProviderKind => 'LLM провайдер';

  @override
  String get teamAgentEditFieldProviderKindHelp =>
      'Ключи провайдера задаются в Настройках → LLM-ключи';

  @override
  String get teamAgentEditFieldActive => 'Активен';

  @override
  String get teamAgentEditSave => 'Сохранить';

  @override
  String get teamAgentEditCancel => 'Отмена';

  @override
  String get teamAgentEditDiscardTitle => 'Отменить изменения?';

  @override
  String get teamAgentEditDiscardBody => 'Черновик будет потерян.';

  @override
  String get teamAgentEditSaveError => 'Не удалось сохранить агента';

  @override
  String get teamAgentEditSaveForbidden =>
      'Недостаточно прав для сохранения агента';

  @override
  String get teamAgentEditConflictError =>
      'Изменение отклонено (конфликт). Попробуйте снова.';

  @override
  String get teamAgentEditNoPrompts => 'Нет доступных промптов';

  @override
  String get teamAgentEditPromptsLoadError => 'Не удалось загрузить промпты';

  @override
  String get teamAgentEditPromptNone => 'Промпт не выбран';

  @override
  String get teamAgentEditUnset => 'Не задано';

  @override
  String get teamAgentEditDiscardConfirm => 'Сбросить';

  @override
  String get teamAgentEditRefetchError =>
      'Сохранено, но не удалось обновить команду';

  @override
  String get teamAgentEditFieldTools => 'Инструменты';

  @override
  String get teamAgentEditToolsLoadError =>
      'Не удалось загрузить каталог инструментов';

  @override
  String get teamAgentEditToolsEmpty => 'Нет доступных инструментов в каталоге';

  @override
  String get teamAgentEditToolsNoneSelected => 'Инструменты не выбраны';

  @override
  String get teamAgentEditToolsValidationError =>
      'Некорректный набор инструментов. Проверьте выбор и попробуйте снова.';

  @override
  String get teamAgentEditToolsRetry => 'Повторить';

  @override
  String teamAgentEditToolsListEntryLabel(String name, String category) {
    return '$name ($category)';
  }

  @override
  String get chatMessageCopyCode => 'Копировать код';

  @override
  String get chatMessageStreamingPlaceholder => 'Печатает…';

  @override
  String get chatMessageImagePlaceholder => '[изображение]';

  @override
  String chatMessageMarkdownImageAlt(String alt) {
    return '[$alt]';
  }

  @override
  String get refresh => 'Обновить';

  @override
  String get copy => 'Скопировать';

  @override
  String get openInBrowser => 'Открыть в браузере';

  @override
  String get fieldRequired => 'Обязательное поле';

  @override
  String get globalSettingsTabLLMProviders => 'LLM-провайдеры';

  @override
  String get globalSettingsTabClaudeCode => 'Claude Code';

  @override
  String get globalSettingsTabDevTeam => 'DevTeam';

  @override
  String get llmProvidersSectionTitle => 'LLM-провайдеры';

  @override
  String get llmProvidersAdd => 'Добавить';

  @override
  String get llmProvidersEmpty => 'Провайдеры ещё не настроены.';

  @override
  String get llmProvidersLoadError => 'Не удалось загрузить список провайдеров';

  @override
  String get llmProvidersHealthTooltip => 'Проверка здоровья';

  @override
  String get llmProvidersEditTooltip => 'Редактировать';

  @override
  String get llmProvidersDeleteTooltip => 'Удалить';

  @override
  String get llmProvidersHealthOK => 'Провайдер доступен';

  @override
  String get llmProvidersHealthFail => 'Проверка не пройдена';

  @override
  String get llmProvidersDeleteTitle => 'Удалить провайдера?';

  @override
  String llmProvidersDeleteConfirm(String name) {
    return 'Удалить «$name»? Агенты, привязанные к нему, останутся без провайдера.';
  }

  @override
  String get llmProvidersDeleteFail => 'Ошибка удаления';

  @override
  String get llmProvidersAddTitle => 'Новый LLM-провайдер';

  @override
  String get llmProvidersEditTitle => 'Редактирование провайдера';

  @override
  String get llmProvidersFieldName => 'Имя';

  @override
  String get llmProvidersFieldKind => 'Тип';

  @override
  String get llmProvidersFieldBaseURL => 'Base URL (опционально)';

  @override
  String get llmProvidersFieldCredential => 'API-ключ / токен';

  @override
  String get llmProvidersFieldCredentialOptional =>
      'API-ключ / токен (пусто — не менять)';

  @override
  String get llmProvidersFieldDefaultModel => 'Модель по умолчанию';

  @override
  String get llmProvidersFieldEnabled => 'Включён';

  @override
  String get llmProvidersTest => 'Тест';

  @override
  String get llmProvidersTestOK => 'Тестовое подключение успешно';

  @override
  String get llmProvidersTestFail => 'Тест подключения не пройден';

  @override
  String get claudeCodeAuthLoadError =>
      'Не удалось загрузить статус подписки Claude Code';

  @override
  String get claudeCodeAuthConnectedTitle => 'Подписка Claude Code подключена';

  @override
  String get claudeCodeAuthTokenType => 'Тип токена';

  @override
  String get claudeCodeAuthScopes => 'Scopes';

  @override
  String get claudeCodeAuthExpiresAt => 'Истекает';

  @override
  String get claudeCodeAuthLastRefreshedAt => 'Последнее обновление';

  @override
  String get claudeCodeAuthRevoke => 'Отозвать';

  @override
  String get claudeCodeAuthRevokeOK => 'Подписка отозвана';

  @override
  String get claudeCodeAuthDisconnectedTitle => 'Подписка Claude Code';

  @override
  String get claudeCodeAuthDisconnectedHint =>
      'Войдите по подписке Claude Code, чтобы агенты использовали OAuth-токен вместо долгоживущего API-ключа.';

  @override
  String get claudeCodeAuthLogin => 'Войти по подписке';

  @override
  String get claudeCodeAuthDeviceFlowTitle =>
      'Подтверждение на стороне Anthropic';

  @override
  String get claudeCodeAuthEnterCodeHint =>
      'Откройте ссылку ниже в любом браузере и введите этот код, чтобы авторизовать DevTeam:';

  @override
  String get claudeCodeAuthWaiting => 'Ожидание подтверждения…';

  @override
  String get agentSandboxSettingsTitle => 'Дополнительные настройки агента';

  @override
  String get agentSandboxSettingsLoadError =>
      'Не удалось загрузить настройки агента';

  @override
  String get agentSandboxSettingsTabProvider => 'Модель / провайдер';

  @override
  String get agentSandboxSettingsTabMCP => 'MCP-серверы';

  @override
  String get agentSandboxSettingsTabSkills => 'Skills';

  @override
  String get agentSandboxSettingsTabPermissions => 'Разрешения';

  @override
  String get agentSandboxSettingsProviderLabel => 'LLM-провайдер';

  @override
  String get agentSandboxSettingsProviderNone => '— нет —';

  @override
  String get agentSandboxSettingsCodeBackendLabel => 'Code backend';

  @override
  String get agentSandboxSettingsMCPHelper =>
      'JSON-массив привязок MCP — см. документацию.';

  @override
  String get agentSandboxSettingsSkillsHelper =>
      'JSON-массив Skills (Claude Code) — см. документацию.';

  @override
  String get agentSandboxSettingsDefaultMode => 'Режим по умолчанию';

  @override
  String get agentSandboxSettingsAllow => 'Allow';

  @override
  String get agentSandboxSettingsDeny => 'Deny';

  @override
  String get agentSandboxSettingsAsk => 'Ask';

  @override
  String get agentSandboxSettingsJsonInvalid => 'Некорректный JSON';

  @override
  String get agentSandboxSettingsPatternHint =>
      'Read | Edit | Bash(go test:*) | mcp__server';

  @override
  String get agentSandboxRevokeConfirmTitle => 'Отозвать подписку Claude Code?';

  @override
  String get agentSandboxRevokeConfirmBody =>
      'Агенты будут использовать ANTHROPIC_API_KEY (если задан) при следующих sandbox-задачах. Подписку можно подключить заново в любой момент.';

  @override
  String get teamAgentEditAdvanced => 'Дополнительно';

  @override
  String get commonRequestFailed => 'Ошибка запроса';
}
