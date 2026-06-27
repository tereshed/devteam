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
  String get appShellBrand => 'PolyMaths';

  @override
  String get navDashboard => 'Обзор';

  @override
  String get navProjects => 'Проекты';

  @override
  String get navAgents => 'Агенты';

  @override
  String get navWorktrees => 'Воркtrees';

  @override
  String get navIntegrationsLlm => 'LLM-провайдеры';

  @override
  String get navIntegrationsGit => 'Git-провайдеры';

  @override
  String get navPrompts => 'Промпты';

  @override
  String get navWorkflows => 'Воркфлоу';

  @override
  String get navExecutions => 'Запуски';

  @override
  String get navSettings => 'Настройки';

  @override
  String get navProfile => 'Профиль';

  @override
  String get navApiKeys => 'API-ключи';

  @override
  String get navGroupHome => 'Главная';

  @override
  String get navGroupResources => 'Ресурсы';

  @override
  String get navGroupIntegrations => 'Интеграции';

  @override
  String get navGroupAdmin => 'Администрирование';

  @override
  String get navGroupSettings => 'Настройки';

  @override
  String get navBreadcrumbHome => 'Главная';

  @override
  String get navBreadcrumbNew => 'Новый';

  @override
  String get integrationStatusConnected => 'Подключено';

  @override
  String get integrationStatusDisconnected => 'Не подключено';

  @override
  String get integrationStatusError => 'Ошибка';

  @override
  String get integrationStatusPending => 'Подключение…';

  @override
  String get integrationsLlmTitle => 'LLM-провайдеры';

  @override
  String get integrationsLlmComingSoon =>
      'Управление провайдерами появится на этапе 2. Ниже — превью каталога.';

  @override
  String get integrationsGitTitle => 'Git-провайдеры';

  @override
  String get integrationsGitComingSoon =>
      'Подключение GitHub и GitLab появится на этапе 3.';

  @override
  String get integrationsGitConnectCta => 'Подключить';

  @override
  String get integrationsGitGithubSubtitle =>
      'Чтение репозиториев, push в PR-ветки';

  @override
  String get integrationsGitGitlabSubtitle => 'Cloud и self-hosted GitLab';

  @override
  String get integrationsGitStage3Subtitle =>
      'Подключите GitHub и GitLab, чтобы пушить ветки и открывать MR.';

  @override
  String get integrationsGitSectionConnected => 'Подключено';

  @override
  String get integrationsGitSectionAvailable => 'Доступно';

  @override
  String get integrationsGitDisconnectCta => 'Отключить';

  @override
  String get integrationsGitConnectSelfHostedCta => 'Подключить self-hosted';

  @override
  String get integrationsGitEmptyAvailable =>
      'Все поддерживаемые провайдеры уже подключены.';

  @override
  String get integrationsGitEmptyConnected =>
      'Пока ни одного провайдера не подключено.';

  @override
  String get integrationsGitReasonUserCancelled =>
      'Авторизация отклонена. Попробуйте снова.';

  @override
  String get integrationsGitReasonExpired =>
      'OAuth-сессия истекла. Начните заново.';

  @override
  String get integrationsGitReasonProviderUnreachable =>
      'Git-провайдер недоступен. Попробуйте позже.';

  @override
  String get integrationsGitReasonInvalidHost =>
      'Хост не разрешён (приватная сеть, неподдерживаемая схема или неверный URL).';

  @override
  String get integrationsGitReasonOauthNotConfigured =>
      'Этот провайдер не настроен на сервере.';

  @override
  String get integrationsGitReasonRemoteRevokeFailed =>
      'Подключение удалено локально, но провайдер не подтвердил отзыв. Отзовите токен также в настройках аккаунта.';

  @override
  String get integrationsGitReasonPending => 'Ждём подтверждения в браузере…';

  @override
  String integrationsGitReasonUnknown(String reason) {
    return 'Не удалось подключить: $reason';
  }

  @override
  String get integrationsGitRetry => 'Повторить';

  @override
  String integrationsGitLoadFailed(String message) {
    return 'Не удалось загрузить интеграции: $message';
  }

  @override
  String integrationsGitConnectedHost(String host) {
    return 'Хост: $host';
  }

  @override
  String integrationsGitConnectedAccount(String login) {
    return 'Аккаунт: $login';
  }

  @override
  String integrationsGitBrowserOpenFailed(String url) {
    return 'Не удалось открыть браузер. Откройте URL вручную: $url';
  }

  @override
  String get integrationsGitlabHostDialogTitle =>
      'Подключить self-hosted GitLab';

  @override
  String get integrationsGitlabHostFieldHost => 'Хост GitLab (https://…)';

  @override
  String get integrationsGitlabHostFieldClientId => 'Application ID';

  @override
  String get integrationsGitlabHostFieldClientSecret => 'Application Secret';

  @override
  String get integrationsGitlabHostFieldHostHint =>
      'Сохраняется как есть. Только https (или http для локальной разработки).';

  @override
  String get integrationsGitlabHostFieldSecretHint =>
      'Шифруется AES-256-GCM в базе.';

  @override
  String get integrationsGitlabHostFieldScopes => 'Scopes';

  @override
  String get integrationsGitlabHostFieldScopesHint =>
      'Должны совпадать со scope, включёнными в вашем OAuth-приложении. \'api\' покрывает всё; для гранулярных приложений, например, \'read_api read_repository write_repository\'.';

  @override
  String get integrationsGitlabHostValidationScopesRequired =>
      'Укажите хотя бы один scope';

  @override
  String get integrationsGitlabHostValidationHostRequired =>
      'Укажите URL вашего GitLab';

  @override
  String get integrationsGitlabHostValidationHostScheme =>
      'Хост должен начинаться с https:// (или http:// для локальной разработки)';

  @override
  String get integrationsGitlabHostValidationHostFormat =>
      'Неверный формат URL';

  @override
  String get integrationsGitlabHostValidationClientIdRequired =>
      'Укажите Application ID';

  @override
  String get integrationsGitlabHostValidationClientSecretRequired =>
      'Укажите Application Secret';

  @override
  String get integrationsGitlabHostInstructionsToggle =>
      'Как зарегистрировать Application в моём GitLab';

  @override
  String get integrationsGitlabHostInstructionsStep1 =>
      'Откройте https://<ваш-gitlab-host>/-/user_settings/applications.';

  @override
  String get integrationsGitlabHostInstructionsStep2 =>
      'Жмите «Add new application».';

  @override
  String integrationsGitlabHostInstructionsStep3(String redirectUri) {
    return 'Name: PolyMaths. Redirect URI: $redirectUri.';
  }

  @override
  String get integrationsGitlabHostInstructionsStep4 =>
      'Отметьте Confidential. Scope: api (покрывает clone, push и merge request\'ы).';

  @override
  String get integrationsGitlabHostInstructionsStep5 =>
      'Сохраните, скопируйте Application ID и Secret, вставьте их выше.';

  @override
  String get integrationsGitlabHostSubmitCta => 'Подключить';

  @override
  String get integrationsGitlabHostCancelCta => 'Отмена';

  @override
  String get integrationsComingSoonChip => 'Скоро';

  @override
  String get llmProviderClaudeCode => 'Claude Code';

  @override
  String get llmProviderAntigravity => 'Antigravity';

  @override
  String get llmProviderAntigravityOAuth => 'Подписка Antigravity';

  @override
  String get llmProviderAnthropic => 'Anthropic';

  @override
  String get llmProviderOpenAi => 'OpenAI';

  @override
  String get llmProviderOpenRouter => 'OpenRouter';

  @override
  String get llmProviderDeepSeek => 'DeepSeek';

  @override
  String get llmProviderZhipu => 'Zhipu';

  @override
  String get llmProviderHermes => 'Hermes';

  @override
  String get llmProviderClaudeCodeSubtitle => 'Подписка Anthropic через OAuth';

  @override
  String get llmProviderAntigravitySubtitle => 'Прямой API-ключ Antigravity';

  @override
  String get llmProviderAntigravityOAuthSubtitle =>
      'Подписка Antigravity через OAuth';

  @override
  String get llmProviderAnthropicSubtitle => 'Прямой API-ключ Anthropic';

  @override
  String get llmProviderOpenAiSubtitle => 'GPT-4, GPT-4o, o-серия';

  @override
  String get llmProviderOpenRouterSubtitle => 'Мульти-провайдерный агрегатор';

  @override
  String get llmProviderDeepSeekSubtitle => 'DeepSeek Chat и Coder';

  @override
  String get llmProviderZhipuSubtitle => 'Модели GLM';

  @override
  String get llmProviderHermesSubtitle =>
      'Прямое подключение Nous Portal / Hermes API';

  @override
  String get integrationsLlmStage2Subtitle =>
      'Управление API-ключами и OAuth-подписками для код-агентов.';

  @override
  String get integrationsLlmSectionConnected => 'Подключённые';

  @override
  String get integrationsLlmSectionAvailable => 'Доступные';

  @override
  String get integrationsLlmConnectCta => 'Подключить';

  @override
  String get integrationsLlmDisconnectCta => 'Отключить';

  @override
  String get integrationsLlmReplaceCta => 'Сменить ключ';

  @override
  String get integrationsLlmEmptyAvailable =>
      'Все поддерживаемые провайдеры уже подключены.';

  @override
  String get integrationsLlmReasonUserCancelled =>
      'Доступ отклонён. Попробуйте снова.';

  @override
  String get integrationsLlmReasonExpired => 'Сессия устарела. Начните заново.';

  @override
  String get integrationsLlmReasonProviderUnreachable =>
      'Провайдер недоступен. Повторите позже.';

  @override
  String integrationsLlmReasonUnknown(String reason) {
    return 'Не удалось подключить: $reason';
  }

  @override
  String get integrationsLlmReasonPending => 'Ждём подтверждения в браузере…';

  @override
  String get integrationsLlmRetry => 'Попробовать снова';

  @override
  String integrationsLlmDialogApiKeyTitle(String provider) {
    return 'Подключение $provider';
  }

  @override
  String get integrationsLlmDialogApiKeyField => 'API-ключ';

  @override
  String get integrationsLlmDialogApiKeyHint =>
      'Хранится зашифрованным (AES-256-GCM).';

  @override
  String get integrationsLlmClaudeCodeManualTitle => 'Ввести токен Claude Code';

  @override
  String get integrationsLlmClaudeCodeManualHint =>
      'Используйте, если OAuth-приложение Anthropic ещё не настроено или у вас уже есть готовый setup-token.';

  @override
  String get integrationsLlmClaudeCodeManualAccessField =>
      'Access token (sk-ant-oat01-...)';

  @override
  String get integrationsLlmClaudeCodeManualRefreshField =>
      'Refresh token (опционально)';

  @override
  String get integrationsLlmClaudeCodeManualCta => 'Использовать готовый токен';

  @override
  String get integrationsLlmClaudeCodeManualAccessRequired =>
      'Введите непустой access token';

  @override
  String get integrationsLlmAntigravityManualTitle =>
      'Ввести токен Antigravity';

  @override
  String get integrationsLlmAntigravityManualHint =>
      'Используйте, если OAuth-приложение Antigravity ещё не настроено или у вас уже есть готовый токен.';

  @override
  String get integrationsLlmAntigravityManualAccessField => 'Access token';

  @override
  String get integrationsLlmAntigravityManualRefreshField =>
      'Refresh token (опционально)';

  @override
  String get integrationsLlmAntigravityManualCta =>
      'Использовать готовый токен';

  @override
  String get integrationsLlmAntigravityManualAccessRequired =>
      'Введите непустой access token';

  @override
  String get integrationsLlmDialogApiKeyRequired => 'Введите непустой API-ключ';

  @override
  String get integrationsLlmDialogCancel => 'Отмена';

  @override
  String get integrationsLlmDialogSave => 'Сохранить';

  @override
  String get integrationsLlmClaudeCodeOAuthTitle => 'Подключение Claude Code';

  @override
  String get integrationsLlmClaudeCodeOAuthStep1 =>
      'Откройте Anthropic в браузере, введите код ниже и подтвердите вход.';

  @override
  String get integrationsLlmClaudeCodeOpenBrowser => 'Открыть браузер';

  @override
  String get integrationsLlmClaudeCodeOAuthCode => 'Код:';

  @override
  String get integrationsLlmClaudeCodeOAuthCopy => 'Скопировать код';

  @override
  String get integrationsLlmClaudeCodeOAuthWaiting =>
      'Ждём подтверждения… Можно закрыть это окно и вернуться позже — статус обновится автоматически.';

  @override
  String get integrationsLlmClaudeCodeOAuthTimeout =>
      'Авторизация истекла через 20 минут. Попробуйте снова.';

  @override
  String get integrationsLlmAntigravityOAuthTitle => 'Подключение Antigravity';

  @override
  String get integrationsLlmAntigravityOAuthStep1 =>
      'Откройте Antigravity в браузере, введите код ниже и подтвердите вход.';

  @override
  String get integrationsLlmAntigravityOpenBrowser => 'Открыть браузер';

  @override
  String get integrationsLlmAntigravityOAuthCode => 'Код:';

  @override
  String get integrationsLlmAntigravityOAuthCopy => 'Скопировать код';

  @override
  String get integrationsLlmAntigravityOAuthWaiting =>
      'Ждём подтверждения… Можно закрыть это окно и вернуться позже — статус обновится автоматически.';

  @override
  String get integrationsLlmAntigravityOAuthTimeout =>
      'Авторизация истекла через 20 минут. Попробуйте снова.';

  @override
  String integrationsLlmLoadFailed(String message) {
    return 'Не удалось загрузить интеграции: $message';
  }

  @override
  String dashboardWelcomeUser(String email) {
    return 'Добро пожаловать, $email';
  }

  @override
  String get dashboardWelcomeAnon => 'Добро пожаловать';

  @override
  String get dashboardHubSubtitle =>
      'Сводка по проектам, агентам и интеграциям.';

  @override
  String dashboardStatProjectsActive(int n) {
    String _temp0 = intl.Intl.pluralLogic(
      n,
      locale: localeName,
      other: '$n активных',
      many: '$n активных',
      few: '$n активных',
      one: '1 активный',
      zero: 'Нет активных',
    );
    return '$_temp0';
  }

  @override
  String dashboardStatProjectsTotal(int n) {
    String _temp0 = intl.Intl.pluralLogic(
      n,
      locale: localeName,
      other: 'Всего $n проектов',
      many: 'Всего $n проектов',
      few: 'Всего $n проекта',
      one: 'Всего 1 проект',
      zero: 'Всего проектов нет',
    );
    return '$_temp0';
  }

  @override
  String dashboardStatAgentsTotal(int n) {
    String _temp0 = intl.Intl.pluralLogic(
      n,
      locale: localeName,
      other: '$n агентов',
      many: '$n агентов',
      few: '$n агента',
      one: '1 агент',
      zero: 'Нет агентов',
    );
    return '$_temp0';
  }

  @override
  String dashboardStatLlmConnected(int n) {
    String _temp0 = intl.Intl.pluralLogic(
      n,
      locale: localeName,
      other: '$n подключений',
      many: '$n подключений',
      few: '$n подключения',
      one: '1 подключение',
      zero: 'Не подключено',
    );
    return '$_temp0';
  }

  @override
  String dashboardStatGitConnected(int n) {
    String _temp0 = intl.Intl.pluralLogic(
      n,
      locale: localeName,
      other: '$n подключений',
      many: '$n подключений',
      few: '$n подключения',
      one: '1 подключение',
      zero: 'Не подключено',
    );
    return '$_temp0';
  }

  @override
  String get dashboardStatManageCta => 'Управлять';

  @override
  String get dashboardStatComingSoon => 'Доступно на следующем этапе';

  @override
  String get dashboardRecentTasksTitle => 'Последние задачи';

  @override
  String get dashboardRecentTasksEmptyTitle => 'Задач пока нет';

  @override
  String get dashboardRecentTasksEmptySubtitle =>
      'Создайте проект и добавьте задачи — они появятся здесь.';

  @override
  String get dashboardRecentTasksError =>
      'Не удалось загрузить последние задачи.';

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
  String get dashboardAdminAgentsV2 => 'Агенты (v2)';

  @override
  String get dashboardAdminWorktrees => 'Worktrees (debug)';

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
  String get errorExternalService =>
      'Не удалось обратиться к внешнему сервису.';

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
      'Ниже — ключи доступа к приложению PolyMaths (MCP). Это не ключи LLM-провайдеров.';

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
  String get projectDashboardChat => 'Дашборд';

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
  String get repositoriesSectionTitle => 'Репозитории';

  @override
  String get repositoriesSectionSubtitle =>
      'Git-репозитории проекта. Decomposer раскладывает подзадачи по репозиториям.';

  @override
  String get repositoriesAddButton => 'Добавить репозиторий';

  @override
  String get repositoriesEmpty => 'Репозиториев пока нет';

  @override
  String get repositoryPrimaryBadge => 'primary';

  @override
  String get repositoryFieldSlug => 'Slug';

  @override
  String get repositoryFieldSlugHint => 'например ui, core, infra';

  @override
  String get repositoryFieldDisplayName => 'Отображаемое имя';

  @override
  String get repositoryFieldUrl => 'Git URL';

  @override
  String get repositoryFieldBranch => 'Ветка по умолчанию';

  @override
  String get repositoryFieldProvider => 'Git-провайдер';

  @override
  String get repositoryFieldRole => 'Роль (для decomposer)';

  @override
  String get repositoryFieldRoleHint =>
      'например Flutter UI, высоконагруженный Go-бэкенд';

  @override
  String get repositoryAddDialogTitle => 'Добавить репозиторий';

  @override
  String get repositoryAddSubmit => 'Добавить';

  @override
  String get repositoryRemoveTooltip => 'Удалить репозиторий';

  @override
  String get repositoryRemoveConfirmTitle => 'Удалить репозиторий?';

  @override
  String repositoryRemoveConfirmBody(String slug) {
    return 'Репозиторий «$slug» будет откреплён от проекта.';
  }

  @override
  String get repositoryRemoveConfirmAction => 'Удалить';

  @override
  String get repositoryLastIndexedLabel => 'Последняя индексация';

  @override
  String get gitAccountSectionTitle => 'Git-аккаунт';

  @override
  String get gitAccountFieldLabel => 'Аккаунт';

  @override
  String get gitAccountHelper =>
      'Какой подключённый аккаунт использовать для клонирования и pull request\'ов.';

  @override
  String get gitAccountNoneHint =>
      'Нет подключённых аккаунтов этого провайдера. Подключите в «Git-провайдеры».';

  @override
  String get gitAccountDefaultOption =>
      'По умолчанию (первый подключённый аккаунт)';

  @override
  String get integrationsGitConnectAnotherCta => 'Подключить ещё аккаунт';

  @override
  String get integrationsGitAccountsSectionTitle => 'Подключённые аккаунты';

  @override
  String get integrationsGitDisconnectAccountTooltip =>
      'Отключить этот аккаунт';

  @override
  String get createProjectAccountLabel => 'Git-аккаунт';

  @override
  String get createProjectAccountLocal => 'Локально (без git)';

  @override
  String get createProjectAccountNoneHint =>
      'Нет подключённых аккаунтов. Подключите в «Git-провайдеры», чтобы выбирать репозитории.';

  @override
  String get projectSettingsSectionVector => 'Векторный индекс';

  @override
  String get projectSettingsSectionTechStack => 'Технологический стек';

  @override
  String get projectSettingsGitDefaultBranchLabel => 'Ветка по умолчанию';

  @override
  String get projectSettingsBranchNamingTitle => 'Именование веток';

  @override
  String get projectSettingsBranchTemplateLabel => 'Шаблон имени ветки';

  @override
  String get projectSettingsBranchTemplateHint =>
      'Плейсхолдеры: ticket, slug, short_id, id, date. Fallback вида ticket|short_id. Пусто — дефолтная ветка.';

  @override
  String get projectSettingsBranchPatternLabel =>
      'Жёсткий формат (regex, опционально)';

  @override
  String get projectSettingsBranchPatternHint =>
      'Валидирует ручные override имени ветки. Пусто — выводится из шаблона.';

  @override
  String get projectSettingsBranchLockLabel =>
      'Запретить ручной override ветки';

  @override
  String get projectSettingsBranchLockSubtitle =>
      'Имя ветки только генерируется из шаблона';

  @override
  String get projectSettingsBranchPreviewLabel => 'Превью';

  @override
  String get projectSettingsMrTitleTitle => 'Название MR / PR';

  @override
  String get projectSettingsMrTitleLabel => 'Шаблон названия MR';

  @override
  String get projectSettingsMrTitleHint =>
      'Плейсхолдеры: title, ticket, slug, branch, repo, short_id, date. Пусто — PolyMaths: title.';

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
  String get projectSettingsTabGeneral => 'Основные';

  @override
  String get projectSettingsTabVariables => 'Переменные (тех. стек)';

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
  String get projectSettingsIndexingStatusLabel => 'Статус индексации';

  @override
  String get projectSettingsLastIndexedCommitLabel =>
      'Последний проиндексированный коммит';

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
  String get taskStatusActive => 'В работе';

  @override
  String get taskStatusDone => 'Готово';

  @override
  String get taskStatusNeedsHuman => 'Нужна помощь';

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
  String get taskDetailSectionOutcome => 'Итог';

  @override
  String get taskDetailSectionSubtasks => 'Подзадачи';

  @override
  String get taskDetailSectionSandboxLogs => 'Логи песочницы (Realtime)';

  @override
  String get taskDetailSandboxLogsEmpty =>
      'Логи песочницы пока отсутствуют. Они появятся здесь в реальном времени при запуске агента (Developer/Tester).';

  @override
  String get taskDetailSandboxLogsClear => 'Очистить';

  @override
  String get taskDetailSandboxLogsCopy => 'Копировать';

  @override
  String get taskDetailSandboxLogsCopied => 'Логи скопированы в буфер обмена';

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
  String get taskActionAlreadyTerminalSnack => 'Задача уже завершена';

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
  String get agentRoleDecomposer => 'Декомпозер';

  @override
  String get agentRoleMerger => 'Мерджер';

  @override
  String get agentRoleRouter => 'Роутер';

  @override
  String get agentRoleAssistant => 'Ассистент';

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
  String get teamAgentEditPromptSystemDefaultHardcoded =>
      'Системный по умолчанию (захардкожен)';

  @override
  String get teamAgentEditPromptSystemDefaultHardcodedHelp =>
      'Используется системный промпт по умолчанию (захардкожен)';

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
  String get teamAgentEditTestRun => 'Тестовый запуск';

  @override
  String get teamAgentEditTestRunSuccess => 'Тестовая задача успешно запущена';

  @override
  String get teamAgentEditTestRunError =>
      'Не удалось запустить тестовую задачу';

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
  String get globalSettingsTabDevTeam => 'PolyMaths';

  @override
  String get assistantScopeGlobal => 'Глобальный чат';

  @override
  String assistantScopeProject(String name) {
    return 'Проект: $name';
  }

  @override
  String get assistantPromptUserTabTitle => 'Ассистент';

  @override
  String get assistantPromptUserHeading =>
      'Промпт ассистента (уровень пользователя)';

  @override
  String get assistantPromptUserHint =>
      'Базовый системный промпт вашего ассистента. Новые проекты наследуют его копию на момент создания — последующие правки здесь на уже созданные проекты не влияют.';

  @override
  String get assistantPromptProjectTabTitle => 'Ассистент';

  @override
  String get assistantPromptProjectHeading =>
      'Промпт ассистента (уровень проекта)';

  @override
  String get assistantPromptProjectHint =>
      'Системный промпт ассистента в этом проекте. Это независимая копия — она замещает пользовательский промпт. Пустое поле возвращает пользовательский промпт.';

  @override
  String get assistantPromptInherited =>
      'Этот проект ещё не имеет собственного промпта — используется пользовательский. Сохраните, чтобы создать копию проекта.';

  @override
  String get assistantPromptSave => 'Сохранить промпт';

  @override
  String get assistantPromptReset => 'Сбросить к пользовательскому';

  @override
  String get assistantPromptSaved => 'Промпт ассистента сохранён';

  @override
  String get assistantPromptSaveError =>
      'Не удалось сохранить промпт ассистента';

  @override
  String get assistantPromptLoadError =>
      'Не удалось загрузить промпт ассистента';

  @override
  String get llmProvidersSectionTitle => 'LLM-провайдеры';

  @override
  String get llmProvidersAdd => 'Добавить';

  @override
  String get llmProvidersEmpty => 'Провайдеры ещё не настроены.';

  @override
  String get llmProvidersLoadError => 'Не удалось загрузить список провайдеров';

  @override
  String get llmProvidersAdminRequired =>
      'Для управления LLM-провайдерами требуются права администратора';

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
      'Откройте ссылку ниже в любом браузере и введите этот код, чтобы авторизовать PolyMaths:';

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
  String get agentSandboxSettingsAttachServicesLabel =>
      'Подключать тест-сервисы проекта';

  @override
  String get agentSandboxSettingsAttachServicesHelper =>
      'Поднимать эфемерные тестовые сервисы проекта (например PostgreSQL) для sandbox-прогонов этого агента. Обычно включается у tester.';

  @override
  String get agentSandboxSettingsCodeBackendLabel => 'Code backend';

  @override
  String get agentSandboxSettingsMCPHelper =>
      'JSON-массив MCP-серверов. Поля инлайн-сервера: name, type (sse/http/stdio), url, headers. В значении заголовка можно сослаться на секрет проекта (синтаксис — в примере выше): он подставляется в рантайме и не пишется в файл. Секреты задаются во вкладке «Переменные».';

  @override
  String get agentSandboxSettingsSkillsHelper =>
      'JSON-массив Skills (Claude Code / Antigravity / Hermes). Поля: name, source (builtin/plugin/path; hermes: builtin/agentskills/path), config.files — карта относительных путей (SKILL.md обязателен, плюс scripts и т.д.) к содержимому файлов. Файлы копируются в sandbox до старта; скрипты агент запускает через bash/python.';

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
  String get agentSandboxSettingsTabToolsets => 'Toolsets';

  @override
  String get agentSandboxSettingsHermesToolsetsLabel => 'Hermes toolsets';

  @override
  String get agentSandboxSettingsHermesToolsetsHelper =>
      'Выберите, какие Hermes toolsets доступны агенту.';

  @override
  String get agentSandboxSettingsHermesPermLabel => 'Permission mode';

  @override
  String get agentSandboxSettingsHermesPermHelper =>
      'В headless-sandbox разрешены только yolo и accept.';

  @override
  String get agentSandboxSettingsHermesMaxTurnsLabel => 'Макс. число шагов';

  @override
  String get agentSandboxSettingsHermesTemperatureLabel => 'Temperature (опц.)';

  @override
  String get agentSandboxRevokeConfirmTitle => 'Отозвать подписку Claude Code?';

  @override
  String get agentSandboxRevokeConfirmBody =>
      'Агенты будут использовать ANTHROPIC_API_KEY (если задан) при следующих sandbox-задачах. Подписку можно подключить заново в любой момент.';

  @override
  String get teamAgentEditAdvanced => 'Дополнительно';

  @override
  String get commonRequestFailed => 'Ошибка запроса';

  @override
  String get commonRequiredField => 'Обязательное поле';

  @override
  String get commonCancel => 'Отмена';

  @override
  String get commonSave => 'Сохранить';

  @override
  String get commonCreate => 'Создать';

  @override
  String get commonConfirm => 'Подтвердить';

  @override
  String get agentsV2Title => 'Агенты (v2)';

  @override
  String get agentsV2Empty => 'Пока нет ни одного агента.';

  @override
  String get agentsV2Refresh => 'Обновить';

  @override
  String get agentsV2CreateButton => 'Новый агент';

  @override
  String get agentsV2CreateTitle => 'Создать агента';

  @override
  String get agentsV2DetailTitle => 'Агент';

  @override
  String get agentsV2KindLlm => 'LLM';

  @override
  String get agentsV2KindSandbox => 'Sandbox';

  @override
  String get agentsV2FieldId => 'ID';

  @override
  String get agentsV2FieldName => 'Имя';

  @override
  String get agentsV2FieldRole => 'Роль';

  @override
  String get agentsV2FieldExecutionKind => 'Тип исполнения';

  @override
  String get agentsV2FieldRoleDescription =>
      'Описание роли (попадает в промпт Router\'а)';

  @override
  String get agentsV2FieldSystemPrompt => 'Системный промпт';

  @override
  String get agentsV2FieldModel => 'Модель';

  @override
  String get agentsV2FieldTemperature => 'Температура';

  @override
  String get agentsV2FieldMaxTokens => 'Max tokens';

  @override
  String get agentsV2FieldCodeBackend => 'Code backend';

  @override
  String get agentsV2FieldIsActive => 'Активен';

  @override
  String get agentsV2SectionConfig => 'Конфигурация';

  @override
  String get agentsV2AddSecretButton => 'Добавить / обновить секрет';

  @override
  String get agentsV2SavedSnackbar => 'Агент сохранён.';

  @override
  String get agentsV2SecretSaved => 'Секрет сохранён (зашифрован).';

  @override
  String get agentsV2SecretDialogTitle => 'Установить секрет агента';

  @override
  String get agentsV2SecretKeyName => 'Имя ключа';

  @override
  String get agentsV2SecretValue => 'Значение';

  @override
  String get agentsV2SecretValueHelper =>
      'Шифруется AES-256-GCM. Прочитать обратно нельзя — введите заново для ротации.';

  @override
  String get agentsV2SecretsHint =>
      'Секреты хранятся зашифрованными на сервере и никогда не возвращаются клиенту. Используйте кнопку выше, чтобы установить/обновить значение.';

  @override
  String get tasksCancelButton => 'Отменить задачу';

  @override
  String get tasksCancelConfirmTitle => 'Отменить задачу?';

  @override
  String get tasksCancelConfirmBody =>
      'Все активные агенты будут прерваны, задача переведётся в статус cancelled.';

  @override
  String get tasksCancelInflightSuccess =>
      'Отмена отправлена. Агенты остановятся в ближайшее время.';

  @override
  String get tasksCustomTimeoutLabel =>
      'Свой таймаут (например 4h, 90m, 3600s)';

  @override
  String get tasksCustomTimeoutHelper =>
      'Переопределяет дефолтные 4 часа оркестрации. Мин 1m, макс 72h.';

  @override
  String get tasksCustomTimeoutInvalid =>
      'Некорректный формат. Используйте Nh / Nm / Ns.';

  @override
  String get tasksCustomTimeoutSectionTitle => 'Таймаут';

  @override
  String get tasksCustomTimeoutNone => 'По умолчанию (4h)';

  @override
  String get tasksCustomTimeoutEdit => 'Изменить';

  @override
  String get tasksExternalKeyTitle => 'Ключ тикета';

  @override
  String get tasksExternalKeyNone => 'нет';

  @override
  String get tasksExternalKeyEdit => 'Изменить ключ тикета';

  @override
  String get tasksExternalKeyLabel => 'Ключ тикета';

  @override
  String get tasksExternalKeyHelper =>
      'Напр. DEV-123. Буквы, цифры, дефис и подчёркивание, до 64 символов.';

  @override
  String get tasksExternalKeyInvalid => 'Неверный формат ключа тикета';

  @override
  String get tasksExternalKeySave => 'Сохранить';

  @override
  String get tasksExternalKeySavedSnack => 'Ключ тикета сохранён';

  @override
  String get tasksExternalKeyClearedSnack => 'Ключ тикета сброшен';

  @override
  String get tasksCustomTimeoutSave => 'Сохранить';

  @override
  String get tasksCustomTimeoutClear => 'Сбросить к дефолту';

  @override
  String get tasksCustomTimeoutClearDialogTitle => 'Сбросить таймаут?';

  @override
  String get tasksCustomTimeoutClearDialogBody =>
      'Оркестратор откатится к глобальным 4 часам по умолчанию для этой задачи.';

  @override
  String get tasksCustomTimeoutSavedSnack => 'Таймаут обновлён.';

  @override
  String get tasksCustomTimeoutClearedSnack => 'Таймаут сброшен к дефолту.';

  @override
  String get worktreesTitle => 'Worktrees (отладка)';

  @override
  String get worktreesEmpty => 'Нет активных worktree\'ов.';

  @override
  String get worktreesColTask => 'Задача';

  @override
  String get worktreesColBranch => 'Ветка';

  @override
  String get worktreesColState => 'Статус';

  @override
  String get worktreesColAllocated => 'Создан';

  @override
  String get worktreesReleaseButton => 'Принудительно освободить';

  @override
  String get worktreesReleasedSnackbar => 'Worktree освобождён.';

  @override
  String get worktreesReleaseDialogTitle =>
      'Принудительно освободить worktree?';

  @override
  String get worktreesReleaseDialogBody =>
      'git worktree remove --force произойдёт прямо сейчас. Агент (если работает) потеряет рабочий каталог и незакоммиченные изменения.';

  @override
  String get worktreesReleaseAlreadyReleased => 'Worktree уже был освобождён.';

  @override
  String get worktreesReleaseFailed => 'Не удалось освободить worktree.';

  @override
  String get worktreesReleaseNotConfigured =>
      'Worktree manager не сконфигурирован на сервере (WORKTREES_ROOT / REPO_ROOT не заданы). Попросите оператора включить фичу.';

  @override
  String get worktreesFilterAll => 'Все';

  @override
  String get worktreesFilterAllocated => 'Allocated';

  @override
  String get worktreesFilterInUse => 'In use';

  @override
  String get worktreesFilterReleased => 'Released';

  @override
  String get routerTimelineSection => 'Лента решений Router\'а';

  @override
  String get routerTimelineEmpty => 'Решений Router\'а пока нет.';

  @override
  String get artifactsSection => 'Артефакты';

  @override
  String get artifactsEmpty => 'Артефактов пока нет.';

  @override
  String get artifactViewerOpen => 'Открыть артефакт полностью';

  @override
  String artifactViewerTitle(String kind, String idShort) {
    return '$kind · $idShort';
  }

  @override
  String get artifactViewerClose => 'Закрыть';

  @override
  String get artifactViewerCopyFull => 'Скопировать всё содержимое';

  @override
  String artifactViewerCopyFullForKind(String kind) {
    return 'Скопировать весь $kind';
  }

  @override
  String artifactViewerCopiedSnack(int bytes) {
    return 'Скопировано $bytes байт в буфер обмена.';
  }

  @override
  String get artifactViewerCopyFailedSnack =>
      'Не удалось скопировать в буфер обмена.';

  @override
  String artifactViewerShowFull(int kb) {
    return 'Показать полностью ($kb КБ)';
  }

  @override
  String artifactViewerShowNext(int n) {
    return 'Показать следующие $n';
  }

  @override
  String artifactViewerTruncatedNotice(int kb, int totalKb) {
    return 'Показаны первые $kb КБ из $totalKb КБ.';
  }

  @override
  String get artifactViewerEmpty => 'У артефакта нет сохранённого содержимого.';

  @override
  String artifactViewerLoadFailed(String error) {
    return 'Не удалось загрузить артефакт: $error';
  }

  @override
  String get artifactViewerReviewDecision => 'Решение';

  @override
  String get artifactViewerReviewIssues => 'Замечания';

  @override
  String get artifactViewerReviewSummary => 'Итог';

  @override
  String get artifactViewerReviewNoIssues => 'Замечаний нет.';

  @override
  String get artifactViewerTestPassed => 'Прошло';

  @override
  String get artifactViewerTestFailed => 'Упало';

  @override
  String get artifactViewerTestSkipped => 'Пропущено';

  @override
  String get artifactViewerTestDuration => 'Длительность';

  @override
  String artifactViewerTestDurationMs(int ms) {
    return '$ms мс';
  }

  @override
  String artifactViewerTestFailuresHeader(int n) {
    return 'Падения ($n)';
  }

  @override
  String artifactViewerTestFailureFile(String file, int line) {
    return '$file:$line';
  }

  @override
  String get artifactViewerTestNoFailures => 'Все проверки зелёные.';

  @override
  String get artifactViewerTestVerdict => 'Вердикт';

  @override
  String get artifactViewerTestAcceptance => 'Критерии приёмки';

  @override
  String get artifactViewerTestChecks => 'Проверки';

  @override
  String get artifactsNoSummary => '(без описания)';

  @override
  String get artifactViewerTestUnnamed => '(без имени)';

  @override
  String artifactViewerFullTitle(String kind) {
    return '$kind · полностью';
  }

  @override
  String get assistantSidebarTitle => 'Ассистент';

  @override
  String get assistantTabChat => 'Чат';

  @override
  String get assistantTabTasks => 'Задачи';

  @override
  String get assistantEmptyChat =>
      'Спросите ассистента о проектах, задачах или настройках.';

  @override
  String get assistantInputHint => 'Сообщение ассистенту…';

  @override
  String get assistantSend => 'Отправить';

  @override
  String get assistantStop => 'Остановить';

  @override
  String get assistantCopyMessage => 'Копировать';

  @override
  String get assistantCopied => 'Скопировано';

  @override
  String get assistantConfirmTitle => 'Подтвердите действие';

  @override
  String get assistantConfirmApprove => 'Подтвердить';

  @override
  String get assistantConfirmDeny => 'Отклонить';

  @override
  String get assistantNoActiveTasks => 'Нет активных задач ни в одном проекте.';

  @override
  String get assistantActiveTaskInProgress => 'В работе';

  @override
  String get assistantToggleTooltip => 'Скрыть/показать ассистента';

  @override
  String get assistantSessionBusy => 'Ассистент работает…';

  @override
  String get assistantSessionStale =>
      'Сессия не отвечает — попробуйте чуть позже.';

  @override
  String assistantToolCallTitle(String tool) {
    return 'Инструмент $tool';
  }

  @override
  String get assistantToolResultStatusOk => 'OK';

  @override
  String get assistantToolResultStatusForbidden => 'Запрещено';

  @override
  String get assistantToolResultStatusError => 'Ошибка';

  @override
  String get assistantToolResultStatusDenied => 'Отклонено';

  @override
  String get assistantToolResultStatusTruncated => 'Усечено';

  @override
  String get assistantToolResultStatusPending => 'Ожидает';

  @override
  String get assistantToolResultLabel => 'Результат';

  @override
  String get assistantToolArgumentsLabel => 'Аргументы';

  @override
  String get assistantNewSession => 'Новый чат';

  @override
  String get assistantSessionUntitled => 'Без названия';

  @override
  String get assistantOpenTask => 'Открыть';

  @override
  String get assistantLoadOlder => 'Загрузить ещё';

  @override
  String get assistantRetry => 'Повторить';

  @override
  String get assistantErrorGeneric =>
      'Что-то пошло не так. Попробуйте ещё раз.';

  @override
  String assistantConfirmSummaryFallback(String tool) {
    return 'Ассистент хочет вызвать $tool. Подтвердите, чтобы продолжить.';
  }

  @override
  String get assistantMessageRoleUser => 'Вы';

  @override
  String get assistantMessageRoleAssistant => 'Ассистент';

  @override
  String get assistantMessageRoleSystem => 'Система';

  @override
  String get assistantLockScreenMessage =>
      'Ассистент не настроен. Для начала работы укажите ключи доступа к LLM.';

  @override
  String get assistantLockScreenButton => 'Перейти к настройке ключей';

  @override
  String get assistantTaskStateActive => 'В работе';

  @override
  String get assistantTaskStateDone => 'Готово';

  @override
  String get assistantTaskStateFailed => 'Ошибка';

  @override
  String get assistantTaskStateCancelled => 'Отменена';

  @override
  String get assistantTaskStateNeedsHuman => 'Нужна помощь';

  @override
  String get assistantTaskStatePaused => 'Пауза';

  @override
  String assistantStatusError(String error) {
    return 'Ошибка загрузки статуса: $error';
  }

  @override
  String get assistantStatusAdminSetup =>
      'Ассистент требует настройки администратором.';

  @override
  String get navMcpServers => 'MCP-серверы';

  @override
  String get navRolePrompts => 'Промпты ролей';

  @override
  String get agentConfigScreenTitle => 'Настройка агента';

  @override
  String get agentConfigSaveButton => 'Сохранить';

  @override
  String get agentConfigLoadError => 'Не удалось загрузить конфигурацию агента';

  @override
  String get agentConfigActiveLabel => 'Активен';

  @override
  String get agentConfigActiveOn => 'Агент активен и может получать задачи';

  @override
  String get agentConfigActiveOff =>
      'Агент неактивен и не будет получать задачи';

  @override
  String get agentConfigRoleSectionTitle => 'Роль';

  @override
  String get agentConfigTypeSectionTitle => 'Тип выполнения';

  @override
  String get agentConfigLLMSectionTitle => 'Настройки LLM';

  @override
  String get agentConfigMCPSectionTitle => 'MCP-инструменты';

  @override
  String get agentConfigSkillsSectionTitle => 'Навыки';

  @override
  String get agentConfigRoleLabel => 'Роль';

  @override
  String get agentConfigRoleReadOnly => 'Авто-созданная роль (только чтение)';

  @override
  String get agentConfigTypeAPI => 'API (LLM)';

  @override
  String get agentConfigTypeSandbox => 'Sandbox';

  @override
  String get agentConfigProviderLabel => 'LLM-провайдер';

  @override
  String get agentConfigModelLabel => 'Модель';

  @override
  String get agentConfigModelHint => 'напр. claude-sonnet-4-20250514';

  @override
  String get agentConfigTemperatureLabel => 'Temperature';

  @override
  String get agentConfigTemperatureDefault => 'по умолчанию';

  @override
  String get agentConfigDevTeamMCP => 'PolyMaths MCP';

  @override
  String get agentConfigDevTeamMCPDesc =>
      'Встроенные инструменты PolyMaths (управление задачами, поиск по коду и т.д.)';

  @override
  String get agentConfigExternalMCPTitle => 'Внешние MCP-серверы';

  @override
  String get agentConfigNoExternalMCP => 'Внешние MCP-серверы не настроены';

  @override
  String get agentConfigAddMCPServer => 'Добавить MCP-сервер';

  @override
  String get agentConfigNoSkills => 'Навыки не настроены';

  @override
  String get agentConfigAddSkill => 'Добавить навык';

  @override
  String get agentConfigSaveSuccess => 'Конфигурация агента сохранена';

  @override
  String get agentConfigSaveError => 'Не удалось сохранить конфигурацию агента';

  @override
  String get projectVariablesTitle => 'Переменные проекта';

  @override
  String get projectVariablesHint =>
      'Секреты в рамках проекта. Агенты подставляют их через плейсхолдеры в настройках.';

  @override
  String get projectVariablesLoadError =>
      'Не удалось загрузить секреты проекта';

  @override
  String get projectVariablesEmpty => 'Секретов проекта пока нет';

  @override
  String get projectVariablesAddButton => 'Добавить секрет';

  @override
  String get projectVariablesEditTitle => 'Редактировать секрет';

  @override
  String get projectVariablesAddTitle => 'Добавить секрет';

  @override
  String get projectVariablesKeyLabel => 'Имя ключа';

  @override
  String get projectVariablesKeyRequired => 'Имя ключа обязательно';

  @override
  String get projectVariablesKeyInvalid =>
      'Должен начинаться с A-Z, далее A-Z 0-9 _ (макс. 128)';

  @override
  String get projectVariablesValueLabel => 'Значение';

  @override
  String get projectVariablesValueRequired => 'Значение обязательно';

  @override
  String get projectVariablesInjectLabel => 'Подставлять в песочницу (env)';

  @override
  String get projectVariablesInjectHint =>
      'Значение станет переменной окружения в песочнице, а имя — попадёт в промпт агента как доступная переменная.';

  @override
  String get projectVariablesDescriptionLabel => 'Описание (опц.)';

  @override
  String get projectVariablesEnvBadge => 'env';

  @override
  String get projectVariablesCancelButton => 'Отмена';

  @override
  String get projectVariablesSaveButton => 'Сохранить';

  @override
  String get projectVariablesDeleteTitle => 'Удалить секрет';

  @override
  String get projectVariablesDeleteConfirm => 'Безвозвратно удалить секрет';

  @override
  String get projectVariablesDeleteButton => 'Удалить';

  @override
  String get userVariablesTitle => 'Личные переменные';

  @override
  String get userVariablesHint =>
      'Секреты вашего аккаунта. Доступны всем агентам, работающим от вашего имени.';

  @override
  String get userVariablesLoadError => 'Не удалось загрузить личные секреты';

  @override
  String get userVariablesEmpty => 'Личных секретов пока нет';

  @override
  String get userVariablesAddButton => 'Добавить секрет';

  @override
  String get userVariablesAddTitle => 'Добавить личный секрет';

  @override
  String get userVariablesKeyLabel => 'Имя ключа';

  @override
  String get userVariablesKeyRequired => 'Имя ключа обязательно';

  @override
  String get userVariablesKeyInvalid =>
      'Должен начинаться с A-Z, далее A-Z 0-9 _ (макс. 128)';

  @override
  String get userVariablesValueLabel => 'Значение';

  @override
  String get userVariablesValueRequired => 'Значение обязательно';

  @override
  String get userVariablesCancelButton => 'Отмена';

  @override
  String get userVariablesSaveButton => 'Сохранить';

  @override
  String get userVariablesDeleteTitle => 'Удалить секрет';

  @override
  String get userVariablesDeleteConfirm => 'Безвозвратно удалить секрет';

  @override
  String get userVariablesDeleteButton => 'Удалить';

  @override
  String get mcpRegistryScreenTitle => 'Реестр MCP-серверов';

  @override
  String get mcpRegistryRefreshTooltip => 'Обновить';

  @override
  String get mcpRegistryLoadError => 'Не удалось загрузить MCP-серверы';

  @override
  String get mcpRegistryEmpty => 'MCP-серверы ещё не зарегистрированы';

  @override
  String get mcpRegistryDeleteTitle => 'Удалить MCP-сервер';

  @override
  String get mcpRegistryDeleteConfirm => 'Деактивировать MCP-сервер';

  @override
  String get mcpRegistryCancelButton => 'Отмена';

  @override
  String get mcpRegistryDeleteButton => 'Удалить';

  @override
  String get mcpRegistryAddTitle => 'Добавить MCP-сервер';

  @override
  String get mcpRegistryEditTitle => 'Редактировать MCP-сервер';

  @override
  String get mcpRegistryNameLabel => 'Имя';

  @override
  String get mcpRegistryNameRequired => 'Имя обязательно';

  @override
  String get mcpRegistryDescLabel => 'Описание';

  @override
  String get mcpRegistryTransportLabel => 'Транспорт';

  @override
  String get mcpRegistryCommandLabel => 'Команда';

  @override
  String get mcpRegistryURLLabel => 'URL';

  @override
  String get mcpRegistryScopeLabel => 'Область';

  @override
  String get mcpRegistryActiveLabel => 'Активен';

  @override
  String get mcpRegistrySaveButton => 'Сохранить';

  @override
  String get rolePromptsScreenTitle => 'Промпты ролей агентов';

  @override
  String get rolePromptsRefreshTooltip => 'Обновить';

  @override
  String get rolePromptsLoadError => 'Не удалось загрузить промпты ролей';

  @override
  String get rolePromptsEmpty => 'Промпты ролей ещё не настроены';

  @override
  String get rolePromptsEditTitle => 'Редактировать промпт роли';

  @override
  String get rolePromptsContentLabel => 'Содержание промпта';

  @override
  String get rolePromptsCancelButton => 'Отмена';

  @override
  String get rolePromptsSaveButton => 'Сохранить';

  @override
  String get onboardingConnectLlmProvider =>
      'Для начала работы подключите LLM-провайдера и выберите модель для ассистента. Без этого ассистент не сможет обрабатывать сообщения.';

  @override
  String get onboardingConfigureAssistant =>
      'Ваш ассистент почти готов! Выберите LLM-провайдера и модель в настройках, чтобы начать работу.';

  @override
  String get onboardingGoToSettings => 'Перейти в настройки';

  @override
  String get onboardingConfigureProjectAgents =>
      'Настройте агента router — выберите LLM-провайдера и модель, чтобы запустить оркестрацию задач.';

  @override
  String get onboardingGoToTeam => 'Настроить агентов';

  @override
  String get chatInputVoiceTooltip => 'Голосовой ввод (Alt+V)';

  @override
  String get chatInputVoiceDisabledTooltip =>
      'Голосовой ввод не активен (настройте модель распознавания речи в настройках ассистента)';

  @override
  String chatInputVoiceRecordingHint(int seconds) {
    return 'Идет запись... Говорите ($secondsс). Нажмите Alt+V для завершения';
  }

  @override
  String get agentMatrixTitle => 'Матрица агентов';

  @override
  String get agentMatrixTimelineTab => 'Таймлайн';

  @override
  String get agentMatrixGraphTab => 'Граф';

  @override
  String get agentMatrixStatusPending => 'В ожидании';

  @override
  String get agentMatrixStatusRunning => 'В работе';

  @override
  String get agentMatrixStatusSuccess => 'Успешно';

  @override
  String get agentMatrixStatusFailed => 'Ошибка';

  @override
  String get taskVizTabTrace => 'Трейс';

  @override
  String get taskVizTabFlow => 'Граф';

  @override
  String get taskTraceWaiting => 'Ожидание первого решения роутера…';

  @override
  String get taskTraceRouterLane => 'роутер';

  @override
  String get taskTraceLegendRouter => 'решение роутера';

  @override
  String get taskTraceLegendDependency => 'зависимость';

  @override
  String get taskTraceChanges => 'Нужны правки';

  @override
  String get projectKpiTotal => 'Всего';

  @override
  String get projectKpiActive => 'В работе';

  @override
  String get projectKpiDone => 'Готово';

  @override
  String get projectKpiAttention => 'Внимание';

  @override
  String get projectKpiFailed => 'Ошибки';

  @override
  String get projectTaskFilterAll => 'Все';

  @override
  String get projectTaskFilterIssues => 'Проблемы';

  @override
  String get projectOpenTask => 'Открыть задачу';

  @override
  String get tasksColStatus => 'Статус';

  @override
  String get tasksColTask => 'Задача';

  @override
  String get tasksColPriority => 'Приоритет';

  @override
  String get tasksColAgent => 'Агент';

  @override
  String get tasksColUpdated => 'Обновлено';

  @override
  String get teamAgentProviderNotConnected =>
      'Провайдер не подключён — настройте в Интеграциях';

  @override
  String get teamAgentNoConfiguredProviders =>
      'Нет подключённых провайдеров — настройте в Интеграциях';

  @override
  String get teamAgentBackendRequired => 'Выберите бекенд';

  @override
  String get teamAgentBackendNeedsProvider =>
      'Hermes требует выбранного провайдера';

  @override
  String get teamAgentProviderBackendMismatch =>
      'Провайдер несовместим с выбранным бекендом';

  @override
  String get teamAgentBackendLlmDisabled => 'LLM-роль не использует бекенд';

  @override
  String get teamAgentProviderNotConnectedShort => 'не подключён';

  @override
  String get appShellNavCollapse => 'Свернуть меню';

  @override
  String get appShellNavExpand => 'Развернуть меню';

  @override
  String get agentMatrixInspectorTitle => 'Инспекция агента';

  @override
  String get agentMatrixInspectorSubtasks => 'Подзадачи';

  @override
  String get agentMatrixInspectorLogs => 'Логи';

  @override
  String get agentMatrixInspectorArtifacts => 'Артефакты';

  @override
  String get agentMatrixInspectorNoSubtasks =>
      'Этот агент еще не выполнял подзадач.';

  @override
  String get agentMatrixInspectorNoArtifacts =>
      'Этот агент еще не создал ни одного артефакта.';

  @override
  String get agentMatrixInspectorSelectSubtask => 'Выбор подзадачи';

  @override
  String get agentMatrixInspectorGeneralDiscussion => 'Обсуждение и действия';

  @override
  String get webhooksTitle => 'Вебхуки';

  @override
  String get webhooksEmpty => 'Вебхуки не настроены';

  @override
  String get webhookCreate => 'Создать вебхук';

  @override
  String get webhookEdit => 'Редактировать вебхук';

  @override
  String get webhookName => 'Название';

  @override
  String get webhookNameHint => 'мой-сервис-вебхук';

  @override
  String get webhookRouteTo => 'Направить в';

  @override
  String get webhookRouteProject => 'Чат Проекта';

  @override
  String get webhookRouteTeam => 'Задачу для Команды';

  @override
  String get webhookSelectTeam => 'Выберите Команду';

  @override
  String get webhookInstructions => 'Вводное сообщение (инструкции)';

  @override
  String get webhookInstructionsHint =>
      'Инструкции для агента о том, как обработать этот вебхук';

  @override
  String get webhookDescription => 'Описание';

  @override
  String get webhookDescriptionHint => 'Для чего используется этот вебхук?';

  @override
  String get webhookUrl => 'URL вебхука';

  @override
  String get webhookSecret => 'Секрет';

  @override
  String get webhookRegenerateSecret => 'Сгенерировать новый секрет';

  @override
  String get webhookRequireSecret => 'Требовать подпись (HMAC SHA-256)';

  @override
  String get webhookAllowedIps => 'Разрешенные IP (через запятую)';

  @override
  String get webhookIsActive => 'Активен';

  @override
  String get webhookDeleteConfirm =>
      'Вы уверены, что хотите удалить этот вебхук?';

  @override
  String get webhookDelete => 'Удалить';

  @override
  String get webhookSave => 'Сохранить';

  @override
  String get webhookSaved => 'Вебхук сохранен';

  @override
  String get webhookCreated => 'Вебхук создан';

  @override
  String get webhookRequiredName => 'Название обязательно';

  @override
  String get webhookTaskMappingTitle => 'Настройка задачи (Task Mapping)';

  @override
  String get webhookTaskTitleTemplate => 'Шаблон заголовка';

  @override
  String get webhookTaskTitleTemplateHint => 'Например: [Bug] <issue.title>';

  @override
  String get webhookTaskDescTemplate => 'Шаблон описания';

  @override
  String get webhookTaskDescTemplateHint =>
      'Например: Завел <user.name>\\n\\n<issue.body>';

  @override
  String get webhookTaskPriorityTemplate => 'Шаблон приоритета';

  @override
  String get webhookTaskPriorityTemplateHint =>
      'Например: <issue.priority> (Ожидается: low, medium, high, critical)';

  @override
  String get projectDashboardSchedules => 'Расписание';

  @override
  String get schedulesTitle => 'Регулярные задачи';

  @override
  String get schedulesEmpty => 'Пока нет регулярных задач';

  @override
  String get schedulesAdd => 'Новая регулярная задача';

  @override
  String get schedulesLoadError => 'Не удалось загрузить расписания';

  @override
  String get scheduleActive => 'Активно';

  @override
  String get scheduleInactive => 'Выключено';

  @override
  String get scheduleNextRunLabel => 'Следующий запуск';

  @override
  String get scheduleLastRunLabel => 'Последний запуск';

  @override
  String get scheduleNeverRun => 'ещё не запускалась';

  @override
  String get scheduleEnableTooltip => 'Включить';

  @override
  String get scheduleDisableTooltip => 'Выключить';

  @override
  String get scheduleEdit => 'Редактировать';

  @override
  String get scheduleDelete => 'Удалить';

  @override
  String get scheduleDeleteTitle => 'Удалить расписание?';

  @override
  String get scheduleDeleteMessage =>
      'Расписание будет удалено. Уже созданные задачи останутся.';

  @override
  String get scheduleCreateTitle => 'Новая регулярная задача';

  @override
  String get scheduleEditTitle => 'Редактировать расписание';

  @override
  String get scheduleNameLabel => 'Название';

  @override
  String get scheduleNameHint => 'Ночной рефактор';

  @override
  String get scheduleNameRequired => 'Введите название';

  @override
  String get scheduleDescriptionLabel => 'Описание задачи';

  @override
  String get scheduleDescriptionHint => 'Что нужно сделать в каждой задаче';

  @override
  String get scheduleTeamLabel => 'Команда';

  @override
  String get scheduleTeamNone => 'Без команды';

  @override
  String get schedulePriorityLabel => 'Приоритет';

  @override
  String get scheduleFrequencyLabel => 'Частота';

  @override
  String get scheduleFreqDaily => 'Ежедневно';

  @override
  String get scheduleFreqWeekly => 'Еженедельно';

  @override
  String get scheduleFreqHourly => 'Каждые N часов';

  @override
  String get scheduleFreqCustom => 'Своё (cron)';

  @override
  String get scheduleTimeLabel => 'Время';

  @override
  String get scheduleIntervalHoursLabel => 'Интервал, часов';

  @override
  String get scheduleWeekdaysLabel => 'Дни недели';

  @override
  String get scheduleWeekdaysRequired => 'Выберите хотя бы один день';

  @override
  String get scheduleCronLabel => 'Cron-выражение';

  @override
  String get scheduleCronHint => '0 9 * * 1-5';

  @override
  String get scheduleCronInvalid =>
      'Некорректное cron-выражение (нужно 5 полей)';

  @override
  String get scheduleCronPreviewLabel => 'Cron';

  @override
  String get scheduleSave => 'Сохранить';

  @override
  String get scheduleCancel => 'Отмена';

  @override
  String get scheduleSavedSnack => 'Расписание сохранено';

  @override
  String get scheduleDeletedSnack => 'Расписание удалено';

  @override
  String get weekdayShortMon => 'Пн';

  @override
  String get weekdayShortTue => 'Вт';

  @override
  String get weekdayShortWed => 'Ср';

  @override
  String get weekdayShortThu => 'Чт';

  @override
  String get weekdayShortFri => 'Пт';

  @override
  String get weekdayShortSat => 'Сб';

  @override
  String get weekdayShortSun => 'Вс';

  @override
  String get sandboxServicesTabTitle => 'Тестовое окружение';

  @override
  String get sandboxServicesHeading => 'Эфемерные тестовые сервисы';

  @override
  String get sandboxServicesDescription =>
      'Объявите одноразовые сервисы (например PostgreSQL), которые поднимаются рядом с sandbox-агентом для интеграционных тестов с БД. Агент подключает их тумблером «подключать тест-сервисы» (обычно tester). Пароль генерится на каждый прогон и не хранится.';

  @override
  String get sandboxServicesEmpty => 'Тестовые сервисы ещё не настроены.';

  @override
  String get sandboxServicesAddButton => 'Добавить сервис';

  @override
  String get sandboxServicesLoadError =>
      'Не удалось загрузить тестовые сервисы.';

  @override
  String get sandboxServicesSavedSnack => 'Сервис сохранён';

  @override
  String get sandboxServicesDeletedSnack => 'Сервис удалён';

  @override
  String get sandboxServiceEnabledLabel => 'Включён';

  @override
  String get sandboxServiceFormTitleNew => 'Новый тестовый сервис';

  @override
  String get sandboxServiceFormTitleEdit => 'Редактирование тестового сервиса';

  @override
  String get sandboxServiceAliasLabel => 'Alias (hostname, напр. db)';

  @override
  String get sandboxServiceImageLabel => 'Образ';

  @override
  String get sandboxServiceDbNameLabel => 'Имя БД';

  @override
  String get sandboxServiceDbUserLabel => 'Пользователь БД';

  @override
  String get sandboxServicePortLabel => 'Порт';

  @override
  String get sandboxServiceReadyTimeoutLabel => 'Таймаут готовности (сек)';

  @override
  String get sandboxServiceSeedKindLabel => 'Сид';

  @override
  String get sandboxServiceSeedValueLabel =>
      'Значение сида (путь в репо или SQL)';

  @override
  String get sandboxServiceSave => 'Сохранить';

  @override
  String get sandboxServiceCancel => 'Отмена';

  @override
  String get sandboxServiceDelete => 'Удалить';

  @override
  String get sandboxServiceDeleteTitle => 'Удалить сервис?';

  @override
  String sandboxServiceDeleteConfirm(String alias) {
    return 'Удалить тестовый сервис «$alias»?';
  }

  @override
  String get scoutTabTitle => 'Разведчик';

  @override
  String get scoutHeading => 'Разведчик проекта';

  @override
  String get scoutDescription =>
      'Когда пользователь приходит с проблемой, а не с готовой задачей, разведчик запускает headless-прогон в sandbox на вашей подписке, читает репозитории проекта и собирает досье контекста (релевантные файлы, как устроено, подходы, открытые вопросы, предлагаемые критерии приёмки) — чтобы помочь сформулировать задачу. Доступен ассистенту проекта, когда включён.';

  @override
  String get scoutEnabledLabel => 'Включён';

  @override
  String get scoutEnabledHint =>
      'Позволяет ассистенту проекта запускать разведчика для сбора контекста.';

  @override
  String get scoutBackendLabel => 'Бэкенд';

  @override
  String get scoutBackendHint =>
      'CLI в sandbox. Запуск сейчас поддерживает claude-code (подписка).';

  @override
  String get scoutTimeoutLabel => 'Таймаут прогона, секунды';

  @override
  String get scoutTimeoutHint =>
      '60–3600. Жёсткий потолок прогона разведчика в sandbox.';

  @override
  String get scoutSubscriptionNote =>
      'Разведчик работает на подключённой подписке Claude владельца проекта (не на metered API). Без подключённой подписки прогон не запустится.';

  @override
  String get scoutSaveButton => 'Сохранить';

  @override
  String get scoutSavedSnack => 'Настройки разведчика сохранены';

  @override
  String get scoutPromptHeading => 'Промпт разведчика';

  @override
  String get scoutPromptHint =>
      'Инструкции для разведчика. Пусто — используется встроенный дефолтный промпт.';

  @override
  String get scoutPromptDefaultNotice =>
      'Используется встроенный дефолтный промпт.';

  @override
  String get scoutRunsTitle => 'Прогоны';

  @override
  String get scoutRunsEmpty =>
      'Прогонов ещё нет. Запустите разведку вручную или дайте ассистенту это сделать.';

  @override
  String get scoutRunButton => 'Запустить разведку';

  @override
  String get scoutRunStartedSnack =>
      'Разведка запущена — досье появится в списке прогонов';

  @override
  String get scoutRunDialogTitle => 'Запуск разведки';

  @override
  String get scoutRunDialogHint =>
      'Опишите проблему своими словами: что болит и какой желаемый исход.';

  @override
  String get scoutRunDialogCancel => 'Отмена';

  @override
  String get scoutRunDialogStart => 'Запустить';

  @override
  String get scoutRunStatusRunning => 'Выполняется';

  @override
  String get scoutRunStatusDone => 'Готово';

  @override
  String get scoutRunStatusFailed => 'Ошибка';

  @override
  String get scoutDossierTitle => 'Досье';

  @override
  String get scoutDossierEmpty => 'Досье нет';

  @override
  String get scoutLoadError => 'Не удалось загрузить данные разведчика';

  @override
  String get scoutProviderLabel => 'Провайдер';

  @override
  String get scoutProviderHint =>
      'Аутентификация/провайдер. claude-code: anthropic_oauth = подписка. hermes: anthropic/openrouter/hermes.';

  @override
  String get scoutProviderNone => '— не задан —';

  @override
  String get scoutProviderRequired => 'Бэкенд hermes требует явного провайдера';

  @override
  String get scoutModelLabel => 'Модель';

  @override
  String get scoutModelHint =>
      'напр. claude-sonnet-4-6, anthropic/claude-3.5-sonnet. Пусто — дефолт бэкенда.';

  @override
  String get scoutTemperatureLabel => 'Temperature';

  @override
  String get scoutAdvancedTitle => 'Расширенные настройки sandbox';

  @override
  String get scoutMcpLabel => 'MCP-серверы (JSON)';

  @override
  String get scoutMcpHint =>
      'JSON-массив mcp_servers в том же формате, что у агента. Пусто — нет.';

  @override
  String get scoutSkillsLabel => 'Скиллы (JSON)';

  @override
  String get scoutSkillsHint =>
      'JSON-массив skills в том же формате, что у агента. Пусто — нет.';

  @override
  String get scoutPermissionsLabel => 'Permissions (JSON)';

  @override
  String get scoutPermissionsHint =>
      'JSON-объект: allow/deny/ask/defaultMode (Claude Code). Пусто — дефолт.';

  @override
  String get scoutInvalidJsonSnack =>
      'Невалидный JSON в расширенных настройках — поправьте и сохраните снова';

  @override
  String get enhancerTabTitle => 'Улучшение';

  @override
  String get enhancerHeading => 'Энхансер проекта';

  @override
  String get enhancerDescription =>
      'Мета-агент анализирует историю выполнения задач (петли роутера, круги ревью, фидбек) и предлагает точечные улучшения: добавки к промптам агентов проекта и правки описания проекта. Все предложения попадают на ревью — ничего не применяется автоматически.';

  @override
  String get enhancerEnabledLabel => 'Включён';

  @override
  String get enhancerAutonomyLabel => 'Режим применения';

  @override
  String get enhancerAutonomyPropose =>
      'Предлагать изменения (ревью человеком)';

  @override
  String get enhancerAutonomyAutoApply => 'Применять автоматически';

  @override
  String get enhancerAutonomyAutoApplySoon =>
      'Скоро: автоприменение с замером эффекта и автооткатом';

  @override
  String get enhancerCronLabel => 'Расписание автозапуска (cron)';

  @override
  String get enhancerCronHint =>
      'Например: 0 9 * * 1 — каждый понедельник в 9:00. Пусто — только ручной запуск.';

  @override
  String get enhancerWindowLabel => 'Окно анализа, дней';

  @override
  String get enhancerMaxChangesLabel => 'Лимит предложений за прогон';

  @override
  String get enhancerSaveButton => 'Сохранить';

  @override
  String get enhancerSavedSnack => 'Настройки энхансера сохранены';

  @override
  String get enhancerRunNowButton => 'Запустить анализ';

  @override
  String get enhancerRunStartedSnack =>
      'Анализ запущен — отчёт появится в списке прогонов';

  @override
  String get enhancerRunInProgressSnack =>
      'Прогон уже выполняется, дождитесь завершения';

  @override
  String get enhancerRunsTitle => 'Прогоны';

  @override
  String get enhancerRunsEmpty =>
      'Прогонов ещё не было. Запустите анализ вручную или настройте расписание.';

  @override
  String get enhancerRunStatusRunning => 'Выполняется';

  @override
  String get enhancerRunStatusDone => 'Завершён';

  @override
  String get enhancerRunStatusFailed => 'Ошибка';

  @override
  String get enhancerTriggerManual => 'вручную';

  @override
  String get enhancerTriggerCron => 'по расписанию';

  @override
  String get enhancerReportTitle => 'Отчёт';

  @override
  String get enhancerReportEmpty => 'Отчёт пуст';

  @override
  String get enhancerChangesTitle => 'Предложения изменений';

  @override
  String get enhancerChangesEmpty => 'Предложений в этом прогоне нет';

  @override
  String get enhancerChangeReasonLabel => 'Обоснование';

  @override
  String get enhancerChangeEffectLabel => 'Ожидаемый эффект';

  @override
  String get enhancerChangePayloadLabel => 'Изменение';

  @override
  String get enhancerChangeStatusProposed => 'Предложено';

  @override
  String get enhancerChangeStatusApproved => 'Одобрено';

  @override
  String get enhancerChangeStatusApplied => 'Применено';

  @override
  String get enhancerChangeStatusRejected => 'Отклонено';

  @override
  String get enhancerChangeStatusRolledBack => 'Откатано';

  @override
  String get enhancerTargetAgentOverride => 'Промпт/настройки агента';

  @override
  String get enhancerTargetProjectDescription => 'Описание проекта';

  @override
  String get enhancerTargetProjectSettings => 'Настройки проекта';

  @override
  String get enhancerLoadError => 'Не удалось загрузить данные энхансера';

  @override
  String get enhancerChangeApplyButton => 'Применить';

  @override
  String get enhancerChangeRejectButton => 'Отклонить';

  @override
  String get enhancerChangeRollbackButton => 'Откатить';

  @override
  String get enhancerChangeAppliedSnack => 'Предложение применено';

  @override
  String get enhancerChangeRejectedSnack => 'Предложение отклонено';

  @override
  String get enhancerChangeRolledBackSnack => 'Изменение откачено';

  @override
  String get enhancerChangeConflictSnack =>
      'Не получилось: значение менялось после формирования предложения — обновите список и проверьте вручную';

  @override
  String get repoEnvFilesTabTitle => 'Env-файлы';

  @override
  String get repoEnvFilesHeading => 'Инъекция env-файла репозитория';

  @override
  String get repoEnvFilesDescription =>
      'Инъектируйте файл (например, .env) в рабочую копию репозитория перед запуском агента. Файл доступен агенту и тестам, но исключается из git (не коммитится и не пушится). Содержимое хранится в зашифрованном виде.';

  @override
  String get repoEnvFilesSelectRepo => 'Репозиторий';

  @override
  String get repoEnvFilesNoRepos =>
      'Сначала добавьте репозиторий в проект (вкладка «Основные»).';

  @override
  String get repoEnvFilesNotConfigured =>
      'Для этого репозитория env-файл ещё не настроен.';

  @override
  String get repoEnvFilesEmpty => 'Для этого репозитория ещё нет env-файлов.';

  @override
  String get repoEnvFilesAddButton => 'Добавить файл';

  @override
  String get repoEnvFilesCreateTitle => 'Новый env-файл';

  @override
  String get repoEnvFilesEditTitle => 'Редактировать env-файл';

  @override
  String get repoEnvFilesConfiguredHidden =>
      'Содержимое скрыто — сохранение перезапишет файл целиком.';

  @override
  String get repoEnvFilesUpdatedLabel => 'Обновлён:';

  @override
  String get repoEnvFilesFileNameLabel => 'Имя файла';

  @override
  String get repoEnvFilesFileNameHint => 'например, .env';

  @override
  String get repoEnvFilesTargetDirLabel => 'Папка назначения (необязательно)';

  @override
  String get repoEnvFilesTargetDirHint =>
      'Относительный путь внутри репо; пусто — корень';

  @override
  String get repoEnvFilesContentLabel => 'Содержимое файла';

  @override
  String get repoEnvFilesContentHint => 'KEY=значение\nANOTHER=значение';

  @override
  String get repoEnvFilesSave => 'Сохранить';

  @override
  String get repoEnvFilesDelete => 'Удалить';

  @override
  String get repoEnvFilesDeleteConfirm =>
      'Удалить env-файл для этого репозитория?';

  @override
  String get repoEnvFilesSaved => 'Env-файл сохранён';

  @override
  String get repoEnvFilesDeleted => 'Env-файл удалён';

  @override
  String get repoEnvFilesLoadError => 'Не удалось загрузить env-файл.';

  @override
  String get repoEnvFilesSaveError => 'Не удалось сохранить env-файл.';

  @override
  String get repoEnvFilesValidationFileNameRequired => 'Укажите имя файла';

  @override
  String get repoEnvFilesValidationContentRequired => 'Укажите содержимое';

  @override
  String get assistantMcpTabTitle => 'MCP-серверы';

  @override
  String get assistantMcpHeading => 'MCP-серверы ассистента';

  @override
  String get assistantMcpDescription =>
      'Внешние MCP-серверы (remote http/sse) — их инструменты становятся доступны ассистенту этого проекта. В значениях заголовков можно подставлять секреты проекта (см. подсказку в форме).';

  @override
  String get assistantMcpEmpty => 'MCP-серверы пока не добавлены.';

  @override
  String get assistantMcpAddButton => 'Добавить сервер';

  @override
  String get assistantMcpLoadError => 'Не удалось загрузить MCP-серверы.';

  @override
  String get assistantMcpSavedSnack => 'Сервер сохранён';

  @override
  String get assistantMcpDeletedSnack => 'Сервер удалён';

  @override
  String get assistantMcpFormTitleNew => 'Новый MCP-сервер';

  @override
  String get assistantMcpFormTitleEdit => 'Редактировать MCP-сервер';

  @override
  String get assistantMcpNameLabel => 'Имя';

  @override
  String get assistantMcpTransportLabel => 'Транспорт';

  @override
  String get assistantMcpUrlLabel => 'URL';

  @override
  String get assistantMcpHeadersLabel =>
      'Заголовки (по одному в строке: Name: value)';

  @override
  String get assistantMcpHeadersHint =>
      'По заголовку в строке. Чтобы не вписывать секрет в открытую, сошлитесь на секрет проекта (создаётся во вкладке «Переменные») — он подставится на сервере в рантайме. Синтаксис ссылки:';

  @override
  String get assistantMcpRequireConfirmationLabel => 'Спрашивать подтверждение';

  @override
  String get assistantMcpEnabledLabel => 'Включён';

  @override
  String get assistantMcpSave => 'Сохранить';

  @override
  String get assistantMcpCancel => 'Отмена';

  @override
  String get assistantMcpDelete => 'Удалить';

  @override
  String get assistantMcpDeleteTitle => 'Удалить сервер?';

  @override
  String assistantMcpDeleteConfirm(String name) {
    return 'Удалить MCP-сервер «$name»?';
  }
}
