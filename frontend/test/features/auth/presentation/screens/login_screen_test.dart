// @dart=2.19
// @Tags(['widget'])

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/features/auth/data/auth_providers.dart';
import 'package:frontend/features/auth/data/auth_repository.dart';
import 'package:frontend/features/auth/domain/models.dart';
import 'package:frontend/features/auth/presentation/screens/login_screen.dart';
import 'package:frontend/l10n/app_localizations.dart';
import 'package:mocktail/mocktail.dart';

// Моки
class MockAuthRepository extends Mock implements AuthRepository {}

void main() {
  group('LoginScreen', () {
    late MockAuthRepository mockRepository;

    setUp(() {
      mockRepository = MockAuthRepository();
      FlutterSecureStorage.setMockInitialValues({});
    });

    testWidgets('должен отображать форму входа', (tester) async {
      // Act
      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            authRepositoryProvider.overrideWithValue(mockRepository),
          ],
          child: MaterialApp(
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            home: const LoginScreen(),
          ),
        ),
      );

      // Assert
      expect(find.byType(LoginScreen), findsOneWidget);
      expect(find.byType(TextFormField), findsNWidgets(2)); // Email и Password
      expect(find.byType(ElevatedButton), findsOneWidget);
    });

    testWidgets('должен валидировать email поле', (tester) async {
      // Act
      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            authRepositoryProvider.overrideWithValue(mockRepository),
          ],
          child: MaterialApp(
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            home: const LoginScreen(),
          ),
        ),
      );

      // Вводим невалидный email
      final emailField = find.byType(TextFormField).first;
      await tester.enterText(emailField, 'invalid-email');
      await tester.tap(find.byType(ElevatedButton));
      await tester.pumpAndSettle(); // Wait for all animations to complete

      // Assert
      // Форма должна показать ошибку валидации
      expect(find.text('Enter a valid email'), findsOneWidget);
    });

    testWidgets('должен валидировать password поле', (tester) async {
      // Act
      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            authRepositoryProvider.overrideWithValue(mockRepository),
          ],
          child: MaterialApp(
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            home: const LoginScreen(),
          ),
        ),
      );

      // Вводим короткий пароль
      final passwordField = find.byType(TextFormField).last;
      await tester.enterText(passwordField, 'short');
      await tester.tap(find.byType(ElevatedButton));
      await tester.pumpAndSettle(); // Wait for all animations to complete

      // Assert
      // Форма должна показать ошибку валидации
      expect(find.text('Password must be at least 8 characters'), findsOneWidget);
    });

    testWidgets('должен показывать индикатор загрузки при входе', (tester) async {
      // Arrange
      final user = UserModel(
        id: 'user-123',
        email: 'test@example.com',
        role: 'user',
        emailVerified: true,
      );
      final loginResponse = {
        'access_token': 'test_token',
        'refresh_token': 'refresh_token',
      };

      // Добавляем задержку чтобы успеть проверить состояние загрузки
      // Используем Completer для контроля времени ответа
      when(() => mockRepository.login(
            email: 'test@example.com',
            password: 'password123',
          )).thenAnswer((_) async {
        // Имитируем сетевую задержку, но без реального времени
        await Future.delayed(const Duration(milliseconds: 50));
        return loginResponse;
      });
      when(() => mockRepository.getCurrentUser())
          .thenAnswer((_) async => user);

      // Act
      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            authRepositoryProvider.overrideWithValue(mockRepository),
          ],
          child: MaterialApp(
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            home: const LoginScreen(),
          ),
        ),
      );

      // Вводим валидные данные
      final emailField = find.byType(TextFormField).first;
      final passwordField = find.byType(TextFormField).last;
      await tester.enterText(emailField, 'test@example.com');
      await tester.enterText(passwordField, 'password123');
      await tester.tap(find.byType(ElevatedButton));
      
      // Первый pump запускает обработчик нажатия
      await tester.pump();
      
      // Проверяем индикатор загрузки (пока Future.delayed еще выполняется)
      // Важно: не используем pumpAndSettle здесь, иначе пропустим состояние загрузки
      expect(find.byType(CircularProgressIndicator), findsOneWidget);

      // Теперь ждем завершения всего (таймеров и анимаций)
      await tester.pumpAndSettle();
      
      // После завершения индикатора быть не должно (мы ушли на другой экран или обновили UI)
      expect(find.byType(CircularProgressIndicator), findsNothing);
    });
  });
}
