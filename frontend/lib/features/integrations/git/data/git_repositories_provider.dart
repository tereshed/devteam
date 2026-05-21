import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:frontend/features/integrations/git/data/git_integrations_providers.dart';
import 'package:frontend/features/integrations/git/domain/git_integration_model.dart';
import 'package:frontend/features/integrations/git/domain/git_repository_model.dart';

/// Provider to load repository lists for a given Git provider.
final gitRepositoriesProvider = FutureProvider.family<List<GitRepositoryModel>, GitIntegrationProvider>((ref, provider) async {
  final repository = ref.watch(gitIntegrationsRepositoryProvider);
  return repository.fetchRepositories(provider);
});
