/// Remote Git URL для формы создания проекта (не локальный провайдер).
///
/// Допускает http(s), `ssh://`, `git://`, а также SCP-вид `git@host:path/to/repo.git`.
/// Финальную проверку выполняет API (`binding:"omitempty,url"`).
bool isValidGitRemoteUrl(String value) {
  final s = value.trim();
  if (s.isEmpty || RegExp(r'\s').hasMatch(s)) {
    return false;
  }

  final u = Uri.tryParse(s);
  if (u != null && u.hasScheme) {
    final scheme = u.scheme.toLowerCase();
    if (scheme == 'http' ||
        scheme == 'https' ||
        scheme == 'ssh' ||
        scheme == 'git') {
      return u.host.isNotEmpty;
    }
    return false;
  }

  // SCP-style: git@github.com:org/repo.git
  return RegExp(r'^[^\s@]+@[^\s:]+:.+$').hasMatch(s);
}
