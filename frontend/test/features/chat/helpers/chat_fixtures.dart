import 'package:frontend/features/chat/domain/models.dart';
import 'package:frontend/features/chat/domain/requests.dart';
import 'package:frontend/features/projects/domain/models.dart';

/// Совпадает с [kTestProjectUuid] из projects helpers и литералами из прежних тестов чата.
const String kTestChatProjectUuid = '550e8400-e29b-41d4-a716-446655440000';

const String kTestChatConversationUuid = '6ba7b810-9dad-11d1-80b4-00c04fd430c8';

/// Тело сообщения ассистента в смоках / золотых ожиданиях.
const String kChatFixtureAssistantHelloWorld = 'Hello world';

/// Заголовок беседы из «API» в смоках.
const String kChatFixtureConversationTitle = 'Chat title';

/// Текст после успешного retry отправки.
const String kChatFixtureUserTypedText = 'typed text';

/// Короткий текст для семантики пользователя в чате (п.10 приёмки).
const String kChatFixtureSemanticHelloUser = 'Hello';

/// Короткий текст для семантики ассистента (п.10 приёмки).
const String kChatFixtureSemanticHelloAssistant = 'Hi there';

/// Ответ в ленте для deep-link смока.
const String kChatFixtureDeepLinkAssistantBody = 'route ok';

/// Старый chunk при loadOlder (whitelist латиницы).
const String kChatFixtureOlderChunkBody = 'older chunk';

/// Тело сообщения после сценария invalidate в widget-тесте.
const String kChatFixtureAfterInvalidateReloadBody = 'after reload';

ConversationModel makeConversation({
  String id = kTestChatConversationUuid,
  String projectId = kTestChatProjectUuid,
  String title = kChatFixtureConversationTitle,
  String status = 'active',
  DateTime? createdAt,
  DateTime? updatedAt,
}) =>
    ConversationModel(
      id: id,
      projectId: projectId,
      title: title,
      status: status,
      createdAt: createdAt ?? DateTime.utc(2026, 1, 1),
      updatedAt: updatedAt ?? DateTime.utc(2026, 1, 2),
    );

ConversationMessageModel makeMessage({
  String id = 'm1',
  String conversationId = kTestChatConversationUuid,
  required String role,
  required String content,
  DateTime? createdAt,
  List<String> linkedTaskIds = const [],
  Map<String, dynamic>? metadata,
}) =>
    ConversationMessageModel(
      id: id,
      conversationId: conversationId,
      role: role,
      content: content,
      linkedTaskIds: linkedTaskIds,
      metadata: metadata,
      createdAt: createdAt ?? DateTime.utc(2026, 1, 3),
    );

MessageListResponse makeMessageListResponse({
  List<ConversationMessageModel> messages = const [],
  bool hasNext = false,
}) =>
    MessageListResponse(
      messages: messages,
      hasNext: hasNext,
    );

/// Минимальный [ProjectModel] для обхода HTTP в widget-тестах с [GoRouter].
ProjectModel makeTestChatProject() => ProjectModel(
      id: kTestChatProjectUuid,
      name: 'Chat test project',
      description: 'widget test',
      gitProvider: 'local',
      gitUrl: 'https://example.com/repo.git',
      gitDefaultBranch: 'main',
      vectorCollection: 'col',
      status: 'ready',
      createdAt: DateTime.utc(2026, 1, 1),
      updatedAt: DateTime.utc(2026, 1, 2),
    );

/// Строка «стены» для прокрутки / drag-тестов (интерполяции только здесь, не в expect).
String kChatFixtureWallLine(int index, {int repeatWidth = 80}) =>
    'Line $index ${'x' * repeatWidth}';

