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
  String get chatScreenSendButton => 'Отправить';

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
  String get chatLinkedTasksRealtimeNote =>
      'Актуальный статус задач в реальном времени подключится в задаче 11.9 (WebSocket).';

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
  String get chatMessageCopyCode => 'Копировать код';

  @override
  String get chatMessageStreamingPlaceholder => 'Печатает…';

  @override
  String get chatMessageImagePlaceholder => '[изображение]';

  @override
  String chatMessageMarkdownImageAlt(String alt) {
    return '[$alt]';
  }
}
