// @dart=2.19
// @Tags(['widget'])

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/shared/widgets/error_widget.dart';

void main() {
  group('AppErrorWidget', () {
    testWidgets('должен отображать сообщение об ошибке', (tester) async {
      // Arrange & Act
      await tester.pumpWidget(
        const MaterialApp(
          home: Scaffold(
            body: AppErrorWidget(message: 'Произошла ошибка'),
          ),
        ),
      );

      // Assert
      expect(find.text('Произошла ошибка'), findsOneWidget);
      expect(find.byIcon(Icons.error_outline), findsOneWidget);
    });

    testWidgets('должен отображать кнопку повтора при наличии onRetry', (tester) async {
      // Arrange
      var retryCalled = false;

      // Act
      await tester.pumpWidget(
        MaterialApp(
          home: Scaffold(
            body: AppErrorWidget(
              message: 'Ошибка загрузки',
              onRetry: () {
                retryCalled = true;
              },
            ),
          ),
        ),
      );

      // Assert
      expect(find.text('Повторить'), findsOneWidget);
      expect(find.byType(ElevatedButton), findsOneWidget);

      // Проверяем, что кнопка работает
      await tester.tap(find.text('Повторить'));
      expect(retryCalled, isTrue);
    });

    testWidgets('не должен отображать кнопку повтора без onRetry', (tester) async {
      // Arrange & Act
      await tester.pumpWidget(
        const MaterialApp(
          home: Scaffold(
            body: AppErrorWidget(message: 'Ошибка загрузки'),
          ),
        ),
      );

      // Assert
      expect(find.text('Повторить'), findsNothing);
      expect(find.byType(ElevatedButton), findsNothing);
    });

    testWidgets('должен использовать правильный цвет для иконки ошибки', (tester) async {
      // Arrange & Act
      await tester.pumpWidget(
        MaterialApp(
          theme: ThemeData(
            colorScheme: ColorScheme.fromSeed(seedColor: Colors.blue),
          ),
          home: const Scaffold(
            body: AppErrorWidget(message: 'Ошибка'),
          ),
        ),
      );

      // Assert
      final iconFinder = find.byIcon(Icons.error_outline);
      expect(iconFinder, findsOneWidget);

      final icon = tester.widget<Icon>(iconFinder);
      expect(icon.color, isNotNull);
    });
  });
}
