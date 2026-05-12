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

/// Текущий статус подписки. `keepAlive: false` — авто-сброс при размонтировании секции.
@riverpod
Future<ClaudeCodeAuthStatus> claudeCodeAuthStatus(Ref ref) async {
  final repo = ref.watch(claudeCodeAuthRepositoryProvider);
  return repo.status();
}
