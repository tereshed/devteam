// @dart=2.19
// @TestOn('vm')
// @Tags(['unit'])

import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/features/auth/data/api_key_repository.dart';
import 'package:frontend/features/auth/domain/auth_exceptions.dart';
import 'package:frontend/features/auth/domain/models.dart';
import 'package:mocktail/mocktail.dart';

class MockDio extends Mock implements Dio {}

void main() {
  group('ApiKeyRepository', () {
    late MockDio mockDio;
    late ApiKeyRepository repository;

    setUp(() {
      mockDio = MockDio();
      repository = ApiKeyRepository(dio: mockDio);
    });

    group('listKeys', () {
      test('должен успешно получить список ключей', () async {
        // Arrange
        final responseData = [
          {
            'id': 'key-1',
            'name': 'Test Key 1',
            'key_prefix': 'wibe_abc',
            'scopes': '*',
            'expires_at': null,
            'last_used_at': null,
            'created_at': '2025-01-01T00:00:00Z',
          },
          {
            'id': 'key-2',
            'name': 'Test Key 2',
            'key_prefix': 'wibe_def',
            'scopes': 'read',
            'expires_at': '2025-12-31T23:59:59Z',
            'last_used_at': '2025-06-15T12:00:00Z',
            'created_at': '2025-01-15T00:00:00Z',
          },
        ];
        final response = Response(
          data: responseData,
          statusCode: 200,
          requestOptions: RequestOptions(path: '/auth/api-keys'),
        );

        when(() => mockDio.get('/auth/api-keys'))
            .thenAnswer((_) async => response);

        // Act
        final result = await repository.listKeys();

        // Assert
        expect(result, isA<List<ApiKeyModel>>());
        expect(result.length, equals(2));
        expect(result[0].name, equals('Test Key 1'));
        expect(result[0].keyPrefix, equals('wibe_abc'));
        expect(result[0].scopes, equals('*'));
        expect(result[1].name, equals('Test Key 2'));
        expect(result[1].scopes, equals('read'));
        verify(() => mockDio.get('/auth/api-keys')).called(1);
      });

      test('должен вернуть пустой список когда ключей нет', () async {
        // Arrange
        final response = Response(
          data: <dynamic>[],
          statusCode: 200,
          requestOptions: RequestOptions(path: '/auth/api-keys'),
        );

        when(() => mockDio.get('/auth/api-keys'))
            .thenAnswer((_) async => response);

        // Act
        final result = await repository.listKeys();

        // Assert
        expect(result, isA<List<ApiKeyModel>>());
        expect(result, isEmpty);
      });

      test('должен выбросить исключение при ошибке авторизации', () async {
        // Arrange
        final dioException = DioException(
          requestOptions: RequestOptions(path: '/auth/api-keys'),
          response: Response(
            data: {'message': 'Unauthorized'},
            statusCode: 401,
            requestOptions: RequestOptions(path: '/auth/api-keys'),
          ),
        );

        when(() => mockDio.get('/auth/api-keys')).thenThrow(dioException);

        // Act & Assert
        expect(
          () => repository.listKeys(),
          throwsA(isA<AccessDeniedException>()),
        );
      });
    });

    group('createKey', () {
      test('должен успешно создать ключ', () async {
        // Arrange
        final responseData = {
          'id': 'key-new',
          'name': 'New Key',
          'key_prefix': 'wibe_new123',
          'scopes': '*',
          'expires_at': null,
          'last_used_at': null,
          'created_at': '2025-01-01T00:00:00Z',
          'raw_key': 'wibe_full_secret_key_here',
        };
        final response = Response(
          data: responseData,
          statusCode: 201,
          requestOptions: RequestOptions(path: '/auth/api-keys'),
        );

        when(() => mockDio.post(
              '/auth/api-keys',
              data: any(named: 'data'),
            )).thenAnswer((_) async => response);

        // Act
        final result = await repository.createKey(name: 'New Key');

        // Assert
        expect(result, isA<ApiKeyCreatedModel>());
        expect(result.name, equals('New Key'));
        expect(result.rawKey, equals('wibe_full_secret_key_here'));
        expect(result.scopes, equals('*'));
        verify(() => mockDio.post(
              '/auth/api-keys',
              data: {'name': 'New Key'},
            )).called(1);
      });

      test('должен передать scopes и expires_in при создании', () async {
        // Arrange
        final responseData = {
          'id': 'key-scoped',
          'name': 'Scoped Key',
          'key_prefix': 'wibe_sco',
          'scopes': 'read,write',
          'expires_at': '2025-12-31T23:59:59Z',
          'last_used_at': null,
          'created_at': '2025-01-01T00:00:00Z',
          'raw_key': 'wibe_scoped_key_here',
        };
        final response = Response(
          data: responseData,
          statusCode: 201,
          requestOptions: RequestOptions(path: '/auth/api-keys'),
        );

        when(() => mockDio.post(
              '/auth/api-keys',
              data: any(named: 'data'),
            )).thenAnswer((_) async => response);

        // Act
        final result = await repository.createKey(
          name: 'Scoped Key',
          scopes: 'read,write',
          expiresInSeconds: 86400,
        );

        // Assert
        expect(result.scopes, equals('read,write'));
        verify(() => mockDio.post(
              '/auth/api-keys',
              data: {
                'name': 'Scoped Key',
                'scopes': 'read,write',
                'expires_in': 86400,
              },
            )).called(1);
      });

      test('должен выбросить исключение при ошибке создания', () async {
        // Arrange
        final dioException = DioException(
          requestOptions: RequestOptions(path: '/auth/api-keys'),
          response: Response(
            data: {'message': 'Bad request'},
            statusCode: 400,
            requestOptions: RequestOptions(path: '/auth/api-keys'),
          ),
        );

        when(() => mockDio.post(
              '/auth/api-keys',
              data: any(named: 'data'),
            )).thenThrow(dioException);

        // Act & Assert
        expect(
          () => repository.createKey(name: 'Test'),
          throwsA(isA<AuthException>()),
        );
      });
    });

    group('revokeKey', () {
      test('должен успешно отозвать ключ', () async {
        // Arrange
        final response = Response(
          data: {'message': 'API key revoked successfully'},
          statusCode: 200,
          requestOptions:
              RequestOptions(path: '/auth/api-keys/key-123/revoke'),
        );

        when(() => mockDio.post('/auth/api-keys/key-123/revoke'))
            .thenAnswer((_) async => response);

        // Act
        await repository.revokeKey('key-123');

        // Assert
        verify(() => mockDio.post('/auth/api-keys/key-123/revoke')).called(1);
      });

      test('должен выбросить исключение при отсутствии ключа', () async {
        // Arrange
        final dioException = DioException(
          requestOptions:
              RequestOptions(path: '/auth/api-keys/nonexistent/revoke'),
          response: Response(
            data: {'message': 'API key not found'},
            statusCode: 404,
            requestOptions:
                RequestOptions(path: '/auth/api-keys/nonexistent/revoke'),
          ),
        );

        when(() => mockDio.post('/auth/api-keys/nonexistent/revoke'))
            .thenThrow(dioException);

        // Act & Assert
        expect(
          () => repository.revokeKey('nonexistent'),
          throwsA(isA<AuthException>()),
        );
      });

      test('должен выбросить AccessDeniedException при 403', () async {
        // Arrange
        final dioException = DioException(
          requestOptions:
              RequestOptions(path: '/auth/api-keys/other-key/revoke'),
          response: Response(
            data: {'message': 'You can only revoke your own API keys'},
            statusCode: 403,
            requestOptions:
                RequestOptions(path: '/auth/api-keys/other-key/revoke'),
          ),
        );

        when(() => mockDio.post('/auth/api-keys/other-key/revoke'))
            .thenThrow(dioException);

        // Act & Assert
        expect(
          () => repository.revokeKey('other-key'),
          throwsA(isA<AccessDeniedException>()),
        );
      });
    });

    group('deleteKey', () {
      test('должен успешно удалить ключ', () async {
        // Arrange
        final response = Response(
          data: null,
          statusCode: 204,
          requestOptions: RequestOptions(path: '/auth/api-keys/key-123'),
        );

        when(() => mockDio.delete('/auth/api-keys/key-123'))
            .thenAnswer((_) async => response);

        // Act
        await repository.deleteKey('key-123');

        // Assert
        verify(() => mockDio.delete('/auth/api-keys/key-123')).called(1);
      });

      test('должен выбросить исключение при ошибке удаления', () async {
        // Arrange
        final dioException = DioException(
          requestOptions: RequestOptions(path: '/auth/api-keys/key-123'),
          response: Response(
            data: {'message': 'API key not found'},
            statusCode: 404,
            requestOptions: RequestOptions(path: '/auth/api-keys/key-123'),
          ),
        );

        when(() => mockDio.delete('/auth/api-keys/key-123'))
            .thenThrow(dioException);

        // Act & Assert
        expect(
          () => repository.deleteKey('key-123'),
          throwsA(isA<AuthException>()),
        );
      });
    });
  });
}
