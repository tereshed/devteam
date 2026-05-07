import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/features/chat/presentation/widgets/chat_input.dart';
import 'package:frontend/l10n/app_localizations.dart';

import '../../helpers/test_wrappers.dart';

void main() {
  /// Как прежний harness: фиксированный logical size (**11.11** / **useViewSize**).
  void bindDefaultView(WidgetTester tester) {
    useViewSize(tester, const Size(800, 1200));
  }

  Widget harness({
    required Widget Function(AppLocalizations l10n) body,
    Locale locale = const Locale('en'),
    TextScaler textScaler = TextScaler.noScaling,
  }) =>
      wrapChatInputHarness(
        locale: locale,
        textScaler: textScaler,
        body: Builder(
          builder: (context) {
            final l10n = AppLocalizations.of(context)!;
            return body(l10n);
          },
        ),
      );

  Future<void> focusInput(WidgetTester tester) async {
    await tester.tap(find.byKey(const ValueKey('chat_input_field')));
    await tester.pump();
  }

  Future<void> sendCtrlEnter(WidgetTester tester) async {
    await tester.sendKeyDownEvent(LogicalKeyboardKey.controlLeft);
    await tester.sendKeyDownEvent(LogicalKeyboardKey.enter);
    await tester.sendKeyUpEvent(LogicalKeyboardKey.enter);
    await tester.sendKeyUpEvent(LogicalKeyboardKey.controlLeft);
    await tester.pump();
  }

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
    bindDefaultView(tester);
    final c = TextEditingController();
    final focus = FocusNode();
    addTearDown(() {
      c.dispose();
      focus.dispose();
    });

    await tester.pumpWidget(
      harness(
        body: (l10n) => ChatInput(
          controller: c,
          focusNode: focus,
          onSend: (_) {},
          sendTooltip: l10n.chatInputSendTooltip,
          hintText: l10n.chatInputHint,
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
    bindDefaultView(tester);
    final c = TextEditingController(text: 'x');
    final focus = FocusNode();
    addTearDown(() {
      c.dispose();
      focus.dispose();
    });

    await tester.pumpWidget(
      harness(
        body: (l10n) => ChatInput(
          controller: c,
          focusNode: focus,
          onSend: (_) {},
          sendTooltip: l10n.chatInputSendTooltip,
          hintText: l10n.chatInputHint,
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
    bindDefaultView(tester);
    final c = TextEditingController(text: 'hello');
    final focus = FocusNode();
    var n = 0;
    addTearDown(() {
      c.dispose();
      focus.dispose();
    });

    await tester.pumpWidget(
      harness(
        body: (l10n) => ChatInput(
          controller: c,
          focusNode: focus,
          onSend: (_) => n++,
          sendTooltip: l10n.chatInputSendTooltip,
          hintText: l10n.chatInputHint,
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
    bindDefaultView(tester);
    final c = TextEditingController(text: '  hi  ');
    final focus = FocusNode();
    String? received;
    addTearDown(() {
      c.dispose();
      focus.dispose();
    });

    await tester.pumpWidget(
      harness(
        body: (l10n) => ChatInput(
          controller: c,
          focusNode: focus,
          onSend: (s) => received = s,
          sendTooltip: l10n.chatInputSendTooltip,
          hintText: l10n.chatInputHint,
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
    bindDefaultView(tester);
    final c = TextEditingController(text: 'a');
    final focus = FocusNode();
    var n = 0;
    addTearDown(() {
      c.dispose();
      focus.dispose();
    });

    await tester.pumpWidget(
      harness(
        body: (l10n) => ChatInput(
          controller: c,
          focusNode: focus,
          onSend: (_) => n++,
          sendTooltip: l10n.chatInputSendTooltip,
          hintText: l10n.chatInputHint,
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
    bindDefaultView(tester);
    final c = TextEditingController(text: 'a');
    final focus = FocusNode();
    var n = 0;
    addTearDown(() {
      c.dispose();
      focus.dispose();
    });

    await tester.pumpWidget(
      harness(
        body: (l10n) => ChatInput(
          controller: c,
          focusNode: focus,
          onSend: (_) => n++,
          sendTooltip: l10n.chatInputSendTooltip,
          hintText: l10n.chatInputHint,
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
    bindDefaultView(tester);
    final c = TextEditingController(text: 'ok');
    final focus = FocusNode();
    var n = 0;
    addTearDown(() {
      c.dispose();
      focus.dispose();
    });

    await tester.pumpWidget(
      harness(
        body: (l10n) => ChatInput(
          controller: c,
          focusNode: focus,
          onSend: (_) => n++,
          sendTooltip: l10n.chatInputSendTooltip,
          hintText: l10n.chatInputHint,
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
    bindDefaultView(tester);
    final c = TextEditingController(text: ' \n ');
    final focus = FocusNode();
    var n = 0;
    addTearDown(() {
      c.dispose();
      focus.dispose();
    });

    await tester.pumpWidget(
      harness(
        body: (l10n) => ChatInput(
          controller: c,
          focusNode: focus,
          onSend: (_) => n++,
          sendTooltip: l10n.chatInputSendTooltip,
          hintText: l10n.chatInputHint,
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
    bindDefaultView(tester);
    final c = TextEditingController();
    final focus = FocusNode();
    var n = 0;
    addTearDown(() {
      c.dispose();
      focus.dispose();
    });

    await tester.pumpWidget(
      harness(
        body: (l10n) => ChatInput(
          controller: c,
          focusNode: focus,
          onSend: (_) => n++,
          sendTooltip: l10n.chatInputSendTooltip,
          hintText: l10n.chatInputHint,
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
    bindDefaultView(tester);
    final c = TextEditingController(text: 'y');
    final focus = FocusNode();
    var n = 0;
    addTearDown(() {
      c.dispose();
      focus.dispose();
    });

    await tester.pumpWidget(
      harness(
        body: (l10n) => ChatInput(
          controller: c,
          focusNode: focus,
          onSend: (_) => n++,
          sendTooltip: l10n.chatInputSendTooltip,
          hintText: l10n.chatInputHint,
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
    bindDefaultView(tester);
    final c = TextEditingController(text: 'z');
    final focus = FocusNode();
    addTearDown(() {
      c.dispose();
      focus.dispose();
    });

    await tester.pumpWidget(
      harness(
        body: (l10n) => ChatInput(
          controller: c,
          focusNode: focus,
          onSend: (_) {},
          sendTooltip: l10n.chatInputSendTooltip,
          hintText: l10n.chatInputHint,
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

  testWidgets('chat_input_ru_locale_hint_and_tooltip_from_arb', (
    WidgetTester tester,
  ) async {
    bindDefaultView(tester);
    final c = TextEditingController(text: 'z');
    final focus = FocusNode();
    addTearDown(() {
      c.dispose();
      focus.dispose();
    });

    await tester.pumpWidget(
      harness(
        locale: const Locale('ru'),
        body: (l10n) => ChatInput(
          controller: c,
          focusNode: focus,
          onSend: (_) {},
          sendTooltip: l10n.chatInputSendTooltip,
          hintText: l10n.chatInputHint,
        ),
      ),
    );

    final ctx = tester.element(find.byType(ChatInput));
    final l10n = AppLocalizations.of(ctx)!;
    expect(find.byTooltip(l10n.chatInputSendTooltip), findsOneWidget);
    expect(find.text(l10n.chatInputHint), findsWidgets);
  });

  testWidgets('chat_input_text_scaler_2_no_overflow_usable_send', (
    WidgetTester tester,
  ) async {
    bindDefaultView(tester);
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
        textScaler: const TextScaler.linear(2),
        body: (l10n) => ChatInput(
          controller: c,
          focusNode: focus,
          onSend: (_) {},
          sendTooltip: l10n.chatInputSendTooltip,
          hintText: l10n.chatInputHint,
        ),
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
