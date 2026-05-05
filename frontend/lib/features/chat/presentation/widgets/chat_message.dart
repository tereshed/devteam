import 'package:flutter/material.dart';
import 'package:flutter_markdown_plus/flutter_markdown_plus.dart';
import 'package:frontend/features/chat/domain/models/conversation_message_model.dart';
import 'package:frontend/features/chat/presentation/widgets/chat_message_builders.dart';
import 'package:frontend/features/chat/presentation/widgets/chat_message_fence.dart';
import 'package:frontend/l10n/app_localizations.dart';
import 'package:markdown/markdown.dart' as md;

typedef _SheetMemoKey = ({
  String effectiveRole,
  Brightness brightness,
  Color primary,
  Color onSurface,
  Color onSurfaceVariant,
  Color surfaceContainerHighest,
  double? bodyFontSize,
  String? bodyFontFamily,
  FontWeight? bodyFontWeight,
});

typedef _BuildersMemoKey = ({
  _SheetMemoKey sheet,
  bool isStreaming,
  String copyTooltip,
});

/// Тело пузыря: markdown + fenced code (без raw HTML и без загрузки картинок по URL).
///
/// Длинные URL не модифицируются невидимыми символами — сохраняются autolink и акцент [ChatLinkBuilder].
///
/// Throttle частоты обновления при стриме — задача 11.9 (рекомендуется ≥ 100 ms).
///
/// [MarkdownStyleSheet] и карта [builders] мемоизируются в state по ключу темы / роли / стрима /
/// строки копирования, чтобы [MarkdownBody] не сбрасывал AST из‑за нового `Map`/`ChatPreBuilder`
/// на каждый ребилд ленты.
class ChatMessage extends StatefulWidget {
  const ChatMessage({
    super.key,
    required this.role,
    required this.content,
    this.isStreaming = false,
  });

  /// Одно из [conversationMessageRoles]: `user` | `assistant` | `system`.
  final String role;

  /// Сырой markdown-текст с бэкенда / контроллера.
  final String content;

  /// Хвост сообщения ещё дописывается (копирование кода по умолчанию отключено).
  final bool isStreaming;

  /// Убираем [md.InlineHtmlSyntax] из GFM inline-набора — иначе `<script>` в строке стал бы узлом HTML.
  /// Блоковый HTML из [BlockParser.standardBlockSyntaxes] ([HtmlBlockSyntax]) даёт в AST обычный
  /// текст сырой строки, не DOM — XSS через block-теги не строится (см. html_block_syntax.dart).
  static final md.ExtensionSet safeGfmExtensionSet = md.ExtensionSet(
    md.ExtensionSet.gitHubFlavored.blockSyntaxes,
    List<md.InlineSyntax>.from(
      md.ExtensionSet.gitHubFlavored.inlineSyntaxes.where(
        (s) => s is! md.InlineHtmlSyntax,
      ),
      growable: false,
    ),
  );

  @override
  State<ChatMessage> createState() => _ChatMessageState();
}

class _ChatMessageState extends State<ChatMessage> {
  late String _processed;

  MarkdownStyleSheet? _memoSheet;
  _SheetMemoKey? _memoSheetKey;

  Map<String, MarkdownElementBuilder>? _memoBuilders;
  _BuildersMemoKey? _memoBuildersKey;

  String _computeProcessed() {
    return preprocessUnclosedFence(
      widget.content,
      isStreaming: widget.isStreaming,
    );
  }

  static _SheetMemoKey _sheetMemoKey(ThemeData theme, String effectiveRole) {
    final b = theme.textTheme.bodyMedium;
    return (
      effectiveRole: effectiveRole,
      brightness: theme.brightness,
      primary: theme.colorScheme.primary,
      onSurface: theme.colorScheme.onSurface,
      onSurfaceVariant: theme.colorScheme.onSurfaceVariant,
      surfaceContainerHighest: theme.colorScheme.surfaceContainerHighest,
      bodyFontSize: b?.fontSize,
      bodyFontFamily: b?.fontFamily,
      bodyFontWeight: b?.fontWeight,
    );
  }

  static TextStyle? _tint(TextStyle? style, Color color) =>
      style?.copyWith(color: color);

  static MarkdownStyleSheet _buildMarkdownSheet(
    ThemeData theme,
    String effectiveRole,
  ) {
    final body = effectiveRole == 'system'
        ? theme.textTheme.bodyMedium?.copyWith(
            color: theme.colorScheme.onSurfaceVariant,
          )
        : theme.textTheme.bodyMedium;
    final fs = body?.fontSize;
    final codeStyle = body?.copyWith(
      fontFamily: 'monospace',
      fontSize: fs != null ? fs * 0.92 : null,
    );

    final base = MarkdownStyleSheet.fromTheme(theme).copyWith(
      p: body,
      code: codeStyle,
      blockSpacing: 8,
    );

    if (effectiveRole != 'system') {
      return base;
    }

    final c = theme.colorScheme.onSurfaceVariant;
    return base.copyWith(
      a: _tint(base.a, c),
      p: body,
      code: codeStyle,
      h1: _tint(base.h1, c),
      h2: _tint(base.h2, c),
      h3: _tint(base.h3, c),
      h4: _tint(base.h4, c),
      h5: _tint(base.h5, c),
      h6: _tint(base.h6, c),
      em: _tint(base.em, c)?.copyWith(fontStyle: FontStyle.italic),
      strong: _tint(base.strong, c)?.copyWith(fontWeight: FontWeight.bold),
      del: _tint(base.del, c)
          ?.copyWith(decoration: TextDecoration.lineThrough),
      blockquote: _tint(base.blockquote, c),
      img: _tint(base.img, c),
      checkbox: _tint(base.checkbox, c),
      listBullet: _tint(base.listBullet, c),
      tableHead: _tint(base.tableHead, c)
          ?.copyWith(fontWeight: FontWeight.w600),
      tableBody: _tint(base.tableBody, c),
      blockSpacing: 8,
    );
  }

  @override
  void initState() {
    super.initState();
    _processed = _computeProcessed();
  }

  @override
  void didUpdateWidget(ChatMessage oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.content != widget.content ||
        oldWidget.isStreaming != widget.isStreaming) {
      _processed = _computeProcessed();
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    late final String effectiveRole;
    if (conversationMessageRoles.contains(widget.role)) {
      effectiveRole = widget.role;
    } else {
      assert(
        false,
        'Unknown ChatMessage role: "${widget.role}". '
        'Ожидаются значения из conversationMessageRoles.',
      );
      effectiveRole = 'system';
    }

    if (widget.content.isEmpty && !widget.isStreaming) {
      return const SizedBox.shrink();
    }

    final l10n = AppLocalizations.of(context)!;

    if (widget.content.isEmpty && widget.isStreaming) {
      return Text(
        l10n.chatMessageStreamingPlaceholder,
        style: theme.textTheme.bodyMedium?.copyWith(
          color: theme.colorScheme.onSurfaceVariant,
        ),
      );
    }

    final sheetKey = _sheetMemoKey(theme, effectiveRole);
    if (_memoSheet == null || _memoSheetKey != sheetKey) {
      _memoSheet = _buildMarkdownSheet(theme, effectiveRole);
      _memoSheetKey = sheetKey;
      _memoBuilders = null;
      _memoBuildersKey = null;
    }
    final sheet = _memoSheet!;

    final buildersKey = (
      sheet: sheetKey,
      isStreaming: widget.isStreaming,
      copyTooltip: l10n.chatMessageCopyCode,
    );
    if (_memoBuilders == null || _memoBuildersKey != buildersKey) {
      _memoBuilders = <String, MarkdownElementBuilder>{
        'pre': ChatPreBuilder(
          isStreaming: widget.isStreaming,
          styleSheet: sheet,
          copyTooltip: l10n.chatMessageCopyCode,
        ),
        'a': ChatLinkBuilder.instance,
      };
      _memoBuildersKey = buildersKey;
    }
    final builders = _memoBuilders!;

    final bodyStyle = sheet.p ?? theme.textTheme.bodyMedium;

    return RepaintBoundary(
      child: MarkdownBody(
        data: _processed,
        selectable: true,
        softLineBreak: true,
        extensionSet: ChatMessage.safeGfmExtensionSet,
        styleSheet: sheet,
        builders: builders,
        imageBuilder: (uri, title, alt) {
          final altText = alt ?? '';
          final label = altText.isEmpty
              ? l10n.chatMessageImagePlaceholder
              : l10n.chatMessageMarkdownImageAlt(altText);
          return Text.rich(
            TextSpan(text: label, style: bodyStyle),
          );
        },
      ),
    );
  }
}
