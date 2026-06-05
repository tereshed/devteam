import 'package:dio/dio.dart';
import 'package:frontend/features/schedules/data/scheduled_task_providers.dart';
import 'package:frontend/features/schedules/domain/models/scheduled_task_model.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

part 'schedules_controller.g.dart';

/// Контроллер списка регулярных задач проекта: загрузка + мутации.
@riverpod
class SchedulesController extends _$SchedulesController {
  @override
  Future<List<ScheduledTaskModel>> build(String projectId) async {
    final cancelToken = CancelToken();
    ref.onDispose(cancelToken.cancel);
    return ref
        .read(scheduledTaskRepositoryProvider)
        .list(projectId, cancelToken: cancelToken);
  }

  Future<void> createSchedule({
    required String name,
    required String cronExpression,
    String description = '',
    String? priority,
    String? teamId,
    bool? isActive,
  }) async {
    await ref.read(scheduledTaskRepositoryProvider).create(
          projectId,
          name: name,
          cronExpression: cronExpression,
          description: description,
          priority: priority,
          teamId: teamId,
          isActive: isActive,
        );
    await _reload();
  }

  Future<void> updateSchedule(
    String scheduleId, {
    String? name,
    String? cronExpression,
    String? description,
    String? priority,
    String? teamId,
    bool clearTeam = false,
    bool? isActive,
  }) async {
    await ref.read(scheduledTaskRepositoryProvider).update(
          projectId,
          scheduleId,
          name: name,
          cronExpression: cronExpression,
          description: description,
          priority: priority,
          teamId: teamId,
          clearTeam: clearTeam,
          isActive: isActive,
        );
    await _reload();
  }

  Future<void> toggleActive(ScheduledTaskModel schedule) async {
    await updateSchedule(schedule.id, isActive: !schedule.isActive);
  }

  Future<void> deleteSchedule(String scheduleId) async {
    await ref.read(scheduledTaskRepositoryProvider).delete(projectId, scheduleId);
    await _reload();
  }

  Future<void> _reload() async {
    state = const AsyncValue.loading();
    state = await AsyncValue.guard(() {
      final cancelToken = CancelToken();
      ref.onDispose(cancelToken.cancel);
      return ref
          .read(scheduledTaskRepositoryProvider)
          .list(projectId, cancelToken: cancelToken);
    });
  }
}
