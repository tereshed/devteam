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

  group('isTaskMessagesApiPath', () {
    test('true для сообщений задачи', () {
      expect(
        isTaskMessagesApiPath(
          '/tasks/22222222-2222-2222-2222-222222222222/messages',
        ),
        isTrue,
      );
    });

    test('true для абсолютного URL', () {
      expect(
        isTaskMessagesApiPath(
          'http://127.0.0.1:8080/api/v1/tasks/'
          '22222222-2222-2222-2222-222222222222/messages',
        ),
        isTrue,
      );
    });

    test('false для списка задач проекта', () {
      expect(
        isTaskMessagesApiPath('/projects/pid/tasks'),
        isFalse,
      );
    });
  });

  group('isProjectTasksListPath', () {
    test('true для списка задач проекта', () {
      expect(
        isProjectTasksListPath(
          '/projects/11111111-1111-1111-1111-111111111111/tasks',
        ),
        isTrue,
      );
    });

    test('true с префиксом /api/v1 в пути', () {
      expect(
        isProjectTasksListPath(
          '/api/v1/projects/11111111-1111-1111-1111-111111111111/tasks',
        ),
        isTrue,
      );
    });

    test('false если /projects/.../tasks не в начале пути', () {
      expect(
        isProjectTasksListPath(
          '/agents/x/projects/11111111-1111-1111-1111-111111111111/tasks',
        ),
        isFalse,
      );
    });

    test('false для tasks-archive', () {
      expect(isProjectTasksListPath('/projects/pid/tasks-archive'), isFalse);
    });

    test('false для гипотетического /tasks/bulk (не список проекта)', () {
      expect(
        isProjectTasksListPath('/projects/pid/tasks/bulk-import'),
        isFalse,
      );
    });
  });

  group('isTaskCorrectApiPath', () {
    test('true для POST correct', () {
      expect(
        isTaskCorrectApiPath(
          '/tasks/22222222-2222-2222-2222-222222222222/correct',
        ),
        isTrue,
      );
    });

    test('false для pause и messages', () {
      expect(
        isTaskCorrectApiPath('/tasks/uuid/pause'),
        isFalse,
      );
      expect(
        isTaskCorrectApiPath('/tasks/uuid/messages'),
        isFalse,
      );
    });
  });

  group('isTaskResourceApiPath', () {
    test('true для /tasks/id и вложенных путей', () {
      expect(isTaskResourceApiPath('/tasks/uuid'), isTrue);
      expect(isTaskResourceApiPath('/tasks/uuid/pause'), isTrue);
      expect(isTaskResourceApiPath('/tasks/uuid/messages'), isTrue);
    });

    test('true для /api/v1/tasks/...', () {
      expect(
        isTaskResourceApiPath(
          '/api/v1/tasks/22222222-2222-2222-2222-222222222222/pause',
        ),
        isTrue,
      );
    });

    test('false если /tasks/… не в начале пути', () {
      expect(
        isTaskResourceApiPath('/agents/x/tasks/uuid/pause'),
        isFalse,
      );
    });

    test('false для /projects/.../tasks', () {
      expect(
        isTaskResourceApiPath('/projects/pid/tasks'),
        isFalse,
      );
    });
  });
}
