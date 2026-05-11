import 'dart:async';

import 'package:dio/dio.dart';
import 'package:frontend/core/api/api_exceptions.dart';
import 'package:frontend/features/projects/data/project_providers.dart';
import 'package:frontend/features/projects/domain/models/project_model.dart';
import 'package:frontend/features/projects/domain/project_exceptions.dart';
import 'package:frontend/features/projects/domain/requests.dart';
import 'package:frontend/l10n/app_localizations.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

part 'project_settings_controller.g.dart';

String projectSettingsSaveErrorTitle(AppLocalizations l10n, Object error) {
  return switch (error) {
    ProjectConflictException _ => l10n.projectSettingsSaveConflict,
    UnauthorizedException _ => l10n.errorUnauthorized,
    ProjectForbiddenException _ => l10n.projectSettingsActionForbidden,
    ProjectCancelledException _ => l10n.errorRequestCancelled,
    final ProjectApiException e => _projectSettingsApiErrorTitle(l10n, e),
    _ => l10n.projectSettingsSaveGenericError,
  };
}

String _projectSettingsApiErrorTitle(
  AppLocalizations l10n,
  ProjectApiException e,
) {
  final message = e.message;
  if (message == 'Network timeout' || message == 'Network error') {
    return l10n.errorNetwork;
  }
  final code = e.statusCode ?? 0;
  if (code == 502) {
    return l10n.projectSettingsGitRemoteAccessFailed;
  }
  if (code >= 500) {
    return l10n.errorServer;
  }
  if (code == 400) {
    return l10n.projectSettingsSaveValidationError;
  }
  return l10n.projectSettingsSaveGenericError;
}

/// Детальный хвост для SnackBar при ошибках save и reindex (как в форме создания проекта).
String? projectSettingsErrorDetail(Object error) {
  if (error is ProjectCancelledException) {
    return null;
  }
  if (error is! ProjectApiException) {
    return null;
  }
  final m = error.message;
  if (m.isEmpty || m == 'Network timeout' || m == 'Network error') {
    return null;
  }
  const maxLen = 200;
  if (m.length <= maxLen) {
    return m;
  }
  return '${m.substring(0, maxLen)}…';
}

String projectSettingsReindexErrorTitle(AppLocalizations l10n, Object error) {
  return switch (error) {
    ProjectConflictException _ => l10n.projectSettingsReindexConflict,
    UnauthorizedException _ => l10n.errorUnauthorized,
    ProjectForbiddenException _ => l10n.projectSettingsActionForbidden,
    ProjectCancelledException _ => l10n.errorRequestCancelled,
    final ProjectApiException e => _projectSettingsReindexApiTitle(l10n, e),
    _ => l10n.projectSettingsReindexGenericError,
  };
}

String _projectSettingsReindexApiTitle(
  AppLocalizations l10n,
  ProjectApiException e,
) {
  final message = e.message;
  if (message == 'Network timeout' || message == 'Network error') {
    return l10n.errorNetwork;
  }
  final code = e.statusCode ?? 0;
  if (code == 502) {
    return l10n.projectSettingsGitRemoteAccessFailed;
  }
  if (code >= 500) {
    return l10n.errorServer;
  }
  if (code == 400) {
    return l10n.projectSettingsReindexValidationError;
  }
  return l10n.projectSettingsReindexGenericError;
}

@riverpod
class ProjectSettingsController extends _$ProjectSettingsController {
  /// Отдельные токены для save и reindex (независимые HTTP; `cancel()` одного не трогает другой).
  /// После ответа **202** на reindex отмена токена не останавливает пайплайн на бэкенде — см. [ProjectRepository.reindex].
  late final CancelToken _saveCancelToken;
  late final CancelToken _reindexCancelToken;

  @override
  FutureOr<void> build(String projectId) {
    _saveCancelToken = CancelToken();
    _reindexCancelToken = CancelToken();
    ref.onDispose(() {
      _saveCancelToken.cancel();
      _reindexCancelToken.cancel();
    });
  }

  /// Возвращает обновлённый проект или `null`, если патч пустой.
  ///
  /// Не вызывает [projectProvider.invalidate] — экран сам инвалидирует после применения ответа,
  /// чтобы не гоняться с [ref.listen] и лишним `setState` до снятия `_saveBusy`.
  Future<ProjectModel?> save(UpdateProjectRequest? patch) async {
    if (patch == null || patch.toJson().isEmpty) {
      return null;
    }
    final repo = ref.read(projectRepositoryProvider);
    return repo.updateProject(
      projectId,
      patch,
      cancelToken: _saveCancelToken,
    );
  }

  Future<void> reindex() async {
    final repo = ref.read(projectRepositoryProvider);
    await repo.reindex(projectId, cancelToken: _reindexCancelToken);
    ref.invalidate(projectProvider(projectId));
  }
}
