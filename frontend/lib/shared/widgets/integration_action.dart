import 'package:flutter/material.dart';
import 'package:freezed_annotation/freezed_annotation.dart';

part 'integration_action.freezed.dart';

/// Стиль кнопки действия на интеграционной карточке.
///
/// `primary` — основной CTA (Подключить / Обновить).
/// `secondary` — нейтральный (Тест / Подробнее).
/// `destructive` — деструктивный (Отключить / Удалить).
enum IntegrationActionStyle { primary, secondary, destructive }

/// Описание кнопки действия на интеграционной карточке.
///
/// Используется виджетом [IntegrationProviderCard] для рендера ряда кнопок.
/// См. dashboard-redesign-plan.md §4a.3.
@freezed
abstract class IntegrationAction with _$IntegrationAction {
  const factory IntegrationAction({
    required String label,
    required VoidCallback onPressed,
    @Default(IntegrationActionStyle.secondary) IntegrationActionStyle style,
    IconData? icon,
    @Default(false) bool isBusy,
  }) = _IntegrationAction;
}
