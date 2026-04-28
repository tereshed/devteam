import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';
import 'package:frontend/features/auth/data/auth_providers.dart';
import 'package:frontend/features/projects/data/project_repository.dart';

part 'project_providers.g.dart';

/// Провайдер для ProjectRepository — используется в задаче 10.3
@Riverpod(keepAlive: true)
ProjectRepository projectRepository(Ref ref) {
  final dio = ref.watch(dioClientProvider);
  return ProjectRepository(dio: dio);
}
