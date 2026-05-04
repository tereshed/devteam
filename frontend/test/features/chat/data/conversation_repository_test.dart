import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/core/api/api_exceptions.dart';
import 'package:frontend/features/chat/data/conversation_exceptions.dart';
import 'package:frontend/features/chat/data/conversation_repository.dart';
import 'package:frontend/features/chat/domain/models.dart';
import 'package:frontend/features/chat/domain/requests.dart';
import 'package:mockito/annotations.dart';
import 'package:mockito/mockito.dart';

import 'conversation_repository_test.mocks.dart';

@GenerateNiceMocks([MockSpec<Dio>()])
void main() {
  late MockDio mockDio;
  late ConversationRepository repository;

  const projectId = '11111111-1111-1111-1111-111111111111';
  const conversationId = '22222222-2222-2222-2222-222222222222';
  const clientMessageId = '33333333-3333-3333-3333-333333333333';

  Map<String, dynamic> conversationJson() => <String, dynamic>{
        'id': conversationId,
        'project_id': projectId,
        'title': 'Chat',
        'status': 'active',
        'created_at': '2026-04-28T10:00:00Z',
        'updated_at': '2026-04-28T10:00:00Z',
      };

  Map<String, dynamic> messageJson() => <String, dynamic>{
        'id': '44444444-4444-4444-4444-444444444444',
        'conversation_id': conversationId,
        'role': 'user',
        'content': 'hello',
        'linked_task_ids': <String>[],
        'created_at': '2026-04-28T10:01:00Z',
      };

  setUp(() {
    mockDio = MockDio();
    repository = ConversationRepository(dio: mockDio);
  });

  group('listConversations', () {
    test('200 parses list and has_next', () async {
      final body = <String, dynamic>{
        'conversations': [conversationJson()],
        'total': 1,
        'limit': 20,
        'offset': 0,
        'has_next': true,
      };

      when(
        mockDio.get<Map<String, dynamic>>(
          '/projects/$projectId/conversations',
          queryParameters: anyNamed('queryParameters'),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenAnswer(
        (_) async => Response<Map<String, dynamic>>(
          data: body,
          statusCode: 200,
          requestOptions: RequestOptions(
            path: '/projects/$projectId/conversations',
          ),
        ),
      );

      final r = await repository.listConversations(projectId);

      expect(r.conversations, hasLength(1));
      expect(r.hasNext, isTrue);
      expect(r.total, 1);
    });

    test('normalizes limit and offset', () async {
      when(
        mockDio.get<Map<String, dynamic>>(
          '/projects/$projectId/conversations',
          queryParameters: anyNamed('queryParameters'),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenAnswer(
        (_) async => Response<Map<String, dynamic>>(
          data: <String, dynamic>{
            'conversations': <Map<String, dynamic>>[],
            'total': 0,
            'limit': 100,
            'offset': 0,
            'has_next': false,
          },
          statusCode: 200,
          requestOptions: RequestOptions(
            path: '/projects/$projectId/conversations',
          ),
        ),
      );

      await repository.listConversations(projectId, limit: 150, offset: -3);

      verify(
        mockDio.get<Map<String, dynamic>>(
          '/projects/$projectId/conversations',
          queryParameters: argThat(
            allOf([
              containsPair('limit', 100),
              containsPair('offset', 0),
            ]),
            named: 'queryParameters',
          ),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).called(1);
    });

    test('404 -> ProjectNotFoundException', () async {
      when(
        mockDio.get<Map<String, dynamic>>(
          any,
          queryParameters: anyNamed('queryParameters'),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenThrow(
        DioException(
          requestOptions: RequestOptions(
            path: '/projects/$projectId/conversations',
          ),
          response: Response<Map<String, dynamic>>(
            data: {'error': 'not_found', 'message': 'project not found'},
            statusCode: 404,
            requestOptions: RequestOptions(
              path: '/projects/$projectId/conversations',
            ),
          ),
          type: DioExceptionType.badResponse,
        ),
      );

      expect(
        () => repository.listConversations(projectId),
        throwsA(
          isA<ProjectNotFoundException>().having(
            (e) => e.apiErrorCode,
            'apiErrorCode',
            'not_found',
          ),
        ),
      );
    });

    test('404 with absolute URL path still maps to ProjectNotFoundException', () async {
      const fullPath =
          'http://127.0.0.1:8080/api/v1/projects/$projectId/conversations';
      when(
        mockDio.get<Map<String, dynamic>>(
          any,
          queryParameters: anyNamed('queryParameters'),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenThrow(
        DioException(
          requestOptions: RequestOptions(path: fullPath),
          response: Response<Map<String, dynamic>>(
            data: {'error': 'not_found', 'message': 'project not found'},
            statusCode: 404,
            requestOptions: RequestOptions(path: fullPath),
          ),
          type: DioExceptionType.badResponse,
        ),
      );

      expect(
        () => repository.listConversations(projectId),
        throwsA(
          isA<ProjectNotFoundException>().having(
            (e) => e.apiErrorCode,
            'apiErrorCode',
            'not_found',
          ),
        ),
      );
    });

    test(
      '404 with path .../conversations-archive does not map to ProjectNotFoundException',
      () async {
        final path = '/projects/$projectId/conversations-archive';
        when(
          mockDio.get<Map<String, dynamic>>(
            any,
            queryParameters: anyNamed('queryParameters'),
            cancelToken: anyNamed('cancelToken'),
          ),
        ).thenThrow(
          DioException(
            requestOptions: RequestOptions(path: path),
            response: Response<Map<String, dynamic>>(
              data: {'error': 'not_found', 'message': 'x'},
              statusCode: 404,
              requestOptions: RequestOptions(path: path),
            ),
            type: DioExceptionType.badResponse,
          ),
        );

        expect(
          () => repository.listConversations(projectId),
          throwsA(
            isA<ConversationNotFoundException>().having(
              (e) => e.apiErrorCode,
              'apiErrorCode',
              'not_found',
            ),
          ),
        );
      },
    );
  });

  group('createConversation', () {
    test('201 success', () async {
      const req = CreateConversationRequest(title: 'T');

      when(
        mockDio.post<Map<String, dynamic>>(
          '/projects/$projectId/conversations',
          data: anyNamed('data'),
          options: anyNamed('options'),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenAnswer(
        (_) async => Response<Map<String, dynamic>>(
          data: conversationJson(),
          statusCode: 201,
          requestOptions: RequestOptions(
            path: '/projects/$projectId/conversations',
            method: 'POST',
          ),
        ),
      );

      final c = await repository.createConversation(projectId, req);

      expect(c.id, conversationId);
      verify(
        mockDio.post<Map<String, dynamic>>(
          '/projects/$projectId/conversations',
          data: anyNamed('data'),
          options: argThat(
            predicate<Options>((o) {
              if (o.contentType != 'application/json') {
                return false;
              }
              final h = o.headers;
              if (h != null) {
                for (final k in h.keys) {
                  if (k.toLowerCase() == 'x-client-message-id') {
                    return false;
                  }
                }
              }
              return true;
            }),
            named: 'options',
          ),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).called(1);
    });

    test('404 -> ProjectNotFoundException', () async {
      when(
        mockDio.post<Map<String, dynamic>>(
          any,
          data: anyNamed('data'),
          options: anyNamed('options'),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenThrow(
        DioException(
          requestOptions: RequestOptions(
            path: '/projects/$projectId/conversations',
            method: 'POST',
          ),
          response: Response<Map<String, dynamic>>(
            data: {'error': 'not_found', 'message': 'missing'},
            statusCode: 404,
            requestOptions: RequestOptions(
              path: '/projects/$projectId/conversations',
            ),
          ),
          type: DioExceptionType.badResponse,
        ),
      );

      expect(
        () => repository.createConversation(
          projectId,
          const CreateConversationRequest(title: 'x'),
        ),
        throwsA(
          isA<ProjectNotFoundException>().having(
            (e) => e.apiErrorCode,
            'apiErrorCode',
            'not_found',
          ),
        ),
      );
    });
  });

  group('getConversation', () {
    test('200', () async {
      when(
        mockDio.get<Map<String, dynamic>>(
          '/conversations/$conversationId',
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenAnswer(
        (_) async => Response<Map<String, dynamic>>(
          data: conversationJson(),
          statusCode: 200,
          requestOptions: RequestOptions(path: '/conversations/$conversationId'),
        ),
      );

      final c = await repository.getConversation(conversationId);
      expect(c.id, conversationId);
    });

    test('404 -> ConversationNotFoundException', () async {
      when(
        mockDio.get<Map<String, dynamic>>(
          any,
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenThrow(
        DioException(
          requestOptions: RequestOptions(path: '/conversations/$conversationId'),
          response: Response<Map<String, dynamic>>(
            data: {'error': 'not_found', 'message': 'no chat'},
            statusCode: 404,
            requestOptions: RequestOptions(path: '/conversations/$conversationId'),
          ),
          type: DioExceptionType.badResponse,
        ),
      );

      expect(
        () => repository.getConversation(conversationId),
        throwsA(
          isA<ConversationNotFoundException>().having(
            (e) => e.apiErrorCode,
            'apiErrorCode',
            'not_found',
          ),
        ),
      );
    });
  });

  group('404 ConversationNotFound on /conversations paths', () {
    test('getMessages 404', () async {
      when(
        mockDio.get<Map<String, dynamic>>(
          '/conversations/$conversationId/messages',
          queryParameters: anyNamed('queryParameters'),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenThrow(
        DioException(
          requestOptions: RequestOptions(
            path: '/conversations/$conversationId/messages',
          ),
          response: Response<Map<String, dynamic>>(
            data: {'error': 'not_found', 'message': 'no'},
            statusCode: 404,
            requestOptions: RequestOptions(
              path: '/conversations/$conversationId/messages',
            ),
          ),
          type: DioExceptionType.badResponse,
        ),
      );

      expect(
        () => repository.getMessages(conversationId),
        throwsA(
          isA<ConversationNotFoundException>().having(
            (e) => e.apiErrorCode,
            'apiErrorCode',
            'not_found',
          ),
        ),
      );
    });

    test('sendMessage 404', () async {
      when(
        mockDio.post<Map<String, dynamic>>(
          any,
          data: anyNamed('data'),
          options: anyNamed('options'),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenThrow(
        DioException(
          requestOptions: RequestOptions(
            path: '/conversations/$conversationId/messages',
          ),
          response: Response<Map<String, dynamic>>(
            data: {'error': 'not_found', 'message': 'no'},
            statusCode: 404,
            requestOptions: RequestOptions(
              path: '/conversations/$conversationId/messages',
            ),
          ),
          type: DioExceptionType.badResponse,
        ),
      );

      expect(
        () => repository.sendMessage(
          conversationId,
          const SendMessageRequest(content: 'a'),
          clientMessageId: clientMessageId,
        ),
        throwsA(
          isA<ConversationNotFoundException>().having(
            (e) => e.apiErrorCode,
            'apiErrorCode',
            'not_found',
          ),
        ),
      );
    });

    test('deleteConversation 404', () async {
      when(
        mockDio.delete<void>(
          '/conversations/$conversationId',
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenThrow(
        DioException(
          requestOptions: RequestOptions(path: '/conversations/$conversationId'),
          response: Response<Map<String, dynamic>>(
            data: {'error': 'not_found', 'message': 'no'},
            statusCode: 404,
            requestOptions: RequestOptions(path: '/conversations/$conversationId'),
          ),
          type: DioExceptionType.badResponse,
        ),
      );

      expect(
        () => repository.deleteConversation(conversationId),
        throwsA(
          isA<ConversationNotFoundException>().having(
            (e) => e.apiErrorCode,
            'apiErrorCode',
            'not_found',
          ),
        ),
      );
    });
  });

  group('deleteConversation', () {
    test('204', () async {
      when(
        mockDio.delete<void>(
          '/conversations/$conversationId',
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenAnswer(
        (_) async => Response<void>(
          statusCode: 204,
          requestOptions: RequestOptions(
            path: '/conversations/$conversationId',
            method: 'DELETE',
          ),
        ),
      );

      await expectLater(
        repository.deleteConversation(conversationId),
        completes,
      );
    });

    test('non-204 status throws ConversationApiException', () async {
      when(
        mockDio.delete<void>(
          '/conversations/$conversationId',
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenAnswer(
        (_) async => Response<void>(
          statusCode: 200,
          requestOptions: RequestOptions(
            path: '/conversations/$conversationId',
            method: 'DELETE',
          ),
        ),
      );

      expect(
        () => repository.deleteConversation(conversationId),
        throwsA(isA<ConversationApiException>()),
      );
    });
  });

  group('getMessages', () {
    test('200', () async {
      when(
        mockDio.get<Map<String, dynamic>>(
          '/conversations/$conversationId/messages',
          queryParameters: anyNamed('queryParameters'),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenAnswer(
        (_) async => Response<Map<String, dynamic>>(
          data: <String, dynamic>{
            'messages': [messageJson()],
            'total': 1,
            'limit': 20,
            'offset': 0,
            'has_next': false,
          },
          statusCode: 200,
          requestOptions: RequestOptions(
            path: '/conversations/$conversationId/messages',
          ),
        ),
      );

      final r = await repository.getMessages(conversationId);
      expect(r.messages, hasLength(1));
    });

    test('normalizes limit for getMessages', () async {
      when(
        mockDio.get<Map<String, dynamic>>(
          '/conversations/$conversationId/messages',
          queryParameters: anyNamed('queryParameters'),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenAnswer(
        (_) async => Response<Map<String, dynamic>>(
          data: <String, dynamic>{
            'messages': <Map<String, dynamic>>[],
            'total': 0,
            'limit': 20,
            'offset': 0,
            'has_next': false,
          },
          statusCode: 200,
          requestOptions: RequestOptions(
            path: '/conversations/$conversationId/messages',
          ),
        ),
      );

      await repository.getMessages(conversationId, limit: 0);

      verify(
        mockDio.get<Map<String, dynamic>>(
          '/conversations/$conversationId/messages',
          queryParameters: argThat(
            containsPair('limit', 20),
            named: 'queryParameters',
          ),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).called(1);
    });
  });

  group('sendMessage', () {
    test('201 -> created', () async {
      when(
        mockDio.post<Map<String, dynamic>>(
          '/conversations/$conversationId/messages',
          data: anyNamed('data'),
          options: anyNamed('options'),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenAnswer(
        (_) async => Response<Map<String, dynamic>>(
          data: messageJson(),
          statusCode: 201,
          requestOptions: RequestOptions(
            path: '/conversations/$conversationId/messages',
            method: 'POST',
          ),
        ),
      );

      final r = await repository.sendMessage(
        conversationId,
        const SendMessageRequest(content: 'hi'),
        clientMessageId: clientMessageId,
      );

      expect(r.status, MessageSendStatus.created);
      expect(r.message.content, 'hello');

      verify(
        mockDio.post<Map<String, dynamic>>(
          '/conversations/$conversationId/messages',
          data: anyNamed('data'),
          options: argThat(
            predicate<Options>((o) {
              if (o.contentType != 'application/json') {
                return false;
              }
              final h = o.headers;
              if (h == null) {
                return false;
              }
              final v = h['X-Client-Message-ID'] ?? h['x-client-message-id'];
              return v != null && v.toString().contains(clientMessageId);
            }),
            named: 'options',
          ),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).called(1);
    });

    test('200 -> duplicate', () async {
      when(
        mockDio.post<Map<String, dynamic>>(
          any,
          data: anyNamed('data'),
          options: anyNamed('options'),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenAnswer(
        (_) async => Response<Map<String, dynamic>>(
          data: messageJson(),
          statusCode: 200,
          requestOptions: RequestOptions(
            path: '/conversations/$conversationId/messages',
          ),
        ),
      );

      final r = await repository.sendMessage(
        conversationId,
        const SendMessageRequest(content: 'hi'),
        clientMessageId: clientMessageId,
      );

      expect(r.status, MessageSendStatus.duplicate);
    });

    test('invalid clientMessageId -> ArgumentError', () async {
      expect(
        () => repository.sendMessage(
          conversationId,
          const SendMessageRequest(content: 'x'),
          clientMessageId: 'not-a-uuid',
        ),
        throwsA(
          isA<ArgumentError>().having(
            (e) => e.message,
            'message',
            contains('clientMessageId'),
          ),
        ),
      );
    });

    test('unexpected 2xx status throws ConversationApiException', () async {
      when(
        mockDio.post<Map<String, dynamic>>(
          any,
          data: anyNamed('data'),
          options: anyNamed('options'),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenAnswer(
        (_) async => Response<Map<String, dynamic>>(
          data: messageJson(),
          statusCode: 202,
          requestOptions: RequestOptions(
            path: '/conversations/$conversationId/messages',
          ),
        ),
      );

      expect(
        () => repository.sendMessage(
          conversationId,
          const SendMessageRequest(content: 'a'),
          clientMessageId: clientMessageId,
        ),
        throwsA(
          isA<ConversationApiException>().having(
            (e) => e.statusCode,
            'statusCode',
            202,
          ),
        ),
      );
    });
  });

  group('HTTP error mapping', () {
    test('cancel -> ConversationCancelledException', () async {
      when(
        mockDio.get<Map<String, dynamic>>(
          any,
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenThrow(
        DioException(
          requestOptions: RequestOptions(path: '/conversations/$conversationId'),
          type: DioExceptionType.cancel,
        ),
      );

      expect(
        () => repository.getConversation(conversationId),
        throwsA(isA<ConversationCancelledException>()),
      );
    });

    test(
      'JSON error code cancelled (badResponse) is not treated as Dio cancel',
      () async {
        when(
          mockDio.get<Map<String, dynamic>>(
            any,
            cancelToken: anyNamed('cancelToken'),
          ),
        ).thenThrow(
          DioException(
            requestOptions: RequestOptions(
              path: '/conversations/$conversationId',
            ),
            response: Response<Map<String, dynamic>>(
              data: {'error': 'cancelled', 'message': 'operation stopped'},
              statusCode: 400,
              requestOptions: RequestOptions(
                path: '/conversations/$conversationId',
              ),
            ),
            type: DioExceptionType.badResponse,
          ),
        );

        expect(
          () => repository.getConversation(conversationId),
          throwsA(
            isA<ConversationApiException>()
                .having((e) => e.apiErrorCode, 'code', 'cancelled')
                .having((e) => e.statusCode, 'status', 400),
          ),
        );
    });

    test('415 -> ConversationApiException', () async {
      when(
        mockDio.post<Map<String, dynamic>>(
          any,
          data: anyNamed('data'),
          options: anyNamed('options'),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenThrow(
        DioException(
          requestOptions: RequestOptions(
            path: '/conversations/$conversationId/messages',
          ),
          response: Response<Map<String, dynamic>>(
            data: {'error': 'bad_request', 'message': 'wrong type'},
            statusCode: 415,
            requestOptions: RequestOptions(
              path: '/conversations/$conversationId/messages',
            ),
          ),
          type: DioExceptionType.badResponse,
        ),
      );

      expect(
        () => repository.sendMessage(
          conversationId,
          const SendMessageRequest(content: 'a'),
          clientMessageId: clientMessageId,
        ),
        throwsA(
          isA<ConversationApiException>().having(
            (e) => e.statusCode,
            'statusCode',
            415,
          ),
        ),
      );
    });

    test('413 -> ConversationApiException', () async {
      when(
        mockDio.post<Map<String, dynamic>>(
          any,
          data: anyNamed('data'),
          options: anyNamed('options'),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenThrow(
        DioException(
          requestOptions: RequestOptions(
            path: '/conversations/$conversationId/messages',
          ),
          response: Response<Map<String, dynamic>>(
            data: {'error': 'bad_request', 'message': 'Payload too large'},
            statusCode: 413,
            requestOptions: RequestOptions(
              path: '/conversations/$conversationId/messages',
            ),
          ),
          type: DioExceptionType.badResponse,
        ),
      );

      expect(
        () => repository.sendMessage(
          conversationId,
          const SendMessageRequest(content: 'a'),
          clientMessageId: clientMessageId,
        ),
        throwsA(
          isA<ConversationApiException>().having(
            (e) => e.statusCode,
            'statusCode',
            413,
          ),
        ),
      );
    });

    test('429 -> ConversationRateLimitedException', () async {
      when(
        mockDio.get<Map<String, dynamic>>(
          any,
          queryParameters: anyNamed('queryParameters'),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenThrow(
        DioException(
          requestOptions: RequestOptions(path: '/projects/$projectId/conversations'),
          response: Response<Map<String, dynamic>>(
            data: {'error': 'too_many_requests', 'message': 'slow down'},
            statusCode: 429,
            requestOptions: RequestOptions(path: '/projects/$projectId/conversations'),
          ),
          type: DioExceptionType.badResponse,
        ),
      );

      expect(
        () => repository.listConversations(projectId),
        throwsA(
          isA<ConversationRateLimitedException>().having(
            (e) => e.apiErrorCode,
            'apiErrorCode',
            'too_many_requests',
          ),
        ),
      );
    });

    test('422 and 502 -> ConversationApiException with statusCode', () async {
      when(
        mockDio.post<Map<String, dynamic>>(
          any,
          data: anyNamed('data'),
          options: anyNamed('options'),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenThrow(
        DioException(
          requestOptions: RequestOptions(
            path: '/conversations/$conversationId/messages',
          ),
          response: Response<Map<String, dynamic>>(
            data: {'error': 'validation', 'message': 'bad'},
            statusCode: 422,
            requestOptions: RequestOptions(
              path: '/conversations/$conversationId/messages',
            ),
          ),
          type: DioExceptionType.badResponse,
        ),
      );

      try {
        await repository.sendMessage(
          conversationId,
          const SendMessageRequest(content: 'a'),
          clientMessageId: clientMessageId,
        );
        fail('expected throw');
      } on ConversationApiException catch (e) {
        expect(e.statusCode, 422);
        expect(e.apiErrorCode, 'validation');
      }

      when(
        mockDio.post<Map<String, dynamic>>(
          any,
          data: anyNamed('data'),
          options: anyNamed('options'),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenThrow(
        DioException(
          requestOptions: RequestOptions(
            path: '/conversations/$conversationId/messages',
          ),
          response: Response<Map<String, dynamic>>(
            data: {
              'error': 'external_service_error',
              'message': 'upstream',
            },
            statusCode: 502,
            requestOptions: RequestOptions(
              path: '/conversations/$conversationId/messages',
            ),
          ),
          type: DioExceptionType.badResponse,
        ),
      );

      expect(
        () => repository.sendMessage(
          conversationId,
          const SendMessageRequest(content: 'a'),
          clientMessageId: clientMessageId,
        ),
        throwsA(
          isA<ConversationApiException>()
              .having((e) => e.statusCode, 'sc', 502)
              .having(
                (e) => e.apiErrorCode,
                'code',
                'external_service_error',
              ),
        ),
      );
    });

    test('401 access_denied JSON -> UnauthorizedException', () async {
      when(
        mockDio.get<Map<String, dynamic>>(
          any,
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenThrow(
        DioException(
          requestOptions: RequestOptions(path: '/conversations/$conversationId'),
          response: Response<Map<String, dynamic>>(
            data: {'error': 'access_denied', 'message': 'Unauthorized'},
            statusCode: 401,
            requestOptions: RequestOptions(path: '/conversations/$conversationId'),
          ),
          type: DioExceptionType.badResponse,
        ),
      );

      expect(
        () => repository.getConversation(conversationId),
        throwsA(
          isA<UnauthorizedException>().having(
            (e) => e.apiErrorCode,
            'apiErrorCode',
            'access_denied',
          ),
        ),
      );
    });

    test('403 -> ConversationForbiddenException', () async {
      when(
        mockDio.get<Map<String, dynamic>>(
          any,
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenThrow(
        DioException(
          requestOptions: RequestOptions(path: '/conversations/$conversationId'),
          response: Response<Map<String, dynamic>>(
            data: {'error': 'forbidden', 'message': 'no access'},
            statusCode: 403,
            requestOptions: RequestOptions(path: '/conversations/$conversationId'),
          ),
          type: DioExceptionType.badResponse,
        ),
      );

      expect(
        () => repository.getConversation(conversationId),
        throwsA(
          isA<ConversationForbiddenException>().having(
            (e) => e.apiErrorCode,
            'apiErrorCode',
            'forbidden',
          ),
        ),
      );
    });
  });

  group('network', () {
    test('timeout -> ConversationApiException', () async {
      when(
        mockDio.get<Map<String, dynamic>>(
          any,
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenThrow(
        DioException(
          requestOptions: RequestOptions(path: '/conversations/$conversationId'),
          type: DioExceptionType.connectionTimeout,
        ),
      );

      expect(
        () => repository.getConversation(conversationId),
        throwsA(
          isA<ConversationApiException>().having(
            (e) => e.message,
            'message',
            'Network timeout',
          ),
        ),
      );
    });

    test('connection error', () async {
      when(
        mockDio.get<Map<String, dynamic>>(
          any,
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenThrow(
        DioException(
          requestOptions: RequestOptions(path: '/conversations/$conversationId'),
          type: DioExceptionType.connectionError,
        ),
      );

      expect(
        () => repository.getConversation(conversationId),
        throwsA(
          isA<ConversationApiException>().having(
            (e) => e.message,
            'message',
            'Network error',
          ),
        ),
      );
    });
  });

  group('validation', () {
    test('empty ids -> ArgumentError', () async {
      expect(
        () => repository.listConversations(''),
        throwsA(isA<ArgumentError>()),
      );
      expect(
        () => repository.getConversation(''),
        throwsA(isA<ArgumentError>()),
      );
      expect(
        () => repository.sendMessage(
          '',
          const SendMessageRequest(content: 'a'),
          clientMessageId: clientMessageId,
        ),
        throwsA(isA<ArgumentError>()),
      );
    });
  });

  group('sanitization', () {
    test('message with credentials in URL is sanitized in exception', () async {
      when(
        mockDio.get<Map<String, dynamic>>(
          any,
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenThrow(
        DioException(
          requestOptions: RequestOptions(path: '/conversations/$conversationId'),
          response: Response<Map<String, dynamic>>(
            data: {
              'error': 'bad_request',
              'message':
                  'failed https://user:secret@evil.example/path',
            },
            statusCode: 400,
            requestOptions: RequestOptions(path: '/conversations/$conversationId'),
          ),
          type: DioExceptionType.badResponse,
        ),
      );

      try {
        await repository.getConversation(conversationId);
        fail('expected');
      } on ConversationApiException catch (e) {
        expect(e.message, isNot(contains('secret')));
        expect(e.message, contains('https://evil.example/path'));
      }
    });

    test('401 without message does not put stable error code in user text', () async {
      when(
        mockDio.get<Map<String, dynamic>>(
          any,
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenThrow(
        DioException(
          requestOptions: RequestOptions(path: '/conversations/$conversationId'),
          response: Response<Map<String, dynamic>>(
            data: {'error': 'access_denied'},
            statusCode: 401,
            requestOptions: RequestOptions(path: '/conversations/$conversationId'),
          ),
          type: DioExceptionType.badResponse,
        ),
      );

      try {
        await repository.getConversation(conversationId);
        fail('expected');
      } on UnauthorizedException catch (e) {
        expect(e.message, isNot(contains('access_denied')));
        expect(e.message, contains('Request failed'));
        expect(e.apiErrorCode, 'access_denied');
      }
    });
  });

  group('limits', () {
    /// По ТЗ 11.3 (§лимиты): валидация длины на бэкенде; репозиторий не режет тело — только проброс в сеть.
    test('sendMessage does not truncate content over max length', () async {
      final long = 'a' * (kMaxMessageContentLength + 50);
      when(
        mockDio.post<Map<String, dynamic>>(
          any,
          data: anyNamed('data'),
          options: anyNamed('options'),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenAnswer(
        (_) async => Response<Map<String, dynamic>>(
          data: messageJson(),
          statusCode: 201,
          requestOptions: RequestOptions(
            path: '/conversations/$conversationId/messages',
          ),
        ),
      );

      await repository.sendMessage(
        conversationId,
        SendMessageRequest(content: long),
        clientMessageId: clientMessageId,
      );

      verify(
        mockDio.post<Map<String, dynamic>>(
          any,
          data: argThat(
            predicate<Map<String, dynamic>>(
              (m) => (m['content'] as String?)?.length == long.length,
            ),
            named: 'data',
          ),
          options: anyNamed('options'),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).called(1);
    });
  });

  group('POST JSON content-type', () {
    test('createConversation and sendMessage use application/json without charset',
        () async {
      when(
        mockDio.post<Map<String, dynamic>>(
          '/projects/$projectId/conversations',
          data: anyNamed('data'),
          options: anyNamed('options'),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenAnswer(
        (_) async => Response<Map<String, dynamic>>(
          data: conversationJson(),
          statusCode: 201,
          requestOptions: RequestOptions(path: '/projects/$projectId/conversations'),
        ),
      );

      when(
        mockDio.post<Map<String, dynamic>>(
          '/conversations/$conversationId/messages',
          data: anyNamed('data'),
          options: anyNamed('options'),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenAnswer(
        (_) async => Response<Map<String, dynamic>>(
          data: messageJson(),
          statusCode: 201,
          requestOptions: RequestOptions(
            path: '/conversations/$conversationId/messages',
          ),
        ),
      );

      await repository.createConversation(
        projectId,
        const CreateConversationRequest(title: 't'),
      );
      await repository.sendMessage(
        conversationId,
        const SendMessageRequest(content: 'c'),
        clientMessageId: clientMessageId,
      );

      verify(
        mockDio.post<Map<String, dynamic>>(
          '/projects/$projectId/conversations',
          data: anyNamed('data'),
          options: argThat(
            predicate<Options>((o) {
              if (o.contentType != 'application/json') {
                return false;
              }
              final h = o.headers;
              if (h != null) {
                for (final k in h.keys) {
                  if (k.toLowerCase() == 'x-client-message-id') {
                    return false;
                  }
                }
              }
              return true;
            }),
            named: 'options',
          ),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).called(1);

      verify(
        mockDio.post<Map<String, dynamic>>(
          '/conversations/$conversationId/messages',
          data: anyNamed('data'),
          options: argThat(
            predicate<Options>((o) {
              if (o.contentType != 'application/json') {
                return false;
              }
              final h = o.headers;
              if (h == null) {
                return false;
              }
              final v = h['X-Client-Message-ID'] ?? h['x-client-message-id'];
              return v != null && v.toString().contains(clientMessageId);
            }),
            named: 'options',
          ),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).called(1);
    });
  });
}
