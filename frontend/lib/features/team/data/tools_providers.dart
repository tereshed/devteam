import 'package:frontend/core/api/dio_providers.dart';
import 'package:frontend/features/team/data/tools_repository.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

part 'tools_providers.g.dart';

@Riverpod(keepAlive: true)
ToolsRepository toolsRepository(Ref ref) {
  final dio = ref.watch(dioClientProvider);
  return ToolsRepository(dio: dio);
}
