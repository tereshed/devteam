import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/features/chat/presentation/widgets/chat_message_fence.dart';

void main() {
  group('fence lines (CommonMark)', () {
    test('до 3 пробелов — открывающий fence backtick', () {
      expect(isBacktickFenceLine('```dart'), true);
      expect(isBacktickFenceLine('  ```'), true);
      expect(isBacktickFenceLine('   ```'), true);
    });

    test('4 пробела перед ``` — не fence (индентированный код)', () {
      expect(isBacktickFenceLine('    ```'), false);
      expect(isBacktickFenceLine('     ```'), false);
    });

    test('tilde fence', () {
      expect(isTildeFenceLine('~~~bash'), true);
      expect(isTildeFenceLine('  ~~~'), true);
      expect(isTildeFenceLine('    ~~~'), false);
    });
  });

  group('preprocessUnclosedFence', () {
    test('дозакрывает незакрытый ~~~ при стриме', () {
      const md = '~~~bash\nstill typing';
      expect(
        preprocessUnclosedFence(md, isStreaming: true),
        '$md\n~~~',
      );
    });

    test('незакрытый fence: строка «    ```» не считается закрытием — дописывается финальный fence', () {
      const md = '```\nx\n    ```\nstill code';
      final out = preprocessUnclosedFence(md, isStreaming: true);
      expect(out.endsWith('still code\n```'), true);
      expect(isBacktickFenceLine('    ```'), false);
    });
  });

  group('ZWSP не используется в препроцессоре', () {
    test('длинная строка без невидимых символов', () {
      final long = List.filled(400, 'a').join();
      expect(
        preprocessUnclosedFence(long, isStreaming: false).contains('\u200B'),
        false,
      );
    });
  });
}
