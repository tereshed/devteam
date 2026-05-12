// Sprint 14.2 — E2E integration-тест полного UI-flow:
// register (через REST) → авторизованная UI-сессия → /projects → создание
// проекта через форму → новый проект виден в списке.
//
// Авторизация через UI на macOS-test runner упирается в flutter_secure_storage
// + Keychain entitlements (без dev-сертификата FlutterSecureStorage не работает),
// поэтому шаги register/login делаем напрямую через REST, токен инжектим в
// accessTokenProvider при старте приложения через ProviderScope.overrides.
// Это покрывает то, что 14.2 действительно проверяет — полный UI-flow поверх
// реального бэкенда: переходы экранов, формы, отображение данных из API.
//
// Запуск: при поднятом стеке `make e2e-frontend` (см. Makefile).
import 'dart:convert';
import 'dart:io';
import 'dart:math';

import 'package:flutter/material.dart';
import 'package:flutter_localizations/flutter_localizations.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/core/routing/app_router.dart';
import 'package:frontend/core/storage/token_provider.dart';
import 'package:frontend/features/auth/domain/models/user_model.dart';
import 'package:frontend/features/auth/presentation/controllers/auth_controller.dart';
import 'package:frontend/core/theme/app_theme.dart';
import 'package:frontend/l10n/app_localizations.dart';
import 'package:go_router/go_router.dart';
import 'package:integration_test/integration_test.dart';

const _apiBase = 'http://127.0.0.1:8080';

/// Проверяет, что бэкенд доступен; иначе t.Skip аналог.
Future<bool> _backendAvailable() async {
  try {
    final client = HttpClient();
    client.connectionTimeout = const Duration(seconds: 2);
    final req = await client.getUrl(Uri.parse('$_apiBase/health'));
    final resp = await req.close().timeout(const Duration(seconds: 3));
    client.close();
    return resp.statusCode == 200;
  } catch (_) {
    return false;
  }
}

/// Минимальные «креды» теста: токен + пользователь, чтобы инжектить и в
/// accessTokenProvider, и в authControllerProvider (минуя flutter_secure_storage).
class _SeedCreds {
  _SeedCreds(this.token, this.user);
  final String token;
  final UserModel user;
}

/// Создаёт уникального пользователя через REST и возвращает access_token+UserModel.
Future<_SeedCreds> _registerAndSeed() async {
  final r = Random.secure();
  final id = List.generate(8, (_) => r.nextInt(16).toRadixString(16)).join();
  final email = 'flutter-e2e-$id@example.com';
  final body = jsonEncode({'email': email, 'password': 'Password123!'});
  final client = HttpClient();
  try {
    final req = await client.postUrl(Uri.parse('$_apiBase/api/v1/auth/register'));
    req.headers.set('Content-Type', 'application/json');
    req.write(body);
    final resp = await req.close();
    if (resp.statusCode != 201) {
      throw Exception('register failed: ${resp.statusCode}');
    }
    final json = jsonDecode(await resp.transform(utf8.decoder).join()) as Map<String, dynamic>;
    final token = json['access_token'] as String;

    // Получаем UserModel: /me
    final meReq = await client.getUrl(Uri.parse('$_apiBase/api/v1/auth/me'));
    meReq.headers.set('Authorization', 'Bearer $token');
    final meResp = await meReq.close();
    if (meResp.statusCode != 200) {
      throw Exception('/me failed: ${meResp.statusCode}');
    }
    final meJson = jsonDecode(await meResp.transform(utf8.decoder).join()) as Map<String, dynamic>;
    return _SeedCreds(token, UserModel.fromJson(meJson));
  } finally {
    client.close();
  }
}

/// Доходит до состояния, когда финд возвращает >0 виджетов, либо фейлит.
Future<void> _expectEventually(
  WidgetTester tester,
  Finder finder, {
  Duration timeout = const Duration(seconds: 10),
  String? reason,
}) async {
  final deadline = DateTime.now().add(timeout);
  while (DateTime.now().isBefore(deadline)) {
    await tester.pump(const Duration(milliseconds: 200));
    if (finder.evaluate().isNotEmpty) {
      return;
    }
  }
  fail('Timeout waiting for ${reason ?? finder.toString()}');
}

BuildContext _anyContext(WidgetTester tester) {
  return tester.element(find.byType(Scaffold).first);
}

/// Минимальный «корень приложения» с overrides — повторяет `MainApp` из lib/main.dart,
/// но позволяет инжектить accessToken и обойти flutter_secure_storage.
class _TestApp extends StatelessWidget {
  const _TestApp();
  @override
  Widget build(BuildContext context) {
    return MaterialApp.router(
      onGenerateTitle: (context) => AppLocalizations.of(context)!.appTitle,
      theme: AppTheme.lightTheme,
      darkTheme: AppTheme.darkTheme,
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

void main() {
  IntegrationTestWidgetsFlutterBinding.ensureInitialized();

  testWidgets(
    'full UI flow: create project via form → visible in list',
    (tester) async {
      if (!await _backendAvailable()) {
        markTestSkipped(
            'backend at $_apiBase is not reachable; start with `docker compose up -d` and re-run');
        return;
      }

      final creds = await _registerAndSeed();

      // Стартуем приложение с предзаданными access-token и authState (минуя flutter_secure_storage).
      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            accessTokenProvider.overrideWith(() => _SeededAccessToken(creds.token)),
            authControllerProvider.overrideWith(() => _SeededAuthController(creds.user)),
          ],
          child: const _TestApp(),
        ),
      );
      await tester.pumpAndSettle(const Duration(seconds: 2));

      // Переходим на /projects (auth уже есть).
      GoRouter.of(_anyContext(tester)).go('/projects');
      // Дай времени API подгрузиться (/projects?limit=...).
      for (var i = 0; i < 30; i++) {
        await tester.pump(const Duration(milliseconds: 250));
      }

      final l10n = AppLocalizations.of(_anyContext(tester))!;
      try {
        await _expectEventually(tester, find.text(l10n.projectsTitle),
            reason: 'projects list title');
      } catch (e) {
        // ignore: avoid_print
        print('---- visible text on screen ----');
        for (final w in find.byType(Text).evaluate()) {
          final t = (w.widget as Text).data;
          if (t != null) {
            // ignore: avoid_print
            print('  $t');
          }
        }
        // ignore: avoid_print
        print('---- current location: ${GoRouter.of(_anyContext(tester)).routerDelegate.currentConfiguration.uri} ----');
        rethrow;
      }

      // ── Шаг A: открываем форму создания проекта ───────────────────────
      final createBtn = find.text(l10n.createProject);
      await _expectEventually(tester, createBtn,
          reason: 'create project button');
      await tester.tap(createBtn.first);
      await tester.pumpAndSettle(const Duration(seconds: 2));
      await _expectEventually(tester, find.text(l10n.createProjectScreenTitle),
          reason: 'create project screen');

      // ── Шаг B: заполняем форму. Дефолтный gitProvider в UI = "github" и
      // требует валидного git_credential на бэкенде. В тесте — переключаемся на
      // "local" (без удалённого URL): ProjectService разрешает local-провайдер
      // без git_url и без credential.
      final projectName = 'Flutter E2E ${DateTime.now().millisecondsSinceEpoch}';
      final cpFields = find.byType(TextFormField);
      expect(cpFields, findsAtLeast(2),
          reason: 'create project screen has at least name + description');
      await tester.enterText(cpFields.at(0), projectName);
      await tester.enterText(cpFields.at(1), 'Created from Flutter integration test');

      // Открываем dropdown и выбираем «Local».
      await tester.tap(find.byType(DropdownButtonFormField<String>));
      await tester.pumpAndSettle(const Duration(seconds: 1));
      // У DropdownMenuItem текст «Local» — берём текст из l10n.
      final localOption = find.text(l10n.gitProviderLocal);
      expect(localOption, findsAtLeastNWidgets(1),
          reason: 'local provider option in dropdown');
      await tester.tap(localOption.last);
      await tester.pumpAndSettle(const Duration(seconds: 1));

      final createSubmit = find.widgetWithText(FilledButton, l10n.create);
      expect(createSubmit, findsOneWidget);
      await tester.tap(createSubmit);
      // Сетевой запрос → возврат на список → обновление; pumpAndSettle не ждёт IO.
      for (var i = 0; i < 40; i++) {
        await tester.pump(const Duration(milliseconds: 250));
      }

      // ── Шаг C: после Create UI редиректит на /projects/{id}/chat
      // (project dashboard). Возвращаемся на список и проверяем, что
      // новый проект в нём виден.
      GoRouter.of(_anyContext(tester)).go('/projects');
      for (var i = 0; i < 30; i++) {
        await tester.pump(const Duration(milliseconds: 250));
      }
      await _expectEventually(tester, find.text(l10n.projectsTitle),
          timeout: const Duration(seconds: 10),
          reason: 'back to projects list');
      await _expectEventually(tester, find.text(projectName),
          timeout: const Duration(seconds: 10),
          reason: 'newly created project visible in list');
    },
    timeout: const Timeout(Duration(minutes: 2)),
  );
}

/// Сразу выдаёт переданный токен как состояние [accessTokenProvider], минуя
/// flutter_secure_storage. Используется только в тестовом override.
class _SeededAccessToken extends AccessToken {
  _SeededAccessToken(this._seed);
  final String _seed;

  @override
  String? build() => _seed;

  @override
  Future<void> init() async {
    state = _seed;
  }

  @override
  Future<void> setToken(String token) async {
    state = token;
  }

  @override
  Future<void> clear() async {
    state = null;
  }
}

/// Возвращает предзаданного пользователя без обращения к TokenStorage / /me.
/// Это обходит keychain (FlutterSecureStorage) на macOS-test runner'е.
class _SeededAuthController extends AuthController {
  _SeededAuthController(this._user);
  final UserModel _user;

  @override
  Future<UserModel?> build() async => _user;
}
