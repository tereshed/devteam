import 'dart:async';

import 'package:dio/dio.dart';
import 'package:frontend/core/api/api_exceptions.dart';
import 'package:frontend/features/projects/data/project_providers.dart';
import 'package:frontend/features/projects/domain/models/project_model.dart';
import 'package:frontend/features/projects/domain/project_exceptions.dart';
import 'package:frontend/features/projects/domain/requests.dart';
import 'package:frontend/l10n/app_localizations.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

part 'create_project_controller.g.dart';

/// Локализованный заголовок ошибки для SnackBar (без повторной санитизации текста).
String createProjectErrorTitle(AppLocalizations l10n, Object error) {
  return switch (error) {
    ProjectConflictException _ => l10n.createProjectErrorConflict,
    UnauthorizedException _ => l10n.errorUnauthorized,
    ProjectForbiddenException _ => l10n.errorForbidden,
    ProjectCancelledException _ => l10n.errorRequestCancelled,
    final ProjectApiException e => _createProjectApiErrorTitle(l10n, e),
    _ => l10n.createProjectErrorGeneric,
  };
}

String _createProjectApiErrorTitle(
  AppLocalizations l10n,
  ProjectApiException e,
) {
  final message = e.message;
  if (message == 'Network timeout' || message == 'Network error') {
    return l10n.errorNetwork;
  }
  if ((e.statusCode ?? 0) >= 500) {
    return l10n.errorServer;
  }
  return l10n.createProjectErrorGeneric;
}

/// Короткий безопасный хвост из [ProjectApiException.message] (уже санитизирован в репозитории).
String? createProjectErrorDetail(Object error) {
  if (error is ProjectCancelledException) {
    return null;
  }
  if (error is! ProjectApiException) {
    return null;
  }
  final m = error.message;
  if (m.isEmpty) {
    return null;
  }
  if (m == 'Network timeout' || m == 'Network error') {
    return null;
  }
  const maxLen = 200;
  if (m.length <= maxLen) {
    return m;
  }
  return '${m.substring(0, maxLen)}…';
}

@riverpod
class CreateProjectController extends _$CreateProjectController {
  bool _inFlight = false;
  late final CancelToken _cancelToken;

  @override
  FutureOr<ProjectModel?> build() {
    _cancelToken = CancelToken();
    ref.onDispose(() => _cancelToken.cancel());
    return null;
  }

  /// Возвращает созданный проект при успехе; при повторном вызове во время запроса — `null`.
  Future<ProjectModel?> submit(CreateProjectRequest request) async {
    if (_inFlight || state.isLoading) {
      return null;
    }
    _inFlight = true;
    state = const AsyncLoading();
    try {
      final repo = ref.read(projectRepositoryProvider);
      final project = await repo.createProject(
        request,
        cancelToken: _cancelToken,
      );
      ref.invalidate(projectListProvider);
      state = AsyncData(project);
      return project;
    } catch (e, st) {
      state = AsyncError(e, st);
      return null;
    } finally {
      _inFlight = false;
    }
  }
}
