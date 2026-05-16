import 'dart:async';

import 'package:flutter/foundation.dart';
import 'package:flutter/widgets.dart';
import 'package:flutter_localizations/flutter_localizations.dart';
import 'package:intl/intl.dart' as intl;

import 'app_localizations_en.dart';
import 'app_localizations_ru.dart';

// ignore_for_file: type=lint

/// Callers can lookup localized strings with an instance of AppLocalizations
/// returned by `AppLocalizations.of(context)`.
///
/// Applications need to include `AppLocalizations.delegate()` in their app's
/// `localizationDelegates` list, and the locales they support in the app's
/// `supportedLocales` list. For example:
///
/// ```dart
/// import 'l10n/app_localizations.dart';
///
/// return MaterialApp(
///   localizationsDelegates: AppLocalizations.localizationsDelegates,
///   supportedLocales: AppLocalizations.supportedLocales,
///   home: MyApplicationHome(),
/// );
/// ```
///
/// ## Update pubspec.yaml
///
/// Please make sure to update your pubspec.yaml to include the following
/// packages:
///
/// ```yaml
/// dependencies:
///   # Internationalization support.
///   flutter_localizations:
///     sdk: flutter
///   intl: any # Use the pinned version from flutter_localizations
///
///   # Rest of dependencies
/// ```
///
/// ## iOS Applications
///
/// iOS applications define key application metadata, including supported
/// locales, in an Info.plist file that is built into the application bundle.
/// To configure the locales supported by your app, you’ll need to edit this
/// file.
///
/// First, open your project’s ios/Runner.xcworkspace Xcode workspace file.
/// Then, in the Project Navigator, open the Info.plist file under the Runner
/// project’s Runner folder.
///
/// Next, select the Information Property List item, select Add Item from the
/// Editor menu, then select Localizations from the pop-up menu.
///
/// Select and expand the newly-created Localizations item then, for each
/// locale your application supports, add a new item and select the locale
/// you wish to add from the pop-up menu in the Value field. This list should
/// be consistent with the languages listed in the AppLocalizations.supportedLocales
/// property.
abstract class AppLocalizations {
  AppLocalizations(String locale)
    : localeName = intl.Intl.canonicalizedLocale(locale.toString());

  final String localeName;

  static AppLocalizations? of(BuildContext context) {
    return Localizations.of<AppLocalizations>(context, AppLocalizations);
  }

  static const LocalizationsDelegate<AppLocalizations> delegate =
      _AppLocalizationsDelegate();

  /// A list of this localizations delegate along with the default localizations
  /// delegates.
  ///
  /// Returns a list of localizations delegates containing this delegate along with
  /// GlobalMaterialLocalizations.delegate, GlobalCupertinoLocalizations.delegate,
  /// and GlobalWidgetsLocalizations.delegate.
  ///
  /// Additional delegates can be added by appending to this list in
  /// MaterialApp. This list does not have to be used at all if a custom list
  /// of delegates is preferred or required.
  static const List<LocalizationsDelegate<dynamic>> localizationsDelegates =
      <LocalizationsDelegate<dynamic>>[
        delegate,
        GlobalMaterialLocalizations.delegate,
        GlobalCupertinoLocalizations.delegate,
        GlobalWidgetsLocalizations.delegate,
      ];

  /// A list of this localizations delegate's supported locales.
  static const List<Locale> supportedLocales = <Locale>[
    Locale('en'),
    Locale('ru'),
  ];

  /// Название приложения
  ///
  /// In ru, this message translates to:
  /// **'Wibe Flutter Gin Template'**
  String get appTitle;

  /// ui_refactoring stage 1: brand label in AppBar of AppShell
  ///
  /// In ru, this message translates to:
  /// **'DevTeam'**
  String get appShellBrand;

  /// ui_refactoring stage 1: sidebar item /dashboard
  ///
  /// In ru, this message translates to:
  /// **'Обзор'**
  String get navDashboard;

  /// ui_refactoring stage 1: sidebar item /projects
  ///
  /// In ru, this message translates to:
  /// **'Проекты'**
  String get navProjects;

  /// ui_refactoring stage 1: sidebar item /admin/agents-v2
  ///
  /// In ru, this message translates to:
  /// **'Агенты'**
  String get navAgents;

  /// ui_refactoring stage 1: sidebar item /admin/worktrees
  ///
  /// In ru, this message translates to:
  /// **'Воркtrees'**
  String get navWorktrees;

  /// ui_refactoring stage 1: sidebar item /integrations/llm
  ///
  /// In ru, this message translates to:
  /// **'LLM-провайдеры'**
  String get navIntegrationsLlm;

  /// ui_refactoring stage 1: sidebar item /integrations/git
  ///
  /// In ru, this message translates to:
  /// **'Git-провайдеры'**
  String get navIntegrationsGit;

  /// ui_refactoring stage 1: sidebar item /admin/prompts
  ///
  /// In ru, this message translates to:
  /// **'Промпты'**
  String get navPrompts;

  /// ui_refactoring stage 1: sidebar item /admin/workflows
  ///
  /// In ru, this message translates to:
  /// **'Воркфлоу'**
  String get navWorkflows;

  /// ui_refactoring stage 1: sidebar item /admin/executions
  ///
  /// In ru, this message translates to:
  /// **'Запуски'**
  String get navExecutions;

  /// ui_refactoring stage 1: sidebar item /settings
  ///
  /// In ru, this message translates to:
  /// **'Настройки'**
  String get navSettings;

  /// ui_refactoring stage 1: sidebar item /profile
  ///
  /// In ru, this message translates to:
  /// **'Профиль'**
  String get navProfile;

  /// ui_refactoring stage 1: sidebar item /profile/api-keys
  ///
  /// In ru, this message translates to:
  /// **'API-ключи'**
  String get navApiKeys;

  /// ui_refactoring stage 1: sidebar section header
  ///
  /// In ru, this message translates to:
  /// **'Главная'**
  String get navGroupHome;

  /// ui_refactoring stage 1: sidebar section header
  ///
  /// In ru, this message translates to:
  /// **'Ресурсы'**
  String get navGroupResources;

  /// ui_refactoring stage 1: sidebar section header
  ///
  /// In ru, this message translates to:
  /// **'Интеграции'**
  String get navGroupIntegrations;

  /// ui_refactoring stage 1: sidebar section header
  ///
  /// In ru, this message translates to:
  /// **'Администрирование'**
  String get navGroupAdmin;

  /// ui_refactoring stage 1: sidebar section header
  ///
  /// In ru, this message translates to:
  /// **'Настройки'**
  String get navGroupSettings;

  /// ui_refactoring stage 1: breadcrumb root label
  ///
  /// In ru, this message translates to:
  /// **'Главная'**
  String get navBreadcrumbHome;

  /// ui_refactoring stage 1: breadcrumb segment for /new
  ///
  /// In ru, this message translates to:
  /// **'Новый'**
  String get navBreadcrumbNew;

  /// ui_refactoring stage 1: IntegrationProviderCard status chip
  ///
  /// In ru, this message translates to:
  /// **'Подключено'**
  String get integrationStatusConnected;

  /// ui_refactoring stage 1: IntegrationProviderCard status chip
  ///
  /// In ru, this message translates to:
  /// **'Не подключено'**
  String get integrationStatusDisconnected;

  /// ui_refactoring stage 1: IntegrationProviderCard status chip
  ///
  /// In ru, this message translates to:
  /// **'Ошибка'**
  String get integrationStatusError;

  /// ui_refactoring stage 1: IntegrationProviderCard status chip
  ///
  /// In ru, this message translates to:
  /// **'Подключение…'**
  String get integrationStatusPending;

  /// ui_refactoring stage 1: title on /integrations/llm stub
  ///
  /// In ru, this message translates to:
  /// **'LLM-провайдеры'**
  String get integrationsLlmTitle;

  /// ui_refactoring stage 1: subtitle on /integrations/llm stub
  ///
  /// In ru, this message translates to:
  /// **'Управление провайдерами появится на этапе 2. Ниже — превью каталога.'**
  String get integrationsLlmComingSoon;

  /// ui_refactoring stage 1: title on /integrations/git stub
  ///
  /// In ru, this message translates to:
  /// **'Git-провайдеры'**
  String get integrationsGitTitle;

  /// ui_refactoring stage 1: subtitle on /integrations/git stub and snackbar on Connect tap
  ///
  /// In ru, this message translates to:
  /// **'Подключение GitHub и GitLab появится на этапе 3.'**
  String get integrationsGitComingSoon;

  /// ui_refactoring stage 1: primary CTA on git stub cards
  ///
  /// In ru, this message translates to:
  /// **'Подключить'**
  String get integrationsGitConnectCta;

  /// ui_refactoring stage 1: github card subtitle
  ///
  /// In ru, this message translates to:
  /// **'Чтение репозиториев, push в PR-ветки'**
  String get integrationsGitGithubSubtitle;

  /// ui_refactoring stage 1: gitlab card subtitle
  ///
  /// In ru, this message translates to:
  /// **'Cloud и self-hosted GitLab'**
  String get integrationsGitGitlabSubtitle;

  /// ui_refactoring stage 3b: replaces ComingSoon copy on /integrations/git
  ///
  /// In ru, this message translates to:
  /// **'Подключите GitHub и GitLab, чтобы пушить ветки и открывать MR.'**
  String get integrationsGitStage3Subtitle;

  /// ui_refactoring stage 3b
  ///
  /// In ru, this message translates to:
  /// **'Подключено'**
  String get integrationsGitSectionConnected;

  /// ui_refactoring stage 3b
  ///
  /// In ru, this message translates to:
  /// **'Доступно'**
  String get integrationsGitSectionAvailable;

  /// ui_refactoring stage 3b: button label
  ///
  /// In ru, this message translates to:
  /// **'Отключить'**
  String get integrationsGitDisconnectCta;

  /// ui_refactoring stage 3b: secondary CTA on GitLab card
  ///
  /// In ru, this message translates to:
  /// **'Подключить self-hosted'**
  String get integrationsGitConnectSelfHostedCta;

  /// ui_refactoring stage 3b: empty state under Available
  ///
  /// In ru, this message translates to:
  /// **'Все поддерживаемые провайдеры уже подключены.'**
  String get integrationsGitEmptyAvailable;

  /// ui_refactoring stage 3b: §4a.5 user_cancelled
  ///
  /// In ru, this message translates to:
  /// **'Авторизация отклонена. Попробуйте снова.'**
  String get integrationsGitReasonUserCancelled;

  /// ui_refactoring stage 3b: §4a.5 invalid_state
  ///
  /// In ru, this message translates to:
  /// **'OAuth-сессия истекла. Начните заново.'**
  String get integrationsGitReasonExpired;

  /// ui_refactoring stage 3b: §4a.5 provider_unreachable
  ///
  /// In ru, this message translates to:
  /// **'Git-провайдер недоступен. Попробуйте позже.'**
  String get integrationsGitReasonProviderUnreachable;

  /// ui_refactoring stage 3b: §4a.5 invalid_host
  ///
  /// In ru, this message translates to:
  /// **'Хост не разрешён (приватная сеть, неподдерживаемая схема или неверный URL).'**
  String get integrationsGitReasonInvalidHost;

  /// ui_refactoring stage 3b: §4a.5 oauth_not_configured
  ///
  /// In ru, this message translates to:
  /// **'Этот провайдер не настроен на сервере.'**
  String get integrationsGitReasonOauthNotConfigured;

  /// ui_refactoring stage 3b: §4a.1 remote_revoke_failed notice
  ///
  /// In ru, this message translates to:
  /// **'Подключение удалено локально, но провайдер не подтвердил отзыв. Отзовите токен также в настройках аккаунта.'**
  String get integrationsGitReasonRemoteRevokeFailed;

  /// ui_refactoring stage 3b: pending state hint
  ///
  /// In ru, this message translates to:
  /// **'Ждём подтверждения в браузере…'**
  String get integrationsGitReasonPending;

  /// ui_refactoring stage 3b: fallback for unknown reason codes
  ///
  /// In ru, this message translates to:
  /// **'Не удалось подключить: {reason}'**
  String integrationsGitReasonUnknown(String reason);

  /// ui_refactoring stage 3b: retry button
  ///
  /// In ru, this message translates to:
  /// **'Повторить'**
  String get integrationsGitRetry;

  /// ui_refactoring stage 3b: error banner
  ///
  /// In ru, this message translates to:
  /// **'Не удалось загрузить интеграции: {message}'**
  String integrationsGitLoadFailed(String message);

  /// ui_refactoring stage 3b: subtitle for connected self-hosted GitLab
  ///
  /// In ru, this message translates to:
  /// **'Хост: {host}'**
  String integrationsGitConnectedHost(String host);

  /// ui_refactoring stage 3b: subtitle for connected provider
  ///
  /// In ru, this message translates to:
  /// **'Аккаунт: {login}'**
  String integrationsGitConnectedAccount(String login);

  /// ui_refactoring stage 3b: url_launcher fallback
  ///
  /// In ru, this message translates to:
  /// **'Не удалось открыть браузер. Откройте URL вручную: {url}'**
  String integrationsGitBrowserOpenFailed(String url);

  /// ui_refactoring stage 3b: BYO dialog title
  ///
  /// In ru, this message translates to:
  /// **'Подключить self-hosted GitLab'**
  String get integrationsGitlabHostDialogTitle;

  /// ui_refactoring stage 3b: BYO dialog field label
  ///
  /// In ru, this message translates to:
  /// **'Хост GitLab (https://…)'**
  String get integrationsGitlabHostFieldHost;

  /// ui_refactoring stage 3b: BYO dialog field label
  ///
  /// In ru, this message translates to:
  /// **'Application ID'**
  String get integrationsGitlabHostFieldClientId;

  /// ui_refactoring stage 3b: BYO dialog field label
  ///
  /// In ru, this message translates to:
  /// **'Application Secret'**
  String get integrationsGitlabHostFieldClientSecret;

  /// ui_refactoring stage 3b: BYO dialog field helper text
  ///
  /// In ru, this message translates to:
  /// **'Сохраняется как есть. Только https (или http для локальной разработки).'**
  String get integrationsGitlabHostFieldHostHint;

  /// ui_refactoring stage 3b: BYO dialog field helper text
  ///
  /// In ru, this message translates to:
  /// **'Шифруется AES-256-GCM в базе.'**
  String get integrationsGitlabHostFieldSecretHint;

  /// ui_refactoring stage 3b: BYO dialog client-side validation
  ///
  /// In ru, this message translates to:
  /// **'Укажите URL вашего GitLab'**
  String get integrationsGitlabHostValidationHostRequired;

  /// ui_refactoring stage 3b: BYO dialog client-side validation
  ///
  /// In ru, this message translates to:
  /// **'Хост должен начинаться с https:// (или http:// для локальной разработки)'**
  String get integrationsGitlabHostValidationHostScheme;

  /// ui_refactoring stage 3b: BYO dialog client-side validation
  ///
  /// In ru, this message translates to:
  /// **'Неверный формат URL'**
  String get integrationsGitlabHostValidationHostFormat;

  /// ui_refactoring stage 3b: BYO dialog client-side validation
  ///
  /// In ru, this message translates to:
  /// **'Укажите Application ID'**
  String get integrationsGitlabHostValidationClientIdRequired;

  /// ui_refactoring stage 3b: BYO dialog client-side validation
  ///
  /// In ru, this message translates to:
  /// **'Укажите Application Secret'**
  String get integrationsGitlabHostValidationClientSecretRequired;

  /// ui_refactoring stage 3b: expandable instructions header (oauth-setup-guide §5)
  ///
  /// In ru, this message translates to:
  /// **'Как зарегистрировать Application в моём GitLab'**
  String get integrationsGitlabHostInstructionsToggle;

  /// ui_refactoring stage 3b: BYO step (oauth-setup-guide §5)
  ///
  /// In ru, this message translates to:
  /// **'Откройте https://<ваш-gitlab-host>/-/user_settings/applications.'**
  String get integrationsGitlabHostInstructionsStep1;

  /// ui_refactoring stage 3b: BYO step
  ///
  /// In ru, this message translates to:
  /// **'Жмите «Add new application».'**
  String get integrationsGitlabHostInstructionsStep2;

  /// ui_refactoring stage 3b: BYO step with this app's callback URL
  ///
  /// In ru, this message translates to:
  /// **'Name: DevTeam. Redirect URI: {redirectUri}.'**
  String integrationsGitlabHostInstructionsStep3(String redirectUri);

  /// ui_refactoring stage 3b: BYO step
  ///
  /// In ru, this message translates to:
  /// **'Отметьте Confidential. Scopes: api, read_user, read_repository, write_repository.'**
  String get integrationsGitlabHostInstructionsStep4;

  /// ui_refactoring stage 3b: BYO step
  ///
  /// In ru, this message translates to:
  /// **'Сохраните, скопируйте Application ID и Secret, вставьте их выше.'**
  String get integrationsGitlabHostInstructionsStep5;

  /// ui_refactoring stage 3b: BYO dialog primary action
  ///
  /// In ru, this message translates to:
  /// **'Подключить'**
  String get integrationsGitlabHostSubmitCta;

  /// ui_refactoring stage 3b: BYO dialog cancel
  ///
  /// In ru, this message translates to:
  /// **'Отмена'**
  String get integrationsGitlabHostCancelCta;

  /// ui_refactoring stage 1: status chip on disabled stub cards
  ///
  /// In ru, this message translates to:
  /// **'Скоро'**
  String get integrationsComingSoonChip;

  /// ui_refactoring stage 1: LLM provider brand name (kept in l10n for consistency with gitProvider* keys)
  ///
  /// In ru, this message translates to:
  /// **'Claude Code'**
  String get llmProviderClaudeCode;

  /// ui_refactoring stage 1: LLM provider brand name
  ///
  /// In ru, this message translates to:
  /// **'Anthropic'**
  String get llmProviderAnthropic;

  /// ui_refactoring stage 1: LLM provider brand name
  ///
  /// In ru, this message translates to:
  /// **'OpenAI'**
  String get llmProviderOpenAi;

  /// ui_refactoring stage 1: LLM provider brand name
  ///
  /// In ru, this message translates to:
  /// **'OpenRouter'**
  String get llmProviderOpenRouter;

  /// ui_refactoring stage 1: LLM provider brand name
  ///
  /// In ru, this message translates to:
  /// **'DeepSeek'**
  String get llmProviderDeepSeek;

  /// ui_refactoring stage 1: LLM provider brand name
  ///
  /// In ru, this message translates to:
  /// **'Zhipu'**
  String get llmProviderZhipu;

  /// ui_refactoring stage 1: stub card subtitle
  ///
  /// In ru, this message translates to:
  /// **'Подписка Anthropic через OAuth'**
  String get llmProviderClaudeCodeSubtitle;

  /// ui_refactoring stage 1: stub card subtitle
  ///
  /// In ru, this message translates to:
  /// **'Прямой API-ключ Anthropic'**
  String get llmProviderAnthropicSubtitle;

  /// ui_refactoring stage 1: stub card subtitle
  ///
  /// In ru, this message translates to:
  /// **'GPT-4, GPT-4o, o-серия'**
  String get llmProviderOpenAiSubtitle;

  /// ui_refactoring stage 1: stub card subtitle
  ///
  /// In ru, this message translates to:
  /// **'Мульти-провайдерный агрегатор'**
  String get llmProviderOpenRouterSubtitle;

  /// ui_refactoring stage 1: stub card subtitle
  ///
  /// In ru, this message translates to:
  /// **'DeepSeek Chat и Coder'**
  String get llmProviderDeepSeekSubtitle;

  /// ui_refactoring stage 1: stub card subtitle
  ///
  /// In ru, this message translates to:
  /// **'Модели GLM'**
  String get llmProviderZhipuSubtitle;

  /// ui_refactoring stage 2: replaces ComingSoon copy
  ///
  /// In ru, this message translates to:
  /// **'Управление API-ключами и OAuth-подписками для код-агентов.'**
  String get integrationsLlmStage2Subtitle;

  /// ui_refactoring stage 2
  ///
  /// In ru, this message translates to:
  /// **'Подключённые'**
  String get integrationsLlmSectionConnected;

  /// ui_refactoring stage 2
  ///
  /// In ru, this message translates to:
  /// **'Доступные'**
  String get integrationsLlmSectionAvailable;

  /// ui_refactoring stage 2: button label
  ///
  /// In ru, this message translates to:
  /// **'Подключить'**
  String get integrationsLlmConnectCta;

  /// ui_refactoring stage 2: button label
  ///
  /// In ru, this message translates to:
  /// **'Отключить'**
  String get integrationsLlmDisconnectCta;

  /// ui_refactoring stage 2: button label
  ///
  /// In ru, this message translates to:
  /// **'Сменить ключ'**
  String get integrationsLlmReplaceCta;

  /// ui_refactoring stage 2: empty state under Available
  ///
  /// In ru, this message translates to:
  /// **'Все поддерживаемые провайдеры уже подключены.'**
  String get integrationsLlmEmptyAvailable;

  /// ui_refactoring stage 2: §4a.5 cancelled state
  ///
  /// In ru, this message translates to:
  /// **'Доступ отклонён. Попробуйте снова.'**
  String get integrationsLlmReasonUserCancelled;

  /// ui_refactoring stage 2: §4a.5 invalid_state
  ///
  /// In ru, this message translates to:
  /// **'Сессия устарела. Начните заново.'**
  String get integrationsLlmReasonExpired;

  /// ui_refactoring stage 2: §4a.5 provider_unreachable
  ///
  /// In ru, this message translates to:
  /// **'Провайдер недоступен. Повторите позже.'**
  String get integrationsLlmReasonProviderUnreachable;

  /// ui_refactoring stage 2: fallback for unknown reason codes
  ///
  /// In ru, this message translates to:
  /// **'Не удалось подключить: {reason}'**
  String integrationsLlmReasonUnknown(String reason);

  /// ui_refactoring stage 2: pending state hint
  ///
  /// In ru, this message translates to:
  /// **'Ждём подтверждения в браузере…'**
  String get integrationsLlmReasonPending;

  /// ui_refactoring stage 2: retry button
  ///
  /// In ru, this message translates to:
  /// **'Попробовать снова'**
  String get integrationsLlmRetry;

  /// ui_refactoring stage 2: dialog title
  ///
  /// In ru, this message translates to:
  /// **'Подключение {provider}'**
  String integrationsLlmDialogApiKeyTitle(String provider);

  /// ui_refactoring stage 2: input field label
  ///
  /// In ru, this message translates to:
  /// **'API-ключ'**
  String get integrationsLlmDialogApiKeyField;

  /// ui_refactoring stage 2: helper text
  ///
  /// In ru, this message translates to:
  /// **'Хранится зашифрованным (AES-256-GCM).'**
  String get integrationsLlmDialogApiKeyHint;

  /// ui_refactoring stage 2: validation error
  ///
  /// In ru, this message translates to:
  /// **'Введите непустой API-ключ'**
  String get integrationsLlmDialogApiKeyRequired;

  /// ui_refactoring stage 2: dialog cancel
  ///
  /// In ru, this message translates to:
  /// **'Отмена'**
  String get integrationsLlmDialogCancel;

  /// ui_refactoring stage 2: dialog primary action
  ///
  /// In ru, this message translates to:
  /// **'Сохранить'**
  String get integrationsLlmDialogSave;

  /// ui_refactoring stage 2: OAuth dialog title
  ///
  /// In ru, this message translates to:
  /// **'Подключение Claude Code'**
  String get integrationsLlmClaudeCodeOAuthTitle;

  /// ui_refactoring stage 2: OAuth instructions
  ///
  /// In ru, this message translates to:
  /// **'Откройте Anthropic в браузере, введите код ниже и подтвердите вход.'**
  String get integrationsLlmClaudeCodeOAuthStep1;

  /// ui_refactoring stage 2: launch URL button
  ///
  /// In ru, this message translates to:
  /// **'Открыть браузер'**
  String get integrationsLlmClaudeCodeOpenBrowser;

  /// ui_refactoring stage 2: user_code label
  ///
  /// In ru, this message translates to:
  /// **'Код:'**
  String get integrationsLlmClaudeCodeOAuthCode;

  /// ui_refactoring stage 2: tooltip for user_code copy IconButton
  ///
  /// In ru, this message translates to:
  /// **'Скопировать код'**
  String get integrationsLlmClaudeCodeOAuthCopy;

  /// ui_refactoring stage 2: pending body copy
  ///
  /// In ru, this message translates to:
  /// **'Ждём подтверждения… Можно закрыть это окно и вернуться позже — статус обновится автоматически.'**
  String get integrationsLlmClaudeCodeOAuthWaiting;

  /// ui_refactoring stage 2: §4a.5 timeout
  ///
  /// In ru, this message translates to:
  /// **'Авторизация истекла через 20 минут. Попробуйте снова.'**
  String get integrationsLlmClaudeCodeOAuthTimeout;

  /// ui_refactoring stage 2: error banner
  ///
  /// In ru, this message translates to:
  /// **'Не удалось загрузить интеграции: {message}'**
  String integrationsLlmLoadFailed(String message);

  /// ui_refactoring stage 1: dashboard greeting for authenticated user
  ///
  /// In ru, this message translates to:
  /// **'Добро пожаловать, {email}'**
  String dashboardWelcomeUser(String email);

  /// ui_refactoring stage 1: dashboard greeting fallback
  ///
  /// In ru, this message translates to:
  /// **'Добро пожаловать'**
  String get dashboardWelcomeAnon;

  /// ui_refactoring stage 1: subtitle under greeting on /dashboard
  ///
  /// In ru, this message translates to:
  /// **'Сводка по проектам, агентам и интеграциям.'**
  String get dashboardHubSubtitle;

  /// ui_refactoring stage 1: stat card primary value
  ///
  /// In ru, this message translates to:
  /// **'{n, plural, =0{Нет активных} =1{1 активный} few{{n} активных} many{{n} активных} other{{n} активных}}'**
  String dashboardStatProjectsActive(int n);

  /// ui_refactoring stage 1: stat card secondary value
  ///
  /// In ru, this message translates to:
  /// **'{n, plural, =0{Всего проектов нет} =1{Всего 1 проект} few{Всего {n} проекта} many{Всего {n} проектов} other{Всего {n} проектов}}'**
  String dashboardStatProjectsTotal(int n);

  /// ui_refactoring stage 1: stat card primary value
  ///
  /// In ru, this message translates to:
  /// **'{n, plural, =0{Нет агентов} =1{1 агент} few{{n} агента} many{{n} агентов} other{{n} агентов}}'**
  String dashboardStatAgentsTotal(int n);

  /// ui_refactoring stage 1: stat card primary value
  ///
  /// In ru, this message translates to:
  /// **'{n, plural, =0{Не подключено} =1{1 подключение} few{{n} подключения} many{{n} подключений} other{{n} подключений}}'**
  String dashboardStatLlmConnected(int n);

  /// ui_refactoring stage 1: stat card primary value
  ///
  /// In ru, this message translates to:
  /// **'{n, plural, =0{Не подключено} =1{1 подключение} few{{n} подключения} many{{n} подключений} other{{n} подключений}}'**
  String dashboardStatGitConnected(int n);

  /// ui_refactoring stage 1: stat card CTA
  ///
  /// In ru, this message translates to:
  /// **'Управлять'**
  String get dashboardStatManageCta;

  /// ui_refactoring stage 1: stat card secondary line for not-yet-shipped sections
  ///
  /// In ru, this message translates to:
  /// **'Доступно на следующем этапе'**
  String get dashboardStatComingSoon;

  /// ui_refactoring stage 1: section title on dashboard
  ///
  /// In ru, this message translates to:
  /// **'Последние задачи'**
  String get dashboardRecentTasksTitle;

  /// ui_refactoring stage 1: empty-state title
  ///
  /// In ru, this message translates to:
  /// **'Задач пока нет'**
  String get dashboardRecentTasksEmptyTitle;

  /// ui_refactoring stage 1: empty-state subtitle
  ///
  /// In ru, this message translates to:
  /// **'Создайте проект и добавьте задачи — они появятся здесь.'**
  String get dashboardRecentTasksEmptySubtitle;

  /// ui_refactoring stage 1: error state for recent tasks block
  ///
  /// In ru, this message translates to:
  /// **'Не удалось загрузить последние задачи.'**
  String get dashboardRecentTasksError;

  /// Кнопка входа
  ///
  /// In ru, this message translates to:
  /// **'Войти'**
  String get login;

  /// Кнопка выхода
  ///
  /// In ru, this message translates to:
  /// **'Выйти'**
  String get logout;

  /// Кнопка регистрации
  ///
  /// In ru, this message translates to:
  /// **'Регистрация'**
  String get register;

  /// Поле email
  ///
  /// In ru, this message translates to:
  /// **'Email'**
  String get email;

  /// Поле пароля
  ///
  /// In ru, this message translates to:
  /// **'Пароль'**
  String get password;

  /// Подсказка для поля email
  ///
  /// In ru, this message translates to:
  /// **'example@mail.com'**
  String get emailHint;

  /// Сообщение об ошибке валидации email
  ///
  /// In ru, this message translates to:
  /// **'Введите email'**
  String get enterEmail;

  /// Сообщение об ошибке валидации email формата
  ///
  /// In ru, this message translates to:
  /// **'Введите корректный email'**
  String get enterValidEmail;

  /// Сообщение об ошибке валидации пароля
  ///
  /// In ru, this message translates to:
  /// **'Введите пароль'**
  String get enterPassword;

  /// Сообщение об ошибке валидации длины пароля
  ///
  /// In ru, this message translates to:
  /// **'Пароль должен содержать минимум {minLength} символов'**
  String passwordTooShort(int minLength);

  /// Сообщение об ошибке несовпадения паролей
  ///
  /// In ru, this message translates to:
  /// **'Пароли не совпадают'**
  String get passwordsDoNotMatch;

  /// Сообщение об ошибке минимальной длины пароля
  ///
  /// In ru, this message translates to:
  /// **'Пароль должен быть не менее 8 символов'**
  String get passwordMinLength;

  /// Подсказка для поля подтверждения пароля
  ///
  /// In ru, this message translates to:
  /// **'Подтвердите пароль'**
  String get confirmPasswordPlaceholder;

  /// Ссылка на регистрацию на экране входа
  ///
  /// In ru, this message translates to:
  /// **'Нет аккаунта? Зарегистрироваться'**
  String get noAccountRegister;

  /// Ссылка на вход на экране регистрации
  ///
  /// In ru, this message translates to:
  /// **'Уже есть аккаунт? Войти'**
  String get haveAccountLogin;

  /// Приветствие на экране входа
  ///
  /// In ru, this message translates to:
  /// **'Добро пожаловать'**
  String get welcomeBack;

  /// Заголовок экрана входа
  ///
  /// In ru, this message translates to:
  /// **'Вход'**
  String get loginTitle;

  /// Заголовок экрана регистрации
  ///
  /// In ru, this message translates to:
  /// **'Регистрация'**
  String get registerTitle;

  /// Кнопка создания аккаунта
  ///
  /// In ru, this message translates to:
  /// **'Создать аккаунт'**
  String get createAccount;

  /// Заголовок экрана dashboard
  ///
  /// In ru, this message translates to:
  /// **'Dashboard'**
  String get dashboard;

  /// 13.6 кнопка на dashboard: админ — промпты
  ///
  /// In ru, this message translates to:
  /// **'Управление промптами (Админ)'**
  String get dashboardAdminManagePrompts;

  /// 13.6 кнопка на dashboard: админ — воркфлоу
  ///
  /// In ru, this message translates to:
  /// **'Управление воркфлоу (Админ)'**
  String get dashboardAdminManageWorkflows;

  /// 13.6 кнопка на dashboard: админ — журнал LLM
  ///
  /// In ru, this message translates to:
  /// **'Логи LLM (Админ)'**
  String get dashboardAdminViewLlmLogs;

  /// 6.6 кнопка на dashboard: админ — Agents v2 (/admin/agents-v2)
  ///
  /// In ru, this message translates to:
  /// **'Агенты (v2)'**
  String get dashboardAdminAgentsV2;

  /// 6.6 кнопка на dashboard: админ — Worktrees debug (/admin/worktrees)
  ///
  /// In ru, this message translates to:
  /// **'Worktrees (debug)'**
  String get dashboardAdminWorktrees;

  /// Заголовок экрана профиля
  ///
  /// In ru, this message translates to:
  /// **'Профиль'**
  String get profile;

  /// Заголовок блока информации о пользователе
  ///
  /// In ru, this message translates to:
  /// **'Информация о пользователе'**
  String get userInfo;

  /// Роль пользователя
  ///
  /// In ru, this message translates to:
  /// **'Роль'**
  String get role;

  /// Статус подтверждения email
  ///
  /// In ru, this message translates to:
  /// **'Email подтвержден'**
  String get emailVerified;

  /// Подтверждение
  ///
  /// In ru, this message translates to:
  /// **'Да'**
  String get yes;

  /// Отрицание
  ///
  /// In ru, this message translates to:
  /// **'Нет'**
  String get no;

  /// Кнопка перехода в профиль
  ///
  /// In ru, this message translates to:
  /// **'Перейти в профиль'**
  String get goToProfile;

  /// Заголовок блока информации
  ///
  /// In ru, this message translates to:
  /// **'Информация'**
  String get information;

  /// Кнопка обновления данных
  ///
  /// In ru, this message translates to:
  /// **'Обновить данные'**
  String get refreshData;

  /// Сообщение об ошибке загрузки
  ///
  /// In ru, this message translates to:
  /// **'Ошибка загрузки данных'**
  String get dataLoadError;

  /// Кнопка повтора
  ///
  /// In ru, this message translates to:
  /// **'Повторить'**
  String get retry;

  /// Сообщение о неавторизованном пользователе
  ///
  /// In ru, this message translates to:
  /// **'Пользователь не авторизован'**
  String get userNotAuthorized;

  /// Заголовок диалога подтверждения выхода
  ///
  /// In ru, this message translates to:
  /// **'Выход'**
  String get logoutConfirmTitle;

  /// Сообщение в диалоге подтверждения выхода
  ///
  /// In ru, this message translates to:
  /// **'Вы уверены, что хотите выйти?'**
  String get logoutConfirmMessage;

  /// Кнопка отмены
  ///
  /// In ru, this message translates to:
  /// **'Отмена'**
  String get cancel;

  /// Сообщение об ошибке при выходе
  ///
  /// In ru, this message translates to:
  /// **'Ошибка при выходе: {error}'**
  String logoutError(String error);

  /// Ошибка: неверные учетные данные
  ///
  /// In ru, this message translates to:
  /// **'Неверный email или пароль'**
  String get errorInvalidCredentials;

  /// Ошибка: пользователь не найден
  ///
  /// In ru, this message translates to:
  /// **'Пользователь не найден'**
  String get errorUserNotFound;

  /// Ошибка: пользователь уже существует
  ///
  /// In ru, this message translates to:
  /// **'Пользователь уже существует'**
  String get errorUserAlreadyExists;

  /// Ошибка: доступ запрещен
  ///
  /// In ru, this message translates to:
  /// **'Доступ запрещен'**
  String get errorAccessDenied;

  /// Ошибка: проблемы с сетью
  ///
  /// In ru, this message translates to:
  /// **'Ошибка сети. Проверьте подключение к интернету.'**
  String get errorNetwork;

  /// HTTP-запрос отменён (dispose, отмена пользователем и т.п.)
  ///
  /// In ru, this message translates to:
  /// **'Запрос отменён.'**
  String get errorRequestCancelled;

  /// Ошибка: проблема на сервере
  ///
  /// In ru, this message translates to:
  /// **'Ошибка сервера. Попробуйте позже.'**
  String get errorServer;

  /// Ошибка: неизвестная ошибка
  ///
  /// In ru, this message translates to:
  /// **'Произошла неизвестная ошибка.'**
  String get errorUnknown;

  /// Ошибка маршрутизации: маршрут не найден (GoRouter errorBuilder)
  ///
  /// In ru, this message translates to:
  /// **'Не удалось открыть эту страницу.'**
  String get routerNavigationError;

  /// Заголовок лендинга
  ///
  /// In ru, this message translates to:
  /// **'Создавайте быстрее с Wibe'**
  String get landingTitle;

  /// Подзаголовок лендинга
  ///
  /// In ru, this message translates to:
  /// **'Идеальный шаблон Flutter + Gin для вашей следующей идеи.\nГотов к продакшену, масштабируемый и красивый.'**
  String get landingSubtitle;

  /// Кнопка призыва к действию
  ///
  /// In ru, this message translates to:
  /// **'Начать бесплатно'**
  String get startForFree;

  /// Кнопка подробнее
  ///
  /// In ru, this message translates to:
  /// **'Узнать больше'**
  String get learnMore;

  /// Заголовок секции преимуществ
  ///
  /// In ru, this message translates to:
  /// **'Почему Wibe?'**
  String get whyWibe;

  /// Заголовок преимущества производительности
  ///
  /// In ru, this message translates to:
  /// **'Высокая производительность'**
  String get featurePerformanceTitle;

  /// Описание преимущества производительности
  ///
  /// In ru, this message translates to:
  /// **'Создан на Go (Gin) и Flutter для максимальной скорости.'**
  String get featurePerformanceDesc;

  /// Заголовок преимущества безопасности
  ///
  /// In ru, this message translates to:
  /// **'Безопасность по умолчанию'**
  String get featureSecurityTitle;

  /// Описание преимущества безопасности
  ///
  /// In ru, this message translates to:
  /// **'JWT Auth, RBAC и лучшие практики безопасности включены.'**
  String get featureSecurityDesc;

  /// Заголовок преимущества кроссплатформенности
  ///
  /// In ru, this message translates to:
  /// **'Кроссплатформенность'**
  String get featureCrossPlatformTitle;

  /// Описание преимущества кроссплатформенности
  ///
  /// In ru, this message translates to:
  /// **'Отлично работает на Web, iOS, Android и Desktop.'**
  String get featureCrossPlatformDesc;

  /// Кнопка начать
  ///
  /// In ru, this message translates to:
  /// **'Начать'**
  String get getStarted;

  /// Кнопка перехода в панель управления
  ///
  /// In ru, this message translates to:
  /// **'Перейти в Dashboard'**
  String get goToDashboard;

  /// No description provided for @promptsTitle.
  ///
  /// In ru, this message translates to:
  /// **'Управление промптами'**
  String get promptsTitle;

  /// No description provided for @promptsList.
  ///
  /// In ru, this message translates to:
  /// **'Список промптов'**
  String get promptsList;

  /// No description provided for @createPrompt.
  ///
  /// In ru, this message translates to:
  /// **'Создать промпт'**
  String get createPrompt;

  /// No description provided for @editPrompt.
  ///
  /// In ru, this message translates to:
  /// **'Редактировать промпт'**
  String get editPrompt;

  /// No description provided for @deletePrompt.
  ///
  /// In ru, this message translates to:
  /// **'Удалить промпт'**
  String get deletePrompt;

  /// No description provided for @deletePromptConfirmation.
  ///
  /// In ru, this message translates to:
  /// **'Вы уверены, что хотите удалить этот промпт?'**
  String get deletePromptConfirmation;

  /// No description provided for @promptName.
  ///
  /// In ru, this message translates to:
  /// **'Имя (Уникальный ID)'**
  String get promptName;

  /// No description provided for @promptDescription.
  ///
  /// In ru, this message translates to:
  /// **'Описание'**
  String get promptDescription;

  /// No description provided for @promptTemplate.
  ///
  /// In ru, this message translates to:
  /// **'Шаблон'**
  String get promptTemplate;

  /// No description provided for @promptJsonSchema.
  ///
  /// In ru, this message translates to:
  /// **'JSON Схема (Опционально)'**
  String get promptJsonSchema;

  /// No description provided for @promptIsActive.
  ///
  /// In ru, this message translates to:
  /// **'Активен'**
  String get promptIsActive;

  /// No description provided for @promptNameRequired.
  ///
  /// In ru, this message translates to:
  /// **'Имя обязательно'**
  String get promptNameRequired;

  /// No description provided for @promptTemplateRequired.
  ///
  /// In ru, this message translates to:
  /// **'Шаблон обязателен'**
  String get promptTemplateRequired;

  /// No description provided for @invalidJson.
  ///
  /// In ru, this message translates to:
  /// **'Неверный формат JSON'**
  String get invalidJson;

  /// No description provided for @save.
  ///
  /// In ru, this message translates to:
  /// **'Сохранить'**
  String get save;

  /// No description provided for @update.
  ///
  /// In ru, this message translates to:
  /// **'Обновить'**
  String get update;

  /// No description provided for @create.
  ///
  /// In ru, this message translates to:
  /// **'Создать'**
  String get create;

  /// No description provided for @delete.
  ///
  /// In ru, this message translates to:
  /// **'Удалить'**
  String get delete;

  /// No description provided for @managePrompts.
  ///
  /// In ru, this message translates to:
  /// **'Управление промптами (Админ)'**
  String get managePrompts;

  /// No description provided for @templatePlaceholderHelper.
  ///
  /// In ru, this message translates to:
  /// **'Используйте <.Variable> для переменных'**
  String get templatePlaceholderHelper;

  /// Заголовок экрана API-ключей
  ///
  /// In ru, this message translates to:
  /// **'API-ключи'**
  String get apiKeysTitle;

  /// Описание API-ключей
  ///
  /// In ru, this message translates to:
  /// **'API-ключи позволяют вашим приложениям обращаться к API без пароля. Каждый ключ действует от вашего имени.'**
  String get apiKeyDescription;

  /// Кнопка создания ключа
  ///
  /// In ru, this message translates to:
  /// **'Создать ключ'**
  String get apiKeyCreate;

  /// Поле имени ключа
  ///
  /// In ru, this message translates to:
  /// **'Название ключа'**
  String get apiKeyName;

  /// Подсказка для имени
  ///
  /// In ru, this message translates to:
  /// **'Например: Мой скрипт, CI/CD'**
  String get apiKeyNameHint;

  /// Поле срока действия
  ///
  /// In ru, this message translates to:
  /// **'Срок действия'**
  String get apiKeyExpiry;

  /// Без срока
  ///
  /// In ru, this message translates to:
  /// **'Бессрочный'**
  String get apiKeyNoExpiry;

  /// 30 дней
  ///
  /// In ru, this message translates to:
  /// **'30 дней'**
  String get apiKeyExpiry30Days;

  /// 90 дней
  ///
  /// In ru, this message translates to:
  /// **'90 дней'**
  String get apiKeyExpiry90Days;

  /// 1 год
  ///
  /// In ru, this message translates to:
  /// **'1 год'**
  String get apiKeyExpiry1Year;

  /// Заголовок диалога после создания
  ///
  /// In ru, this message translates to:
  /// **'Ключ создан'**
  String get apiKeyCreated;

  /// Предупреждение после создания
  ///
  /// In ru, this message translates to:
  /// **'Скопируйте ключ сейчас! Он больше не будет показан.'**
  String get apiKeyCreatedWarning;

  /// Кнопка копирования
  ///
  /// In ru, this message translates to:
  /// **'Скопировать ключ'**
  String get apiKeyCopy;

  /// Уведомление о копировании
  ///
  /// In ru, this message translates to:
  /// **'Ключ скопирован в буфер обмена'**
  String get apiKeyCopied;

  /// Кнопка подтверждения
  ///
  /// In ru, this message translates to:
  /// **'Понятно, я сохранил ключ'**
  String get apiKeyUnderstood;

  /// Кнопка отзыва
  ///
  /// In ru, this message translates to:
  /// **'Отозвать'**
  String get apiKeyRevoke;

  /// Заголовок диалога отзыва
  ///
  /// In ru, this message translates to:
  /// **'Отзыв ключа'**
  String get apiKeyRevokeTitle;

  /// Подтверждение отзыва
  ///
  /// In ru, this message translates to:
  /// **'Ключ перестанет работать. Это действие необратимо. Продолжить?'**
  String get apiKeyRevokeConfirm;

  /// Заголовок диалога удаления
  ///
  /// In ru, this message translates to:
  /// **'Удаление ключа'**
  String get apiKeyDeleteTitle;

  /// Подтверждение удаления
  ///
  /// In ru, this message translates to:
  /// **'Ключ будет полностью удалён. Продолжить?'**
  String get apiKeyDeleteConfirm;

  /// Чип: ключ истёк
  ///
  /// In ru, this message translates to:
  /// **'Истёк'**
  String get apiKeyExpired;

  /// Метка даты создания
  ///
  /// In ru, this message translates to:
  /// **'Создан'**
  String get apiKeyCreatedAt;

  /// Метка даты истечения
  ///
  /// In ru, this message translates to:
  /// **'Истекает'**
  String get apiKeyExpiresAt;

  /// Метка последнего использования
  ///
  /// In ru, this message translates to:
  /// **'Использован'**
  String get apiKeyLastUsed;

  /// Пустое состояние
  ///
  /// In ru, this message translates to:
  /// **'Нет API-ключей'**
  String get apiKeyEmpty;

  /// Подсказка пустого состояния
  ///
  /// In ru, this message translates to:
  /// **'Создайте ключ, чтобы использовать API из своих приложений'**
  String get apiKeyEmptyHint;

  /// Кнопка перехода к управлению ключами
  ///
  /// In ru, this message translates to:
  /// **'API-ключи'**
  String get apiKeysManage;

  /// 13.5: заголовок экрана и кнопки входа
  ///
  /// In ru, this message translates to:
  /// **'Глобальные настройки LLM'**
  String get globalSettingsScreenTitle;

  /// 13.5 режим B: пояснение
  ///
  /// In ru, this message translates to:
  /// **'Ключи LLM-провайдеров (OpenAI, Anthropic, Gemini и др.) для агентов пока настраиваются на сервере. Полный экран с сохранением появится после готовности API.'**
  String get globalSettingsStubIntro;

  /// 13.5 режим B: подпись к пути
  ///
  /// In ru, this message translates to:
  /// **'Задача backend в репозитории:'**
  String get globalSettingsBlockedByLabel;

  /// 13.5: отличие от ApiKeysScreen
  ///
  /// In ru, this message translates to:
  /// **'Ниже — ключи доступа к приложению DevTeam (MCP). Это не ключи LLM-провайдеров.'**
  String get globalSettingsStubApiKeysNote;

  /// 13.5: кнопка на /profile/api-keys
  ///
  /// In ru, this message translates to:
  /// **'Ключи API приложения'**
  String get globalSettingsOpenDevTeamApiKeys;

  /// Заголовок секции MCP
  ///
  /// In ru, this message translates to:
  /// **'Конфигурация MCP'**
  String get mcpConfigTitle;

  /// Описание MCP-конфигурации
  ///
  /// In ru, this message translates to:
  /// **'Используйте эту конфигурацию для подключения вашего LLM-клиента (Cursor, Claude Desktop, VS Code Copilot) к этому серверу'**
  String get mcpConfigDescription;

  /// Кнопка копирования конфига
  ///
  /// In ru, this message translates to:
  /// **'Скопировать конфиг'**
  String get mcpConfigCopy;

  /// Уведомление о копировании
  ///
  /// In ru, this message translates to:
  /// **'Конфигурация скопирована в буфер обмена'**
  String get mcpConfigCopied;

  /// Заголовок инструкции
  ///
  /// In ru, this message translates to:
  /// **'Инструкция:'**
  String get mcpConfigInstructions;

  /// Шаг 1
  ///
  /// In ru, this message translates to:
  /// **'1. Скопируйте конфигурацию ниже'**
  String get mcpConfigStep1;

  /// Шаг 2
  ///
  /// In ru, this message translates to:
  /// **'2. Откройте настройки вашего LLM-клиента'**
  String get mcpConfigStep2;

  /// Путь для Cursor
  ///
  /// In ru, this message translates to:
  /// **'   - Cursor: .cursor/config.json'**
  String get mcpConfigStep3Cursor;

  /// Путь для Claude
  ///
  /// In ru, this message translates to:
  /// **'   - Claude Desktop: ~/Library/Application Support/Claude/claude_desktop_config.json'**
  String get mcpConfigStep3Claude;

  /// Шаг 4
  ///
  /// In ru, this message translates to:
  /// **'3. Вставьте конфигурацию и перезапустите клиент'**
  String get mcpConfigStep4;

  /// Ошибка загрузки
  ///
  /// In ru, this message translates to:
  /// **'Не удалось загрузить конфигурацию MCP'**
  String get mcpConfigLoadError;

  /// MCP недоступен
  ///
  /// In ru, this message translates to:
  /// **'MCP-сервер выключен'**
  String get mcpConfigDisabled;

  /// Заголовок экрана списка проектов
  ///
  /// In ru, this message translates to:
  /// **'Проекты'**
  String get projectsTitle;

  /// Кнопка создания проекта
  ///
  /// In ru, this message translates to:
  /// **'Создать проект'**
  String get createProject;

  /// Подсказка поля поиска проектов
  ///
  /// In ru, this message translates to:
  /// **'Поиск проектов...'**
  String get searchProjectsHint;

  /// Фильтр: все статусы
  ///
  /// In ru, this message translates to:
  /// **'Все'**
  String get filterAll;

  /// Статус проекта: активный
  ///
  /// In ru, this message translates to:
  /// **'Активный'**
  String get statusActive;

  /// Статус проекта: приостановлен
  ///
  /// In ru, this message translates to:
  /// **'Приостановлен'**
  String get statusPaused;

  /// Статус проекта: архив
  ///
  /// In ru, this message translates to:
  /// **'Архив'**
  String get statusArchived;

  /// Статус проекта: индексация
  ///
  /// In ru, this message translates to:
  /// **'Индексация'**
  String get statusIndexing;

  /// Статус проекта: ошибка индексации
  ///
  /// In ru, this message translates to:
  /// **'Ошибка индексации'**
  String get statusIndexingFailed;

  /// Статус проекта: готов
  ///
  /// In ru, this message translates to:
  /// **'Готов'**
  String get statusReady;

  /// Статус проекта: неизвестный
  ///
  /// In ru, this message translates to:
  /// **'Неизвестно'**
  String get statusUnknown;

  /// Пустой список: нет проектов
  ///
  /// In ru, this message translates to:
  /// **'Проектов пока нет'**
  String get noProjectsYet;

  /// Пустой список: нет совпадений с фильтром
  ///
  /// In ru, this message translates to:
  /// **'Ничего не найдено'**
  String get noProjectsMatchFilter;

  /// Кнопка сброса фильтров
  ///
  /// In ru, this message translates to:
  /// **'Очистить фильтры'**
  String get clearFilters;

  /// Ошибка загрузки списка проектов
  ///
  /// In ru, this message translates to:
  /// **'Не удалось загрузить проекты'**
  String get errorLoadingProjects;

  /// Ошибка: сессия истекла
  ///
  /// In ru, this message translates to:
  /// **'Сессия истекла. Войдите снова'**
  String get errorUnauthorized;

  /// Ошибка: нет доступа
  ///
  /// In ru, this message translates to:
  /// **'Нет доступа к проектам'**
  String get errorForbidden;

  /// Провайдер репозитория: GitHub
  ///
  /// In ru, this message translates to:
  /// **'GitHub'**
  String get gitProviderGithub;

  /// Провайдер репозитория: GitLab
  ///
  /// In ru, this message translates to:
  /// **'GitLab'**
  String get gitProviderGitlab;

  /// Провайдер репозитория: Bitbucket
  ///
  /// In ru, this message translates to:
  /// **'Bitbucket'**
  String get gitProviderBitbucket;

  /// Провайдер репозитория: локальный git
  ///
  /// In ru, this message translates to:
  /// **'Локально'**
  String get gitProviderLocal;

  /// Неизвестный git-провайдер
  ///
  /// In ru, this message translates to:
  /// **'Git'**
  String get gitProviderUnknown;

  /// Заголовок экрана создания проекта
  ///
  /// In ru, this message translates to:
  /// **'Новый проект'**
  String get createProjectScreenTitle;

  /// Подпись поля имени проекта
  ///
  /// In ru, this message translates to:
  /// **'Название'**
  String get projectNameFieldLabel;

  /// Подсказка поля имени
  ///
  /// In ru, this message translates to:
  /// **'Мой проект'**
  String get projectNameFieldHint;

  /// Ошибка: пустое имя
  ///
  /// In ru, this message translates to:
  /// **'Введите название'**
  String get projectNameRequired;

  /// Ошибка: длина имени
  ///
  /// In ru, this message translates to:
  /// **'Не более {max} символов'**
  String projectNameMaxLength(int max);

  /// Подпись поля описания
  ///
  /// In ru, this message translates to:
  /// **'Описание'**
  String get projectDescriptionFieldLabel;

  /// Подсказка описания
  ///
  /// In ru, this message translates to:
  /// **'Для чего этот проект?'**
  String get projectDescriptionFieldHint;

  /// Подпись поля URL git
  ///
  /// In ru, this message translates to:
  /// **'URL репозитория'**
  String get gitUrlFieldLabel;

  /// Подсказка URL
  ///
  /// In ru, this message translates to:
  /// **'https://...'**
  String get gitUrlFieldHint;

  /// Ошибка: пустой URL для remote
  ///
  /// In ru, this message translates to:
  /// **'Укажите URL репозитория'**
  String get gitUrlRequiredForRemote;

  /// Ошибка: неверный URL
  ///
  /// In ru, this message translates to:
  /// **'Введите корректный http(s) URL'**
  String get gitUrlInvalid;

  /// Подпись выбора провайдера
  ///
  /// In ru, this message translates to:
  /// **'Провайдер Git'**
  String get gitProviderFieldLabel;

  /// Ошибка 409 при создании проекта
  ///
  /// In ru, this message translates to:
  /// **'Такое имя уже занято'**
  String get createProjectErrorConflict;

  /// Общая ошибка создания проекта
  ///
  /// In ru, this message translates to:
  /// **'Не удалось создать проект'**
  String get createProjectErrorGeneric;

  /// Нейтральный заголовок AppBar, пока нет имени (загрузка, ошибка, 404)
  ///
  /// In ru, this message translates to:
  /// **'Проект'**
  String get projectDashboardFallbackTitle;

  /// Раздел дашборда: чат
  ///
  /// In ru, this message translates to:
  /// **'Чат'**
  String get projectDashboardChat;

  /// Раздел дашборда: задачи
  ///
  /// In ru, this message translates to:
  /// **'Задачи'**
  String get projectDashboardTasks;

  /// Раздел дашборда: команда
  ///
  /// In ru, this message translates to:
  /// **'Команда'**
  String get projectDashboardTeam;

  /// Раздел дашборда: настройки
  ///
  /// In ru, this message translates to:
  /// **'Настройки'**
  String get projectDashboardSettings;

  /// Только 404: основной текст в body (в AppBar — projectDashboardFallbackTitle)
  ///
  /// In ru, this message translates to:
  /// **'Проект не найден'**
  String get projectDashboardNotFoundTitle;

  /// Кнопка: вернуться к списку
  ///
  /// In ru, this message translates to:
  /// **'К списку проектов'**
  String get projectDashboardNotFoundBackToList;

  /// Заголовок секции Git (13.4)
  ///
  /// In ru, this message translates to:
  /// **'Git-репозиторий'**
  String get projectSettingsSectionGit;

  /// Секция Weaviate (13.4)
  ///
  /// In ru, this message translates to:
  /// **'Векторный индекс'**
  String get projectSettingsSectionVector;

  /// Секция tech stack (13.4)
  ///
  /// In ru, this message translates to:
  /// **'Технологический стек'**
  String get projectSettingsSectionTechStack;

  /// Поле default branch
  ///
  /// In ru, this message translates to:
  /// **'Ветка по умолчанию'**
  String get projectSettingsGitDefaultBranchLabel;

  /// Карточка read-only credential
  ///
  /// In ru, this message translates to:
  /// **'Привязанный Git credential'**
  String get projectSettingsGitCredentialCardTitle;

  /// Кнопка отвязки
  ///
  /// In ru, this message translates to:
  /// **'Отвязать credential'**
  String get projectSettingsUnlinkCredential;

  /// Подсказка после выбора отвязки
  ///
  /// In ru, this message translates to:
  /// **'Отвязка выполнится после сохранения.'**
  String get projectSettingsUnlinkPendingHint;

  /// Поле vector_collection
  ///
  /// In ru, this message translates to:
  /// **'Имя коллекции Weaviate'**
  String get projectSettingsVectorCollectionLabel;

  /// Подсказка имени коллекции
  ///
  /// In ru, this message translates to:
  /// **'например ProjectCode'**
  String get projectSettingsVectorCollectionHint;

  /// Ошибка regex валидации
  ///
  /// In ru, this message translates to:
  /// **'Сначала заглавная латинская буква, далее буквы, цифры или подчёркивание.'**
  String get projectSettingsVectorCollectionInvalid;

  /// Баннер после смены vector_collection
  ///
  /// In ru, this message translates to:
  /// **'Имя коллекции изменилось. Запустите переиндексацию — векторы не переносятся в новую коллекцию автоматически.'**
  String get projectSettingsVectorCollectionRenamed;

  /// Кнопка POST reindex
  ///
  /// In ru, this message translates to:
  /// **'Переиндексировать'**
  String get projectSettingsReindex;

  /// Состояние status indexing
  ///
  /// In ru, this message translates to:
  /// **'Идёт индексация…'**
  String get projectSettingsReindexInProgress;

  /// Пояснение disabled reindex
  ///
  /// In ru, this message translates to:
  /// **'Переиндексация недоступна для локального проекта или при пустом URL репозитория.'**
  String get projectSettingsReindexUnavailable;

  /// SnackBar после 202 reindex
  ///
  /// In ru, this message translates to:
  /// **'Запущена переиндексация'**
  String get projectSettingsReindexStarted;

  /// 409 reindex
  ///
  /// In ru, this message translates to:
  /// **'Индексация уже выполняется или возник конфликт.'**
  String get projectSettingsReindexConflict;

  /// Общая ошибка reindex
  ///
  /// In ru, this message translates to:
  /// **'Не удалось запустить переиндексацию'**
  String get projectSettingsReindexGenericError;

  /// 400 reindex
  ///
  /// In ru, this message translates to:
  /// **'Запрос переиндексации отклонён'**
  String get projectSettingsReindexValidationError;

  /// Кнопка добавления пары key-value
  ///
  /// In ru, this message translates to:
  /// **'Добавить строку'**
  String get projectSettingsTechStackAddRow;

  /// Явная очистка tech stack
  ///
  /// In ru, this message translates to:
  /// **'Очистить tech stack'**
  String get projectSettingsTechStackClear;

  /// Поле ключа tech stack
  ///
  /// In ru, this message translates to:
  /// **'Ключ'**
  String get projectSettingsTechStackKeyLabel;

  /// Поле значения tech stack
  ///
  /// In ru, this message translates to:
  /// **'Значение'**
  String get projectSettingsTechStackValueLabel;

  /// Кнопка PUT
  ///
  /// In ru, this message translates to:
  /// **'Сохранить'**
  String get projectSettingsSave;

  /// Успешный PUT
  ///
  /// In ru, this message translates to:
  /// **'Настройки сохранены'**
  String get projectSettingsSaved;

  /// Пустой dirty-патч
  ///
  /// In ru, this message translates to:
  /// **'Нет изменений для сохранения'**
  String get projectSettingsNoChanges;

  /// 502 Save/reindex Git
  ///
  /// In ru, this message translates to:
  /// **'Не удалось обратиться к Git remote (ошибка клонирования или проверки).'**
  String get projectSettingsGitRemoteAccessFailed;

  /// Нейтральный 403 (в т.ч. reindex)
  ///
  /// In ru, this message translates to:
  /// **'Действие запрещено для вашей учётной записи.'**
  String get projectSettingsActionForbidden;

  /// 409 при сохранении
  ///
  /// In ru, this message translates to:
  /// **'Сохранение отклонено из‑за конфликта.'**
  String get projectSettingsSaveConflict;

  /// Общая ошибка Save
  ///
  /// In ru, this message translates to:
  /// **'Не удалось сохранить настройки'**
  String get projectSettingsSaveGenericError;

  /// 400 при сохранении
  ///
  /// In ru, this message translates to:
  /// **'Некорректные данные — проверьте форму и попробуйте снова.'**
  String get projectSettingsSaveValidationError;

  /// Общая ошибка чата (ChatController)
  ///
  /// In ru, this message translates to:
  /// **'Не удалось загрузить чат'**
  String get chatErrorGeneric;

  /// 404 чата
  ///
  /// In ru, this message translates to:
  /// **'Чат не найден'**
  String get chatErrorConversationNotFound;

  /// 429 при работе с чатом
  ///
  /// In ru, this message translates to:
  /// **'Слишком много запросов. Попробуйте позже'**
  String get chatErrorRateLimited;

  /// AppBar чата, пока метаданные беседы не загружены
  ///
  /// In ru, this message translates to:
  /// **'Чат'**
  String get chatScreenAppBarFallbackTitle;

  /// Экран ветки /chat без :conversationId в URL
  ///
  /// In ru, this message translates to:
  /// **'Выберите беседу или откройте её по прямой ссылке с идентификатором.'**
  String get chatScreenSelectConversationHint;

  /// Семантика пузыря пользователя
  ///
  /// In ru, this message translates to:
  /// **'Вы: {text}'**
  String chatScreenMessageSemanticUser(String text);

  /// Семантика пузыря ассистента
  ///
  /// In ru, this message translates to:
  /// **'Ассистент: {text}'**
  String chatScreenMessageSemanticAssistant(String text);

  /// Семантика системного сообщения
  ///
  /// In ru, this message translates to:
  /// **'Система: {text}'**
  String chatScreenMessageSemanticSystem(String text);

  /// Подсказка поля ввода чата
  ///
  /// In ru, this message translates to:
  /// **'Сообщение…'**
  String get chatScreenInputHint;

  /// Индикатор подгрузки старых сообщений
  ///
  /// In ru, this message translates to:
  /// **'Загрузка истории…'**
  String get chatScreenLoadingOlder;

  /// Статус незавершённой отправки
  ///
  /// In ru, this message translates to:
  /// **'Отправка…'**
  String get chatScreenPendingSending;

  /// Действо для повторной отправки после ошибки
  ///
  /// In ru, this message translates to:
  /// **'Повторить отправку'**
  String get chatScreenPendingRetry;

  /// Кнопка при 404 беседы
  ///
  /// In ru, this message translates to:
  /// **'К списку проектов'**
  String get chatScreenNotFoundBack;

  /// Placeholder поля ввода (виджет ChatInput, 11.8)
  ///
  /// In ru, this message translates to:
  /// **'Сообщение…'**
  String get chatInputHint;

  /// Подсказка кнопки отправки в ChatInput
  ///
  /// In ru, this message translates to:
  /// **'Отправить'**
  String get chatInputSendTooltip;

  /// Отмена исходящего HTTP send (не ответ ассистента)
  ///
  /// In ru, this message translates to:
  /// **'Отменить отправку'**
  String get chatInputStopTooltip;

  /// Кнопка вложений в ChatInput
  ///
  /// In ru, this message translates to:
  /// **'Вложение'**
  String get chatInputAttachTooltip;

  /// Резерв (таблица l10n 11.8): подсказка при отключённом attach; в 11.8 не используется — подключить в 11.11+ или удалить, если так и не понадобится
  ///
  /// In ru, this message translates to:
  /// **'Вложения недоступны'**
  String get chatInputAttachDisabledHint;

  /// Статус задачи pending
  ///
  /// In ru, this message translates to:
  /// **'В ожидании'**
  String get taskStatusPending;

  /// Статус задачи planning
  ///
  /// In ru, this message translates to:
  /// **'Планирование'**
  String get taskStatusPlanning;

  /// Статус задачи in_progress
  ///
  /// In ru, this message translates to:
  /// **'В работе'**
  String get taskStatusInProgress;

  /// Статус задачи review
  ///
  /// In ru, this message translates to:
  /// **'Ревью'**
  String get taskStatusReview;

  /// Статус задачи testing
  ///
  /// In ru, this message translates to:
  /// **'Тестирование'**
  String get taskStatusTesting;

  /// Статус задачи changes_requested
  ///
  /// In ru, this message translates to:
  /// **'Нужны правки'**
  String get taskStatusChangesRequested;

  /// Статус задачи completed
  ///
  /// In ru, this message translates to:
  /// **'Готово'**
  String get taskStatusCompleted;

  /// Статус задачи failed
  ///
  /// In ru, this message translates to:
  /// **'Ошибка'**
  String get taskStatusFailed;

  /// Статус задачи cancelled
  ///
  /// In ru, this message translates to:
  /// **'Отменена'**
  String get taskStatusCancelled;

  /// Статус задачи paused
  ///
  /// In ru, this message translates to:
  /// **'На паузе'**
  String get taskStatusPaused;

  /// Неизвестная строка статуса с бэкенда
  ///
  /// In ru, this message translates to:
  /// **'Неизвестный статус'**
  String get taskStatusUnknownStatus;

  /// Подсказка поля поиска списка задач (12.4)
  ///
  /// In ru, this message translates to:
  /// **'Поиск задач'**
  String get tasksSearchHint;

  /// Пустой список задач без фильтров (12.4)
  ///
  /// In ru, this message translates to:
  /// **'Задач пока нет'**
  String get tasksEmpty;

  /// Пустой список при активных фильтрах (12.4)
  ///
  /// In ru, this message translates to:
  /// **'Нет задач по выбранным фильтрам'**
  String get tasksEmptyFiltered;

  /// Кнопка сброса фильтров на пустом отфильтрованном списке (12.4)
  ///
  /// In ru, this message translates to:
  /// **'Сбросить фильтры'**
  String get tasksEmptyFilteredClear;

  /// Приоритет задачи critical (12.4)
  ///
  /// In ru, this message translates to:
  /// **'Критический'**
  String get taskPriorityCritical;

  /// Приоритет задачи high (12.4)
  ///
  /// In ru, this message translates to:
  /// **'Высокий'**
  String get taskPriorityHigh;

  /// Приоритет задачи medium (12.4)
  ///
  /// In ru, this message translates to:
  /// **'Средний'**
  String get taskPriorityMedium;

  /// Приоритет задачи low (12.4)
  ///
  /// In ru, this message translates to:
  /// **'Низкий'**
  String get taskPriorityLow;

  /// Фоллбэк для неизвестного приоритета (12.4)
  ///
  /// In ru, this message translates to:
  /// **'Неизвестный приоритет'**
  String get taskPriorityUnknown;

  /// Карточка задачи без назначенного агента (12.6)
  ///
  /// In ru, this message translates to:
  /// **'Не назначен'**
  String get taskCardUnassigned;

  /// Строка назначенного агента на карточке задачи (12.6)
  ///
  /// In ru, this message translates to:
  /// **'{name} · {role}'**
  String taskCardAgentLine(String name, String role);

  /// Строка времени обновления с уже отформатированным временем (12.6)
  ///
  /// In ru, this message translates to:
  /// **'Обновлено: {time}'**
  String taskCardUpdatedAt(String time);

  /// Заголовок карточки без title из метаданных
  ///
  /// In ru, this message translates to:
  /// **'Задача {shortId}'**
  String taskStatusCardFallbackTitle(String shortId);

  /// Роль агента worker
  ///
  /// In ru, this message translates to:
  /// **'Исполнитель'**
  String get taskCardAgentRoleWorker;

  /// Роль агента supervisor
  ///
  /// In ru, this message translates to:
  /// **'Супервизор'**
  String get taskCardAgentRoleSupervisor;

  /// Роль агента orchestrator
  ///
  /// In ru, this message translates to:
  /// **'Оркестратор'**
  String get taskCardAgentRoleOrchestrator;

  /// Роль агента planner
  ///
  /// In ru, this message translates to:
  /// **'Планировщик'**
  String get taskCardAgentRolePlanner;

  /// Роль агента developer
  ///
  /// In ru, this message translates to:
  /// **'Разработчик'**
  String get taskCardAgentRoleDeveloper;

  /// Роль агента reviewer
  ///
  /// In ru, this message translates to:
  /// **'Ревьюер'**
  String get taskCardAgentRoleReviewer;

  /// Роль агента tester
  ///
  /// In ru, this message translates to:
  /// **'Тестировщик'**
  String get taskCardAgentRoleTester;

  /// Роль агента devops
  ///
  /// In ru, this message translates to:
  /// **'DevOps'**
  String get taskCardAgentRoleDevops;

  /// Общая ошибка TaskRepository / контроллеров задач (12.3)
  ///
  /// In ru, this message translates to:
  /// **'Не удалось выполнить операцию с задачами'**
  String get taskErrorGeneric;

  /// 404 списка задач проекта
  ///
  /// In ru, this message translates to:
  /// **'Проект не найден'**
  String get taskListErrorProjectNotFound;

  /// 404 карточки задачи
  ///
  /// In ru, this message translates to:
  /// **'Задача не найдена'**
  String get taskDetailErrorTaskNotFound;

  /// Документация для UI: addTaskMessage без ключа идемпотентности (12.3 §45)
  ///
  /// In ru, this message translates to:
  /// **'Повторное нажатие «Отправить» создаёт второе сообщение на сервере (идемпотентность — отдельная задача).'**
  String get taskSendMessageNoIdempotencyHint;

  /// AppBar детали задачи до прихода данных (12.5)
  ///
  /// In ru, this message translates to:
  /// **'Загрузка…'**
  String get taskDetailAppBarLoading;

  /// SnackBar при таймауте обновления детали задачи (12.5)
  ///
  /// In ru, this message translates to:
  /// **'Обновление занимает слишком много времени. Попробуйте снова.'**
  String get taskDetailRefreshTimedOut;

  /// Короткий заголовок при удалённой задаче (12.5)
  ///
  /// In ru, this message translates to:
  /// **'Задача удалена'**
  String get taskDetailDeletedTitle;

  /// Тело экрана при taskDeleted (12.5)
  ///
  /// In ru, this message translates to:
  /// **'Эта задача удалена на сервере. Откройте список задач, чтобы продолжить работу с другими карточками.'**
  String get taskDetailDeletedBody;

  /// Секция описания задачи (12.5)
  ///
  /// In ru, this message translates to:
  /// **'Описание'**
  String get taskDetailSectionDescription;

  /// Секция результата (12.5)
  ///
  /// In ru, this message translates to:
  /// **'Результат'**
  String get taskDetailSectionResult;

  /// Секция артефакта diff (12.5)
  ///
  /// In ru, this message translates to:
  /// **'Изменения (diff)'**
  String get taskDetailSectionDiff;

  /// Секция ленты сообщений (12.5)
  ///
  /// In ru, this message translates to:
  /// **'Лог сообщений'**
  String get taskDetailSectionMessages;

  /// Секция доменного error_message задачи (12.5)
  ///
  /// In ru, this message translates to:
  /// **'Ошибка задачи'**
  String get taskDetailSectionErrorMessage;

  /// Секция подзадач (12.5)
  ///
  /// In ru, this message translates to:
  /// **'Подзадачи'**
  String get taskDetailSectionSubtasks;

  /// Пустой артефакт diff (12.5)
  ///
  /// In ru, this message translates to:
  /// **'Нет diff'**
  String get taskDetailNoDiff;

  /// Пустое описание задачи (12.5)
  ///
  /// In ru, this message translates to:
  /// **'Нет описания'**
  String get taskDetailNoDescription;

  /// Пустой результат (12.5)
  ///
  /// In ru, this message translates to:
  /// **'Нет результата'**
  String get taskDetailNoResult;

  /// Пустая лента сообщений (12.5)
  ///
  /// In ru, this message translates to:
  /// **'Сообщений пока нет'**
  String get taskDetailNoMessages;

  /// Кнопка возврата к списку (12.5)
  ///
  /// In ru, this message translates to:
  /// **'К списку задач'**
  String get taskDetailBackToList;

  /// Несовпадение projectId URL и задачи (12.5)
  ///
  /// In ru, this message translates to:
  /// **'Задача из другого проекта'**
  String get taskDetailProjectMismatch;

  /// Блок мутаций из-за realtime (12.5)
  ///
  /// In ru, this message translates to:
  /// **'Обновление задачи по сети временно недоступно'**
  String get taskDetailRealtimeMutationBlocked;

  /// Терминальный сбой realtime-сессии (12.5)
  ///
  /// In ru, this message translates to:
  /// **'Проблема сессии realtime'**
  String get taskDetailRealtimeSessionFailure;

  /// Transient WS / сервис (12.5)
  ///
  /// In ru, this message translates to:
  /// **'Сбой realtime-сервиса'**
  String get taskDetailRealtimeServiceFailure;

  /// Кнопка Pause на карточке задачи (12.8)
  ///
  /// In ru, this message translates to:
  /// **'Пауза'**
  String get taskActionPause;

  /// Кнопка Resume (12.8)
  ///
  /// In ru, this message translates to:
  /// **'Возобновить'**
  String get taskActionResume;

  /// Кнопка отмены задачи (12.8)
  ///
  /// In ru, this message translates to:
  /// **'Отменить задачу'**
  String get taskActionCancel;

  /// Заголовок диалога подтверждения отмены (12.8)
  ///
  /// In ru, this message translates to:
  /// **'Отменить задачу?'**
  String get taskActionCancelConfirmTitle;

  /// Текст предупреждения перед отменой задачи (12.8)
  ///
  /// In ru, this message translates to:
  /// **'Задача перейдёт в статус «отменена». Это действие нельзя отменить.'**
  String get taskActionCancelConfirmBody;

  /// Подтверждение деструктивного действия в диалоге (12.8)
  ///
  /// In ru, this message translates to:
  /// **'Да, отменить задачу'**
  String get taskActionConfirm;

  /// SnackBar при отказе мутации из-за realtime (12.8)
  ///
  /// In ru, this message translates to:
  /// **'Сейчас нельзя изменить задачу: обновления по сети временно недоступны.'**
  String get taskActionBlockedByRealtimeSnack;

  /// Info-toast когда Cancel проиграл гонку финализации (6.1)
  ///
  /// In ru, this message translates to:
  /// **'Задача уже завершена'**
  String get taskActionAlreadyTerminalSnack;

  /// Фоллбэк message_type (12.5)
  ///
  /// In ru, this message translates to:
  /// **'Неизвестный тип сообщения'**
  String get taskMessageTypeUnknown;

  /// Фоллбэк sender_type (12.5)
  ///
  /// In ru, this message translates to:
  /// **'Неизвестный отправитель'**
  String get taskSenderTypeUnknown;

  /// Тип сообщения instruction (12.5)
  ///
  /// In ru, this message translates to:
  /// **'Инструкция'**
  String get taskMessageTypeInstruction;

  /// Тип сообщения result (12.5)
  ///
  /// In ru, this message translates to:
  /// **'Результат'**
  String get taskMessageTypeResult;

  /// Тип сообщения question (12.5)
  ///
  /// In ru, this message translates to:
  /// **'Вопрос'**
  String get taskMessageTypeQuestion;

  /// Тип сообщения feedback (12.5)
  ///
  /// In ru, this message translates to:
  /// **'Обратная связь'**
  String get taskMessageTypeFeedback;

  /// Тип сообщения error (12.5)
  ///
  /// In ru, this message translates to:
  /// **'Ошибка'**
  String get taskMessageTypeError;

  /// Тип сообщения comment (12.5)
  ///
  /// In ru, this message translates to:
  /// **'Комментарий'**
  String get taskMessageTypeComment;

  /// Тип сообщения summary (12.5)
  ///
  /// In ru, this message translates to:
  /// **'Сводка'**
  String get taskMessageTypeSummary;

  /// Отправитель user (12.5)
  ///
  /// In ru, this message translates to:
  /// **'Пользователь'**
  String get taskSenderTypeUser;

  /// Отправитель agent (12.5)
  ///
  /// In ru, this message translates to:
  /// **'Агент'**
  String get taskSenderTypeAgent;

  /// Фоллбэк для неизвестной роли агента в UI (карточка задачи, детали задачи, команда).
  ///
  /// In ru, this message translates to:
  /// **'Неизвестная роль'**
  String get agentRoleUnknown;

  /// Подпись роли агента «worker» в UI (карточка задачи, детали задачи, команда).
  ///
  /// In ru, this message translates to:
  /// **'Исполнитель'**
  String get agentRoleWorker;

  /// Подпись роли агента «supervisor» в UI (карточка задачи, детали задачи, команда).
  ///
  /// In ru, this message translates to:
  /// **'Супервизор'**
  String get agentRoleSupervisor;

  /// Подпись роли агента «orchestrator» в UI (карточка задачи, детали задачи, команда).
  ///
  /// In ru, this message translates to:
  /// **'Оркестратор'**
  String get agentRoleOrchestrator;

  /// Подпись роли агента «planner» в UI (карточка задачи, детали задачи, команда).
  ///
  /// In ru, this message translates to:
  /// **'Планировщик'**
  String get agentRolePlanner;

  /// Подпись роли агента «developer» в UI (карточка задачи, детали задачи, команда).
  ///
  /// In ru, this message translates to:
  /// **'Разработчик'**
  String get agentRoleDeveloper;

  /// Подпись роли агента «reviewer» в UI (карточка задачи, детали задачи, команда).
  ///
  /// In ru, this message translates to:
  /// **'Ревьюер'**
  String get agentRoleReviewer;

  /// Подпись роли агента «tester» в UI (карточка задачи, детали задачи, команда).
  ///
  /// In ru, this message translates to:
  /// **'Тестировщик'**
  String get agentRoleTester;

  /// Подпись роли агента «devops» в UI (карточка задачи, детали задачи, команда).
  ///
  /// In ru, this message translates to:
  /// **'DevOps'**
  String get agentRoleDevops;

  /// Вкладка команды: пустой список агентов (13.1)
  ///
  /// In ru, this message translates to:
  /// **'В команде пока нет агентов.'**
  String get teamEmptyAgents;

  /// Вкладка команды: модель не задана (13.1)
  ///
  /// In ru, this message translates to:
  /// **'Модель по умолчанию'**
  String get teamAgentModelUnset;

  /// Вкладка команды: пустое имя агента (13.1)
  ///
  /// In ru, this message translates to:
  /// **'Агент без имени'**
  String get teamAgentNameUnset;

  /// Семантика: агент активен (13.1)
  ///
  /// In ru, this message translates to:
  /// **'Активен'**
  String get teamAgentActive;

  /// Семантика: агент неактивен (13.1)
  ///
  /// In ru, this message translates to:
  /// **'Неактивен'**
  String get teamAgentInactive;

  /// 13.3 заголовок диалога
  ///
  /// In ru, this message translates to:
  /// **'Редактирование агента'**
  String get teamAgentEditTitle;

  /// 13.3 подпись поля
  ///
  /// In ru, this message translates to:
  /// **'Модель LLM'**
  String get teamAgentEditFieldModel;

  /// 13.3 подпись поля
  ///
  /// In ru, this message translates to:
  /// **'Промпт'**
  String get teamAgentEditFieldPrompt;

  /// 13.3 подпись поля
  ///
  /// In ru, this message translates to:
  /// **'Code backend'**
  String get teamAgentEditFieldCodeBackend;

  /// Sprint 15.e2e: kind LLM-провайдера агента (anthropic / anthropic_oauth / deepseek / zhipu / openrouter)
  ///
  /// In ru, this message translates to:
  /// **'LLM провайдер'**
  String get teamAgentEditFieldProviderKind;

  /// Sprint 15.e2e: подсказка под дропдауном про per-user creds
  ///
  /// In ru, this message translates to:
  /// **'Ключи провайдера задаются в Настройках → LLM-ключи'**
  String get teamAgentEditFieldProviderKindHelp;

  /// 13.3 переключатель
  ///
  /// In ru, this message translates to:
  /// **'Активен'**
  String get teamAgentEditFieldActive;

  /// 13.3 кнопка сохранить
  ///
  /// In ru, this message translates to:
  /// **'Сохранить'**
  String get teamAgentEditSave;

  /// 13.3 кнопка отмена
  ///
  /// In ru, this message translates to:
  /// **'Отмена'**
  String get teamAgentEditCancel;

  /// 13.3 заголовок discard
  ///
  /// In ru, this message translates to:
  /// **'Отменить изменения?'**
  String get teamAgentEditDiscardTitle;

  /// 13.3 текст discard
  ///
  /// In ru, this message translates to:
  /// **'Черновик будет потерян.'**
  String get teamAgentEditDiscardBody;

  /// 13.3 общая ошибка сохранения
  ///
  /// In ru, this message translates to:
  /// **'Не удалось сохранить агента'**
  String get teamAgentEditSaveError;

  /// 13.3 HTTP 403 при сохранении агента
  ///
  /// In ru, this message translates to:
  /// **'Недостаточно прав для сохранения агента'**
  String get teamAgentEditSaveForbidden;

  /// 13.3 снек при 409
  ///
  /// In ru, this message translates to:
  /// **'Изменение отклонено (конфликт). Попробуйте снова.'**
  String get teamAgentEditConflictError;

  /// 13.3 пустой список промптов
  ///
  /// In ru, this message translates to:
  /// **'Нет доступных промптов'**
  String get teamAgentEditNoPrompts;

  /// 13.3 ошибка GET промптов
  ///
  /// In ru, this message translates to:
  /// **'Не удалось загрузить промпты'**
  String get teamAgentEditPromptsLoadError;

  /// 13.3 пункт сброса промпта
  ///
  /// In ru, this message translates to:
  /// **'Промпт не выбран'**
  String get teamAgentEditPromptNone;

  /// 13.3 сброс опционального поля (code backend)
  ///
  /// In ru, this message translates to:
  /// **'Не задано'**
  String get teamAgentEditUnset;

  /// 13.3 подтверждение discard
  ///
  /// In ru, this message translates to:
  /// **'Сбросить'**
  String get teamAgentEditDiscardConfirm;

  /// 13.3 снек после закрытия при ошибке перезагрузки
  ///
  /// In ru, this message translates to:
  /// **'Сохранено, но не удалось обновить команду'**
  String get teamAgentEditRefetchError;

  /// 13.3.1 заголовок секции инструментов
  ///
  /// In ru, this message translates to:
  /// **'Инструменты'**
  String get teamAgentEditFieldTools;

  /// 13.3.1 ошибка GET /tool-definitions
  ///
  /// In ru, this message translates to:
  /// **'Не удалось загрузить каталог инструментов'**
  String get teamAgentEditToolsLoadError;

  /// 13.3.1 пустой каталог
  ///
  /// In ru, this message translates to:
  /// **'Нет доступных инструментов в каталоге'**
  String get teamAgentEditToolsEmpty;

  /// 13.3.1 подсказка при пустом выборе
  ///
  /// In ru, this message translates to:
  /// **'Инструменты не выбраны'**
  String get teamAgentEditToolsNoneSelected;

  /// 13.3.1 HTTP 400 при сохранении привязок
  ///
  /// In ru, this message translates to:
  /// **'Некорректный набор инструментов. Проверьте выбор и попробуйте снова.'**
  String get teamAgentEditToolsValidationError;

  /// 13.3.1 повторная загрузка каталога
  ///
  /// In ru, this message translates to:
  /// **'Повторить'**
  String get teamAgentEditToolsRetry;

  /// Подпись чипа инструмента в секции «Инструменты» агента: имя + категория
  ///
  /// In ru, this message translates to:
  /// **'{name} ({category})'**
  String teamAgentEditToolsListEntryLabel(String name, String category);

  /// Подсказка кнопки копирования блока кода в чате
  ///
  /// In ru, this message translates to:
  /// **'Копировать код'**
  String get chatMessageCopyCode;

  /// Пустое тело при стриме ассистента
  ///
  /// In ru, this message translates to:
  /// **'Печатает…'**
  String get chatMessageStreamingPlaceholder;

  /// Картинка markdown без alt — без загрузки по сети
  ///
  /// In ru, this message translates to:
  /// **'[изображение]'**
  String get chatMessageImagePlaceholder;

  /// Картинка markdown заменена текстом, только alt
  ///
  /// In ru, this message translates to:
  /// **'[{alt}]'**
  String chatMessageMarkdownImageAlt(String alt);

  /// No description provided for @refresh.
  ///
  /// In ru, this message translates to:
  /// **'Обновить'**
  String get refresh;

  /// No description provided for @copy.
  ///
  /// In ru, this message translates to:
  /// **'Скопировать'**
  String get copy;

  /// No description provided for @openInBrowser.
  ///
  /// In ru, this message translates to:
  /// **'Открыть в браузере'**
  String get openInBrowser;

  /// No description provided for @fieldRequired.
  ///
  /// In ru, this message translates to:
  /// **'Обязательное поле'**
  String get fieldRequired;

  /// No description provided for @globalSettingsTabLLMProviders.
  ///
  /// In ru, this message translates to:
  /// **'LLM-провайдеры'**
  String get globalSettingsTabLLMProviders;

  /// No description provided for @globalSettingsTabClaudeCode.
  ///
  /// In ru, this message translates to:
  /// **'Claude Code'**
  String get globalSettingsTabClaudeCode;

  /// No description provided for @globalSettingsTabDevTeam.
  ///
  /// In ru, this message translates to:
  /// **'DevTeam'**
  String get globalSettingsTabDevTeam;

  /// No description provided for @llmProvidersSectionTitle.
  ///
  /// In ru, this message translates to:
  /// **'LLM-провайдеры'**
  String get llmProvidersSectionTitle;

  /// No description provided for @llmProvidersAdd.
  ///
  /// In ru, this message translates to:
  /// **'Добавить'**
  String get llmProvidersAdd;

  /// No description provided for @llmProvidersEmpty.
  ///
  /// In ru, this message translates to:
  /// **'Провайдеры ещё не настроены.'**
  String get llmProvidersEmpty;

  /// No description provided for @llmProvidersLoadError.
  ///
  /// In ru, this message translates to:
  /// **'Не удалось загрузить список провайдеров'**
  String get llmProvidersLoadError;

  /// No description provided for @llmProvidersHealthTooltip.
  ///
  /// In ru, this message translates to:
  /// **'Проверка здоровья'**
  String get llmProvidersHealthTooltip;

  /// No description provided for @llmProvidersEditTooltip.
  ///
  /// In ru, this message translates to:
  /// **'Редактировать'**
  String get llmProvidersEditTooltip;

  /// No description provided for @llmProvidersDeleteTooltip.
  ///
  /// In ru, this message translates to:
  /// **'Удалить'**
  String get llmProvidersDeleteTooltip;

  /// No description provided for @llmProvidersHealthOK.
  ///
  /// In ru, this message translates to:
  /// **'Провайдер доступен'**
  String get llmProvidersHealthOK;

  /// No description provided for @llmProvidersHealthFail.
  ///
  /// In ru, this message translates to:
  /// **'Проверка не пройдена'**
  String get llmProvidersHealthFail;

  /// No description provided for @llmProvidersDeleteTitle.
  ///
  /// In ru, this message translates to:
  /// **'Удалить провайдера?'**
  String get llmProvidersDeleteTitle;

  /// No description provided for @llmProvidersDeleteConfirm.
  ///
  /// In ru, this message translates to:
  /// **'Удалить «{name}»? Агенты, привязанные к нему, останутся без провайдера.'**
  String llmProvidersDeleteConfirm(String name);

  /// No description provided for @llmProvidersDeleteFail.
  ///
  /// In ru, this message translates to:
  /// **'Ошибка удаления'**
  String get llmProvidersDeleteFail;

  /// No description provided for @llmProvidersAddTitle.
  ///
  /// In ru, this message translates to:
  /// **'Новый LLM-провайдер'**
  String get llmProvidersAddTitle;

  /// No description provided for @llmProvidersEditTitle.
  ///
  /// In ru, this message translates to:
  /// **'Редактирование провайдера'**
  String get llmProvidersEditTitle;

  /// No description provided for @llmProvidersFieldName.
  ///
  /// In ru, this message translates to:
  /// **'Имя'**
  String get llmProvidersFieldName;

  /// No description provided for @llmProvidersFieldKind.
  ///
  /// In ru, this message translates to:
  /// **'Тип'**
  String get llmProvidersFieldKind;

  /// No description provided for @llmProvidersFieldBaseURL.
  ///
  /// In ru, this message translates to:
  /// **'Base URL (опционально)'**
  String get llmProvidersFieldBaseURL;

  /// No description provided for @llmProvidersFieldCredential.
  ///
  /// In ru, this message translates to:
  /// **'API-ключ / токен'**
  String get llmProvidersFieldCredential;

  /// No description provided for @llmProvidersFieldCredentialOptional.
  ///
  /// In ru, this message translates to:
  /// **'API-ключ / токен (пусто — не менять)'**
  String get llmProvidersFieldCredentialOptional;

  /// No description provided for @llmProvidersFieldDefaultModel.
  ///
  /// In ru, this message translates to:
  /// **'Модель по умолчанию'**
  String get llmProvidersFieldDefaultModel;

  /// No description provided for @llmProvidersFieldEnabled.
  ///
  /// In ru, this message translates to:
  /// **'Включён'**
  String get llmProvidersFieldEnabled;

  /// No description provided for @llmProvidersTest.
  ///
  /// In ru, this message translates to:
  /// **'Тест'**
  String get llmProvidersTest;

  /// No description provided for @llmProvidersTestOK.
  ///
  /// In ru, this message translates to:
  /// **'Тестовое подключение успешно'**
  String get llmProvidersTestOK;

  /// No description provided for @llmProvidersTestFail.
  ///
  /// In ru, this message translates to:
  /// **'Тест подключения не пройден'**
  String get llmProvidersTestFail;

  /// No description provided for @claudeCodeAuthLoadError.
  ///
  /// In ru, this message translates to:
  /// **'Не удалось загрузить статус подписки Claude Code'**
  String get claudeCodeAuthLoadError;

  /// No description provided for @claudeCodeAuthConnectedTitle.
  ///
  /// In ru, this message translates to:
  /// **'Подписка Claude Code подключена'**
  String get claudeCodeAuthConnectedTitle;

  /// No description provided for @claudeCodeAuthTokenType.
  ///
  /// In ru, this message translates to:
  /// **'Тип токена'**
  String get claudeCodeAuthTokenType;

  /// No description provided for @claudeCodeAuthScopes.
  ///
  /// In ru, this message translates to:
  /// **'Scopes'**
  String get claudeCodeAuthScopes;

  /// No description provided for @claudeCodeAuthExpiresAt.
  ///
  /// In ru, this message translates to:
  /// **'Истекает'**
  String get claudeCodeAuthExpiresAt;

  /// No description provided for @claudeCodeAuthLastRefreshedAt.
  ///
  /// In ru, this message translates to:
  /// **'Последнее обновление'**
  String get claudeCodeAuthLastRefreshedAt;

  /// No description provided for @claudeCodeAuthRevoke.
  ///
  /// In ru, this message translates to:
  /// **'Отозвать'**
  String get claudeCodeAuthRevoke;

  /// No description provided for @claudeCodeAuthRevokeOK.
  ///
  /// In ru, this message translates to:
  /// **'Подписка отозвана'**
  String get claudeCodeAuthRevokeOK;

  /// No description provided for @claudeCodeAuthDisconnectedTitle.
  ///
  /// In ru, this message translates to:
  /// **'Подписка Claude Code'**
  String get claudeCodeAuthDisconnectedTitle;

  /// No description provided for @claudeCodeAuthDisconnectedHint.
  ///
  /// In ru, this message translates to:
  /// **'Войдите по подписке Claude Code, чтобы агенты использовали OAuth-токен вместо долгоживущего API-ключа.'**
  String get claudeCodeAuthDisconnectedHint;

  /// No description provided for @claudeCodeAuthLogin.
  ///
  /// In ru, this message translates to:
  /// **'Войти по подписке'**
  String get claudeCodeAuthLogin;

  /// No description provided for @claudeCodeAuthDeviceFlowTitle.
  ///
  /// In ru, this message translates to:
  /// **'Подтверждение на стороне Anthropic'**
  String get claudeCodeAuthDeviceFlowTitle;

  /// No description provided for @claudeCodeAuthEnterCodeHint.
  ///
  /// In ru, this message translates to:
  /// **'Откройте ссылку ниже в любом браузере и введите этот код, чтобы авторизовать DevTeam:'**
  String get claudeCodeAuthEnterCodeHint;

  /// No description provided for @claudeCodeAuthWaiting.
  ///
  /// In ru, this message translates to:
  /// **'Ожидание подтверждения…'**
  String get claudeCodeAuthWaiting;

  /// No description provided for @agentSandboxSettingsTitle.
  ///
  /// In ru, this message translates to:
  /// **'Дополнительные настройки агента'**
  String get agentSandboxSettingsTitle;

  /// No description provided for @agentSandboxSettingsLoadError.
  ///
  /// In ru, this message translates to:
  /// **'Не удалось загрузить настройки агента'**
  String get agentSandboxSettingsLoadError;

  /// No description provided for @agentSandboxSettingsTabProvider.
  ///
  /// In ru, this message translates to:
  /// **'Модель / провайдер'**
  String get agentSandboxSettingsTabProvider;

  /// No description provided for @agentSandboxSettingsTabMCP.
  ///
  /// In ru, this message translates to:
  /// **'MCP-серверы'**
  String get agentSandboxSettingsTabMCP;

  /// No description provided for @agentSandboxSettingsTabSkills.
  ///
  /// In ru, this message translates to:
  /// **'Skills'**
  String get agentSandboxSettingsTabSkills;

  /// No description provided for @agentSandboxSettingsTabPermissions.
  ///
  /// In ru, this message translates to:
  /// **'Разрешения'**
  String get agentSandboxSettingsTabPermissions;

  /// No description provided for @agentSandboxSettingsProviderLabel.
  ///
  /// In ru, this message translates to:
  /// **'LLM-провайдер'**
  String get agentSandboxSettingsProviderLabel;

  /// No description provided for @agentSandboxSettingsProviderNone.
  ///
  /// In ru, this message translates to:
  /// **'— нет —'**
  String get agentSandboxSettingsProviderNone;

  /// No description provided for @agentSandboxSettingsCodeBackendLabel.
  ///
  /// In ru, this message translates to:
  /// **'Code backend'**
  String get agentSandboxSettingsCodeBackendLabel;

  /// No description provided for @agentSandboxSettingsMCPHelper.
  ///
  /// In ru, this message translates to:
  /// **'JSON-массив привязок MCP — см. документацию.'**
  String get agentSandboxSettingsMCPHelper;

  /// No description provided for @agentSandboxSettingsSkillsHelper.
  ///
  /// In ru, this message translates to:
  /// **'JSON-массив Skills (Claude Code) — см. документацию.'**
  String get agentSandboxSettingsSkillsHelper;

  /// No description provided for @agentSandboxSettingsDefaultMode.
  ///
  /// In ru, this message translates to:
  /// **'Режим по умолчанию'**
  String get agentSandboxSettingsDefaultMode;

  /// No description provided for @agentSandboxSettingsAllow.
  ///
  /// In ru, this message translates to:
  /// **'Allow'**
  String get agentSandboxSettingsAllow;

  /// No description provided for @agentSandboxSettingsDeny.
  ///
  /// In ru, this message translates to:
  /// **'Deny'**
  String get agentSandboxSettingsDeny;

  /// No description provided for @agentSandboxSettingsAsk.
  ///
  /// In ru, this message translates to:
  /// **'Ask'**
  String get agentSandboxSettingsAsk;

  /// No description provided for @agentSandboxSettingsJsonInvalid.
  ///
  /// In ru, this message translates to:
  /// **'Некорректный JSON'**
  String get agentSandboxSettingsJsonInvalid;

  /// No description provided for @agentSandboxSettingsPatternHint.
  ///
  /// In ru, this message translates to:
  /// **'Read | Edit | Bash(go test:*) | mcp__server'**
  String get agentSandboxSettingsPatternHint;

  /// No description provided for @agentSandboxSettingsTabToolsets.
  ///
  /// In ru, this message translates to:
  /// **'Toolsets'**
  String get agentSandboxSettingsTabToolsets;

  /// No description provided for @agentSandboxSettingsHermesToolsetsLabel.
  ///
  /// In ru, this message translates to:
  /// **'Hermes toolsets'**
  String get agentSandboxSettingsHermesToolsetsLabel;

  /// No description provided for @agentSandboxSettingsHermesToolsetsHelper.
  ///
  /// In ru, this message translates to:
  /// **'Выберите, какие Hermes toolsets доступны агенту.'**
  String get agentSandboxSettingsHermesToolsetsHelper;

  /// No description provided for @agentSandboxSettingsHermesPermLabel.
  ///
  /// In ru, this message translates to:
  /// **'Permission mode'**
  String get agentSandboxSettingsHermesPermLabel;

  /// No description provided for @agentSandboxSettingsHermesPermHelper.
  ///
  /// In ru, this message translates to:
  /// **'В headless-sandbox разрешены только yolo и accept.'**
  String get agentSandboxSettingsHermesPermHelper;

  /// No description provided for @agentSandboxSettingsHermesMaxTurnsLabel.
  ///
  /// In ru, this message translates to:
  /// **'Макс. число шагов'**
  String get agentSandboxSettingsHermesMaxTurnsLabel;

  /// No description provided for @agentSandboxSettingsHermesTemperatureLabel.
  ///
  /// In ru, this message translates to:
  /// **'Temperature (опц.)'**
  String get agentSandboxSettingsHermesTemperatureLabel;

  /// No description provided for @agentSandboxRevokeConfirmTitle.
  ///
  /// In ru, this message translates to:
  /// **'Отозвать подписку Claude Code?'**
  String get agentSandboxRevokeConfirmTitle;

  /// No description provided for @agentSandboxRevokeConfirmBody.
  ///
  /// In ru, this message translates to:
  /// **'Агенты будут использовать ANTHROPIC_API_KEY (если задан) при следующих sandbox-задачах. Подписку можно подключить заново в любой момент.'**
  String get agentSandboxRevokeConfirmBody;

  /// No description provided for @teamAgentEditAdvanced.
  ///
  /// In ru, this message translates to:
  /// **'Дополнительно'**
  String get teamAgentEditAdvanced;

  /// No description provided for @commonRequestFailed.
  ///
  /// In ru, this message translates to:
  /// **'Ошибка запроса'**
  String get commonRequestFailed;

  /// No description provided for @commonRequiredField.
  ///
  /// In ru, this message translates to:
  /// **'Обязательное поле'**
  String get commonRequiredField;

  /// No description provided for @commonCancel.
  ///
  /// In ru, this message translates to:
  /// **'Отмена'**
  String get commonCancel;

  /// No description provided for @commonSave.
  ///
  /// In ru, this message translates to:
  /// **'Сохранить'**
  String get commonSave;

  /// No description provided for @commonCreate.
  ///
  /// In ru, this message translates to:
  /// **'Создать'**
  String get commonCreate;

  /// No description provided for @commonConfirm.
  ///
  /// In ru, this message translates to:
  /// **'Подтвердить'**
  String get commonConfirm;

  /// No description provided for @agentsV2Title.
  ///
  /// In ru, this message translates to:
  /// **'Агенты (v2)'**
  String get agentsV2Title;

  /// No description provided for @agentsV2Empty.
  ///
  /// In ru, this message translates to:
  /// **'Пока нет ни одного агента.'**
  String get agentsV2Empty;

  /// No description provided for @agentsV2Refresh.
  ///
  /// In ru, this message translates to:
  /// **'Обновить'**
  String get agentsV2Refresh;

  /// No description provided for @agentsV2CreateButton.
  ///
  /// In ru, this message translates to:
  /// **'Новый агент'**
  String get agentsV2CreateButton;

  /// No description provided for @agentsV2CreateTitle.
  ///
  /// In ru, this message translates to:
  /// **'Создать агента'**
  String get agentsV2CreateTitle;

  /// No description provided for @agentsV2DetailTitle.
  ///
  /// In ru, this message translates to:
  /// **'Агент'**
  String get agentsV2DetailTitle;

  /// No description provided for @agentsV2KindLlm.
  ///
  /// In ru, this message translates to:
  /// **'LLM'**
  String get agentsV2KindLlm;

  /// No description provided for @agentsV2KindSandbox.
  ///
  /// In ru, this message translates to:
  /// **'Sandbox'**
  String get agentsV2KindSandbox;

  /// No description provided for @agentsV2FieldId.
  ///
  /// In ru, this message translates to:
  /// **'ID'**
  String get agentsV2FieldId;

  /// No description provided for @agentsV2FieldName.
  ///
  /// In ru, this message translates to:
  /// **'Имя'**
  String get agentsV2FieldName;

  /// No description provided for @agentsV2FieldRole.
  ///
  /// In ru, this message translates to:
  /// **'Роль'**
  String get agentsV2FieldRole;

  /// No description provided for @agentsV2FieldExecutionKind.
  ///
  /// In ru, this message translates to:
  /// **'Тип исполнения'**
  String get agentsV2FieldExecutionKind;

  /// No description provided for @agentsV2FieldRoleDescription.
  ///
  /// In ru, this message translates to:
  /// **'Описание роли (попадает в промпт Router\'а)'**
  String get agentsV2FieldRoleDescription;

  /// No description provided for @agentsV2FieldSystemPrompt.
  ///
  /// In ru, this message translates to:
  /// **'Системный промпт'**
  String get agentsV2FieldSystemPrompt;

  /// No description provided for @agentsV2FieldModel.
  ///
  /// In ru, this message translates to:
  /// **'Модель'**
  String get agentsV2FieldModel;

  /// No description provided for @agentsV2FieldTemperature.
  ///
  /// In ru, this message translates to:
  /// **'Температура'**
  String get agentsV2FieldTemperature;

  /// No description provided for @agentsV2FieldMaxTokens.
  ///
  /// In ru, this message translates to:
  /// **'Max tokens'**
  String get agentsV2FieldMaxTokens;

  /// No description provided for @agentsV2FieldCodeBackend.
  ///
  /// In ru, this message translates to:
  /// **'Code backend'**
  String get agentsV2FieldCodeBackend;

  /// No description provided for @agentsV2FieldIsActive.
  ///
  /// In ru, this message translates to:
  /// **'Активен'**
  String get agentsV2FieldIsActive;

  /// No description provided for @agentsV2SectionConfig.
  ///
  /// In ru, this message translates to:
  /// **'Конфигурация'**
  String get agentsV2SectionConfig;

  /// No description provided for @agentsV2AddSecretButton.
  ///
  /// In ru, this message translates to:
  /// **'Добавить / обновить секрет'**
  String get agentsV2AddSecretButton;

  /// No description provided for @agentsV2SavedSnackbar.
  ///
  /// In ru, this message translates to:
  /// **'Агент сохранён.'**
  String get agentsV2SavedSnackbar;

  /// No description provided for @agentsV2SecretSaved.
  ///
  /// In ru, this message translates to:
  /// **'Секрет сохранён (зашифрован).'**
  String get agentsV2SecretSaved;

  /// No description provided for @agentsV2SecretDialogTitle.
  ///
  /// In ru, this message translates to:
  /// **'Установить секрет агента'**
  String get agentsV2SecretDialogTitle;

  /// No description provided for @agentsV2SecretKeyName.
  ///
  /// In ru, this message translates to:
  /// **'Имя ключа'**
  String get agentsV2SecretKeyName;

  /// No description provided for @agentsV2SecretValue.
  ///
  /// In ru, this message translates to:
  /// **'Значение'**
  String get agentsV2SecretValue;

  /// No description provided for @agentsV2SecretValueHelper.
  ///
  /// In ru, this message translates to:
  /// **'Шифруется AES-256-GCM. Прочитать обратно нельзя — введите заново для ротации.'**
  String get agentsV2SecretValueHelper;

  /// No description provided for @agentsV2SecretsHint.
  ///
  /// In ru, this message translates to:
  /// **'Секреты хранятся зашифрованными на сервере и никогда не возвращаются клиенту. Используйте кнопку выше, чтобы установить/обновить значение.'**
  String get agentsV2SecretsHint;

  /// No description provided for @tasksCancelButton.
  ///
  /// In ru, this message translates to:
  /// **'Отменить задачу'**
  String get tasksCancelButton;

  /// No description provided for @tasksCancelConfirmTitle.
  ///
  /// In ru, this message translates to:
  /// **'Отменить задачу?'**
  String get tasksCancelConfirmTitle;

  /// No description provided for @tasksCancelConfirmBody.
  ///
  /// In ru, this message translates to:
  /// **'Все активные агенты будут прерваны, задача переведётся в статус cancelled.'**
  String get tasksCancelConfirmBody;

  /// No description provided for @tasksCancelInflightSuccess.
  ///
  /// In ru, this message translates to:
  /// **'Отмена отправлена. Агенты остановятся в ближайшее время.'**
  String get tasksCancelInflightSuccess;

  /// No description provided for @tasksCustomTimeoutLabel.
  ///
  /// In ru, this message translates to:
  /// **'Свой таймаут (например 4h, 90m, 3600s)'**
  String get tasksCustomTimeoutLabel;

  /// No description provided for @tasksCustomTimeoutHelper.
  ///
  /// In ru, this message translates to:
  /// **'Переопределяет дефолтные 4 часа оркестрации. Мин 1m, макс 72h.'**
  String get tasksCustomTimeoutHelper;

  /// No description provided for @tasksCustomTimeoutInvalid.
  ///
  /// In ru, this message translates to:
  /// **'Некорректный формат. Используйте Nh / Nm / Ns.'**
  String get tasksCustomTimeoutInvalid;

  /// No description provided for @tasksCustomTimeoutSectionTitle.
  ///
  /// In ru, this message translates to:
  /// **'Таймаут'**
  String get tasksCustomTimeoutSectionTitle;

  /// No description provided for @tasksCustomTimeoutNone.
  ///
  /// In ru, this message translates to:
  /// **'По умолчанию (4h)'**
  String get tasksCustomTimeoutNone;

  /// No description provided for @tasksCustomTimeoutEdit.
  ///
  /// In ru, this message translates to:
  /// **'Изменить'**
  String get tasksCustomTimeoutEdit;

  /// No description provided for @tasksCustomTimeoutSave.
  ///
  /// In ru, this message translates to:
  /// **'Сохранить'**
  String get tasksCustomTimeoutSave;

  /// No description provided for @tasksCustomTimeoutClear.
  ///
  /// In ru, this message translates to:
  /// **'Сбросить к дефолту'**
  String get tasksCustomTimeoutClear;

  /// No description provided for @tasksCustomTimeoutClearDialogTitle.
  ///
  /// In ru, this message translates to:
  /// **'Сбросить таймаут?'**
  String get tasksCustomTimeoutClearDialogTitle;

  /// No description provided for @tasksCustomTimeoutClearDialogBody.
  ///
  /// In ru, this message translates to:
  /// **'Оркестратор откатится к глобальным 4 часам по умолчанию для этой задачи.'**
  String get tasksCustomTimeoutClearDialogBody;

  /// No description provided for @tasksCustomTimeoutSavedSnack.
  ///
  /// In ru, this message translates to:
  /// **'Таймаут обновлён.'**
  String get tasksCustomTimeoutSavedSnack;

  /// No description provided for @tasksCustomTimeoutClearedSnack.
  ///
  /// In ru, this message translates to:
  /// **'Таймаут сброшен к дефолту.'**
  String get tasksCustomTimeoutClearedSnack;

  /// No description provided for @worktreesTitle.
  ///
  /// In ru, this message translates to:
  /// **'Worktrees (отладка)'**
  String get worktreesTitle;

  /// No description provided for @worktreesEmpty.
  ///
  /// In ru, this message translates to:
  /// **'Нет активных worktree\'ов.'**
  String get worktreesEmpty;

  /// No description provided for @worktreesColTask.
  ///
  /// In ru, this message translates to:
  /// **'Задача'**
  String get worktreesColTask;

  /// No description provided for @worktreesColBranch.
  ///
  /// In ru, this message translates to:
  /// **'Ветка'**
  String get worktreesColBranch;

  /// No description provided for @worktreesColState.
  ///
  /// In ru, this message translates to:
  /// **'Статус'**
  String get worktreesColState;

  /// No description provided for @worktreesColAllocated.
  ///
  /// In ru, this message translates to:
  /// **'Создан'**
  String get worktreesColAllocated;

  /// No description provided for @worktreesReleaseButton.
  ///
  /// In ru, this message translates to:
  /// **'Принудительно освободить'**
  String get worktreesReleaseButton;

  /// No description provided for @worktreesReleasedSnackbar.
  ///
  /// In ru, this message translates to:
  /// **'Worktree освобождён.'**
  String get worktreesReleasedSnackbar;

  /// No description provided for @worktreesReleaseDialogTitle.
  ///
  /// In ru, this message translates to:
  /// **'Принудительно освободить worktree?'**
  String get worktreesReleaseDialogTitle;

  /// No description provided for @worktreesReleaseDialogBody.
  ///
  /// In ru, this message translates to:
  /// **'git worktree remove --force произойдёт прямо сейчас. Агент (если работает) потеряет рабочий каталог и незакоммиченные изменения.'**
  String get worktreesReleaseDialogBody;

  /// No description provided for @worktreesReleaseAlreadyReleased.
  ///
  /// In ru, this message translates to:
  /// **'Worktree уже был освобождён.'**
  String get worktreesReleaseAlreadyReleased;

  /// No description provided for @worktreesReleaseFailed.
  ///
  /// In ru, this message translates to:
  /// **'Не удалось освободить worktree.'**
  String get worktreesReleaseFailed;

  /// No description provided for @worktreesReleaseNotConfigured.
  ///
  /// In ru, this message translates to:
  /// **'Worktree manager не сконфигурирован на сервере (WORKTREES_ROOT / REPO_ROOT не заданы). Попросите оператора включить фичу.'**
  String get worktreesReleaseNotConfigured;

  /// No description provided for @worktreesFilterAll.
  ///
  /// In ru, this message translates to:
  /// **'Все'**
  String get worktreesFilterAll;

  /// No description provided for @worktreesFilterAllocated.
  ///
  /// In ru, this message translates to:
  /// **'Allocated'**
  String get worktreesFilterAllocated;

  /// No description provided for @worktreesFilterInUse.
  ///
  /// In ru, this message translates to:
  /// **'In use'**
  String get worktreesFilterInUse;

  /// No description provided for @worktreesFilterReleased.
  ///
  /// In ru, this message translates to:
  /// **'Released'**
  String get worktreesFilterReleased;

  /// No description provided for @routerTimelineSection.
  ///
  /// In ru, this message translates to:
  /// **'Лента решений Router\'а'**
  String get routerTimelineSection;

  /// No description provided for @routerTimelineEmpty.
  ///
  /// In ru, this message translates to:
  /// **'Решений Router\'а пока нет.'**
  String get routerTimelineEmpty;

  /// No description provided for @artifactsSection.
  ///
  /// In ru, this message translates to:
  /// **'Артефакты'**
  String get artifactsSection;

  /// No description provided for @artifactsEmpty.
  ///
  /// In ru, this message translates to:
  /// **'Артефактов пока нет.'**
  String get artifactsEmpty;

  /// No description provided for @artifactViewerOpen.
  ///
  /// In ru, this message translates to:
  /// **'Открыть артефакт полностью'**
  String get artifactViewerOpen;

  /// No description provided for @artifactViewerTitle.
  ///
  /// In ru, this message translates to:
  /// **'{kind} · {idShort}'**
  String artifactViewerTitle(String kind, String idShort);

  /// No description provided for @artifactViewerClose.
  ///
  /// In ru, this message translates to:
  /// **'Закрыть'**
  String get artifactViewerClose;

  /// No description provided for @artifactViewerCopyFull.
  ///
  /// In ru, this message translates to:
  /// **'Скопировать всё содержимое'**
  String get artifactViewerCopyFull;

  /// No description provided for @artifactViewerCopyFullForKind.
  ///
  /// In ru, this message translates to:
  /// **'Скопировать весь {kind}'**
  String artifactViewerCopyFullForKind(String kind);

  /// No description provided for @artifactViewerCopiedSnack.
  ///
  /// In ru, this message translates to:
  /// **'Скопировано {bytes} байт в буфер обмена.'**
  String artifactViewerCopiedSnack(int bytes);

  /// No description provided for @artifactViewerCopyFailedSnack.
  ///
  /// In ru, this message translates to:
  /// **'Не удалось скопировать в буфер обмена.'**
  String get artifactViewerCopyFailedSnack;

  /// No description provided for @artifactViewerShowFull.
  ///
  /// In ru, this message translates to:
  /// **'Показать полностью ({kb} КБ)'**
  String artifactViewerShowFull(int kb);

  /// No description provided for @artifactViewerShowNext.
  ///
  /// In ru, this message translates to:
  /// **'Показать следующие {n}'**
  String artifactViewerShowNext(int n);

  /// No description provided for @artifactViewerTruncatedNotice.
  ///
  /// In ru, this message translates to:
  /// **'Показаны первые {kb} КБ из {totalKb} КБ.'**
  String artifactViewerTruncatedNotice(int kb, int totalKb);

  /// No description provided for @artifactViewerEmpty.
  ///
  /// In ru, this message translates to:
  /// **'У артефакта нет сохранённого содержимого.'**
  String get artifactViewerEmpty;

  /// No description provided for @artifactViewerLoadFailed.
  ///
  /// In ru, this message translates to:
  /// **'Не удалось загрузить артефакт: {error}'**
  String artifactViewerLoadFailed(String error);

  /// No description provided for @artifactViewerReviewDecision.
  ///
  /// In ru, this message translates to:
  /// **'Решение'**
  String get artifactViewerReviewDecision;

  /// No description provided for @artifactViewerReviewIssues.
  ///
  /// In ru, this message translates to:
  /// **'Замечания'**
  String get artifactViewerReviewIssues;

  /// No description provided for @artifactViewerReviewSummary.
  ///
  /// In ru, this message translates to:
  /// **'Итог'**
  String get artifactViewerReviewSummary;

  /// No description provided for @artifactViewerReviewNoIssues.
  ///
  /// In ru, this message translates to:
  /// **'Замечаний нет.'**
  String get artifactViewerReviewNoIssues;

  /// No description provided for @artifactViewerTestPassed.
  ///
  /// In ru, this message translates to:
  /// **'Прошло'**
  String get artifactViewerTestPassed;

  /// No description provided for @artifactViewerTestFailed.
  ///
  /// In ru, this message translates to:
  /// **'Упало'**
  String get artifactViewerTestFailed;

  /// No description provided for @artifactViewerTestSkipped.
  ///
  /// In ru, this message translates to:
  /// **'Пропущено'**
  String get artifactViewerTestSkipped;

  /// No description provided for @artifactViewerTestDuration.
  ///
  /// In ru, this message translates to:
  /// **'Длительность'**
  String get artifactViewerTestDuration;

  /// No description provided for @artifactViewerTestDurationMs.
  ///
  /// In ru, this message translates to:
  /// **'{ms} мс'**
  String artifactViewerTestDurationMs(int ms);

  /// No description provided for @artifactViewerTestFailuresHeader.
  ///
  /// In ru, this message translates to:
  /// **'Падения ({n})'**
  String artifactViewerTestFailuresHeader(int n);

  /// No description provided for @artifactViewerTestFailureFile.
  ///
  /// In ru, this message translates to:
  /// **'{file}:{line}'**
  String artifactViewerTestFailureFile(String file, int line);

  /// No description provided for @artifactViewerTestNoFailures.
  ///
  /// In ru, this message translates to:
  /// **'Все проверки зелёные.'**
  String get artifactViewerTestNoFailures;

  /// No description provided for @artifactsNoSummary.
  ///
  /// In ru, this message translates to:
  /// **'(без описания)'**
  String get artifactsNoSummary;

  /// No description provided for @artifactViewerTestUnnamed.
  ///
  /// In ru, this message translates to:
  /// **'(без имени)'**
  String get artifactViewerTestUnnamed;

  /// No description provided for @artifactViewerFullTitle.
  ///
  /// In ru, this message translates to:
  /// **'{kind} · полностью'**
  String artifactViewerFullTitle(String kind);
}

class _AppLocalizationsDelegate
    extends LocalizationsDelegate<AppLocalizations> {
  const _AppLocalizationsDelegate();

  @override
  Future<AppLocalizations> load(Locale locale) {
    return SynchronousFuture<AppLocalizations>(lookupAppLocalizations(locale));
  }

  @override
  bool isSupported(Locale locale) =>
      <String>['en', 'ru'].contains(locale.languageCode);

  @override
  bool shouldReload(_AppLocalizationsDelegate old) => false;
}

AppLocalizations lookupAppLocalizations(Locale locale) {
  // Lookup logic when only language code is specified.
  switch (locale.languageCode) {
    case 'en':
      return AppLocalizationsEn();
    case 'ru':
      return AppLocalizationsRu();
  }

  throw FlutterError(
    'AppLocalizations.delegate failed to load unsupported locale "$locale". This is likely '
    'an issue with the localizations generation tool. Please file an issue '
    'on GitHub with a reproducible sample app and the gen-l10n configuration '
    'that was used.',
  );
}
