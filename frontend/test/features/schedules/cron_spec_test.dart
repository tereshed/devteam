import 'package:flutter/material.dart' show TimeOfDay;
import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/features/schedules/domain/cron_spec.dart';

void main() {
  group('ScheduleSpec.toCron', () {
    test('daily at 09:00', () {
      const spec = ScheduleSpec(
        frequency: ScheduleFrequency.daily,
        timeOfDay: TimeOfDay(hour: 9, minute: 0),
      );
      expect(spec.toCron(), '0 9 * * *');
    });

    test('weekly on Mon-Fri at 09:30', () {
      const spec = ScheduleSpec(
        frequency: ScheduleFrequency.weekly,
        timeOfDay: TimeOfDay(hour: 9, minute: 30),
        weekdays: {1, 2, 3, 4, 5},
      );
      expect(spec.toCron(), '30 9 * * 1,2,3,4,5');
    });

    test('every 3 hours at minute 0', () {
      const spec = ScheduleSpec(
        frequency: ScheduleFrequency.hourly,
        timeOfDay: TimeOfDay(hour: 0, minute: 0),
        intervalHours: 3,
      );
      expect(spec.toCron(), '0 */3 * * *');
    });

    test('custom passes raw through trimmed', () {
      const spec = ScheduleSpec(
        frequency: ScheduleFrequency.custom,
        rawCron: '  5 4 * * 0  ',
      );
      expect(spec.toCron(), '5 4 * * 0');
    });
  });

  group('ScheduleSpec.fromCron', () {
    test('parses daily', () {
      final spec = ScheduleSpec.fromCron('0 9 * * *');
      expect(spec.frequency, ScheduleFrequency.daily);
      expect(spec.timeOfDay, const TimeOfDay(hour: 9, minute: 0));
    });

    test('parses weekly with weekdays (7 normalized to 0)', () {
      final spec = ScheduleSpec.fromCron('30 9 * * 1,5,7');
      expect(spec.frequency, ScheduleFrequency.weekly);
      expect(spec.weekdays, {1, 5, 0});
      expect(spec.timeOfDay, const TimeOfDay(hour: 9, minute: 30));
    });

    test('parses hourly interval', () {
      final spec = ScheduleSpec.fromCron('0 */6 * * *');
      expect(spec.frequency, ScheduleFrequency.hourly);
      expect(spec.intervalHours, 6);
    });

    test('falls back to custom for unsupported expression', () {
      final spec = ScheduleSpec.fromCron('0 9 1 * *');
      expect(spec.frequency, ScheduleFrequency.custom);
      expect(spec.rawCron, '0 9 1 * *');
    });

    test('falls back to custom for wrong field count', () {
      final spec = ScheduleSpec.fromCron('0 9 * *');
      expect(spec.frequency, ScheduleFrequency.custom);
    });
  });

  group('round-trip', () {
    test('daily/weekly/hourly survive toCron→fromCron', () {
      const specs = [
        ScheduleSpec(
          frequency: ScheduleFrequency.daily,
          timeOfDay: TimeOfDay(hour: 7, minute: 15),
        ),
        ScheduleSpec(
          frequency: ScheduleFrequency.weekly,
          timeOfDay: TimeOfDay(hour: 18, minute: 0),
          weekdays: {1, 3, 5},
        ),
        ScheduleSpec(
          frequency: ScheduleFrequency.hourly,
          timeOfDay: TimeOfDay(hour: 0, minute: 0),
          intervalHours: 2,
        ),
      ];
      for (final original in specs) {
        final parsed = ScheduleSpec.fromCron(original.toCron());
        expect(parsed.toCron(), original.toCron());
      }
    });
  });
}
