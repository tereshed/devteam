/// Best-effort клиентский рендер шаблона имени ветки для живого превью.
///
/// Зеркалит основную логику backend (`internal/service/branch_template.go`:
/// подстановка плейсхолдеров, fallback `{ticket|alt}`, схлопывание разделителей,
/// авто-суффикс short_id если в шаблоне нет id-плейсхолдера). Источник правды —
/// сервер (он же валидирует при сохранении); здесь — только предпросмотр.
String branchTemplatePreview(
  String template, {
  String ticket = 'DEV-123',
  String slug = 'fix-login-bug',
  String shortId = 'a1b2c3d4',
}) {
  var tmpl = template.trim();
  if (tmpl.isEmpty) {
    tmpl = 'task/{short_id}-{slug}';
  }
  final hasId = tmpl.contains('{short_id}') || tmpl.contains('{id}');
  final fullId = '$shortId-1111-2222-3333-444455556666';

  String resolve(String name) {
    switch (name) {
      case 'ticket':
        return ticket;
      case 'slug':
      case 'title':
        return slug;
      case 'short_id':
        return shortId;
      case 'id':
        return fullId;
      case 'date':
        return '20260619';
      case 'yyyy':
        return '2026';
      case 'mm':
        return '06';
      case 'dd':
        return '19';
      default:
        return '';
    }
  }

  final placeholder = RegExp(r'\{([a-z_]+)(?:\|([a-z_]+))?\}');
  var out = tmpl.replaceAllMapped(placeholder, (m) {
    var v = resolve(m.group(1)!);
    final fb = m.group(2);
    if (v.isEmpty && fb != null) {
      v = resolve(fb);
    }
    return v;
  });

  // cleanup: схлопывание повторов разделителей, в т.ч. вокруг '/', + зачистка краёв.
  out = out
      .replaceAllMapped(RegExp(r'[-_]{2,}'), (m) => m.group(0)![0])
      .replaceAll(RegExp(r'/[-_]+'), '/')
      .replaceAll(RegExp(r'[-_]+/'), '/')
      .replaceAll(RegExp(r'/{2,}'), '/')
      .replaceAll(RegExp(r'^[-_/]+'), '')
      .replaceAll(RegExp(r'[-_/]+$'), '');

  if (!hasId) {
    out = out.isEmpty ? shortId : '$out-$shortId';
  }
  return out;
}
