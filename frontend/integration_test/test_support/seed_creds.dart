import 'dart:math';

import 'package:frontend/features/auth/domain/models/user_model.dart';

import 'rest_client.dart';

/// Минимальные «креды» для bootstrap'а тестового UI: токен + UserModel.
///
/// Тесты инжектят их в `accessTokenProvider` / `authControllerProvider`,
/// чтобы обойти flutter_secure_storage (на macOS test-runner без entitlements
/// Keychain не работает) и сразу попасть в авторизованный UI.
class SeedCreds {
  SeedCreds({
    required this.token,
    required this.refreshToken,
    required this.user,
    required this.email,
    required this.password,
  });

  final String token;
  final String refreshToken;
  final UserModel user;
  final String email;
  final String password;
}

String _randomHex(int bytes) {
  final r = Random.secure();
  return List.generate(bytes, (_) => r.nextInt(16).toRadixString(16)).join();
}

/// Уникальный e-mail для тестового пользователя.
///
/// `prefix` помогает читать логи бэка: `auth-flow-...@example.com`,
/// `projects-flow-...@example.com`. UUID-подобный хвост даёт Tenant-изоляцию
/// без TRUNCATE (см. docs/integration-tests-plan.md).
String uniqueTestEmail(String prefix) => '$prefix-${_randomHex(8)}@example.com';

/// Регистрирует уникального пользователя через REST `/auth/register`
/// и сразу получает [UserModel] через `/auth/me`.
///
/// Пароль ≥16 символов (политика бэка). Returns [SeedCreds].
///
/// **НЕ** трогает flutter_secure_storage: токен возвращается «голым» —
/// инжекция в провайдеры — обязанность вызывающего теста.
Future<SeedCreds> registerSeedUser({String prefix = 'flutter-e2e'}) async {
  final email = uniqueTestEmail(prefix);
  // Длина ≥16: backend требует крепкий пароль. Random hex + suffix.
  final password = 'Pass-${_randomHex(12)}!';
  final regJson = await TestRestClient.post(
    '/auth/register',
    body: {'email': email, 'password': password},
  );
  final accessToken = regJson['access_token'] as String;
  final refreshToken = (regJson['refresh_token'] as String?) ?? '';

  final meJson = await TestRestClient.get('/auth/me', token: accessToken);
  final user = UserModel.fromJson(meJson);
  return SeedCreds(
    token: accessToken,
    refreshToken: refreshToken,
    user: user,
    email: email,
    password: password,
  );
}

/// Создаёт пустой `local`-проект через REST под seed-юзером.
///
/// Возвращает project_id. Local-провайдер не требует git-credential —
/// идеален для smoke-тестов проектного CRUD и task-lifecycle.
Future<String> createLocalProject(
  String token, {
  String namePrefix = 'flutter-e2e',
  String? description,
}) async {
  final name = '$namePrefix-${_randomHex(6)}';
  final json = await TestRestClient.post(
    '/projects',
    token: token,
    body: {
      'name': name,
      'description':
          description ?? 'auto-created for frontend integration test',
      'git_provider': 'local',
    },
  );
  return json['id'] as String;
}

/// Создаёт задачу через REST `/projects/:id/tasks`.
///
/// Задача стартует в статусе `active` (см. tasks_smoke_test.go) —
/// готова к pause/resume/cancel без участия v2-воркеров. Возвращает
/// `task_id` для последующей навигации в UI.
Future<String> createSeedTask({
  required String token,
  required String projectId,
  String title = 'Frontend integration smoke task',
  String description = '',
}) async {
  final json = await TestRestClient.post(
    '/projects/$projectId/tasks',
    token: token,
    body: {'title': title, 'description': description},
  );
  return json['id'] as String;
}

/// GET `/api/v1/tasks/:id` под seed-токеном — нужен тестам, которые
/// проверяют статус задачи после UI-действия.
Future<Map<String, dynamic>> fetchTaskRaw({
  required String token,
  required String taskId,
}) => TestRestClient.get('/tasks/$taskId', token: token);
