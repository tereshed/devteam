import 'package:flutter/material.dart';
import 'package:flutter_localizations/flutter_localizations.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_riverpod/misc.dart' show Override;
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/core/routing/app_router.dart';
import 'package:frontend/core/storage/token_provider.dart';
import 'package:frontend/core/storage/token_storage.dart';
import 'package:frontend/core/theme/app_theme.dart';
import 'package:frontend/features/auth/domain/models/user_model.dart';
import 'package:frontend/features/auth/presentation/controllers/auth_controller.dart';
import 'package:frontend/l10n/app_localizations.dart';
import 'seed_creds.dart';

/// Минимальный корень приложения для интеграционных тестов.
///
/// Полностью повторяет `MainApp` (`lib/main.dart`), но через [ProviderScope.overrides]
/// мы подменяем `accessTokenProvider` и `authControllerProvider`, чтобы
/// обойти flutter_secure_storage (Keychain недоступен на macOS test-runner
/// без entitlements — см. docs/integration-tests-plan.md §macOS Keychain).
class TestApp extends StatelessWidget {
  const TestApp({super.key});

  @override
  Widget build(BuildContext context) {
    return MaterialApp.router(
      onGenerateTitle: (context) => AppLocalizations.of(context)!.appTitle,
      theme: AppTheme.lightTheme,
      darkTheme: AppTheme.darkTheme,
      // Темная тема в тестах вносит шум при отладке скриншотов — фиксируем light.
      themeMode: ThemeMode.light,
      routerConfig: AppRouter.router,
      localizationsDelegates: const [
        AppLocalizations.delegate,
        GlobalMaterialLocalizations.delegate,
        GlobalWidgetsLocalizations.delegate,
        GlobalCupertinoLocalizations.delegate,
      ],
      supportedLocales: const [Locale('ru', ''), Locale('en', '')],
    );
  }
}

/// Подаёт в `accessTokenProvider` готовый токен — минуя TokenStorage.
class SeededAccessToken extends AccessToken {
  SeededAccessToken(this._seed);
  final String _seed;

  @override
  String? build() => _seed;

  @override
  Future<void> init() async => state = _seed;

  @override
  Future<void> setToken(String token) async => state = token;

  @override
  Future<void> clear() async => state = null;
}

/// Возвращает предзаданного пользователя без `/me` и TokenStorage.
class SeededAuthController extends AuthController {
  SeededAuthController(this._user);
  final UserModel? _user;

  @override
  Future<UserModel?> build() async => _user;
}

/// Очищает «локальные хранилища», которые приложение могло заполнить
/// в предыдущем тесте: flutter_secure_storage (mock-режим), плюс
/// сбрасывает любые задержавшиеся `pumpAndSettle`-таймеры через `tester`.
///
/// SharedPreferences / Hive в текущем приложении не используются
/// (см. exploration-отчёт), поэтому здесь они **намеренно** не упомянуты,
/// чтобы не подменять контракт мок-API того, чего нет.
Future<void> resetTestStorage() async {
  // На platform-каналах, которые реально гоняют плагин в integration_test
  // (macOS file-mode, iOS Keychain dev-build, Android encryptedSharedPreferences),
  // нужно реально стереть прошлые токены, иначе следующий тест увидит
  // stale-state предыдущего и `authGuard` пустит туда, куда не должен.
  // На web вызов `clearTokens` — no-op (LocalStorage), но безопасен.
  try {
    await TokenStorage.clearTokens();
  } on Exception {
    // Платформа без бэка к плагину (например, чистый Dart-юнит вне
    // integration_test binding) — игнорим, тестам всё равно: они инжектят
    // токен оверрайдом.
  }
  // setMockInitialValues подменяет MethodChannel, что страхует юнит-вариант
  // запуска. В integration_test (реальный плагин) — no-op.
  FlutterSecureStorage.setMockInitialValues(const {});
}

/// Фабрика «свежего» приложения для интеграционного теста.
///
/// Контракт:
///   - вызывает [resetTestStorage] (sanity-cleanup перед каждым тестом);
///   - если переданы `token` + `user` — инжектит их в Riverpod
///     (минуя `flutter_secure_storage` / `/auth/me`);
///   - **`ProviderContainer` создаётся новый** на каждый вызов — за счёт
///     того, что `ProviderScope` без `parent:` строит свой child.
///
/// `extraOverrides` позволяет тестам подменять любой провайдер
/// (например, `dioClientProvider` на mock или `webSocketServiceProvider`
/// на стаб). Префиксные overrides идут перед extraOverrides, чтобы у тестов
/// был последний голос.
Future<void> pumpFreshTestApp(
  WidgetTester tester, {
  String? token,
  UserModel? user,
  List<Override> extraOverrides = const [],
  Duration settle = const Duration(seconds: 2),
}) async {
  await resetTestStorage();

  final overrides = <Override>[
    if (token != null)
      accessTokenProvider.overrideWith(() => SeededAccessToken(token)),
    if (user != null || token != null)
      // Если передан только token, но не user — будем считать, что юзер
      // unknown; AuthController в build() сходит на `/me` через настоящий
      // dioClient (контракт `auth_controller.dart`). В абсолютном большинстве
      // тестов вызывающий код передаёт обоих → seeded UserModel.
      authControllerProvider.overrideWith(() => SeededAuthController(user)),
    ...extraOverrides,
  ];

  await tester.pumpWidget(
    ProviderScope(overrides: overrides, child: const TestApp()),
  );
  // pumpAndSettle не ждёт асинхронных I/O (например, /me / list projects).
  // Используем bounded loop в самих тестах через expectEventually, а здесь —
  // короткий settle, чтобы первый кадр и роутер успели подняться.
  await tester.pumpAndSettle(settle);
}

/// Аутентификацию-через-UI чаще всего проверяет один-два теста.
/// Все остальные используют bootstrap «уже залогинены»: регистрируем
/// пользователя REST'ом и сразу инжектим креды в провайдеры.
///
/// Возвращает [SeedCreds] (полезно тестам, которые потом сами создают
/// проекты/задачи через REST).
Future<SeedCreds> pumpFreshAuthedApp(
  WidgetTester tester, {
  String prefix = 'flutter-e2e',
  List<Override> extraOverrides = const [],
}) async {
  final creds = await registerSeedUser(prefix: prefix);
  await pumpFreshTestApp(
    tester,
    token: creds.token,
    user: creds.user,
    extraOverrides: extraOverrides,
  );
  return creds;
}
