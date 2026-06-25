import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:frontend/features/integrations/git/data/git_integrations_providers.dart';
import 'package:frontend/features/integrations/git/domain/git_integration_model.dart';
import 'package:frontend/features/integrations/git/domain/git_repository_model.dart';

/// Ключ семейства списка репозиториев: провайдер + выбранный аккаунт.
///
/// Мульти-аккаунт: для двух аккаунтов одного провайдера ключ ДОЛЖЕН различаться,
/// иначе Riverpod вернёт закэшированный список первого аккаунта (баг, который чиним).
/// Dart-record даёт структурное равенство «из коробки» → корректное кэширование per-account.
typedef GitRepositoriesArgs = ({GitIntegrationProvider provider, String? accountId});

/// Provider to load repository lists for a given Git provider + account.
final gitRepositoriesProvider =
    FutureProvider.family<List<GitRepositoryModel>, GitRepositoriesArgs>((ref, args) async {
  final repository = ref.watch(gitIntegrationsRepositoryProvider);
  return repository.fetchRepositories(args.provider, accountId: args.accountId);
});
