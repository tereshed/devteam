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
