import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:frontend/core/l10n/require.dart';
import 'package:go_router/go_router.dart';

/// Общий UI для [GoRouter.errorBuilder] (prod и тестовые роутеры).
///
/// При изменении текста/верстки править только здесь.
///
/// В релизе пользователь видит только локализованную строку. В debug/profile
/// пишем [GoRouterState.error] в консоль (в release вырезано).
/// TODO(devteam): Sprint 12 — при необходимости прод-доставка в Crashlytics/Sentry.
Widget buildRouterErrorScreen(BuildContext context, GoRouterState state) {
  if (!kReleaseMode) {
    debugPrint('GoRouter errorBuilder: ${state.error}');
  }
  final l10n = requireAppLocalizations(context, where: 'buildRouterErrorScreen');
  return Scaffold(
    body: Center(child: Text(l10n.routerNavigationError)),
  );
}
