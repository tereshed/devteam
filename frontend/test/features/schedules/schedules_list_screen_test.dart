import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/features/schedules/data/scheduled_task_providers.dart';
import 'package:frontend/features/schedules/data/scheduled_task_repository.dart';
import 'package:frontend/features/schedules/domain/models/scheduled_task_model.dart';
import 'package:frontend/features/schedules/presentation/screens/schedules_list_screen.dart';
import 'package:frontend/l10n/app_localizations.dart';

/// Фейковый репозиторий — реальный контроллер выполняется поверх него.
class _FakeRepo extends ScheduledTaskRepository {
  _FakeRepo(this.items) : super(dio: Dio());
  final List<ScheduledTaskModel> items;

  @override
  Future<List<ScheduledTaskModel>> list(
    String projectId, {
    CancelToken? cancelToken,
  }) async =>
      items;
}

ScheduledTaskModel _sample() => ScheduledTaskModel(
      id: '11111111-1111-1111-1111-111111111111',
      projectId: 'p1',
      createdBy: 'u1',
      name: 'Nightly refactor',
      description: 'do the thing',
      cronExpression: '0 3 * * *',
      createdAt: DateTime(2026, 6, 1),
      updatedAt: DateTime(2026, 6, 1),
    );

Widget _harness(List<ScheduledTaskModel> items) {
  return ProviderScope(
    retry: (_, _) => null,
    overrides: [
      scheduledTaskRepositoryProvider.overrideWithValue(_FakeRepo(items)),
    ],
    child: const MaterialApp(
      localizationsDelegates: AppLocalizations.localizationsDelegates,
      supportedLocales: AppLocalizations.supportedLocales,
      locale: Locale('en'),
      home: SchedulesListScreen(projectId: 'p1'),
    ),
  );
}

void main() {
  testWidgets('shows empty state when no schedules', (tester) async {
    await tester.pumpWidget(_harness([]));
    await tester.pumpAndSettle();

    expect(find.text('No scheduled tasks yet'), findsOneWidget);
    // FAB + empty-state button both labelled "New scheduled task".
    expect(find.text('New scheduled task'), findsWidgets);
  });

  testWidgets('renders a schedule card with cron and name', (tester) async {
    await tester.pumpWidget(_harness([_sample()]));
    await tester.pumpAndSettle();

    expect(find.text('Nightly refactor'), findsOneWidget);
    expect(find.text('0 3 * * *'), findsOneWidget);
    expect(find.text('Active'), findsOneWidget);
  });
}
