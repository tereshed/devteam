import 'package:frontend/core/api/dio_providers.dart';
import 'package:frontend/features/chat/data/conversation_repository.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

part 'chat_providers.g.dart';

@Riverpod(keepAlive: true)
ConversationRepository conversationRepository(Ref ref) {
  final dio = ref.watch(dioClientProvider);
  return ConversationRepository(dio: dio);
}
