import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/features/team/data/agent_config_providers.dart';
import 'package:frontend/features/team/data/project_secret_repository.dart';
import 'package:frontend/features/team/data/user_secret_repository.dart';
import 'package:frontend/features/team/domain/agent_config_exceptions.dart';
import 'package:frontend/features/team/domain/models/agent_config_model.dart';
import 'package:mocktail/mocktail.dart';

class MockProjectSecretRepository extends Mock
    implements ProjectSecretRepository {}

class MockUserSecretRepository extends Mock implements UserSecretRepository {}

class FakeCancelToken extends Fake implements CancelToken {}

void main() {
  setUpAll(() {
    registerFallbackValue(FakeCancelToken());
  });

  late MockProjectSecretRepository projectRepo;
  late MockUserSecretRepository userRepo;

  setUp(() {
    projectRepo = MockProjectSecretRepository();
    userRepo = MockUserSecretRepository();
  });

  ProviderContainer makeContainer() {
    final container = ProviderContainer(
      retry: (_, _) => null,
      overrides: [
        projectSecretRepositoryProvider.overrideWithValue(projectRepo),
        userSecretRepositoryProvider.overrideWithValue(userRepo),
      ],
    );
    addTearDown(container.dispose);
    return container;
  }

  group('projectSecretsProvider', () {
    test('loads secrets via repository', () async {
      final secrets = [
        const SecretRefModel(id: 's1', keyName: 'API_KEY'),
        const SecretRefModel(id: 's2', keyName: 'DB_PASS'),
      ];
      when(() => projectRepo.list(
            'proj-1',
            cancelToken: any(named: 'cancelToken'),
          )).thenAnswer((_) async => secrets);

      final container = makeContainer();
      final result =
          await container.read(projectSecretsProvider('proj-1').future);

      expect(result, secrets);
      expect(result.length, 2);
      expect(result.first.keyName, 'API_KEY');
    });

    test('caches separate entries per projectId', () async {
      when(() => projectRepo.list(
            any(),
            cancelToken: any(named: 'cancelToken'),
          )).thenAnswer((_) async => const []);

      final container = makeContainer();
      await container.read(projectSecretsProvider('p1').future);
      await container.read(projectSecretsProvider('p2').future);

      verify(() =>
              projectRepo.list('p1', cancelToken: any(named: 'cancelToken')))
          .called(1);
      verify(() =>
              projectRepo.list('p2', cancelToken: any(named: 'cancelToken')))
          .called(1);
    });

    test('cancels request on container dispose', () async {
      CancelToken? captured;
      when(() => projectRepo.list(
            any(),
            cancelToken: any(named: 'cancelToken'),
          )).thenAnswer((inv) async {
        captured = inv.namedArguments[#cancelToken] as CancelToken?;
        return const [];
      });

      final container = makeContainer();
      await container.read(projectSecretsProvider('proj-1').future);
      container.dispose();

      expect(captured?.isCancelled, isTrue);
    });

    test('propagates repository errors', () async {
      when(() => projectRepo.list(
            any(),
            cancelToken: any(named: 'cancelToken'),
          )).thenAnswer(
        (_) => Future.error(
            AgentConfigApiException('server error', statusCode: 500)),
      );

      final container = makeContainer();

      await expectLater(
        container.read(projectSecretsProvider('proj-1').future),
        throwsA(isA<AgentConfigApiException>()),
      );
    });
  });

  group('userSecretsProvider', () {
    test('loads secrets via repository', () async {
      final secrets = [
        const SecretRefModel(id: 'u1', keyName: 'MY_TOKEN'),
      ];
      when(() => userRepo.list(cancelToken: any(named: 'cancelToken')))
          .thenAnswer((_) async => secrets);

      final container = makeContainer();
      final result = await container.read(userSecretsProvider.future);

      expect(result, secrets);
      expect(result.length, 1);
      expect(result.first.keyName, 'MY_TOKEN');
    });

    test('returns empty list when no secrets', () async {
      when(() => userRepo.list(cancelToken: any(named: 'cancelToken')))
          .thenAnswer((_) async => const []);

      final container = makeContainer();
      final result = await container.read(userSecretsProvider.future);

      expect(result, isEmpty);
    });

    test('cancels request on container dispose', () async {
      CancelToken? captured;
      when(() => userRepo.list(cancelToken: any(named: 'cancelToken')))
          .thenAnswer((inv) async {
        captured = inv.namedArguments[#cancelToken] as CancelToken?;
        return const [];
      });

      final container = makeContainer();
      await container.read(userSecretsProvider.future);
      container.dispose();

      expect(captured?.isCancelled, isTrue);
    });

    test('propagates repository errors', () async {
      when(() => userRepo.list(cancelToken: any(named: 'cancelToken')))
          .thenAnswer(
        (_) => Future.error(
            AgentConfigApiException('forbidden', statusCode: 403)),
      );

      final container = makeContainer();

      await expectLater(
        container.read(userSecretsProvider.future),
        throwsA(isA<AgentConfigApiException>()),
      );
    });
  });
}
