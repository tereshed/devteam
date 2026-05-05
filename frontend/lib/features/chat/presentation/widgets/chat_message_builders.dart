import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_markdown_plus/flutter_markdown_plus.dart';
import 'package:frontend/features/chat/presentation/widgets/chat_message_href.dart';
import 'package:markdown/markdown.dart' as md;

/// Запас по оси inline-end под кнопку копирования кода (dp); см. тест инварианта padding.
const double kChatMessageCodeCopyReserveEnd = 40;

/// Горизонтальный скролл кода: один [ScrollController] на пару [Scrollbar] +
/// [SingleChildScrollView] (тот же паттерн, что в flutter_markdown_plus `builder.dart`
/// — общий контроллер для Scrollbar и горизонтального [SingleChildScrollView]),
/// иначе на desktop/web [Scrollbar] без controller ломается (ambient PrimaryScrollController).
class _CodeBlockScroll extends StatefulWidget {
  const _CodeBlockScroll({
    required this.padding,
    required this.decoration,
    required this.child,
  });

  final EdgeInsetsGeometry? padding;
  final Decoration? decoration;
  final Widget child;

  @override
  State<_CodeBlockScroll> createState() => _CodeBlockScrollState();
}

class _CodeBlockScrollState extends State<_CodeBlockScroll> {
  final ScrollController _controller = ScrollController();

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return Container(
      key: const ValueKey<String>('chat_message_code_hscroll'),
      clipBehavior: Clip.hardEdge,
      decoration: widget.decoration,
      child: Scrollbar(
        controller: _controller,
        child: SingleChildScrollView(
          controller: _controller,
          scrollDirection: Axis.horizontal,
          primary: false,
          padding: widget.padding,
          child: widget.child,
        ),
      ),
    );
  }
}

/// Блок кода: горизонтальный скролл, копирование текста тела fence.
///
/// В буфер уходит тот же литерал, что и в [Text.rich] (после снятия одного
/// финального `\n` от лексера перед закрывающими fence — как при копировании с GitHub).
///
/// Регистрируется только как builder для тега `pre`.
class ChatPreBuilder extends MarkdownElementBuilder {
  ChatPreBuilder({
    required this.isStreaming,
    required this.styleSheet,
    required this.copyTooltip,
  }) {
    // Счётчик только в debug/profile для теста мемоизации; в release tree-shaking убирает assert.
    assert(() {
      _debugInstantiationCount += 1;
      return true;
    }());
  }

  final bool isStreaming;
  final MarkdownStyleSheet styleSheet;
  final String copyTooltip;

  static int _debugInstantiationCount = 0;

  /// Сброс перед тестом мемоизации builders.
  @visibleForTesting
  static void resetDebugInstantiationCount() {
    _debugInstantiationCount = 0;
  }

  @visibleForTesting
  static int get debugInstantiationCount => _debugInstantiationCount;

  final StringBuffer _codeBody = StringBuffer();

  @override
  bool isBlockElement() => true;

  @override
  void visitElementBefore(md.Element element) {
    _codeBody.clear();
  }

  @override
  Widget? visitText(md.Text text, TextStyle? preferredStyle) {
    _codeBody.write(text.text);
    return null;
  }

  @override
  Widget? visitElementAfterWithContext(
    BuildContext context,
    md.Element element,
    TextStyle? preferredStyle,
    TextStyle? parentStyle,
  ) {
    final raw = _codeBody.toString();
    final display = raw.replaceAll(RegExp(r'\n$'), '');
    final codeStyle = styleSheet.code ?? preferredStyle;

    // Кастомный builder не оборачивается markdown-пакетом в SelectableText — используем
    // [SelectableText.rich], чтобы выделение работало как у остальных блоков при selectable: true.
    final scrollChild = SelectableText.rich(
      TextSpan(text: display, style: codeStyle),
    );

    final scrollPadding = (styleSheet.codeblockPadding ?? EdgeInsets.zero).add(
      const EdgeInsetsDirectional.only(end: kChatMessageCodeCopyReserveEnd),
    );

    return Stack(
      clipBehavior: Clip.none,
      children: [
        _CodeBlockScroll(
          padding: scrollPadding,
          decoration: styleSheet.codeblockDecoration,
          child: scrollChild,
        ),
        PositionedDirectional(
          top: 4,
          end: 4,
          child: IconButton(
            tooltip: copyTooltip,
            // Иконка компактная; без shrinkWrap — Material сохраняет hit-area ≥48dp (guidelines).
            style: IconButton.styleFrom(
              visualDensity: VisualDensity.compact,
            ),
            onPressed: isStreaming
                ? null
                : () => Clipboard.setData(ClipboardData(text: display)),
            icon: Icon(
              Icons.copy,
              size: 18,
              color: Theme.of(context).colorScheme.onSurfaceVariant,
            ),
          ),
        ),
      ],
    );
  }
}

/// Ссылки: без навигации; допустимые схемы — акцентным цветом, остальное — как текст.
///
/// `Element.textContent` в markdown AST сжимает вложенный inline (**жирная ссылка**) до плоского текста —
/// осознанный компромисс к whitelist href (иначе нужен кастомный обход дефолтного рендера `a`).
///
/// Возвращаем именно [RichText]: flutter_markdown_plus при сборке абзаца извлекает [InlineSpan]
/// из дочернего [RichText]/[Text] и сливает с родительским деревом — иначе ломаются единое
/// выделение и тесты на [TextDecoration]. Не заменять на [Container] с произвольным child.
class ChatLinkBuilder extends MarkdownElementBuilder {
  ChatLinkBuilder._();

  /// Stateless builder — один экземпляр на карту `builders`.
  static final ChatLinkBuilder instance = ChatLinkBuilder._();

  @override
  Widget? visitElementAfterWithContext(
    BuildContext context,
    md.Element element,
    TextStyle? preferredStyle,
    TextStyle? parentStyle,
  ) {
    final href = element.attributes['href'];
    final label = element.textContent;
    final theme = Theme.of(context);
    final base = parentStyle ?? preferredStyle ?? theme.textTheme.bodyMedium;
    final TextStyle effective;
    if (base == null) {
      effective = TextStyle(color: theme.colorScheme.onSurface);
    } else if (isAllowedHref(href)) {
      effective = base.copyWith(
        color: theme.colorScheme.primary,
        decoration: TextDecoration.underline,
      );
    } else {
      effective = base;
    }
    return RichText(
      text: TextSpan(text: label, style: effective),
    );
  }
}
