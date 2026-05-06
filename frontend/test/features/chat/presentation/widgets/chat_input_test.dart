import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_localizations/flutter_localizations.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/features/chat/presentation/widgets/chat_input.dart';
import 'package:frontend/l10n/app_localizations.dart';

void main() {
  Widget harness(
    Widget child, {
    Locale locale = const Locale('en'),
    TextScaler textScaler = TextScaler.noScaling,
  }) {
    return MaterialApp(
      locale: locale,
      localizationsDelegates: const [
        AppLocalizations.delegate,
        GlobalMaterialLocalizations.delegate,
        GlobalWidgetsLocalizations.delegate,
        GlobalCupertinoLocalizations.delegate,
      ],
      supportedLocales: const [Locale('en'), Locale('ru')],
      home: MediaQuery(
        data: MediaQueryData(size: const Size(800, 1200), textScaler: textScaler),
        child: Scaffold(body: child),
      ),
    );
  }

  Future<void> focusInput(WidgetTester tester) async {
    await tester.tap(find.byKey(const ValueKey('chat_input_field')));
    await tester.pump();
  }

  /// Ctrl+Enter: стабильная последовательность для [Shortcuts] + [Actions].
  Future<void> sendCtrlEnter(WidgetTester tester) async {
    await tester.sendKeyDownEvent(LogicalKeyboardKey.controlLeft);
    await tester.sendKeyDownEvent(LogicalKeyboardKey.enter);
    await tester.sendKeyUpEvent(LogicalKeyboardKey.enter);
    await tester.sendKeyUpEvent(LogicalKeyboardKey.controlLeft);
    await tester.pump();
  }

  /// Meta+Enter (для CI без привязки к macOS).
  Future<void> sendMetaEnter(WidgetTester tester) async {
    await tester.sendKeyDownEvent(LogicalKeyboardKey.metaLeft);
    await tester.sendKeyDownEvent(LogicalKeyboardKey.enter);
    await tester.sendKeyUpEvent(LogicalKeyboardKey.enter);
    await tester.sendKeyUpEvent(LogicalKeyboardKey.metaLeft);
    await tester.pump();
  }

  testWidgets('chat_input_send_button_disabled_when_empty_or_whitespace', (
    WidgetTester tester,
  ) async {
    final c = TextEditingController();
    final focus = FocusNode();
    addTearDown(() {
      c.dispose();
      focus.dispose();
    });

    await tester.pumpWidget(
      harness(
        ChatInput(
          controller: c,
          focusNode: focus,
          onSend: (_) {},
          sendTooltip: 'Send',
        ),
      ),
    );

    IconButton sendBtn() => tester.widget<IconButton>(
          find.byKey(const ValueKey('chat_send_button')),
        );

    expect(sendBtn().onPressed, isNull);

    c.text = '   ';
    await tester.pump();
    expect(sendBtn().onPressed, isNull);

    c.text = '\n\t';
    await tester.pump();
    expect(sendBtn().onPressed, isNull);
  });

  testWidgets('chat_input_send_button_enabled_when_non_empty', (
    WidgetTester tester,
  ) async {
    final c = TextEditingController(text: 'x');
    final focus = FocusNode();
    addTearDown(() {
      c.dispose();
      focus.dispose();
    });

    await tester.pumpWidget(
      harness(
        ChatInput(
          controller: c,
          focusNode: focus,
          onSend: (_) {},
          sendTooltip: 'Send',
          isSending: false,
        ),
      ),
    );

    final btn = tester.widget<IconButton>(
      find.byKey(const ValueKey('chat_send_button')),
    );
    expect(btn.onPressed, isNotNull);
  });

  testWidgets('chat_input_on_send_called_once_on_button_tap', (
    WidgetTester tester,
  ) async {
    final c = TextEditingController(text: 'hello');
    final focus = FocusNode();
    var n = 0;
    addTearDown(() {
      c.dispose();
      focus.dispose();
    });

    await tester.pumpWidget(
      harness(
        ChatInput(
          controller: c,
          focusNode: focus,
          onSend: (_) => n++,
          sendTooltip: 'Send',
        ),
      ),
    );

    await tester.tap(find.byKey(const ValueKey('chat_send_button')));
    await tester.pump();
    expect(n, 1);
  });

  testWidgets('chat_input_on_send_called_with_raw_text_no_trim', (
    WidgetTester tester,
  ) async {
    final c = TextEditingController(text: '  hi  ');
    final focus = FocusNode();
    String? received;
    addTearDown(() {
      c.dispose();
      focus.dispose();
    });

    await tester.pumpWidget(
      harness(
        ChatInput(
          controller: c,
          focusNode: focus,
          onSend: (s) => received = s,
          sendTooltip: 'Send',
        ),
      ),
    );

    await tester.tap(find.byKey(const ValueKey('chat_send_button')));
    await tester.pump();
    expect(received, '  hi  ');
  });

  testWidgets('chat_input_on_send_called_once_on_ctrl_enter', (
    WidgetTester tester,
  ) async {
    final c = TextEditingController(text: 'a');
    final focus = FocusNode();
    var n = 0;
    addTearDown(() {
      c.dispose();
      focus.dispose();
    });

    await tester.pumpWidget(
      harness(
        ChatInput(
          controller: c,
          focusNode: focus,
          onSend: (_) => n++,
          sendTooltip: 'Send',
        ),
      ),
    );

    await focusInput(tester);
    await sendCtrlEnter(tester);
    expect(n, 1);
  });

  testWidgets('chat_input_on_send_called_once_on_meta_enter', (
    WidgetTester tester,
  ) async {
    final c = TextEditingController(text: 'a');
    final focus = FocusNode();
    var n = 0;
    addTearDown(() {
      c.dispose();
      focus.dispose();
    });

    await tester.pumpWidget(
      harness(
        ChatInput(
          controller: c,
          focusNode: focus,
          onSend: (_) => n++,
          sendTooltip: 'Send',
        ),
      ),
    );

    await focusInput(tester);
    await sendMetaEnter(tester);
    expect(n, 1);
  });

  testWidgets('chat_input_shortcuts_do_not_duplicate_actions_listener', (
    WidgetTester tester,
  ) async {
    final c = TextEditingController(text: 'ok');
    final focus = FocusNode();
    var n = 0;
    addTearDown(() {
      c.dispose();
      focus.dispose();
    });

    await tester.pumpWidget(
      harness(
        ChatInput(
          controller: c,
          focusNode: focus,
          onSend: (_) => n++,
          sendTooltip: 'Send',
        ),
      ),
    );

    await focusInput(tester);
    await tester.tap(find.byKey(const ValueKey('chat_send_button')));
    await tester.pump();
    await sendCtrlEnter(tester);
    expect(n, 2);
  });

  testWidgets('chat_input_on_send_not_called_whitespace_and_shortcut', (
    WidgetTester tester,
  ) async {
    final c = TextEditingController(text: ' \n ');
    final focus = FocusNode();
    var n = 0;
    addTearDown(() {
      c.dispose();
      focus.dispose();
    });

    await tester.pumpWidget(
      harness(
        ChatInput(
          controller: c,
          focusNode: focus,
          onSend: (_) => n++,
          sendTooltip: 'Send',
        ),
      ),
    );

    await focusInput(tester);
    await sendCtrlEnter(tester);
    expect(n, 0);
  });

  testWidgets('chat_input_on_send_not_called_during_ime_composition', (
    WidgetTester tester,
  ) async {
    final c = TextEditingController();
    final focus = FocusNode();
    var n = 0;
    addTearDown(() {
      c.dispose();
      focus.dispose();
    });

    await tester.pumpWidget(
      harness(
        ChatInput(
          controller: c,
          focusNode: focus,
          onSend: (_) => n++,
          sendTooltip: 'Send',
        ),
      ),
    );

    c.value = const TextEditingValue(
      text: 'ni',
      selection: TextSelection.collapsed(offset: 2),
      composing: TextRange(start: 0, end: 2),
    );
    await tester.pump();

    await focusInput(tester);
    await sendCtrlEnter(tester);
    expect(n, 0);
  });

  testWidgets('chat_input_send_blocked_when_is_sending_true', (
    WidgetTester tester,
  ) async {
    final c = TextEditingController(text: 'y');
    final focus = FocusNode();
    var n = 0;
    addTearDown(() {
      c.dispose();
      focus.dispose();
    });

    await tester.pumpWidget(
      harness(
        ChatInput(
          controller: c,
          focusNode: focus,
          onSend: (_) => n++,
          sendTooltip: 'Send',
          isSending: true,
        ),
      ),
    );

    final btn = tester.widget<IconButton>(
      find.byKey(const ValueKey('chat_send_button')),
    );
    expect(btn.onPressed, isNull);

    await focusInput(tester);
    await sendCtrlEnter(tester);
    expect(n, 0);
    await sendMetaEnter(tester);
    expect(n, 0);
  });

  testWidgets('chat_input_semantics_send_tooltip_matches_localizations', (
    WidgetTester tester,
  ) async {
    final c = TextEditingController(text: 'z');
    final focus = FocusNode();
    addTearDown(() {
      c.dispose();
      focus.dispose();
    });

    await tester.pumpWidget(
      MaterialApp(
        localizationsDelegates: const [
          AppLocalizations.delegate,
          GlobalMaterialLocalizations.delegate,
          GlobalWidgetsLocalizations.delegate,
          GlobalCupertinoLocalizations.delegate,
        ],
        supportedLocales: const [Locale('en')],
        home: Builder(
          builder: (context) {
            final l10n = AppLocalizations.of(context)!;
            return Scaffold(
              body: ChatInput(
                controller: c,
                focusNode: focus,
                onSend: (_) {},
                sendTooltip: l10n.chatInputSendTooltip,
              ),
            );
          },
        ),
      ),
    );

    final context = tester.element(find.byType(ChatInput));
    final l10n = AppLocalizations.of(context)!;

    expect(
      find.byTooltip(l10n.chatInputSendTooltip),
      findsOneWidget,
    );
  });

  testWidgets('chat_input_text_scaler_2_no_overflow_usable_send', (
    WidgetTester tester,
  ) async {
    final longText = List<String>.filled(
      25,
      'Line that should wrap inside the field with maxLines six.\n',
    ).join();
    final c = TextEditingController(text: longText);
    final focus = FocusNode();
    addTearDown(() {
      c.dispose();
      focus.dispose();
    });

    await tester.pumpWidget(
      harness(
        ChatInput(
          controller: c,
          focusNode: focus,
          onSend: (_) {},
          sendTooltip: 'Send',
          hintText: 'Hint',
        ),
        textScaler: const TextScaler.linear(2),
      ),
    );

    expect(tester.takeException(), isNull);

    await tester.scrollUntilVisible(
      find.textContaining('maxLines six', findRichText: true),
      200,
      scrollable: find.descendant(
        of: find.byKey(const ValueKey('chat_input_field')),
        matching: find.byType(Scrollable),
      ),
    );

    await tester.tap(find.byKey(const ValueKey('chat_send_button')));
    await tester.pump();
    expect(tester.takeException(), isNull);
  });
}
