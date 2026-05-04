import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/core/api/dio_message_path.dart';

void main() {
  group('isConversationMessagesApiPath', () {
    test('true для относительного пути сообщений', () {
      expect(
        isConversationMessagesApiPath(
          '/conversations/22222222-2222-2222-2222-222222222222/messages',
        ),
        isTrue,
      );
    });

    test('true для абсолютного URL', () {
      expect(
        isConversationMessagesApiPath(
          'http://127.0.0.1:8080/api/v1/conversations/'
          '22222222-2222-2222-2222-222222222222/messages',
        ),
        isTrue,
      );
    });

    test('false для списка чатов проекта', () {
      expect(
        isConversationMessagesApiPath(
          '/projects/11111111-1111-1111-1111-111111111111/conversations',
        ),
        isFalse,
      );
    });

    test('false для суффикса после conversations (не messages)', () {
      expect(
        isConversationMessagesApiPath(
          '/projects/pid/conversations-archive',
        ),
        isFalse,
      );
    });
  });

  group('isProjectConversationsListPath', () {
    test('true для списка чатов проекта', () {
      expect(
        isProjectConversationsListPath(
          '/projects/11111111-1111-1111-1111-111111111111/conversations',
        ),
        isTrue,
      );
    });

    test('true для абсолютного URL', () {
      expect(
        isProjectConversationsListPath(
          'http://127.0.0.1:8080/api/v1/projects/'
          '11111111-1111-1111-1111-111111111111/conversations',
        ),
        isTrue,
      );
    });

    test('false для conversations-archive', () {
      expect(
        isProjectConversationsListPath('/projects/pid/conversations-archive'),
        isFalse,
      );
    });
  });
}
