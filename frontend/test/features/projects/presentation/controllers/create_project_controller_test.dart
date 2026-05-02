import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/features/projects/data/project_providers.dart';
import 'package:frontend/features/projects/data/project_repository.dart';
import 'package:frontend/features/projects/domain/models.dart';
import 'package:frontend/features/projects/domain/requests.dart';
import 'package:frontend/features/projects/presentation/controllers/create_project_controller.dart';
import 'package:mockito/annotations.dart';
import 'package:mockito/mockito.dart';

import 'create_project_controller_test.mocks.dart';

@GenerateNiceMocks([MockSpec<Dio>()])
void main() {
  late MockDio mockDio;
  late ProviderContainer container;

  Map<String, dynamic> createdProjectJson() {
    return <String, dynamic>{
      'id': '123e4567-e89b-12d3-a456-426614174000',
      'name': 'New Project',
      'description': '',
      'git_provider': 'local',
      'git_url': '',
      'git_default_branch': 'main',
      'git_credential': null,
      'vector_collection': '',
      'tech_stack': <String, dynamic>{},
      'status': 'active',
      'settings': <String, dynamic>{},
      'created_at': '2026-04-28T10:00:00Z',
      'updated_at': '2026-04-28T10:00:00Z',
    };
  }

  setUp(() {
    mockDio = MockDio();
    container = ProviderContainer(
      overrides: [
        projectRepositoryProvider.overrideWithValue(
          ProjectRepository(dio: mockDio),
        ),
      ],
    );
    container.listen(createProjectControllerProvider, (_, _) {});
    addTearDown(container.dispose);
  });

  group('CreateProjectController', () {
    test('parallel submit calls POST only once', () async {
      when(
        mockDio.post(
          '/projects',
          data: anyNamed('data'),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenAnswer((invocation) async {
        await Future<void>.delayed(const Duration(milliseconds: 40));
        return Response<dynamic>(
          data: Map<String, dynamic>.from(createdProjectJson()),
          statusCode: 201,
          requestOptions: RequestOptions(path: '/projects', method: 'POST'),
        );
      });

      const req = CreateProjectRequest(
        name: 'N',
        gitProvider: kLocalGitProvider,
        gitUrl: '',
        vectorCollection: '',
      );

      final notifier = container.read(createProjectControllerProvider.notifier);
      final a = notifier.submit(req);
      final b = notifier.submit(req);
      await Future.wait<void>([a, b]);

      verify(
        mockDio.post(
          '/projects',
          data: anyNamed('data'),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).called(1);
    });
  });
}
