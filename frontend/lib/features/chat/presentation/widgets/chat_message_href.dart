/// Проверка href для отображения и будущего открытия ссылок (задача 11.6).
bool isAllowedHref(String? href) {
  if (href == null || href.isEmpty) {
    return false;
  }
  final u = Uri.tryParse(href);
  if (u == null) {
    return false;
  }
  final scheme = u.scheme.toLowerCase();
  if (scheme == 'http' || scheme == 'https') {
    return u.host.isNotEmpty;
  }
  if (scheme == 'mailto') {
    return u.path.isNotEmpty;
  }
  return false;
}
