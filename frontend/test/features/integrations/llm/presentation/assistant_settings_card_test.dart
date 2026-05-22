import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/core/api/websocket_events.dart';
import 'package:frontend/core/api/websocket_providers.dart';
import 'package:frontend/core/api/websocket_service.dart';
import 'package:frontend/features/admin/agents_v2/domain/agent_v2_model.dart';
import 'package:frontend/features/assistant/data/assistant_providers.dart';
import 'package:frontend/features/assistant/domain/assistant_status_model.dart';
import 'package:frontend/features/integrations/llm/data/llm_integrations_providers.dart';
import 'package:frontend/features/integrations/llm/data/llm_integrations_repository.dart';
import 'package:frontend/features/integrations/llm/domain/antigravity_status_model.dart';
import 'package:frontend/features/integrations/llm/domain/claude_code_status_model.dart';
import 'package:frontend/features/integrations/llm/domain/llm_provider_model.dart';
import 'package:frontend/features/integrations/llm/presentation/widgets/assistant_settings_card.dart';
import 'package:frontend/features/onboarding/data/my_agents_providers.dart';
import 'package:frontend/features/onboarding/data/my_agents_repository.dart';
import 'package:frontend/l10n/app_localizations.dart';

class _FakeMyAgentsRepository implements MyAgentsRepository {
  _FakeMyAgentsRepository({required this.agents});
  
  AgentV2Page agents;
  bool updateCalled = false;
  String? lastId;
  String? lastModel;
  String? lastProviderKind;

  @override
  Future<AgentV2Page> list({cancelToken}) async {
    return agents;
  }

  @override
  Future<AgentV2> update(
    String id, {
    String? model,
    String? providerKind,
    cancelToken,
  }) async {
    updateCalled = true;
    lastId = id;
    lastModel = model;
    lastProviderKind = providerKind;
    
    final idx = agents.items.indexWhere((a) => a.id == id);
    final updatedAgent = AgentV2(
      id: id,
      name: idx != -1 ? agents.items[idx].name : 'Assistant',
      role: idx != -1 ? agents.items[idx].role : 'assistant',
      roleDescription: idx != -1 ? agents.items[idx].roleDescription : '',
      executionKind: idx != -1 ? agents.items[idx].executionKind : 'llm',
      isActive: idx != -1 ? agents.items[idx].isActive : true,
      internalMcpEnabled: idx != -1 ? agents.items[idx].internalMcpEnabled : false,
      createdAt: DateTime.now(),
      updatedAt: DateTime.now(),
      model: model,
      providerKind: providerKind,
    );
    if (idx != -1) {
      final list = List<AgentV2>.from(agents.items);
      list[idx] = updatedAgent;
      agents = AgentV2Page(
        total: agents.total,
        items: list,
        limit: agents.limit,
        offset: agents.offset,
      );
    }
    return updatedAgent;
  }
}

class _FakeLlmIntegrationsRepository implements LlmIntegrationsRepository {
  _FakeLlmIntegrationsRepository({
    this.apiKey = const <LlmProviderConnection>[],
  });

  List<LlmProviderConnection> apiKey;
  ClaudeCodeIntegrationStatus get claude => const ClaudeCodeIntegrationStatus(connected: false);
  AntigravityIntegrationStatus get antigravity => const AntigravityIntegrationStatus(connected: false);

  @override
  Future<List<LlmProviderConnection>> fetchApiKeyConnections({
    cancelToken,
  }) async {
    return apiKey;
  }

  @override
  Future<ClaudeCodeIntegrationStatus> fetchClaudeCodeStatus({
    cancelToken,
  }) async {
    return claude;
  }

  @override
  Future<AntigravityIntegrationStatus> fetchAntigravityStatus({
    cancelToken,
  }) async {
    return antigravity;
  }

  @override
  dynamic noSuchMethod(Invocation invocation) {
    throw UnimplementedError('${invocation.memberName}');
  }
}

class _FakeWebSocketService extends WebSocketService {
  _FakeWebSocketService()
    : super(
        baseUrl: 'http://127.0.0.1:8080/api/v1',
        channelFactory: (_, {protocols}) =>
            throw UnimplementedError('not used in tests'),
        authProvider: () async => const WsAuth.none(),
      );

  @override
  Stream<WsClientEvent> get events => const Stream.empty();
}

void main() {
  group('AssistantSettingsCard widget tests', () {
    testWidgets('Shows loading indicator initially', (tester) async {
      final myAgentsRepo = _FakeMyAgentsRepository(
        agents: const AgentV2Page(total: 0, items: [], limit: 10, offset: 0),
      );
      final llmRepo = _FakeLlmIntegrationsRepository();
      final ws = _FakeWebSocketService();

      final container = ProviderContainer(
        overrides: [
          myAgentsRepositoryProvider.overrideWithValue(myAgentsRepo),
          llmIntegrationsRepositoryProvider.overrideWithValue(llmRepo),
          webSocketServiceProvider.overrideWithValue(ws),
          assistantStatusProvider.overrideWith(
            (ref) async => const AssistantStatusModel(isConfigured: false, requiredProvider: ''),
          ),
        ],
      );
      addTearDown(container.dispose);

      await tester.pumpWidget(
        UncontrolledProviderScope(
          container: container,
          child: const MaterialApp(
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            home: Scaffold(body: AssistantSettingsCard()),
          ),
        ),
      );

      // Loading is showing because myAgentsListProvider is still loading when list() is pending
      expect(find.byType(CircularProgressIndicator), findsOneWidget);
    });

    testWidgets('Shows config card and allows saving settings when providers are connected', (tester) async {
      final assistantAgent = AgentV2(
        id: 'agent-123',
        name: 'My Assistant',
        role: 'assistant',
        roleDescription: 'Personal assistant',
        executionKind: 'llm',
        isActive: true,
        internalMcpEnabled: false,
        createdAt: DateTime.now(),
        updatedAt: DateTime.now(),
        providerKind: 'deepseek',
        model: 'deepseek-chat',
      );

      final myAgentsRepo = _FakeMyAgentsRepository(
        agents: AgentV2Page(total: 1, items: [assistantAgent], limit: 10, offset: 0),
      );

      // Connect DeepSeek and OpenRouter
      final llmRepo = _FakeLlmIntegrationsRepository(
        apiKey: const [
          LlmProviderConnection(
            provider: LlmIntegrationProvider.deepseek,
            status: LlmProviderConnectionStatus.connected,
          ),
          LlmProviderConnection(
            provider: LlmIntegrationProvider.openrouter,
            status: LlmProviderConnectionStatus.connected,
          ),
        ],
      );
      final ws = _FakeWebSocketService();

      final container = ProviderContainer(
        overrides: [
          myAgentsRepositoryProvider.overrideWithValue(myAgentsRepo),
          llmIntegrationsRepositoryProvider.overrideWithValue(llmRepo),
          webSocketServiceProvider.overrideWithValue(ws),
          assistantStatusProvider.overrideWith(
            (ref) async => const AssistantStatusModel(isConfigured: true, requiredProvider: 'deepseek'),
          ),
        ],
      );
      addTearDown(container.dispose);

      // Pre-refresh integrations controller
      container.read(llmIntegrationsControllerProvider).refresh();

      await tester.pumpWidget(
        UncontrolledProviderScope(
          container: container,
          child: const MaterialApp(
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            home: Scaffold(body: AssistantSettingsCard()),
          ),
        ),
      );

      await tester.pumpAndSettle();

      // Check card header
      expect(find.text('Настройки ассистента'), findsOneWidget);

      // Check current model is in TextFormField
      final modelFieldFinder = find.byType(TextFormField);
      expect(modelFieldFinder, findsOneWidget);
      final modelFieldWidget = tester.widget<TextFormField>(modelFieldFinder);
      expect(modelFieldWidget.controller?.text, 'deepseek-chat');

      // Tap model field to open SearchAnchor suggestions
      await tester.tap(modelFieldFinder);
      await tester.pumpAndSettle();

      // Tapping a suggestion (e.g. deepseek-reasoner) changes model field value
      final suggestionFinder = find.text('deepseek-reasoner');
      expect(suggestionFinder, findsOneWidget);
      await tester.tap(suggestionFinder);
      await tester.pumpAndSettle();

      final updatedModelFieldWidget = tester.widget<TextFormField>(modelFieldFinder);
      expect(updatedModelFieldWidget.controller?.text, 'deepseek-reasoner');

      // Tap Save Settings button
      final saveBtnFinder = find.text('Сохранить настройки');
      expect(saveBtnFinder, findsOneWidget);
      await tester.tap(saveBtnFinder);
      await tester.pumpAndSettle();

      // Check that MyAgentsRepository.update was called with correct parameters
      expect(myAgentsRepo.updateCalled, isTrue);
      expect(myAgentsRepo.lastId, 'agent-123');
      expect(myAgentsRepo.lastModel, 'deepseek-reasoner');
      expect(myAgentsRepo.lastProviderKind, 'deepseek');

      // Success SnackBar is shown
      expect(find.text('Настройки ассистента успешно сохранены'), findsOneWidget);
    });
  });
}
