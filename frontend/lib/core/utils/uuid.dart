/// Структурный матч UUID `8-4-4-4-12` (hex). Версия/вариант (RFC v1–v7) не ограничиваем —
/// чтобы v7 и будущие форматы не ломали редиректы; каноническая валидация остаётся на API.
final RegExp _projectUuidRegExp = RegExp(
  r'^[0-9a-fA-F]{8}-(?:[0-9a-fA-F]{4}-){3}[0-9a-fA-F]{12}$',
);

/// Проверка строки как UUID в формате, совместимом с идентификаторами проекта в API.
bool isValidProjectUuid(String id) => _projectUuidRegExp.hasMatch(id);
