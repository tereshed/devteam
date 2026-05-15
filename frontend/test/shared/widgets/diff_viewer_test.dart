// @dart=2.19
// @Tags(['widget'])

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/shared/widgets/diff_viewer.dart';

void main() {
  ThemeData m3Theme({required Color seed, Brightness brightness = Brightness.light}) {
    return ThemeData(
      useMaterial3: true,
      colorScheme: ColorScheme.fromSeed(seedColor: seed, brightness: brightness),
    );
  }

  Widget harness(
    Widget child, {
    ThemeData? theme,
    Size screenSize = const Size(800, 600),
  }) {
    return MaterialApp(
      theme: theme ?? m3Theme(seed: Colors.deepPurple),
      home: MediaQuery(
        data: MediaQueryData(size: screenSize),
        child: Scaffold(body: child),
      ),
    );
  }

  /// Обход служебных полностью прозрачных [ColoredBox] над строкой (напр. у [SelectionArea]).
  ColoredBox firstOpaqueColoredBoxAncestor(WidgetTester tester, Finder lineFinder) {
    final ancestors = find.ancestor(
      of: lineFinder,
      matching: find.byType(ColoredBox),
    );
    final n = ancestors.evaluate().length;
    for (var i = 0; i < n; i++) {
      final box = tester.widget<ColoredBox>(ancestors.at(i));
      if (box.color.a > 0) {
        return box;
      }
    }
    throw StateError('No opaque ColoredBox ancestor');
  }

  const typicalDiff = '''
diff --git a/lib/foo.dart b/lib/foo.dart
index 111..222 100644
--- a/lib/foo.dart
+++ b/lib/foo.dart
@@ -1,3 +1,4 @@
 line0
-removed
+added
 context
''';

  group('DiffViewer', () {
    testWidgets('unified diff: hunk и строки +/- присутствуют', (tester) async {
      await tester.pumpWidget(
        harness(const DiffViewer(diff: typicalDiff)),
      );
      expect(find.textContaining('@@'), findsWidgets);
      expect(find.textContaining('-removed'), findsOneWidget);
      expect(find.textContaining('+added'), findsOneWidget);
    });

    testWidgets('очень длинная строка: нет overflow', (tester) async {
      final long = 'x' * 500;
      final diff = '''
--- a/x
+++ b/x
@@ -1,1 +1,1 @@
-$long
+$long
''';
      await tester.pumpWidget(
        harness(DiffViewer(diff: diff)),
      );
      expect(tester.takeException(), isNull);
    });

    testWidgets('plain text без признаков diff рендерится без исключения', (tester) async {
      await tester.pumpWidget(
        harness(const DiffViewer(diff: 'hello world\nno diff here')),
      );
      expect(find.textContaining('hello world'), findsOneWidget);
      expect(tester.takeException(), isNull);
    });

    testWidgets('тёмная тема: без FlutterError', (tester) async {
      await tester.pumpWidget(
        harness(
          const DiffViewer(diff: typicalDiff),
          theme: m3Theme(seed: Colors.teal, brightness: Brightness.dark),
        ),
      );
      expect(tester.takeException(), isNull);
    });

    testWidgets('maxHeight: высота ≤ maxHeight + 2', (tester) async {
      const maxH = 220.0;
      await tester.pumpWidget(
        harness(
          const DiffViewer(diff: typicalDiff, maxHeight: maxH),
        ),
      );
      final h = tester.getSize(find.byType(DiffViewer)).height;
      expect(h, lessThanOrEqualTo(maxH + 2.0));
    });

    testWidgets('maxHeight null: clamp к формуле экрана', (tester) async {
      const screenH = 400.0;
      final expected = (screenH * 0.4).clamp(180.0, 480.0);
      await tester.pumpWidget(
        harness(
          const DiffViewer(diff: typicalDiff),
          screenSize: const Size(360, screenH),
        ),
      );
      final h = tester.getSize(find.byType(DiffViewer)).height;
      expect(h, lessThanOrEqualTo(expected + 2.0));
    });

    testWidgets('SelectionArea присутствует', (tester) async {
      await tester.pumpWidget(
        harness(const DiffViewer(diff: typicalDiff)),
      );
      expect(find.byType(SelectionArea), findsOneWidget);
    });

    testWidgets('+/-: фон ColoredBox совпадает с ColorScheme', (tester) async {
      final theme = m3Theme(seed: Colors.indigo);
      await tester.pumpWidget(
        harness(const DiffViewer(diff: typicalDiff), theme: theme),
      );
      final scheme = theme.colorScheme;
      final plusFinder = find.descendant(
        of: find.byType(ListView),
        matching: find.text('+added'),
      );
      final minusFinder = find.descendant(
        of: find.byType(ListView),
        matching: find.text('-removed'),
      );
      expect(
        firstOpaqueColoredBoxAncestor(tester, plusFinder).color,
        scheme.primaryContainer,
      );
      expect(
        firstOpaqueColoredBoxAncestor(tester, minusFinder).color,
        scheme.surfaceContainerHighest,
      );
    });

    testWidgets('смена темы меняет цвет фона +', (tester) async {
      final t1 = m3Theme(seed: Colors.blue);
      await tester.pumpWidget(
        harness(
          const DiffViewer(key: ValueKey<Object>('a'), diff: typicalDiff),
          theme: t1,
        ),
      );
      await tester.pumpAndSettle();
      final plusFinder = find.descendant(
        of: find.byType(ListView),
        matching: find.text('+added'),
      );
      final scheme1 =
          Theme.of(tester.element(find.byType(DiffViewer))).colorScheme;
      final c1 = firstOpaqueColoredBoxAncestor(tester, plusFinder).color;

      final t2 = m3Theme(seed: Colors.deepOrange);
      await tester.pumpWidget(
        harness(
          const DiffViewer(key: ValueKey<Object>('b'), diff: typicalDiff),
          theme: t2,
        ),
      );
      await tester.pumpAndSettle();
      final plusFinder2 = find.descendant(
        of: find.byType(ListView),
        matching: find.text('+added'),
      );
      final scheme2 =
          Theme.of(tester.element(find.byType(DiffViewer))).colorScheme;
      final c2 = firstOpaqueColoredBoxAncestor(tester, plusFinder2).color;

      expect(c1, scheme1.primaryContainer);
      expect(c2, scheme2.primaryContainer);
      expect(c1, isNot(equals(c2)));
    });

    testWidgets('внутри hunk строка удаления с содержимым --- не заголовок файла', (tester) async {
      const diff = '''
--- a/x.txt
+++ b/x.txt
@@ -1,2 +1,2 @@
 keep
- --- not-a-file-header
+keep2
''';
      await tester.pumpWidget(
        harness(const DiffViewer(diff: diff)),
      );
      final target = find.descendant(
        of: find.byType(ListView),
        matching: find.text('- --- not-a-file-header'),
      );
      expect(target, findsOneWidget);
      final theme = m3Theme(seed: Colors.deepPurple);
      expect(
        firstOpaqueColoredBoxAncestor(tester, target).color,
        theme.colorScheme.surfaceContainerHighest,
      );
    });

    testWidgets('внутри hunk --- foo / +++ foo — deletion и addition, не fileHeader', (tester) async {
      const diff = '''
--- a/x.txt
+++ b/x.txt
@@ -1,1 +1,1 @@
--- foo
+++ foo
''';
      await tester.pumpWidget(
        harness(const DiffViewer(diff: diff)),
      );
      final del = find.descendant(
        of: find.byType(ListView),
        matching: find.text('--- foo'),
      );
      final add = find.descendant(
        of: find.byType(ListView),
        matching: find.text('+++ foo'),
      );
      expect(del, findsOneWidget);
      expect(add, findsOneWidget);
      final theme = m3Theme(seed: Colors.deepPurple);
      expect(
        firstOpaqueColoredBoxAncestor(tester, del).color,
        theme.colorScheme.surfaceContainerHighest,
      );
      expect(
        firstOpaqueColoredBoxAncestor(tester, add).color,
        theme.colorScheme.primaryContainer,
      );
    });

    testWidgets('внутри hunk добавление с +++ в содержимом', (tester) async {
      const diff = '''
--- a/x.txt
+++ b/x.txt
@@ -1,1 +1,2 @@
 x
+ +++ not header
''';
      await tester.pumpWidget(
        harness(const DiffViewer(diff: diff)),
      );
      final target = find.descendant(
        of: find.byType(ListView),
        matching: find.text('+ +++ not header'),
      );
      expect(target, findsOneWidget);
      final theme = m3Theme(seed: Colors.deepPurple);
      expect(
        firstOpaqueColoredBoxAncestor(tester, target).color,
        theme.colorScheme.primaryContainer,
      );
    });

    testWidgets(
        'TestDiffViewer_LargeDiff_DoesNotBuildAllTilesAtOnce: 5000-строчный diff '
        'строит < 100 DiffLineRow в viewport (lazy ListView.builder + itemExtent)',
        (tester) async {
      // 5000 строк-additions — попадаем в типичный размер code_diff'а агента.
      // С `itemExtent` и ListView.builder только viewport-видимые ряды
      // материализуются; всё остальное — lazy.
      final sb = StringBuffer()
        ..writeln('--- a/x')
        ..writeln('+++ b/x')
        ..writeln('@@ -0,0 +1,5000 @@');
      for (var i = 0; i < 5000; i++) {
        sb.writeln('+line $i');
      }
      await tester.pumpWidget(
        harness(DiffViewer(diff: sb.toString(), maxHeight: 400)),
      );
      await tester.pumpAndSettle();
      // 400px / 18px ≈ 22 строки + cacheExtent ⇒ десятки, точно < 100.
      final builtRows = find.byType(DiffLineRow).evaluate().length;
      expect(builtRows, lessThan(100),
          reason:
              'ListView должен рендерить только viewport; всё $builtRows материализовано');
      expect(tester.takeException(), isNull);
    });

    testWidgets('itemExtent применён в ListView (фиксированная высота строки)',
        (tester) async {
      await tester.pumpWidget(
        harness(const DiffViewer(diff: typicalDiff)),
      );
      final lv = tester.widget<ListView>(find.byType(ListView));
      expect(lv.itemExtent, DiffViewer.lineExtent);
    });

    testWidgets('большой diff ≥1000 строк: pumpAndSettle без исключений', (tester) async {
      final sb = StringBuffer()
        ..writeln('--- a/x')
        ..writeln('+++ b/x')
        ..writeln('@@ -0,0 +1,1000 @@');
      for (var i = 0; i < 1000; i++) {
        sb.writeln('+line $i');
      }
      await tester.pumpWidget(
        harness(DiffViewer(diff: sb.toString())),
      );
      await tester.pumpAndSettle();
      expect(tester.takeException(), isNull);
    });

    testWidgets('Binary files … differ — метаданные', (tester) async {
      const diff = '''
diff --git a/img.bin b/img.bin
Binary files a/img.bin and b/img.bin differ
''';
      await tester.pumpWidget(
        harness(const DiffViewer(diff: diff)),
      );
      expect(find.textContaining('Binary files'), findsOneWidget);
      expect(tester.takeException(), isNull);
    });

    testWidgets('удалённый файл --- a/ + +++ /dev/null', (tester) async {
      const diff = '''
diff --git a/gone.txt b/gone.txt
deleted file mode 100644
--- a/gone.txt
+++ /dev/null
@@ -1,1 +0,0 @@
-old
''';
      await tester.pumpWidget(
        harness(const DiffViewer(diff: diff)),
      );
      expect(find.textContaining('+++ /dev/null'), findsOneWidget);
      expect(tester.takeException(), isNull);
    });

    testWidgets('новый файл --- /dev/null + +++ b/', (tester) async {
      const diff = '''
diff --git a/new.txt b/new.txt
new file mode 100644
--- /dev/null
+++ b/new.txt
@@ -0,0 +1,1 @@
+hi
''';
      await tester.pumpWidget(
        harness(const DiffViewer(diff: diff)),
      );
      expect(find.textContaining('--- /dev/null'), findsOneWidget);
      expect(find.textContaining('+hi'), findsOneWidget);
    });

    testWidgets('rename from / rename to / similarity index', (tester) async {
      const diff = '''
diff --git a/old b/new
similarity index 95%
rename from old
rename to new
''';
      await tester.pumpWidget(
        harness(const DiffViewer(diff: diff)),
      );
      expect(find.textContaining('similarity index'), findsOneWidget);
      expect(find.textContaining('rename from'), findsOneWidget);
      expect(find.textContaining('rename to'), findsOneWidget);
    });

    testWidgets('Subproject commit', (tester) async {
      const diff = '''
--- a/sub
+++ b/sub
Submodule path/to 1234abcd..5678efgh
Subproject commit deadbeefdeadbeefdeadbeefdeadbeef
''';
      await tester.pumpWidget(
        harness(const DiffViewer(diff: diff)),
      );
      expect(find.textContaining('Subproject commit'), findsOneWidget);
    });

    testWidgets(r'\ No newline at end of file', (tester) async {
      const diff = r'''
--- a/x
+++ b/x
@@ -1,1 +1,1 @@
-a
+b
\ No newline at end of file
''';
      await tester.pumpWidget(
        harness(const DiffViewer(diff: diff)),
      );
      expect(find.textContaining(r'\ No newline at end of file'), findsOneWidget);
    });
  });
}
