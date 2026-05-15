import 'package:flutter/material.dart';
import 'package:frontend/shared/widgets/diff_viewer_parsing.dart';

/// Отображение unified git diff с подсветкой по [ColorScheme] (M3).
///
/// [diff] — непустой raw-текст; пустой diff на стороне call-site (12.7).
///
/// Использует [ListView.builder] с фиксированным [itemExtent] — на больших
/// диффах (5K-15K строк, типичный output developer-агента) рендерится только
/// окно viewport, а layout остаётся O(1) на строку (6.4 performance).
class DiffViewer extends StatefulWidget {
  const DiffViewer({
    super.key,
    required this.diff,
    this.maxHeight,
  });

  /// Фиксированная высота одной diff-строки.
  ///
  /// Текст — monospace bodySmall (~12pt) с межстрочным разделителем. Значение
  /// подобрано так, чтобы ни одна стандартная строка не клиппилась; небольшой
  /// запас (≈2px) безопаснее, чем точная подгонка по теме.
  static const double lineExtent = 18.0;

  final String diff;
  final double? maxHeight;

  @override
  State<DiffViewer> createState() => _DiffViewerState();
}

class _DiffViewerState extends State<DiffViewer> {
  List<DiffParsedLine>? _parsed;
  String? _cachedDiff;

  @override
  void initState() {
    super.initState();
    _parsed = parseUnifiedDiff(widget.diff);
    _cachedDiff = widget.diff;
  }

  @override
  void didUpdateWidget(covariant DiffViewer oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (widget.diff != _cachedDiff) {
      _cachedDiff = widget.diff;
      _parsed = parseUnifiedDiff(widget.diff);
    }
  }

  double _effectiveMaxHeight(BuildContext context) {
    if (widget.maxHeight != null) {
      return widget.maxHeight!;
    }
    final h = MediaQuery.sizeOf(context).height;
    return (h * 0.4).clamp(180.0, 480.0);
  }

  TextStyle _monoStyle(BuildContext context) {
    final base = Theme.of(context).textTheme.bodySmall;
    return base?.copyWith(fontFamily: 'monospace') ??
        const TextStyle(fontFamily: 'monospace', fontSize: 12);
  }

  @override
  Widget build(BuildContext context) {
    final parsed = _parsed ?? parseUnifiedDiff(widget.diff);
    final scheme = Theme.of(context).colorScheme;
    final maxH = _effectiveMaxHeight(context);
    final mono = _monoStyle(context);

    return ConstrainedBox(
      constraints: BoxConstraints(maxHeight: maxH),
      child: SelectionArea(
        child: ListView.builder(
          shrinkWrap: true,
          primary: false,
          physics: const ClampingScrollPhysics(),
          itemCount: parsed.length,
          itemExtent: DiffViewer.lineExtent,
          itemBuilder: (context, index) {
            final line = parsed[index];
            return DiffLineRow(
              line: line,
              monoStyle: mono,
              scheme: scheme,
            );
          },
        ),
      ),
    );
  }
}

/// Один ряд diff. Public, чтобы тесты могли посчитать [LineTile]-метрики
/// (см. `TestDiffViewer_LargeDiff_DoesNotBuildAllTilesAtOnce`).
class DiffLineRow extends StatelessWidget {
  const DiffLineRow({
    super.key,
    required this.line,
    required this.monoStyle,
    required this.scheme,
  });

  final DiffParsedLine line;
  final TextStyle monoStyle;
  final ColorScheme scheme;

  @override
  Widget build(BuildContext context) {
    switch (line.kind) {
      case DiffParsedLineKind.plainBlock:
        return SingleChildScrollView(
          scrollDirection: Axis.horizontal,
          child: Text(
            line.text,
            style: monoStyle.copyWith(color: scheme.onSurface),
            softWrap: false,
          ),
        );
      case DiffParsedLineKind.metadata:
      case DiffParsedLineKind.fileHeader:
        return _row(
          bg: null,
          fg: scheme.onSurfaceVariant,
        );
      case DiffParsedLineKind.hunkHeader:
        return _row(
          bg: null,
          fg: scheme.primary,
        );
      case DiffParsedLineKind.context:
        return _row(
          bg: scheme.surface,
          fg: scheme.onSurface,
        );
      case DiffParsedLineKind.addition:
        return _row(
          bg: scheme.primaryContainer,
          fg: scheme.onPrimaryContainer,
        );
      case DiffParsedLineKind.deletion:
        return _row(
          bg: scheme.surfaceContainerHighest,
          fg: scheme.onSurface,
        );
    }
  }

  Widget _row({required Color? bg, required Color fg}) {
    final text = Text(
      line.text,
      style: monoStyle.copyWith(color: fg),
      softWrap: false,
    );
    final inner = SingleChildScrollView(
      scrollDirection: Axis.horizontal,
      child: text,
    );
    if (bg == null) {
      return Align(
        alignment: Alignment.centerLeft,
        child: inner,
      );
    }
    return ColoredBox(
      color: bg,
      child: Align(
        alignment: Alignment.centerLeft,
        child: inner,
      ),
    );
  }
}
