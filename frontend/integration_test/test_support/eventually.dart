import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

/// Опрос `finder` до тех пор, пока он что-то не найдёт, или фейл по таймауту.
///
/// `pumpAndSettle` не ждёт сетевых I/O (REST/WS), поэтому в интеграционных
/// тестах используем bounded loop с `pump`-интервалом.
Future<void> expectEventually(
  WidgetTester tester,
  Finder finder, {
  Duration timeout = const Duration(seconds: 10),
  Duration interval = const Duration(milliseconds: 200),
  String? reason,
}) async {
  final deadline = DateTime.now().add(timeout);
  while (DateTime.now().isBefore(deadline)) {
    await tester.pump(interval);
    if (finder.evaluate().isNotEmpty) {
      return;
    }
  }
  fail('expectEventually timeout: ${reason ?? finder.toString()}');
}

/// «Мягкий» вариант: возвращает true/false вместо fail. Полезен для
/// best-effort веток (например, WS-доставка ответа в [assistant_e2e_test]).
Future<bool> waitUntil(
  WidgetTester tester,
  Finder finder, {
  Duration timeout = const Duration(seconds: 10),
  Duration interval = const Duration(milliseconds: 200),
}) async {
  final deadline = DateTime.now().add(timeout);
  while (DateTime.now().isBefore(deadline)) {
    await tester.pump(interval);
    if (finder.evaluate().isNotEmpty) {
      return true;
    }
  }
  return false;
}

/// Ожидание, что произвольный predicate станет true. Удобно проверять
/// состояние, для которого нет дешёвого UI-маркера (например, status задачи
/// из REST после нажатия кнопки в UI).
Future<void> expectEventuallyTrue(
  WidgetTester tester,
  Future<bool> Function() check, {
  Duration timeout = const Duration(seconds: 15),
  Duration interval = const Duration(milliseconds: 250),
  String? reason,
}) async {
  final deadline = DateTime.now().add(timeout);
  while (DateTime.now().isBefore(deadline)) {
    await tester.pump(interval);
    if (await check()) {
      return;
    }
  }
  fail('expectEventuallyTrue timeout: ${reason ?? 'predicate never true'}');
}

/// `BuildContext` любого Scaffold в дереве — нужен для `AppLocalizations.of`,
/// `GoRouter.of` и т.п. Без него тесту приходится таскать local-функцию
/// (см. ревью DRY-нарушений Phase 3).
///
/// Берёт ПЕРВЫЙ Scaffold: в integration_test'ах активный экран обычно один,
/// а если их несколько (overlay), первый — это корневой роутер-pane.
BuildContext anyScaffoldContext(WidgetTester tester) =>
    tester.element(find.byType(Scaffold).first);

/// Циклит `tester.pump(250ms)` суммарно `seconds` секунд.
///
/// Нужен там, где сетевой I/O (REST/WS) не учитывается `pumpAndSettle`:
/// после `tester.tap` запрос летит в backend, и UI обновится только через
/// несколько кадров. Эта функция не fail'ит — это просто bounded wait;
/// проверку «дождались ли мы UI-состояния» делайте через [expectEventually].
Future<void> pumpForSeconds(WidgetTester tester, int seconds) async {
  // 4 шага по 250 мс на секунду: совпадает с интервалом `expectEventually`
  // и даёт ровные «8 кадров / сек» вместо неровного pumpAndSettle с
  // максимумом в 10 минут.
  for (var i = 0; i < seconds * 4; i++) {
    await tester.pump(const Duration(milliseconds: 250));
  }
}
