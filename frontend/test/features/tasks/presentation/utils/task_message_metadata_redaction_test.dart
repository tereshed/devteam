@TestOn('vm')
@Tags(['unit'])
library;

import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/features/tasks/presentation/utils/task_message_metadata_redaction.dart';

void main() {
  group('redactTaskMessageMetadata', () {
    test('deny-list по имени ключа (регистронезависимо)', () {
      final out = redactTaskMessageMetadata({
        'API_KEY': 'visible',
        'openai_api_version': '2024-01-01',
      });
      expect(out['API_KEY'], '***');
      expect(out['openai_api_version'], '2024-01-01');
    });

    test('sk- и Bearer по значению', () {
      final out = redactTaskMessageMetadata({
        'x': 'sk-abc1234567890abcdefghij',
        'y': 'Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.token.sig',
      });
      expect(out['x'], '***');
      expect(out['y'], '***');
    });

    test('JWT-эвристика (три сегмента длиной по правилу)', () {
      const jwt =
          'abcdefghijklmnopqrstuvwxyz0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ-_'
          '.abcdefghijklmnopqrstuvwxyz0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ-_'
          '.abcdefghijklmnop';
      expect(redactTaskMessageMetadata({'t': jwt})['t'], '***');
    });

    test('ложное срабатывание 1.2.3 не JWT', () {
      expect(redactTaskMessageMetadata({'v': '1.2.3'})['v'], '1.2.3');
    });

    test('вложенная карта', () {
      final out = redactTaskMessageMetadata({
        'nested': <String, dynamic>{
          'authorization': 'keep-name-redacted-value',
        },
      });
      final nested = out['nested'];
      expect(nested, isA<Map<String, dynamic>>());
      expect((nested as Map<String, dynamic>)['authorization'], '***');
    });

    test('sk- внутри списка редактируется', () {
      final out = redactTaskMessageMetadata({
        'tools': ['safe', 'sk-abcdef1234567890ABCDEF'],
      });
      expect((out['tools'] as List)[0], 'safe');
      expect((out['tools'] as List)[1], '***');
    });

    test('JWT внутри списка', () {
      final jwt =
          '${List.filled(16, 'a').join()}.${List.filled(16, 'b').join()}.${List.filled(8, 'c').join()}';
      expect(
        (redactTaskMessageMetadata({'xs': [jwt]})['xs'] as List)[0],
        '***',
      );
    });

    test('Bearer и JWT внутри смешанного списка', () {
      final jwt =
          '${List.filled(16, 'x').join()}.${List.filled(16, 'y').join()}.${List.filled(8, 'z').join()}';
      final out = redactTaskMessageMetadata({
        'logs': [
          'ok',
          'sk-ABCDEFGHIJKLMNOP',
          'Bearer eyJhbGciOiJIUzI1NiJ9.payload.signature',
          jwt,
        ],
      });
      final logs = out['logs'] as List;
      expect(logs[0], 'ok');
      expect(logs[1], '***');
      expect(logs[2], '***');
      expect(logs[3], '***');
    });

    test('контракт deny-list: authentic и author (подстрока auth) → значение скрыто', () {
      expect(redactTaskMessageMetadata({'authentic': 'x'})['authentic'], '***');
      expect(redactTaskMessageMetadata({'author': 'Jane'})['author'], '***');
    });

    test('контракт deny-list: passphrase не содержит password — значение проходит', () {
      expect(
        redactTaskMessageMetadata({'passphrase': 'user-visible'})['passphrase'],
        'user-visible',
      );
    });

    test('контракт deny-list: tokenize_input содержит token — ключ блокируется', () {
      expect(
        redactTaskMessageMetadata({'tokenize_input': 'x'})['tokenize_input'],
        '***',
      );
    });
  });
}
