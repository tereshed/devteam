/// Безопасное представление `metadata` сообщения задачи для UI (12.5, review.md).
final RegExp _jwtLikePattern = RegExp(
  r'^[A-Za-z0-9_-]{16,}\.[A-Za-z0-9_-]{16,}\.[A-Za-z0-9_-]{8,}$',
);

const List<String> _deniedKeySubstrings = [
  'token',
  'secret',
  'auth',
  'password',
  'cookie',
  'api_key',
  'apikey',
  'authorization',
  'bearer',
];

bool _metadataKeyDenied(String key) {
  final lower = key.toLowerCase();
  for (final n in _deniedKeySubstrings) {
    if (lower.contains(n)) {
      return true;
    }
  }
  return false;
}

bool _stringValueSensitive(String raw) {
  final v = raw.trim();
  if (v.startsWith('sk-')) {
    return true;
  }
  if (v.startsWith('Bearer ')) {
    return true;
  }
  if (_jwtLikePattern.hasMatch(v)) {
    return true;
  }
  return false;
}

dynamic _redactMetadataRecursive(dynamic value) {
  if (value is Map<String, dynamic>) {
    return redactTaskMessageMetadata(value);
  }
  if (value is Map) {
    return Map<String, dynamic>.fromEntries(
      value.entries.map(
        (e) => MapEntry(
          e.key.toString(),
          redactTaskMessageMetadataEntry(e.key.toString(), e.value),
        ),
      ),
    );
  }
  if (value is List) {
    return value.map(_redactMetadataRecursive).toList();
  }
  // Строки внутри списков и прочие листья — те же эвристики, что у top-level String.
  if (value is String && _stringValueSensitive(value)) {
    return '***';
  }
  return value;
}

/// Редукция одной пары ключ → значение.
dynamic redactTaskMessageMetadataEntry(String key, dynamic value) {
  if (_metadataKeyDenied(key)) {
    return '***';
  }
  if (value is String) {
    return _stringValueSensitive(value) ? '***' : value;
  }
  return _redactMetadataRecursive(value);
}

/// Рекурсивная редукция корня `metadata`.
Map<String, dynamic> redactTaskMessageMetadata(Map<String, dynamic> raw) {
  return Map<String, dynamic>.fromEntries(
    raw.entries.map(
      (e) => MapEntry(e.key, redactTaskMessageMetadataEntry(e.key, e.value)),
    ),
  );
}
