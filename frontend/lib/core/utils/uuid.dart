import 'package:uuid/uuid.dart';

/// Структурный матч UUID `8-4-4-4-12` (hex). Версия/вариант (RFC v1–v7) не ограничиваем —
/// чтобы v7 и будущие форматы не ломали редиректы; каноническая валидация остаётся на API.
final RegExp _uuidHexRegExp = RegExp(
  r'^[0-9a-fA-F]{8}-(?:[0-9a-fA-F]{4}-){3}[0-9a-fA-F]{12}$',
);

const Uuid _uuid = Uuid();

/// Новый UUID v4 для заголовка **`X-Client-Message-ID`** (идемпотентность отправки сообщений).
///
/// Единая точка генерации: фичи не импортируют `package:uuid/uuid.dart` напрямую. Проверка
/// формата на стороне API/репозитория — [isValidUuid].
String generateClientMessageId() => _uuid.v4();

/// Проверка строки как UUID в формате `8-4-4-4-12` (проекты, сообщения, идемпотентность и т.д.).
bool isValidUuid(String id) => _uuidHexRegExp.hasMatch(id);
