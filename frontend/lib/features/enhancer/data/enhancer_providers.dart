import 'package:frontend/core/api/dio_providers.dart';
import 'package:frontend/features/enhancer/data/enhancer_repository.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

part 'enhancer_providers.g.dart';

@Riverpod(keepAlive: true)
EnhancerRepository enhancerRepository(Ref ref) {
  final dio = ref.watch(dioClientProvider);
  return EnhancerRepository(dio: dio);
}
