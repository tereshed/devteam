/// Strips user credentials embedded in URLs inside API error strings so UI/logs
/// cannot leak tokens from server messages (e.g. clone URLs with userinfo).
String sanitizeUserFacingMessage(String input) {
  if (input.isEmpty) {
    return input;
  }

  return input.replaceAllMapped(
    RegExp(r'\b[a-zA-Z][a-zA-Z0-9+.-]*://[^\s<>"{}|\\^`\[\]]+'),
    _sanitizeUriToken,
  );
}

String _sanitizeUriToken(Match m) {
  final raw = m[0]!;
  try {
    final uri = Uri.parse(raw);
    if (uri.userInfo.isEmpty) {
      return raw;
    }
    return uri.replace(userInfo: '').toString();
  } catch (_) {
    return raw.replaceFirstMapped(
      RegExp(r'^(.*?://)(?:[^/@?#]+)@'),
      (mm) => mm[1]!,
    );
  }
}
