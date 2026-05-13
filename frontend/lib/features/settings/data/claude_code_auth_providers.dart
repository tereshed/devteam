import 'package:dio/dio.dart';
import 'package:frontend/core/api/dio_providers.dart';
import 'package:frontend/features/settings/data/claude_code_auth_repository.dart';
import 'package:frontend/features/settings/domain/models/claude_code_auth_status.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

part 'claude_code_auth_providers.g.dart';

@Riverpod(keepAlive: true)
ClaudeCodeAuthRepository claudeCodeAuthRepository(Ref ref) {
  final dio = ref.watch(dioClientProvider);
  return ClaudeCodeAuthRepository(dio: dio);
}

/// Sprint 15.M6 — CancelToken+ref.onDispose: текущий статус подписки.
@riverpod
Future<ClaudeCodeAuthStatus> claudeCodeAuthStatus(Ref ref) async {
  final repo = ref.watch(claudeCodeAuthRepositoryProvider);
  final cancelToken = CancelToken();
  ref.onDispose(() =>
      cancelToken.cancel('claudeCodeAuthStatus provider disposed'));
  return repo.status(cancelToken: cancelToken);
}
