// @dart=2.19
// @Tags(['widget'])

import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/features/auth/data/api_key_providers.dart';
import 'package:frontend/features/auth/data/api_key_repository.dart';
import 'package:frontend/features/auth/data/auth_providers.dart';
import 'package:frontend/features/auth/data/auth_repository.dart';
import 'package:frontend/features/auth/domain/models.dart';
import 'package:frontend/features/auth/presentation/screens/api_keys_screen.dart';
import 'package:frontend/l10n/app_localizations.dart';
import 'package:mocktail/mocktail.dart';

class MockApiKeyRepository extends Mock implements ApiKeyRepository {}

class MockAuthRepository extends Mock implements AuthRepository {}

void main() {
  group('ApiKeysScreen', () {
    late MockApiKeyRepository mockApiKeyRepo;
    late MockAuthRepository mockAuthRepo;

    setUp(() {
      mockApiKeyRepo = MockApiKeyRepository();
      mockAuthRepo = MockAuthRepository();
      FlutterSecureStorage.setMockInitialValues({});
    });

    Widget buildTestWidget({
      required MockApiKeyRepository apiKeyRepo,
      required MockAuthRepository authRepo,
    }) {
      return ProviderScope(
        overrides: [
          apiKeyRepositoryProvider.overrideWithValue(apiKeyRepo),
          authRepositoryProvider.overrideWithValue(authRepo),
        ],
        child: MaterialApp(
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const ApiKeysScreen(),
        ),
      );
    }

    testWidgets('должен показывать индикатор загрузки при начальной загрузке',
        (tester) async {
      // Arrange — используем Completer чтобы контролировать завершение Future
      final completer = Completer<List<ApiKeyModel>>();
      when(() => mockApiKeyRepo.listKeys())
          .thenAnswer((_) => completer.future);

      // Act
      await tester.pumpWidget(buildTestWidget(
        apiKeyRepo: mockApiKeyRepo,
        authRepo: mockAuthRepo,
      ));
      await tester.pump();

      // Assert
      expect(find.byType(CircularProgressIndicator), findsOneWidget);

      // Завершаем Future чтобы не оставлять pending timers
      completer.complete(<ApiKeyModel>[]);
      await tester.pumpAndSettle();
    });

    testWidgets('должен показывать пустое состояние когда ключей нет',
        (tester) async {
      // Arrange
      when(() => mockApiKeyRepo.listKeys())
          .thenAnswer((_) async => <ApiKeyModel>[]);

      // Act
      await tester.pumpWidget(buildTestWidget(
        apiKeyRepo: mockApiKeyRepo,
        authRepo: mockAuthRepo,
      ));
      await tester.pumpAndSettle();

      // Assert — виджет _EmptyState с иконкой vpn_key_off
      expect(find.byIcon(Icons.vpn_key_off), findsOneWidget);
    });

    testWidgets('должен показывать список ключей', (tester) async {
      // Arrange
      final keys = [
        ApiKeyModel(
          id: 'key-1',
          name: 'Production Key',
          keyPrefix: 'wibe_prod12',
          scopes: '*',
          createdAt: DateTime(2025, 1, 1),
        ),
        ApiKeyModel(
          id: 'key-2',
          name: 'Dev Key',
          keyPrefix: 'wibe_dev123',
          scopes: 'read',
          createdAt: DateTime(2025, 6, 15),
          expiresAt: DateTime(2026, 6, 15),
        ),
      ];

      when(() => mockApiKeyRepo.listKeys()).thenAnswer((_) async => keys);

      // Act
      await tester.pumpWidget(buildTestWidget(
        apiKeyRepo: mockApiKeyRepo,
        authRepo: mockAuthRepo,
      ));
      await tester.pumpAndSettle();

      // Assert
      expect(find.text('Production Key'), findsOneWidget);
      expect(find.text('Dev Key'), findsOneWidget);
      expect(find.text('wibe_prod12...'), findsOneWidget);
      expect(find.text('wibe_dev123...'), findsOneWidget);
    });

    testWidgets('должен показывать FAB для создания ключа', (tester) async {
      // Arrange
      when(() => mockApiKeyRepo.listKeys())
          .thenAnswer((_) async => <ApiKeyModel>[]);

      // Act
      await tester.pumpWidget(buildTestWidget(
        apiKeyRepo: mockApiKeyRepo,
        authRepo: mockAuthRepo,
      ));
      await tester.pumpAndSettle();

      // Assert
      expect(find.byType(FloatingActionButton), findsOneWidget);
      expect(find.byIcon(Icons.add), findsOneWidget);
    });

    testWidgets('должен показывать ошибку и кнопку повтора при ошибке загрузки',
        (tester) async {
      // Arrange
      when(() => mockApiKeyRepo.listKeys())
          .thenThrow(Exception('Network error'));

      // Act
      await tester.pumpWidget(buildTestWidget(
        apiKeyRepo: mockApiKeyRepo,
        authRepo: mockAuthRepo,
      ));
      await tester.pumpAndSettle();

      // Assert — кнопка повтора
      expect(find.byType(ElevatedButton), findsOneWidget);
    });

    testWidgets('должен открывать диалог создания ключа при нажатии FAB',
        (tester) async {
      // Arrange
      when(() => mockApiKeyRepo.listKeys())
          .thenAnswer((_) async => <ApiKeyModel>[]);

      // Act
      await tester.pumpWidget(buildTestWidget(
        apiKeyRepo: mockApiKeyRepo,
        authRepo: mockAuthRepo,
      ));
      await tester.pumpAndSettle();

      // Нажимаем FAB
      await tester.tap(find.byType(FloatingActionButton));
      await tester.pumpAndSettle();

      // Assert — диалог открыт с полем ввода имени
      expect(find.byType(AlertDialog), findsOneWidget);
      expect(find.byType(TextField), findsOneWidget);
      expect(find.byType(DropdownButtonFormField<String>), findsOneWidget);
    });

    testWidgets(
        'должен показывать PopupMenuButton для каждого ключа в списке',
        (tester) async {
      // Arrange
      final keys = [
        ApiKeyModel(
          id: 'key-1',
          name: 'My Key',
          keyPrefix: 'wibe_mykey1',
          scopes: '*',
          createdAt: DateTime(2025, 1, 1),
        ),
      ];

      when(() => mockApiKeyRepo.listKeys()).thenAnswer((_) async => keys);

      // Act
      await tester.pumpWidget(buildTestWidget(
        apiKeyRepo: mockApiKeyRepo,
        authRepo: mockAuthRepo,
      ));
      await tester.pumpAndSettle();

      // Assert
      expect(find.byType(PopupMenuButton<String>), findsOneWidget);
    });
  });
}
