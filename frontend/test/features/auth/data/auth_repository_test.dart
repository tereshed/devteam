// @dart=2.19
// @TestOn('vm')
// @Tags(['unit'])

import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/features/auth/data/auth_repository.dart';
import 'package:frontend/features/auth/domain/user_model.dart';
import 'package:mocktail/mocktail.dart';

// Моки для Dio
class MockDio extends Mock implements Dio {}

void main() {
  group('AuthRepository', () {
    late MockDio mockDio;
    late AuthRepository repository;

    setUp(() {
      mockDio = MockDio();
      repository = AuthRepository(dio: mockDio);
    });

    group('register', () {
      test('должен успешно зарегистрировать пользователя', () async {
        // Arrange
        final responseData = {
          'access_token': 'test_access_token',
          'refresh_token': 'test_refresh_token',
        };
        final response = Response(
          data: responseData,
          statusCode: 200,
          requestOptions: RequestOptions(path: '/auth/register'),
        );

        when(() => mockDio.post(
              '/auth/register',
              data: any(named: 'data'),
            )).thenAnswer((_) async => response);

        // Act
        final result = await repository.register(
          email: 'test@example.com',
          password: 'password123',
        );

        // Assert
        expect(result, equals(responseData));
        verify(() => mockDio.post(
              '/auth/register',
              data: {
                'email': 'test@example.com',
                'password': 'password123',
              },
            )).called(1);
      });

      test('должен выбросить исключение при ошибке регистрации', () async {
        // Arrange
        final dioException = DioException(
          requestOptions: RequestOptions(path: '/auth/register'),
          response: Response(
            data: {'error': 'User already exists'},
            statusCode: 409,
            requestOptions: RequestOptions(path: '/auth/register'),
          ),
        );

        when(() => mockDio.post(
              '/auth/register',
              data: any(named: 'data'),
            )).thenThrow(dioException);

        // Act & Assert
        expect(
          () => repository.register(
            email: 'test@example.com',
            password: 'password123',
          ),
          throwsA(isA<Exception>()),
        );
      });
    });

    group('login', () {
      test('должен успешно войти пользователя', () async {
        // Arrange
        final responseData = {
          'access_token': 'test_access_token',
          'refresh_token': 'test_refresh_token',
        };
        final response = Response(
          data: responseData,
          statusCode: 200,
          requestOptions: RequestOptions(path: '/auth/login'),
        );

        when(() => mockDio.post(
              '/auth/login',
              data: any(named: 'data'),
            )).thenAnswer((_) async => response);

        // Act
        final result = await repository.login(
          email: 'test@example.com',
          password: 'password123',
        );

        // Assert
        expect(result, equals(responseData));
        verify(() => mockDio.post(
              '/auth/login',
              data: {
                'email': 'test@example.com',
                'password': 'password123',
              },
            )).called(1);
      });

      test('должен выбросить исключение при неверных учетных данных', () async {
        // Arrange
        final dioException = DioException(
          requestOptions: RequestOptions(path: '/auth/login'),
          response: Response(
            data: {'error': 'Invalid credentials'},
            statusCode: 401,
            requestOptions: RequestOptions(path: '/auth/login'),
          ),
        );

        when(() => mockDio.post(
              '/auth/login',
              data: any(named: 'data'),
            )).thenThrow(dioException);

        // Act & Assert
        expect(
          () => repository.login(
            email: 'test@example.com',
            password: 'wrong_password',
          ),
          throwsA(isA<Exception>()),
        );
      });
    });

    group('getCurrentUser', () {
      test('должен успешно получить данные текущего пользователя', () async {
        // Arrange
        final userData = {
          'id': 'user-123',
          'email': 'test@example.com',
          'role': 'user',
          'email_verified': true,
        };
        final response = Response(
          data: userData,
          statusCode: 200,
          requestOptions: RequestOptions(path: '/auth/me'),
        );

        when(() => mockDio.get('/auth/me'))
            .thenAnswer((_) async => response);

        // Act
        final result = await repository.getCurrentUser();

        // Assert
        expect(result, isA<UserModel>());
        expect(result.id, equals('user-123'));
        expect(result.email, equals('test@example.com'));
        expect(result.role, equals('user'));
        expect(result.emailVerified, equals(true));
        verify(() => mockDio.get('/auth/me')).called(1);
      });

      test('должен выбросить исключение при ошибке получения пользователя', () async {
        // Arrange
        final dioException = DioException(
          requestOptions: RequestOptions(path: '/auth/me'),
          response: Response(
            data: {'error': 'Unauthorized'},
            statusCode: 401,
            requestOptions: RequestOptions(path: '/auth/me'),
          ),
        );

        when(() => mockDio.get('/auth/me')).thenThrow(dioException);

        // Act & Assert
        expect(
          () => repository.getCurrentUser(),
          throwsA(isA<Exception>()),
        );
      });
    });

    group('logout', () {
      test('должен успешно выйти из системы', () async {
        // Arrange
        final response = Response(
          data: null,
          statusCode: 200,
          requestOptions: RequestOptions(path: '/auth/logout'),
        );

        when(() => mockDio.post('/auth/logout'))
            .thenAnswer((_) async => response);

        // Act
        await repository.logout();

        // Assert
        verify(() => mockDio.post('/auth/logout')).called(1);
      });
    });

    group('refreshToken', () {
      test('должен успешно обновить токен', () async {
        // Arrange
        final responseData = {
          'access_token': 'new_access_token',
          'refresh_token': 'new_refresh_token',
        };
        final response = Response(
          data: responseData,
          statusCode: 200,
          requestOptions: RequestOptions(path: '/auth/refresh'),
        );

        when(() => mockDio.post(
              '/auth/refresh',
              data: any(named: 'data'),
            )).thenAnswer((_) async => response);

        // Act
        final result = await repository.refreshToken(
          refreshToken: 'old_refresh_token',
        );

        // Assert
        expect(result, equals(responseData));
        verify(() => mockDio.post(
              '/auth/refresh',
              data: {'refresh_token': 'old_refresh_token'},
            )).called(1);
      });
    });
  });
}

