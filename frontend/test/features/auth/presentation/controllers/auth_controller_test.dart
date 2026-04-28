// @dart=2.19
// @TestOn('vm')
// @Tags(['unit'])

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/core/storage/token_storage.dart';
import 'package:frontend/features/auth/data/auth_providers.dart';
import 'package:frontend/features/auth/data/auth_repository.dart';
import 'package:frontend/features/auth/domain/models.dart';
import 'package:frontend/features/auth/presentation/controllers/auth_controller.dart';
import 'package:mocktail/mocktail.dart';

// Моки
class MockAuthRepository extends Mock implements AuthRepository {}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  group('AuthController', () {
    late MockAuthRepository mockRepository;

    setUp(() {
      mockRepository = MockAuthRepository();
      FlutterSecureStorage.setMockInitialValues({});
    });

    group('login', () {
      test('должен успешно войти пользователя', () async {
        // Arrange
        const user = UserModel(
          id: 'user-123',
          email: 'test@example.com',
          role: 'user',
          emailVerified: true,
        );
        final loginResponse = {
          'access_token': 'test_access_token',
          'refresh_token': 'test_refresh_token',
        };

        when(() => mockRepository.login(
              email: 'test@example.com',
              password: 'password123',
            )).thenAnswer((_) async => loginResponse);
        when(() => mockRepository.getCurrentUser())
            .thenAnswer((_) async => user);

        // Мокаем TokenStorage
        await TokenStorage.clearTokens();

        // Act & Assert
        final container = ProviderContainer(
          overrides: [
            authRepositoryProvider.overrideWithValue(mockRepository),
          ],
        );

        final controller = container.read(authControllerProvider.notifier);

        await controller.login(
          email: 'test@example.com',
          password: 'password123',
        );

        final state = container.read(authControllerProvider);
        await state.when(
          data: (user) async {
            expect(user, isNotNull);
            expect(user!.email, equals('test@example.com'));
          },
          loading: () async => fail('Should not be loading'),
          error: (error, stack) async => fail('Should not have error: $error'),
        );

        verify(() => mockRepository.login(
              email: 'test@example.com',
              password: 'password123',
            )).called(1);
        verify(() => mockRepository.getCurrentUser()).called(1);

        container.dispose();
      });

      test('должен обработать ошибку входа', () async {
        // Arrange
        when(() => mockRepository.login(
              email: 'test@example.com',
              password: 'wrong_password',
            )).thenThrow(Exception('Invalid credentials'));

        await TokenStorage.clearTokens();

        // Act & Assert
        final container = ProviderContainer(
          overrides: [
            authRepositoryProvider.overrideWithValue(mockRepository),
          ],
        );

        final controller = container.read(authControllerProvider.notifier);

        expect(
          () => controller.login(
            email: 'test@example.com',
            password: 'wrong_password',
          ),
          throwsA(isA<Exception>()),
        );

        final state = container.read(authControllerProvider);
        await state.when(
          data: (user) async => expect(user, isNull),
          loading: () async => fail('Should not be loading'),
          error: (error, stack) async {
            expect(error, isA<Exception>());
          },
        );

        container.dispose();
      });
    });

    group('register', () {
      test('должен успешно зарегистрировать пользователя', () async {
        // Arrange
        const user = UserModel(
          id: 'user-123',
          email: 'newuser@example.com',
          role: 'user',
          emailVerified: false,
        );
        final registerResponse = {
          'access_token': 'test_access_token',
          'refresh_token': 'test_refresh_token',
        };

        when(() => mockRepository.register(
              email: 'newuser@example.com',
              password: 'password123',
            )).thenAnswer((_) async => registerResponse);
        when(() => mockRepository.getCurrentUser())
            .thenAnswer((_) async => user);

        await TokenStorage.clearTokens();

        // Act & Assert
        final container = ProviderContainer(
          overrides: [
            authRepositoryProvider.overrideWithValue(mockRepository),
          ],
        );

        final controller = container.read(authControllerProvider.notifier);

        await controller.register(
          email: 'newuser@example.com',
          password: 'password123',
        );

        final state = container.read(authControllerProvider);
        await state.when(
          data: (user) async {
            expect(user, isNotNull);
            expect(user!.email, equals('newuser@example.com'));
          },
          loading: () async => fail('Should not be loading'),
          error: (error, stack) async => fail('Should not have error: $error'),
        );

        verify(() => mockRepository.register(
              email: 'newuser@example.com',
              password: 'password123',
            )).called(1);
        verify(() => mockRepository.getCurrentUser()).called(1);

        container.dispose();
      });
    });

    group('logout', () {
      test('должен успешно выйти из системы', () async {
        // Arrange
        when(() => mockRepository.logout()).thenAnswer((_) async => {});

        // Act & Assert
        final container = ProviderContainer(
          overrides: [
            authRepositoryProvider.overrideWithValue(mockRepository),
          ],
        );

        final controller = container.read(authControllerProvider.notifier);

        await controller.logout();

        final state = container.read(authControllerProvider);
        await state.when(
          data: (user) async => expect(user, isNull),
          loading: () async => fail('Should not be loading'),
          error: (error, stack) async => fail('Should not have error: $error'),
        );

        verify(() => mockRepository.logout()).called(1);

        container.dispose();
      });
    });

    group('refreshUser', () {
      test('должен успешно обновить данные пользователя', () async {
        // Arrange
        const user = UserModel(
          id: 'user-123',
          email: 'test@example.com',
          role: 'user',
          emailVerified: true,
        );

        when(() => mockRepository.getCurrentUser())
            .thenAnswer((_) async => user);

        // Act & Assert
        final container = ProviderContainer(
          overrides: [
            authRepositoryProvider.overrideWithValue(mockRepository),
          ],
        );

        final controller = container.read(authControllerProvider.notifier);

        await controller.refreshUser();

        final state = container.read(authControllerProvider);
        await state.when(
          data: (user) async {
            expect(user, isNotNull);
            expect(user!.email, equals('test@example.com'));
          },
          loading: () async => fail('Should not be loading'),
          error: (error, stack) async => fail('Should not have error: $error'),
        );

        verify(() => mockRepository.getCurrentUser()).called(1);

        container.dispose();
      });
    });
  });
}
