import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/features/projects/data/project_providers.dart';
import 'package:frontend/features/projects/data/project_repository.dart';
import 'package:frontend/features/projects/domain/models.dart';
import 'package:frontend/core/api/api_exceptions.dart';
import 'package:frontend/features/projects/domain/project_exceptions.dart';
import 'package:frontend/features/projects/domain/requests.dart';
import 'package:mocktail/mocktail.dart';

class MockProjectRepository extends Mock implements ProjectRepository {}

class FakeProjectListFilter extends Fake implements ProjectListFilter {}

class FakeCancelToken extends Fake implements CancelToken {}

void main() {
  setUpAll(() {
    registerFallbackValue(FakeProjectListFilter());
    registerFallbackValue(FakeCancelToken());
  });

  late MockProjectRepository repo;

  ProviderContainer makeContainer() {
    final container = ProviderContainer(
      overrides: [
        projectRepositoryProvider.overrideWithValue(repo),
      ],
      // Disable auto-retry so errors propagate immediately in unit tests.
      // ProviderContainer.defaultRetry retries Exception-based errors up to
      // 10 times with exponential backoff, which would cause tests to timeout.
      retry: (_, _) => null,
    );
    addTearDown(container.dispose);
    return container;
  }

  ProjectModel makeProject({String id = 'abc-123'}) {
    return ProjectModel(
      id: id,
      name: 'Test Project',
      description: 'A test project',
      gitProvider: 'github',
      gitUrl: 'https://github.com/user/repo.git',
      gitDefaultBranch: 'main',
      vectorCollection: 'test_collection',
      status: 'active',
      createdAt: DateTime.utc(2026, 4, 28),
      updatedAt: DateTime.utc(2026, 4, 28),
    );
  }

  setUp(() {
    repo = MockProjectRepository();
  });

  group('projectListProvider', () {
    test('loads projects via repository', () async {
      const response = ProjectListResponse(
        projects: [],
        total: 0,
        limit: 20,
        offset: 0,
      );
      when(() => repo.listProjects(
            filter: any(named: 'filter'),
            limit: any(named: 'limit'),
            offset: any(named: 'offset'),
            cancelToken: any(named: 'cancelToken'),
          )).thenAnswer((_) async => response);

      final container = makeContainer();
      final result = await container.read(projectListProvider().future);

      expect(result, response);
      verify(() => repo.listProjects(
            filter: null,
            limit: 20,
            offset: 0,
            cancelToken: any(named: 'cancelToken'),
          )).called(1);
    });

    test('passes filter and pagination to repository', () async {
      const filter = ProjectListFilter(status: 'active');
      const response = ProjectListResponse(projects: [], total: 0, limit: 10, offset: 5);
      when(() => repo.listProjects(
            filter: any(named: 'filter'),
            limit: any(named: 'limit'),
            offset: any(named: 'offset'),
            cancelToken: any(named: 'cancelToken'),
          )).thenAnswer((_) async => response);

      final container = makeContainer();
      await container.read(
        projectListProvider(filter: filter, limit: 10, offset: 5).future,
      );

      verify(() => repo.listProjects(
            filter: filter,
            limit: 10,
            offset: 5,
            cancelToken: any(named: 'cancelToken'),
          )).called(1);
    });

    test('different params create separate cache entries', () async {
      when(() => repo.listProjects(
            filter: any(named: 'filter'),
            limit: any(named: 'limit'),
            offset: any(named: 'offset'),
            cancelToken: any(named: 'cancelToken'),
          )).thenAnswer((_) async => const ProjectListResponse(
            projects: [],
            total: 0,
            limit: 20,
            offset: 0,
          ));

      final container = makeContainer();
      await container.read(projectListProvider(offset: 0).future);
      await container.read(projectListProvider(offset: 20).future);

      verify(() => repo.listProjects(
            filter: null,
            limit: 20,
            offset: 0,
            cancelToken: any(named: 'cancelToken'),
          )).called(1);
      verify(() => repo.listProjects(
            filter: null,
            limit: 20,
            offset: 20,
            cancelToken: any(named: 'cancelToken'),
          )).called(1);
    });

    test('cancels request on container dispose', () async {
      CancelToken? captured;
      when(() => repo.listProjects(
            filter: any(named: 'filter'),
            limit: any(named: 'limit'),
            offset: any(named: 'offset'),
            cancelToken: any(named: 'cancelToken'),
          )).thenAnswer((invocation) async {
        captured = invocation.namedArguments[#cancelToken] as CancelToken?;
        return const ProjectListResponse(projects: [], total: 0, limit: 20, offset: 0);
      });

      final container = makeContainer();
      await container.read(projectListProvider().future);
      container.dispose();

      expect(captured?.isCancelled, isTrue);
    });

    test('propagates UnauthorizedException as AsyncError', () async {
      when(() => repo.listProjects(
            filter: any(named: 'filter'),
            limit: any(named: 'limit'),
            offset: any(named: 'offset'),
            cancelToken: any(named: 'cancelToken'),
          )).thenAnswer((_) => Future.error(UnauthorizedException('token expired')));

      final container = makeContainer();

      await expectLater(
        container.read(projectListProvider().future),
        throwsA(isA<UnauthorizedException>()),
      );
    });

    test('propagates ProjectApiException as AsyncError', () async {
      when(() => repo.listProjects(
            filter: any(named: 'filter'),
            limit: any(named: 'limit'),
            offset: any(named: 'offset'),
            cancelToken: any(named: 'cancelToken'),
          )).thenAnswer((_) => Future.error(ProjectApiException('server error', statusCode: 500)));

      final container = makeContainer();

      await expectLater(
        container.read(projectListProvider().future),
        throwsA(isA<ProjectApiException>()),
      );
    });
  });

  group('projectProvider', () {
    test('loads project by id', () async {
      final project = makeProject();
      when(() => repo.getProject(
            'abc-123',
            cancelToken: any(named: 'cancelToken'),
          )).thenAnswer((_) async => project);

      final container = makeContainer();
      final result = await container.read(projectProvider('abc-123').future);

      expect(result, project);
    });

    test('cancels request on container dispose', () async {
      CancelToken? captured;
      when(() => repo.getProject(
            any(),
            cancelToken: any(named: 'cancelToken'),
          )).thenAnswer((invocation) async {
        captured = invocation.namedArguments[#cancelToken] as CancelToken?;
        return makeProject();
      });

      final container = makeContainer();
      await container.read(projectProvider('abc-123').future);
      container.dispose();

      expect(captured?.isCancelled, isTrue);
    });

    test('propagates ProjectNotFoundException as AsyncError', () async {
      when(() => repo.getProject(
            any(),
            cancelToken: any(named: 'cancelToken'),
          )).thenAnswer((_) => Future.error(ProjectNotFoundException('not found')));

      final container = makeContainer();

      await expectLater(
        container.read(projectProvider('xxx').future),
        throwsA(isA<ProjectNotFoundException>()),
      );
    });

    test('propagates ProjectForbiddenException as AsyncError', () async {
      when(() => repo.getProject(
            any(),
            cancelToken: any(named: 'cancelToken'),
          )).thenAnswer((_) => Future.error(ProjectForbiddenException('forbidden')));

      final container = makeContainer();

      await expectLater(
        container.read(projectProvider('xxx').future),
        throwsA(isA<ProjectForbiddenException>()),
      );
    });

    test('propagates ArgumentError for empty id as AsyncError', () async {
      when(() => repo.getProject(
            '',
            cancelToken: any(named: 'cancelToken'),
          )).thenThrow(ArgumentError('id is required'));

      final container = makeContainer();

      await expectLater(
        container.read(projectProvider('').future),
        throwsA(isA<ArgumentError>()),
      );
    });

    test('caches separate entries per id', () async {
      when(() => repo.getProject(
            any(),
            cancelToken: any(named: 'cancelToken'),
          )).thenAnswer((invocation) async {
        final id = invocation.positionalArguments.first as String;
        return makeProject(id: id);
      });

      final container = makeContainer();
      final a = await container.read(projectProvider('id-1').future);
      final b = await container.read(projectProvider('id-2').future);

      expect(a.id, 'id-1');
      expect(b.id, 'id-2');
      verify(() => repo.getProject('id-1', cancelToken: any(named: 'cancelToken'))).called(1);
      verify(() => repo.getProject('id-2', cancelToken: any(named: 'cancelToken'))).called(1);
    });
  });
}
