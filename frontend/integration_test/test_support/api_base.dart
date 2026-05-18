/// Базовый URL backend'а для интеграционных тестов.
///
/// По умолчанию `http://127.0.0.1:8080` — соответствует прод-конфигурации
/// `lib/core/api/dio_providers.dart`. Если когда-нибудь dioClient научится
/// читать `--dart-define=API_BASE`, тесты подцепят то же значение через
/// `kApiBase` без правок здесь.
library;

const String kApiBase = String.fromEnvironment(
  'API_BASE',
  defaultValue: 'http://127.0.0.1:8080',
);

/// REST-префикс. Все эндпоинты вида `${kApiV1}/auth/register` и т.п.
const String kApiV1 = '$kApiBase/api/v1';
