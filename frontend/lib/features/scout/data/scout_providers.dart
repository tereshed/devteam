import 'package:frontend/core/api/dio_providers.dart';
import 'package:frontend/features/scout/data/scout_repository.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

part 'scout_providers.g.dart';

@Riverpod(keepAlive: true)
ScoutRepository scoutRepository(Ref ref) {
  final dio = ref.watch(dioClientProvider);
  return ScoutRepository(dio: dio);
}
