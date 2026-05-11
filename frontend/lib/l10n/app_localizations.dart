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

  /// Фоллбэк роли агента (12.5)
  ///
  /// In ru, this message translates to:
  /// **'Неизвестная роль'**
  String get taskAgentRoleUnknown;

  /// Роль агента worker на детали задачи (12.5)
  ///
  /// In ru, this message translates to:
  /// **'Исполнитель'**
  String get taskAgentRoleWorker;

  /// Роль агента supervisor (12.5)
  ///
  /// In ru, this message translates to:
  /// **'Супервизор'**
  String get taskAgentRoleSupervisor;

  /// Роль агента orchestrator (12.5)
  ///
  /// In ru, this message translates to:
  /// **'Оркестратор'**
  String get taskAgentRoleOrchestrator;

  /// Роль агента planner (12.5)
  ///
  /// In ru, this message translates to:
  /// **'Планировщик'**
  String get taskAgentRolePlanner;

  /// Роль агента developer (12.5)
  ///
  /// In ru, this message translates to:
  /// **'Разработчик'**
  String get taskAgentRoleDeveloper;

  /// Роль агента reviewer (12.5)
  ///
  /// In ru, this message translates to:
  /// **'Ревьюер'**
  String get taskAgentRoleReviewer;

  /// Роль агента tester (12.5)
  ///
  /// In ru, this message translates to:
  /// **'Тестировщик'**
  String get taskAgentRoleTester;

  /// Роль агента devops (12.5)
  ///
  /// In ru, this message translates to:
  /// **'DevOps'**
  String get taskAgentRoleDevops;

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
