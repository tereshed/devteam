import 'package:flutter/material.dart' show TimeOfDay;

/// Тип частоты в дружелюбном конструкторе расписания.
enum ScheduleFrequency { daily, weekly, hourly, custom }

/// ScheduleSpec — UI-представление расписания, конвертируемое в стандартное
/// 5-польное cron-выражение (minute hour dom month dow), которое хранит backend.
///
/// Маппинг дней недели на cron `dow`: воскресенье = 0, понедельник..суббота = 1..6.
class ScheduleSpec {
  const ScheduleSpec({
    required this.frequency,
    this.timeOfDay = const TimeOfDay(hour: 9, minute: 0),
    this.weekdays = const <int>{1, 2, 3, 4, 5},
    this.intervalHours = 1,
    this.rawCron = '',
  });

  final ScheduleFrequency frequency;
  final TimeOfDay timeOfDay;

  /// Дни недели в cron-нотации (0 = Вс, 1..6 = Пн..Сб). Для [ScheduleFrequency.weekly].
  final Set<int> weekdays;

  /// Интервал в часах для [ScheduleFrequency.hourly] (1..23).
  final int intervalHours;

  /// Сырое cron-выражение для [ScheduleFrequency.custom].
  final String rawCron;

  ScheduleSpec copyWith({
    ScheduleFrequency? frequency,
    TimeOfDay? timeOfDay,
    Set<int>? weekdays,
    int? intervalHours,
    String? rawCron,
  }) {
    return ScheduleSpec(
      frequency: frequency ?? this.frequency,
      timeOfDay: timeOfDay ?? this.timeOfDay,
      weekdays: weekdays ?? this.weekdays,
      intervalHours: intervalHours ?? this.intervalHours,
      rawCron: rawCron ?? this.rawCron,
    );
  }

  /// Преобразует спецификацию в стандартное 5-польное cron-выражение.
  String toCron() {
    final m = timeOfDay.minute;
    final h = timeOfDay.hour;
    switch (frequency) {
      case ScheduleFrequency.daily:
        return '$m $h * * *';
      case ScheduleFrequency.weekly:
        final days = (weekdays.toList()..sort()).join(',');
        final dow = days.isEmpty ? '*' : days;
        return '$m $h * * $dow';
      case ScheduleFrequency.hourly:
        final n = intervalHours < 1 ? 1 : intervalHours;
        return '$m */$n * * *';
      case ScheduleFrequency.custom:
        return rawCron.trim();
    }
  }

  /// Best-effort разбор cron-выражения обратно в спецификацию (для редактирования).
  /// При непонятном выражении возвращает [ScheduleFrequency.custom] с rawCron.
  static ScheduleSpec fromCron(String expr) {
    final trimmed = expr.trim();
    final parts = trimmed.split(RegExp(r'\s+'));
    final custom = ScheduleSpec(
      frequency: ScheduleFrequency.custom,
      rawCron: trimmed,
    );
    if (parts.length != 5) {
      return custom;
    }
    final minute = int.tryParse(parts[0]);
    final dom = parts[2];
    final month = parts[3];
    final dow = parts[4];

    // Каждые N часов: "M */N * * *"
    final hourEvery = RegExp(r'^\*/(\d+)$').firstMatch(parts[1]);
    if (minute != null && hourEvery != null && dom == '*' && month == '*' && dow == '*') {
      final n = int.tryParse(hourEvery.group(1)!) ?? 1;
      return ScheduleSpec(
        frequency: ScheduleFrequency.hourly,
        timeOfDay: TimeOfDay(hour: 0, minute: minute),
        intervalHours: n,
      );
    }

    final hour = int.tryParse(parts[1]);
    if (minute == null || hour == null || month != '*') {
      return custom;
    }

    // Ежедневно: "M H * * *"
    if (dom == '*' && dow == '*') {
      return ScheduleSpec(
        frequency: ScheduleFrequency.daily,
        timeOfDay: TimeOfDay(hour: hour, minute: minute),
      );
    }

    // Еженедельно по дням: "M H * * d,d,d"
    if (dom == '*') {
      final days = <int>{};
      for (final token in dow.split(',')) {
        final d = int.tryParse(token.trim());
        if (d == null) {
          return custom;
        }
        // cron допускает 7 как воскресенье — нормализуем к 0.
        days.add(d == 7 ? 0 : d);
      }
      if (days.isEmpty) {
        return custom;
      }
      return ScheduleSpec(
        frequency: ScheduleFrequency.weekly,
        timeOfDay: TimeOfDay(hour: hour, minute: minute),
        weekdays: days,
      );
    }

    return custom;
  }
}
