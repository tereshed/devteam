# Тесты Frontend

Этот каталог содержит тесты для Flutter приложения.

## Структура тестов

```
test/
├── features/              # Тесты по функциональным модулям
│   └── auth/
│       ├── data/          # Unit-тесты для репозиториев
│       └── presentation/  # Unit и Widget тесты для контроллеров и экранов
├── shared/                # Тесты для общих компонентов
│   └── widgets/           # Widget-тесты для UI Kit
└── core/                  # Тесты для core компонентов
    └── widgets/           # Widget-тесты для базовых виджетов

integration_test/          # E2E интеграционные тесты
└── app_test.dart         # Основные сценарии пользователя
```

## Типы тестов

### Unit-тесты
Тестируют бизнес-логику без UI:
- Контроллеры (AuthController)
- Репозитории (AuthRepository)
- Утилиты и хелперы

**Теги:** `@Tags(['unit'])`

**Шапка unit-файла (VM):** для unit-тестов домена, репозиториев и утилит без браузерного рантайма используйте в начале файла (до импортов):

```dart
// @dart=2.19
// @TestOn('vm')
// @Tags(['unit'])
```

`@TestOn('vm')` ограничивает запуск виртуальной машиной Dart (не `chrome`/wasm). Полный пример — `test/core/utils/uuid_test.dart`.

### Widget-тесты
Тестируют UI компоненты:
- Виджеты из `shared/widgets` (UI Kit)
- Экраны (Screens)
- Переиспользуемые компоненты

**Теги:** `@Tags(['widget'])`

### Integration-тесты
Тестируют полные пользовательские сценарии:
- Вход -> Dashboard -> Выход
- Регистрация нового пользователя
- Навигация между экранами

**Расположение:** `integration_test/`

## Запуск тестов

### Все тесты
```bash
make frontend-test
```

### Только unit-тесты
```bash
make frontend-test-unit
```

### Только widget-тесты
```bash
make frontend-test-widget
```

### Интеграционные тесты
```bash
make frontend-test-integration
```

### Запуск конкретного теста
```bash
cd frontend
flutter test test/features/auth/data/auth_repository_test.dart
```

## Зависимости для тестирования

- `flutter_test` - базовый фреймворк тестирования Flutter
- `mocktail` - создание моков для зависимостей
- `riverpod_test` - утилиты для тестирования Riverpod провайдеров
- `integration_test` - интеграционное тестирование

## Написание тестов

### Unit-тест пример

```dart
// @dart=2.19
// @TestOn('vm')
// @Tags(['unit'])
import 'package:flutter_test/flutter_test.dart';
import 'package:mocktail/mocktail.dart';

class MockRepository extends Mock implements Repository {}

void main() {
  group('MyService', () {
    test('должен выполнить действие', () {
      // Arrange
      final mockRepo = MockRepository();
      final service = MyService(mockRepo);
      
      // Act
      final result = service.doSomething();
      
      // Assert
      expect(result, equals(expectedValue));
    });
  });
}
```

### Widget-тест пример

```dart
// @Tags(['widget'])
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets('должен отображать виджет', (tester) async {
    // Arrange & Act
    await tester.pumpWidget(
      MaterialApp(
        home: MyWidget(),
      ),
    );
    
    // Assert
    expect(find.text('Hello'), findsOneWidget);
  });
}
```

## Покрытие кода

Для проверки покрытия кода тестами:

```bash
cd frontend
flutter test --coverage
genhtml coverage/lcov.info -o coverage/html
open coverage/html/index.html
```

## Примечания

- Все тесты должны быть независимыми и изолированными
- Используйте моки для внешних зависимостей (API, Storage)
- Widget-тесты должны тестировать только UI логику
- Integration-тесты требуют запущенного приложения или mock сервера

