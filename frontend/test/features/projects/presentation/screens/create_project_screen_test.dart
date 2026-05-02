import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/features/projects/data/project_providers.dart';
import 'package:frontend/features/projects/data/project_repository.dart';
import 'package:frontend/features/projects/presentation/screens/create_project_screen.dart';
import 'package:frontend/l10n/app_localizations.dart';
import 'package:go_router/go_router.dart';
import 'package:mockito/annotations.dart';
import 'package:mockito/mockito.dart';

import 'create_project_screen_test.mocks.dart';

@GenerateNiceMocks([MockSpec<Dio>()])
void main() {
  late MockDio mockDio;

  setUp(() {
    mockDio = MockDio();
  });

  /// MaterialApp + [CreateProjectScreen] + [GoRouter] (en) + репозиторий.
  Future<void> pumpScreen(WidgetTester tester) async {
    await tester.pumpWidget(
      ProviderScope(
        retry: (_, _) => null,
        overrides: [
          projectRepositoryProvider.overrideWithValue(
            ProjectRepository(dio: mockDio),
          ),
        ],
        child: MaterialApp.router(
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          locale: const Locale('en'),
          routerConfig: GoRouter(
            routes: [
              GoRoute(
                path: '/',
                builder: (context, state) => const CreateProjectScreen(),
              ),
              GoRoute(
                path: '/projects',
                builder: (context, state) => const Scaffold(body: Text('list')),
              ),
              GoRoute(
                path: '/projects/:id',
                builder: (context, state) => const SizedBox(),
              ),
            ],
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();
  }

  testWidgets('409: SnackBar shows localized conflict message', (tester) async {
    when(
      mockDio.post(
        '/projects',
        data: anyNamed('data'),
        cancelToken: anyNamed('cancelToken'),
      ),
    ).thenThrow(
      DioException(
        requestOptions: RequestOptions(path: '/projects', method: 'POST'),
        response: Response<Map<String, dynamic>>(
          data: {'message': 'duplicate'},
          statusCode: 409,
          requestOptions: RequestOptions(path: '/projects'),
        ),
        type: DioExceptionType.badResponse,
      ),
    );

    await pumpScreen(tester);

    await tester.enterText(find.byType(TextFormField).first, 'Dup');
    await tester.tap(find.byType(DropdownButtonFormField<String>));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Local').last);
    await tester.pumpAndSettle();
    await tester.tap(find.text('Create'));
    await tester.pump();
    await tester.pump(const Duration(seconds: 1));

    expect(find.byType(SnackBar), findsOneWidget);
    expect(find.text('This name is already in use'), findsOneWidget);
  });

  testWidgets('local provider: empty git URL still calls API with empty git_url',
      (tester) async {
    when(
      mockDio.post(
        '/projects',
        data: argThat(
          predicate<Map<String, dynamic>>((m) => m['git_url'] == ''),
          named: 'data',
        ),
        cancelToken: anyNamed('cancelToken'),
      ),
    ).thenAnswer(
      (_) async => Response<dynamic>(
        data: <String, dynamic>{
          'id': 'local-ok-123e4567-e89b-12d3-a456-426614174000',
          'name': 'L',
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
        },
        statusCode: 201,
        requestOptions: RequestOptions(path: '/projects', method: 'POST'),
      ),
    );

    await pumpScreen(tester);

    await tester.enterText(find.byType(TextFormField).first, 'L');
    await tester.tap(find.byType(DropdownButtonFormField<String>));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Local').last);
    await tester.pumpAndSettle();
    await tester.tap(find.text('Create'));
    await tester.pumpAndSettle();

    verify(
      mockDio.post(
        '/projects',
        data: anyNamed('data'),
        cancelToken: anyNamed('cancelToken'),
      ),
    ).called(1);
  });

  testWidgets('github: empty URL does not call API', (tester) async {
    await pumpScreen(tester);
    await tester.enterText(find.byType(TextFormField).first, 'G');
    await tester.tap(find.text('Create'));
    await tester.pumpAndSettle();

    verifyNever(
      mockDio.post(
        any,
        data: anyNamed('data'),
        cancelToken: anyNamed('cancelToken'),
      ),
    );
  });

  testWidgets('success navigates to /projects/:id', (tester) async {
    const id = '123e4567-e89b-12d3-a456-426614174000';
    when(
      mockDio.post(
        '/projects',
        data: anyNamed('data'),
        cancelToken: anyNamed('cancelToken'),
      ),
    ).thenAnswer(
      (_) async => Response<dynamic>(
        data: <String, dynamic>{
          'id': id,
          'name': 'Ok',
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
        },
        statusCode: 201,
        requestOptions: RequestOptions(path: '/projects', method: 'POST'),
      ),
    );

    await tester.pumpWidget(
      ProviderScope(
        retry: (_, _) => null,
        overrides: [
          projectRepositoryProvider.overrideWithValue(
            ProjectRepository(dio: mockDio),
          ),
        ],
        child: MaterialApp.router(
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          locale: const Locale('en'),
          routerConfig: GoRouter(
            routes: [
              GoRoute(
                path: '/',
                builder: (context, state) => const CreateProjectScreen(),
              ),
              GoRoute(
                path: '/projects/:id',
                builder: (context, state) =>
                    Text('hub-${state.pathParameters['id']}'),
              ),
            ],
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    await tester.enterText(find.byType(TextFormField).first, 'Ok');
    await tester.tap(find.byType(DropdownButtonFormField<String>));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Local').last);
    await tester.pumpAndSettle();
    await tester.tap(find.text('Create'));
    await tester.pumpAndSettle();

    expect(find.text('hub-$id'), findsOneWidget);
  });

  testWidgets('after API error, name field is not cleared', (tester) async {
    when(
      mockDio.post(
        '/projects',
        data: anyNamed('data'),
        cancelToken: anyNamed('cancelToken'),
      ),
    ).thenThrow(
      DioException(
        requestOptions: RequestOptions(path: '/projects', method: 'POST'),
        response: Response<Map<String, dynamic>>(
          data: {'message': 'bad'},
          statusCode: 400,
          requestOptions: RequestOptions(path: '/projects'),
        ),
        type: DioExceptionType.badResponse,
      ),
    );

    await pumpScreen(tester);
    await tester.enterText(find.byType(TextFormField).first, 'KeepName');
    await tester.tap(find.byType(DropdownButtonFormField<String>));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Local').last);
    await tester.pumpAndSettle();
    await tester.tap(find.text('Create'));
    await tester.pumpAndSettle();

    final nameField = tester.widget<TextFormField>(find.byType(TextFormField).first);
    final ctrl = nameField.controller;
    expect(ctrl?.text, 'KeepName');
  });

  testWidgets('double tap Create: only one POST while request in flight', (tester) async {
    when(
      mockDio.post(
        '/projects',
        data: anyNamed('data'),
        cancelToken: anyNamed('cancelToken'),
      ),
    ).thenAnswer((_) async {
      await Future<void>.delayed(const Duration(milliseconds: 120));
      return Response<dynamic>(
        data: <String, dynamic>{
          'id': '123e4567-e89b-12d3-a456-426614174000',
          'name': 'D',
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
        },
        statusCode: 201,
        requestOptions: RequestOptions(path: '/projects', method: 'POST'),
      );
    });

    await pumpScreen(tester);
    await tester.enterText(find.byType(TextFormField).first, 'D');
    await tester.tap(find.byType(DropdownButtonFormField<String>));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Local').last);
    await tester.pumpAndSettle();
    await tester.tap(find.text('Create'));
    await tester.tap(find.text('Create'));
    await tester.pumpAndSettle();

    verify(
      mockDio.post(
        '/projects',
        data: anyNamed('data'),
        cancelToken: anyNamed('cancelToken'),
      ),
    ).called(1);
  });
}
