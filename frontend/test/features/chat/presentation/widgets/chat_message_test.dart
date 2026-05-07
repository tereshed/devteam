import 'package:flutter/material.dart';
import 'package:flutter/semantics.dart';
import 'package:flutter/services.dart';
import 'package:flutter_markdown_plus/flutter_markdown_plus.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/features/chat/presentation/widgets/chat_message.dart';
import 'package:frontend/features/chat/presentation/widgets/chat_message_builders.dart';
import 'package:frontend/l10n/app_localizations.dart';
import 'package:markdown/markdown.dart' as md;

import '../../helpers/test_wrappers.dart';

Widget _harness(Widget child) => wrapChatSimple(child: child);

TextDecoration? _decorationForSubstring(InlineSpan? root, String needle) {
  if (root is! TextSpan) {
    return null;
  }
  TextDecoration? found;
  void visit(TextSpan node) {
    final t = node.text;
    if (t != null && t.contains(needle)) {
      found = node.style?.decoration;
    }
    for (final c in node.children ?? const <InlineSpan>[]) {
      if (c is TextSpan) {
        visit(c);
      }
    }
  }

  visit(root);
  return found;
}

Color? _colorForSubstring(InlineSpan? root, String needle) {
  if (root is! TextSpan) {
    return null;
  }
  Color? found;
  void visit(TextSpan node) {
    final t = node.text;
    if (t != null && t.contains(needle)) {
      found = node.style?.color;
    }
    for (final c in node.children ?? const <InlineSpan>[]) {
      if (c is TextSpan) {
        visit(c);
      }
    }
  }

  visit(root);
  return found;
}

FontWeight? _fontWeightForSubstring(InlineSpan? root, String needle) {
  if (root is! TextSpan) {
    return null;
  }
  FontWeight? found;
  void visit(TextSpan node) {
    final t = node.text;
    if (t != null && t.contains(needle)) {
      found = node.style?.fontWeight;
    }
    for (final c in node.children ?? const <InlineSpan>[]) {
      if (c is TextSpan) {
        visit(c);
      }
    }
  }

  visit(root);
  return found;
}

String _flattenSpan(InlineSpan? root) {
  if (root == null) {
    return '';
  }
  if (root is! TextSpan) {
    return '';
  }
  final buf = StringBuffer()..write(root.text ?? '');
  for (final c in root.children ?? const <InlineSpan>[]) {
    if (c is TextSpan) {
      buf.write(_flattenSpan(c));
    }
  }
  return buf.toString();
}

void main() {
  test('safeGfmExtensionSet не содержит inline-html', () {
    expect(
      ChatMessage.safeGfmExtensionSet.inlineSyntaxes
          .any((s) => s is md.InlineHtmlSyntax),
      isFalse,
    );
  });

  testWidgets('user, assistant, system — рендер без падения', (tester) async {
    for (final role in <String>['user', 'assistant', 'system']) {
      await tester.pumpWidget(
        _harness(
          ChatMessage(
            role: role,
            content: 'ping',
            isStreaming: false,
          ),
        ),
      );
      expect(find.textContaining('ping'), findsWidgets);
    }
  });

  testWidgets('абзац остаётся SelectableText (регрессия к SelectableText)', (tester) async {
    await tester.pumpWidget(
      _harness(
        const ChatMessage(
          role: 'assistant',
          content: 'Plain paragraph line.',
        ),
      ),
    );
    expect(find.byType(SelectableText), findsWidgets);
  });

  testWidgets('простой markdown: жирный и список', (tester) async {
    const md = '**bold** line\n\n- one\n- two';
    await tester.pumpWidget(
      _harness(
        const ChatMessage(
          role: 'assistant',
          content: md,
        ),
      ),
    );
    expect(find.textContaining('bold'), findsWidgets);
    expect(find.textContaining('one'), findsWidgets);
  });

  testWidgets('fenced code: в блоке есть SelectableText.rich для выделения', (tester) async {
    const md = '```dart\nint x = 1;\n```';
    await tester.pumpWidget(
      _harness(
        const ChatMessage(
          role: 'assistant',
          content: md,
        ),
      ),
    );
    final codeBlock = find.byKey(const ValueKey<String>('chat_message_code_hscroll'));
    expect(
      find.descendant(
        of: codeBlock,
        matching: find.byType(SelectableText),
      ),
      findsWidgets,
    );
  });

  testWidgets('fenced ~~~ (tilde): код и копирование', (tester) async {
    const md = '~~~bash\necho hello\n~~~';
    await tester.pumpWidget(
      _harness(
        const ChatMessage(
          role: 'assistant',
          content: md,
        ),
      ),
    );
    expect(find.byIcon(Icons.copy), findsOneWidget);
    expect(find.textContaining('echo hello'), findsWidgets);
    expect(find.byKey(const ValueKey<String>('chat_message_code_hscroll')), findsOneWidget);
  });

  testWidgets('длинный autolink https: акцент ссылки (подчёркивание)', (tester) async {
    const kTailLen = 55;
    final tail = List.filled(kTailLen, 'z').join();
    final markdownBody = 'https://example.com/$tail/path';
    assert(
      markdownBody.length >= 60,
      'setup: длина URL должна пересекать порог autolink в markdown',
    );
    await tester.pumpWidget(
      _harness(
        ChatMessage(
          role: 'assistant',
          content: markdownBody,
        ),
      ),
    );
    final span = tester.widget<SelectableText>(find.byType(SelectableText).first).textSpan;
    expect(_decorationForSubstring(span, 'example'), TextDecoration.underline);
  });

  testWidgets('мемо builders: setState родителя не пересоздаёт ChatPreBuilder', (tester) async {
    ChatPreBuilder.resetDebugInstantiationCount();
    await tester.pumpWidget(
      _harness(
        StatefulBuilder(
          builder: (context, setState) {
            return Column(
              children: [
                TextButton(
                  onPressed: () => setState(() {}),
                  child: const Text('rebuild'),
                ),
                const ChatMessage(
                  role: 'assistant',
                  content: '```\nx\n```',
                ),
              ],
            );
          },
        ),
      ),
    );
    expect(ChatPreBuilder.debugInstantiationCount, 1);
    await tester.tap(find.text('rebuild'));
    await tester.pump();
    expect(ChatPreBuilder.debugInstantiationCount, 1);
  });

  testWidgets('fenced code: кнопка, Scrollbar с controller, горизонтальный скролл', (tester) async {
    const md = '```dart\nint x = 1;\n```';
    await tester.pumpWidget(
      _harness(
        const ChatMessage(
          role: 'assistant',
          content: md,
        ),
      ),
    );
    expect(find.byIcon(Icons.copy), findsOneWidget);
    final l10n =
        AppLocalizations.of(tester.element(find.byType(ChatMessage)))!;
    expect(find.byTooltip(l10n.chatMessageCopyCode), findsOneWidget);

    final codeBlock = find.byKey(const ValueKey<String>('chat_message_code_hscroll'));
    expect(codeBlock, findsOneWidget);
    final scrollbar = tester.widget<Scrollbar>(
      find.descendant(
        of: codeBlock,
        matching: find.byType(Scrollbar),
      ),
    );
    expect(scrollbar.controller, isNotNull);
  });

  testWidgets(
    'код-блок: scroll-padding по inline-end >= резерва под кнопку копирования',
    (tester) async {
      await tester.pumpWidget(
        _harness(
          const ChatMessage(
            role: 'assistant',
            content: '```\nx\n```',
          ),
        ),
      );
      final scroll = tester.widget<SingleChildScrollView>(
        find.descendant(
          of: find.byKey(const ValueKey<String>('chat_message_code_hscroll')),
          matching: find.byType(SingleChildScrollView),
        ),
      );
      final paddingGeom = scroll.padding;
      expect(paddingGeom, isNotNull);
      final paddingLtr = paddingGeom!.resolve(TextDirection.ltr);
      final paddingRtl = paddingGeom.resolve(TextDirection.rtl);
      expect(
        paddingLtr.right,
        greaterThanOrEqualTo(kChatMessageCodeCopyReserveEnd),
      );
      expect(
        paddingRtl.left,
        greaterThanOrEqualTo(kChatMessageCodeCopyReserveEnd),
      );
    },
  );

  testWidgets('копирование кода: тело fence в буфер (как на экране)', (tester) async {
    const md = '```dart\nint x = 1;\nprint(x);\n```';
    await tester.pumpWidget(
      _harness(
        const ChatMessage(
          role: 'assistant',
          content: md,
        ),
      ),
    );

    String? captured;
    tester.binding.defaultBinaryMessenger.setMockMethodCallHandler(
      SystemChannels.platform,
      (call) async {
        if (call.method == 'Clipboard.setData') {
          final args = call.arguments as Map<Object?, Object?>?;
          captured = args?['text'] as String?;
        }
        return null;
      },
    );
    addTearDown(() {
      tester.binding.defaultBinaryMessenger
          .setMockMethodCallHandler(SystemChannels.platform, null);
    });

    await tester.tap(find.byIcon(Icons.copy));
    await tester.pump();

    expect(captured, contains('int x = 1;'));
    expect(captured, contains('print(x);'));
    expect(captured, isNot(contains('<')));
    expect(captured, isNot(contains('Instance of')));
    expect(captured, equals('int x = 1;\nprint(x);'));
  });

  testWidgets('рост content: два pump — финальный текст', (tester) async {
    await tester.pumpWidget(
      _harness(
        const ChatMessage(
          role: 'user',
          content: 'a',
        ),
      ),
    );
    expect(find.textContaining('a'), findsWidgets);

    await tester.pumpWidget(
      _harness(
        const ChatMessage(
          role: 'user',
          content: 'ab',
        ),
      ),
    );
    expect(find.textContaining('ab'), findsWidgets);
    expect(find.textContaining('a'), findsWidgets);
  });

  testWidgets('незакрытый fence: line1 и line2 в одном горизонтальном скролле кода', (tester) async {
    const md = '```dart\nline1\nline2';
    await tester.pumpWidget(
      _harness(
        const ChatMessage(
          role: 'assistant',
          content: md,
          isStreaming: true,
        ),
      ),
    );
    final codeBlock = find.byKey(const ValueKey<String>('chat_message_code_hscroll'));
    expect(codeBlock, findsOneWidget);
    expect(
      find.descendant(
        of: codeBlock,
        matching: find.textContaining('line1'),
      ),
      findsOneWidget,
    );
    expect(
      find.descendant(
        of: codeBlock,
        matching: find.textContaining('line2'),
      ),
      findsOneWidget,
    );
  });

  testWidgets('raw HTML: не интерактивная вёрстка — текст остаётся литералом', (tester) async {
    const md = 'Text <script>alert(1)</script> end';
    await tester.pumpWidget(
      _harness(
        const ChatMessage(
          role: 'assistant',
          content: md,
        ),
      ),
    );
    expect(find.textContaining('<script>'), findsWidgets);
    expect(
      find.descendant(
        of: find.byType(MarkdownBody),
        matching: find.byType(InkWell),
      ),
      findsNothing,
    );
    expect(find.byType(HtmlElementView), findsNothing);
  });

  testWidgets('whitelist: https — подчёркивание', (tester) async {
    await tester.pumpWidget(
      _harness(
        const ChatMessage(
          role: 'assistant',
          content: '[a](https://example.com)',
        ),
      ),
    );
    final span =
        tester.widget<SelectableText>(find.byType(SelectableText).first).textSpan;
    expect(_decorationForSubstring(span, 'a'), TextDecoration.underline);
  });

  testWidgets('whitelist: mailto — подчёркивание', (tester) async {
    await tester.pumpWidget(
      _harness(
        const ChatMessage(
          role: 'assistant',
          content: '[m](mailto:user@example.com)',
        ),
      ),
    );
    final span =
        tester.widget<SelectableText>(find.byType(SelectableText).first).textSpan;
    expect(_decorationForSubstring(span, 'm'), TextDecoration.underline);
  });

  testWidgets('javascript: не в whitelist — без подчёркивания', (tester) async {
    await tester.pumpWidget(
      _harness(
        const ChatMessage(
          role: 'assistant',
          content: '[go](javascript:void(0))',
        ),
      ),
    );
    final span =
        tester.widget<SelectableText>(find.byType(SelectableText).first).textSpan;
    expect(
      _decorationForSubstring(span, 'go'),
      isNot(equals(TextDecoration.underline)),
    );
  });

  testWidgets('autolink ftp:// не в whitelist — без подчёркивания', (tester) async {
    await tester.pumpWidget(
      _harness(
        const ChatMessage(
          role: 'assistant',
          content: 'see ftp://evil.co/x',
        ),
      ),
    );
    final span =
        tester.widget<SelectableText>(find.byType(SelectableText).first).textSpan;
    final dec = _decorationForSubstring(span, 'ftp');
    expect(dec, isNot(equals(TextDecoration.underline)));
  });

  testWidgets('явная ссылка data: не в whitelist — без подчёркивания', (tester) async {
    await tester.pumpWidget(
      _harness(
        const ChatMessage(
          role: 'assistant',
          content: '[x](data:text/plain,hi)',
        ),
      ),
    );
    final span =
        tester.widget<SelectableText>(find.byType(SelectableText).first).textSpan;
    expect(_decorationForSubstring(span, 'x'), isNot(equals(TextDecoration.underline)));
  });

  testWidgets('markdown image: нет Image; точный текст плейсхолдера [alt]', (tester) async {
    const md = '![alt](https://evil.example/x.png)';
    await tester.pumpWidget(
      _harness(
        const ChatMessage(
          role: 'assistant',
          content: md,
        ),
      ),
    );
    expect(find.byType(Image), findsNothing);
    final l10n =
        AppLocalizations.of(tester.element(find.byType(ChatMessage)))!;
    expect(find.text(l10n.chatMessageMarkdownImageAlt('alt')), findsOneWidget);
  });

  testWidgets('пустой стрим: плейсхолдер', (tester) async {
    await tester.pumpWidget(
      _harness(
        const ChatMessage(
          role: 'assistant',
          content: '',
          isStreaming: true,
        ),
      ),
    );
    final ctx = tester.element(find.byType(ChatMessage));
    final l10n = AppLocalizations.of(ctx)!;
    expect(find.text(l10n.chatMessageStreamingPlaceholder), findsOneWidget);
  });

  testWidgets('копирование кода отключено при isStreaming', (tester) async {
    const md = '```\nx\n```';
    await tester.pumpWidget(
      _harness(
        const ChatMessage(
          role: 'assistant',
          content: md,
          isStreaming: true,
        ),
      ),
    );
    final button = tester.widget<IconButton>(find.byType(IconButton).first);
    expect(button.onPressed, isNull);
  });

  testWidgets('MediaQuery.textScaler увеличивает высоту строки', (tester) async {
    Future<double> lineHeight(double scale) async {
      await tester.pumpWidget(
        wrapChatSimple(
          textScaler: TextScaler.linear(scale),
          child: const ChatMessage(
            role: 'user',
            content: 'scaled',
          ),
        ),
      );
      await tester.pump();
      return tester.getSize(find.textContaining('scaled')).height;
    }

    final h1 = await lineHeight(1);
    final h2 = await lineHeight(2);
    expect(h2, greaterThan(h1 * 1.4));
  });

  testWidgets('длинный непрерывный токен вне fence: нет overflow при узком maxWidth', (tester) async {
    useViewSize(tester, const Size(400, 900));
    final long = List.filled(400, 'a').join();
    await tester.pumpWidget(
      wrapChatSimple(
        child: Align(
          alignment: Alignment.topCenter,
          child: ConstrainedBox(
            constraints: const BoxConstraints(maxWidth: 352),
            child: ChatMessage(
              role: 'assistant',
              content: long,
            ),
          ),
        ),
      ),
    );
    await tester.pump();
    expect(tester.takeException(), isNull);
  });

  testWidgets('длинный токен в параграфе: нет U+200B в плоском тексте для выделения', (tester) async {
    useViewSize(tester, const Size(400, 900));
    final long = List.filled(200, 'z').join();
    await tester.pumpWidget(
      wrapChatSimple(
        child: ChatMessage(
          role: 'assistant',
          content: long,
        ),
      ),
    );
    await tester.pump();
    final span = tester.widget<SelectableText>(find.byType(SelectableText).first).textSpan;
    expect(_flattenSpan(span).contains('\u200B'), isFalse);
  });

  testWidgets('ссылка [**boldlabel**](https): жирное начертание не сохраняется (компромисс ChatLinkBuilder)', (tester) async {
    await tester.pumpWidget(
      _harness(
        const ChatMessage(
          role: 'assistant',
          content: '[**boldlabel**](https://example.com/path)',
        ),
      ),
    );
    final span = tester.widget<SelectableText>(find.byType(SelectableText).first).textSpan;
    expect(_fontWeightForSubstring(span, 'boldlabel'), isNot(FontWeight.bold));
  });

  testWidgets('роль system даёт onSurfaceVariant для абзаца, assistant — обычный onSurface', (tester) async {
    final theme = ThemeData(
      useMaterial3: true,
      colorScheme: ColorScheme.fromSeed(seedColor: Colors.teal),
    );
    await tester.pumpWidget(
      wrapChatMaterialApp(
        theme: theme,
        home: const Scaffold(
          body: ChatMessage(
            role: 'system',
            content: 'ping',
          ),
        ),
      ),
    );
    final systemSpan =
        tester.widget<SelectableText>(find.byType(SelectableText).first).textSpan;
    final cSystem = _colorForSubstring(systemSpan, 'ping');

    await tester.pumpWidget(
      wrapChatMaterialApp(
        theme: theme,
        home: const Scaffold(
          body: ChatMessage(
            role: 'assistant',
            content: 'ping',
          ),
        ),
      ),
    );
    final assistantSpan =
        tester.widget<SelectableText>(find.byType(SelectableText).first).textSpan;
    final cAssistant = _colorForSubstring(assistantSpan, 'ping');

    expect(cSystem, theme.colorScheme.onSurfaceVariant);
    expect(cAssistant, isNot(cSystem));
  });

  testWidgets('роль system: элемент списка с тем же тинтом onSurfaceVariant', (tester) async {
    final theme = ThemeData(
      useMaterial3: true,
      colorScheme: ColorScheme.fromSeed(seedColor: Colors.deepOrange),
    );
    await tester.pumpWidget(
      wrapChatMaterialApp(
        theme: theme,
        home: const Scaffold(
          body: ChatMessage(
            role: 'system',
            content: '- bullet item\n',
          ),
        ),
      ),
    );
    final span =
        tester.widget<SelectableText>(find.byType(SelectableText).first).textSpan;
    final c = _colorForSubstring(span, 'bullet');
    expect(c, theme.colorScheme.onSurfaceVariant);
  });

  testWidgets('пустое нестримящееся сообщение: только SizedBox, без MaterialApp', (tester) async {
    await tester.pumpWidget(
      const Directionality(
        textDirection: TextDirection.ltr,
        child: ChatMessage(
          role: 'user',
          content: '',
          isStreaming: false,
        ),
      ),
    );
    expect(find.byType(SizedBox), findsOneWidget);
  });

  testWidgets('стрим: ни один Semantics-узел не помечен liveRegion (11.5/11.6)', (tester) async {
    await tester.pumpWidget(
      _harness(
        const ChatMessage(
          role: 'assistant',
          content: 'hi',
          isStreaming: true,
        ),
      ),
    );
    await tester.pumpAndSettle();

    // ignore: deprecated_member_use — pipelineOwner (RendererBinding API эволюционирует)
    final owner = tester.binding.pipelineOwner.semanticsOwner;
    expect(owner, isNotNull);
    final root = owner!.rootSemanticsNode!;
    void walk(SemanticsNode n) {
      expect(
        // ignore: deprecated_member_use
        n.hasFlag(SemanticsFlag.isLiveRegion),
        isFalse,
        reason: 'liveRegion при стриме заставляет скринридер повторять каждый чанк',
      );
      n.visitChildren((SemanticsNode c) {
        walk(c);
        return true;
      });
    }

    walk(root);
  });
}
