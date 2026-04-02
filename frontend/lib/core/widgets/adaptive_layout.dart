import 'package:flutter/material.dart';
import 'package:frontend/core/utils/responsive.dart';

/// AdaptiveLayout - виджет для адаптивной компоновки
///
/// Автоматически выбирает layout на основе размера экрана:
/// - Mobile: всегда вертикальная компоновка
/// - Tablet/Desktop: может быть горизонтальная или вертикальная
class AdaptiveLayout extends StatelessWidget {
  /// Основной контент (на мобильных всегда сверху, на Desktop - слева)
  final Widget primary;

  /// Дополнительный контент (опционально, на Desktop - справа)
  final Widget? secondary;

  /// Соотношение ширины primary к secondary на Desktop (по умолчанию 1:1)
  final double flexPrimary;
  final double flexSecondary;

  /// Минимальная ширина для горизонтальной компоновки
  /// По умолчанию - Tablet breakpoint
  final double minWidthForHorizontal;

  const AdaptiveLayout({
    super.key,
    required this.primary,
    this.secondary,
    this.flexPrimary = 1,
    this.flexSecondary = 1,
    this.minWidthForHorizontal = 600,
  });

  @override
  Widget build(BuildContext context) {
    final width = MediaQuery.of(context).size.width;
    final isHorizontal = width >= minWidthForHorizontal && secondary != null;

    if (isHorizontal) {
      return Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Expanded(flex: flexPrimary.toInt(), child: primary),
          SizedBox(width: Spacing.medium(context)),
          Expanded(flex: flexSecondary.toInt(), child: secondary!),
        ],
      );
    }

    // Вертикальная компоновка для мобильных или без secondary
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        primary,
        if (secondary != null) ...[
          SizedBox(height: Spacing.medium(context)),
          secondary!,
        ],
      ],
    );
  }
}

/// AdaptiveContainer - контейнер с адаптивными отступами и ограничением ширины
class AdaptiveContainer extends StatelessWidget {
  final Widget child;
  final bool usePadding;
  final bool constrainWidth;

  const AdaptiveContainer({
    super.key,
    required this.child,
    this.usePadding = true,
    this.constrainWidth = true,
  });

  @override
  Widget build(BuildContext context) {
    var content = child;

    if (constrainWidth) {
      content = ConstrainedBox(
        constraints: BoxConstraints(
          maxWidth: Responsive.getMaxContentWidth(context) ?? double.infinity,
        ),
        child: content,
      );
    }

    if (usePadding) {
      content = Padding(
        padding: Responsive.getPadding(context),
        child: content,
      );
    }

    return content;
  }
}
