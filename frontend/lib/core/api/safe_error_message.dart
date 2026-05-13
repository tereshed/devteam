import 'package:flutter/widgets.dart';
import 'package:frontend/core/api/api_exceptions.dart';
import 'package:frontend/core/api/feature_exception.dart';
import 'package:frontend/core/l10n/require.dart';

/// Sprint 15.Major DRY — sanitize Object → пользовательская строка для SnackBar/Text.
///
/// Голый `'$err'` для DioException печатает `RequestOptions.data` (JSON с plaintext credential).
/// Этот helper возвращает только `.message` от типизированного исключения
/// (FeatureException или ApiException), либо короткую локализованную заглушку.
///
/// До этого 3 файла содержали по своей копии `_safeErrText` / `_safeErrorMessage`.
String safeErrorMessage(BuildContext context, Object err) {
  if (err is ApiException) return err.message;
  if (err is FeatureException) return err.message;
  return requireAppLocalizations(context, where: 'safeErrorMessage')
      .commonRequestFailed;
}

