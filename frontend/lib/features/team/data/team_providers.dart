import 'package:dio/dio.dart';
import 'package:frontend/core/api/dio_providers.dart';
import 'package:frontend/features/projects/domain/models.dart';
import 'package:frontend/features/team/data/team_repository.dart';
import 'package:frontend/features/team/domain/models/team_type_model.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

part 'team_providers.g.dart';

@Riverpod(keepAlive: true)
TeamRepository teamRepository(Ref ref) {
  final dio = ref.watch(dioClientProvider);
  return TeamRepository(dio: dio);
}

@riverpod
Future<TeamModel> team(Ref ref, String projectId) {
  final cancelToken = CancelToken();
  ref.onDispose(cancelToken.cancel);
  return ref.watch(teamRepositoryProvider).getTeam(
        projectId,
        cancelToken: cancelToken,
      );
}

@riverpod
Future<List<TeamModel>> teams(Ref ref, String projectId) {
  final cancelToken = CancelToken();
  ref.onDispose(cancelToken.cancel);
  return ref.watch(teamRepositoryProvider).getTeams(
        projectId,
        cancelToken: cancelToken,
      );
}

@riverpod
Future<List<TeamTypeModel>> teamTypes(Ref ref) {
  final cancelToken = CancelToken();
  ref.onDispose(cancelToken.cancel);
  return ref.watch(teamRepositoryProvider).getTeamTypes(
        cancelToken: cancelToken,
      );
}

