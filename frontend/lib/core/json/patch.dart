/// Три-состояние для полей JSON PATCH: ключ отсутствует / `null` / значение.
///
/// См. контракт бэкенда `PatchAgentRequest` (omit / explicit null / value).
class Patch<T> {
  final int _tag;
  final T? _value;

  /// Ключ не отправлять — поле не меняется.
  const Patch.omit() : _tag = 0, _value = null;

  /// В JSON уходит `null` — сброс в БД.
  const Patch.clear() : _tag = 1, _value = null;

  /// В JSON уходит значение.
  const Patch.value(T value) : _tag = 2, _value = value;

  bool get isOmit => _tag == 0;
  bool get isClear => _tag == 1;
  bool get isValue => _tag == 2;

  T get requireValue {
    if (!isValue) {
      throw StateError('Patch is not value');
    }
    return _value as T;
  }
}
