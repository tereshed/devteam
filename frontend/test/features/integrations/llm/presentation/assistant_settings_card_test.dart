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
  Map<String, dynamic>? lastSettings;

  @override
  Future<AgentV2Page> list({cancelToken}) async {
    return agents;
  }

  @override
  Future<AgentV2> update(
    String id, {
    String? model,
    String? providerKind,
    Map<String, dynamic>? settings,
    cancelToken,
  }) async {
    updateCalled = true;
    lastId = id;
    lastModel = model;
    lastProviderKind = providerKind;
    lastSettings = settings;
    
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
      settings: settings ?? const {},
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
          child: MaterialApp(
            theme: ThemeData(
              splashFactory: NoSplash.splashFactory,
            ),
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            home: const Scaffold(body: AssistantSettingsCard()),
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
          child: MaterialApp(
            theme: ThemeData(
              splashFactory: NoSplash.splashFactory,
            ),
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            home: const Scaffold(body: AssistantSettingsCard()),
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

    testWidgets('Allows selecting speech provider and model and saves in settings', (tester) async {
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
        settings: const {
          'stt_provider': 'system',
        },
      );

      final myAgentsRepo = _FakeMyAgentsRepository(
        agents: AgentV2Page(total: 1, items: [assistantAgent], limit: 10, offset: 0),
      );

      final llmRepo = _FakeLlmIntegrationsRepository(
        apiKey: const [
          LlmProviderConnection(
            provider: LlmIntegrationProvider.openai,
            status: LlmProviderConnectionStatus.connected,
          ),
          LlmProviderConnection(
            provider: LlmIntegrationProvider.openrouter,
            status: LlmProviderConnectionStatus.connected,
          ),
          LlmProviderConnection(
            provider: LlmIntegrationProvider.deepseek,
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

      container.read(llmIntegrationsControllerProvider).refresh();

      await tester.pumpWidget(
        UncontrolledProviderScope(
          container: container,
          child: MaterialApp(
            theme: ThemeData(
              splashFactory: NoSplash.splashFactory,
            ),
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            home: const Scaffold(body: AssistantSettingsCard()),
          ),
        ),
      );

      await tester.pumpAndSettle();

      // Find the speech provider dropdown
      final providerDropdownFinder = find.byWidgetPredicate(
        (widget) => widget is DropdownButtonFormField<String> && 
                    widget.decoration.labelText == 'Провайдер распознавания речи',
      );
      expect(providerDropdownFinder, findsOneWidget);

      // Verify initial selected provider is 'system'
      final providerDropdownState = tester.state<FormFieldState<String>>(providerDropdownFinder);
      expect(providerDropdownState.value, 'system');

      // Change provider to 'openai'
      await tester.tap(providerDropdownFinder);
      await tester.pumpAndSettle();

      final openaiOptionFinder = find.text('OpenAI (API)').last;
      await tester.tap(openaiOptionFinder);
      await tester.pumpAndSettle();

      // Find the speech model field (should now be visible)
      final speechModelFieldFinder = find.byType(TextFormField).last;
      expect(speechModelFieldFinder, findsOneWidget);

      final speechModelFieldWidget = tester.widget<TextFormField>(speechModelFieldFinder);
      // Verify initial suggested/default value is 'whisper-1'
      expect(speechModelFieldWidget.controller?.text, 'whisper-1');

      // Save settings
      final saveBtnFinder = find.text('Сохранить настройки');
      await tester.tap(saveBtnFinder);
      await tester.pumpAndSettle();

      expect(myAgentsRepo.updateCalled, isTrue);
      expect(myAgentsRepo.lastSettings, isNotNull);
      expect(myAgentsRepo.lastSettings!['stt_provider'], 'openai');
      expect(myAgentsRepo.lastSettings!['stt_model'], 'whisper-1');
    });

    testWidgets('Allows selecting and saving a disconnected provider', (tester) async {
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

      // DeepSeek is connected, but OpenRouter is disconnected (not in apiKey connections)
      final llmRepo = _FakeLlmIntegrationsRepository(
        apiKey: const [
          LlmProviderConnection(
            provider: LlmIntegrationProvider.deepseek,
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

      container.read(llmIntegrationsControllerProvider).refresh();

      await tester.pumpWidget(
        UncontrolledProviderScope(
          container: container,
          child: MaterialApp(
            theme: ThemeData(
              splashFactory: NoSplash.splashFactory,
            ),
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            home: const Scaffold(body: AssistantSettingsCard()),
          ),
        ),
      );

      await tester.pumpAndSettle();

      // Find the LLM Provider dropdown
      final providerDropdownFinder = find.byWidgetPredicate(
        (widget) => widget is DropdownButtonFormField<LlmIntegrationProvider> && 
                    widget.decoration.labelText == 'LLM Провайдер',
      );
      expect(providerDropdownFinder, findsOneWidget);

      // Tap to open LLM Provider dropdown
      await tester.tap(providerDropdownFinder);
      await tester.pumpAndSettle();

      // Select OpenRouter which is NOT connected (so it should display "OpenRouter (Не подключен)")
      final openRouterOptionFinder = find.text('OpenRouter (Не подключен)').last;
      await tester.tap(openRouterOptionFinder);
      await tester.pumpAndSettle();

      // Verify selected provider updated to openrouter
      final providerDropdownState = tester.state<FormFieldState<LlmIntegrationProvider>>(providerDropdownFinder);
      expect(providerDropdownState.value, LlmIntegrationProvider.openrouter);

      // Verify that model search controller text changed to one of the default suggestions for openrouter (e.g. 'deepseek/deepseek-v4-pro')
      final modelFieldFinder = find.byType(TextFormField).first;
      final modelFieldWidget = tester.widget<TextFormField>(modelFieldFinder);
      expect(modelFieldWidget.controller?.text, 'deepseek/deepseek-v4-pro');

      // Save settings
      final saveBtnFinder = find.text('Сохранить настройки');
      await tester.tap(saveBtnFinder);
      await tester.pumpAndSettle();

      // Verify myAgentsRepo update was called with openrouter details
      expect(myAgentsRepo.updateCalled, isTrue);
      expect(myAgentsRepo.lastProviderKind, 'openrouter');
      expect(myAgentsRepo.lastModel, 'deepseek/deepseek-v4-pro');
    });
  });
}
