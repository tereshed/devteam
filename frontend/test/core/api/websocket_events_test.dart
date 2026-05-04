import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/core/api/websocket_events.dart';

void main() {
  group('WsAuth', () {
    test('toString не содержит секретов', () {
      const jwt = 'ultra-secret-jwt';
      const key = 'ultra-secret-key';
      expect(
        '${const WsAuth.bearer(jwt)}',
        isNot(contains(jwt)),
      );
      expect(
        '${const WsAuth.apiKey(key)}',
        isNot(contains(key)),
      );
      expect('${const WsAuth.bearer(jwt)}', contains('***'));
      expect('${const WsAuth.apiKey(key)}', contains('***'));
    });
  });

  group('parseWsTimestamp', () {
    test('Z → isUtc', () {
      final d = parseWsTimestamp('2026-04-19T10:21:33.124Z');
      expect(d.isUtc, isTrue);
    });

    test('дробная часть ≥ 6 цифр не падает', () {
      final d = parseWsTimestamp('2026-04-19T10:21:33.123456789Z');
      expect(d.isUtc, isTrue);
      expect(d.microsecond, inInclusiveRange(0, 999999));
    });

    test('строка без Z и без offset → FormatException', () {
      expect(
        () => parseWsTimestamp('2026-04-19T10:21:33'),
        throwsA(isA<FormatException>()),
      );
    });

    test('+00:00 допустим', () {
      final d = parseWsTimestamp('2026-04-19T10:21:33+00:00');
      expect(d.isUtc, isTrue);
    });
  });

  group('parseWsServerEnvelope', () {
    test('BOM снимается перед jsonDecode', () {
      const json =
          '\uFEFF{"type":"task_status","v":1,"ts":"2026-01-01T00:00:00.000Z","project_id":"550e8400-e29b-41d4-a716-446655440000","data":{"task_id":"660e8400-e29b-41d4-a716-446655440001","previous_status":"pending","status":"in_progress"}}';
      final ev = parseWsServerEnvelope(json);
      expect(ev, isA<WsServerEvent>());
    });

    test('type case-sensitive: Task_status → unknown', () {
      const json =
          '{"type":"Task_status","v":1,"ts":"2026-01-01T00:00:00.000Z","project_id":"550e8400-e29b-41d4-a716-446655440000","data":{}}';
      final ev = parseWsServerEnvelope(json);
      final ok = ev.map(
        taskStatus: (_) => false,
        taskMessage: (_) => false,
        agentLog: (_) => false,
        error: (_) => false,
        unknown: (u) => u.value.type == 'Task_status',
      );
      expect(ok, isTrue);
    });

    test('лимит UTF-8: длинная кириллица/emoji по байтам, не по length', () {
      final pad = 'я' * 200000;
      final json =
          '{"type":"task_status","v":1,"ts":"2026-01-01T00:00:00.000Z","project_id":"550e8400-e29b-41d4-a716-446655440000","data":{"task_id":"$pad","previous_status":"a","status":"b"}}';
      expect(utf8.encode(json).length, greaterThan(kWsMaxIncomingFrameUtf8Bytes));
      expect(
        () => parseWsServerEnvelope(json),
        throwsA(isA<WsParseError>()),
      );
    });

    test('stream_overflow → needsRestRefetch', () {
      const json =
          '{"type":"error","v":1,"ts":"2026-01-01T00:00:00.000Z","project_id":"550e8400-e29b-41d4-a716-446655440000","data":{"code":"stream_overflow","message":"x"}}';
      final ev = parseWsServerEnvelope(json);
      final ok = ev.map(
        taskStatus: (_) => false,
        taskMessage: (_) => false,
        agentLog: (_) => false,
        error: (e) => e.value.needsRestRefetch,
        unknown: (_) => false,
      );
      expect(ok, isTrue);
    });
  });

  group('buildProjectWebSocketUri', () {
    test('http → ws, суффикс путь', () {
      final u = buildProjectWebSocketUri(
        'http://127.0.0.1:8080/api/v1/',
        '550e8400-e29b-41d4-a716-446655440000',
      );
      expect(u.scheme, 'ws');
      expect(u.path, '/api/v1/projects/550e8400-e29b-41d4-a716-446655440000/ws');
    });

    test('https → wss', () {
      final u = buildProjectWebSocketUri(
        'https://x.example.com/api/v1',
        '550e8400-e29b-41d4-a716-446655440000',
      );
      expect(u.scheme, 'wss');
    });

    test('неверная схема → ArgumentError', () {
      expect(
        () => buildProjectWebSocketUri('ftp://x', '550e8400-e29b-41d4-a716-446655440000'),
        throwsArgumentError,
      );
    });
  });
}
