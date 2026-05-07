import 'package:frontend/core/api/websocket_service.dart';
import 'package:frontend/features/chat/data/conversation_repository.dart';
import 'package:mockito/annotations.dart';

@GenerateNiceMocks([
  MockSpec<ConversationRepository>(),
  MockSpec<WebSocketService>(),
])
void main() {}
