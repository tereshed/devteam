// @dart=2.19
@TestOn('vm')
@Tags(['unit'])
library;

import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/core/api/api_exceptions.dart';
import 'package:frontend/core/utils/uuid.dart';
import 'package:frontend/features/chat/data/chat_providers.dart';
import 'package:frontend/features/chat/data/conversation_exceptions.dart';
import 'package:frontend/features/chat/data/conversation_repository.dart';
import 'package:frontend/features/chat/domain/models.dart';
import 'package:frontend/features/chat/domain/requests.dart';
import 'package:frontend/features/chat/presentation/controllers/chat_controller.dart';
import 'package:frontend/l10n/app_localizations_en.dart';
import 'package:mockito/annotations.dart';
import 'package:mockito/mockito.dart';

import 'chat_controller_test.mocks.dart';

@GenerateNiceMocks([MockSpec<ConversationRepository>()])
void main() {
  const pid = '550e8400-e29b-41d4-a716-446655440000';
  const cid = '6ba7b810-9dad-11d1-80b4-00c04fd430c8';

  late MockConversationRepository mockRepo;
  late ProviderContainer container;

  ConversationModel conv() => ConversationModel(
        id: cid,
        projectId: pid,
        title: 'T',
        status: 'active',
        createdAt: DateTime.utc(2026, 1, 1),
        updatedAt: DateTime.utc(2026, 1, 2),
      );

  ConversationMessageModel msg(
    String id,
    String content,
    DateTime createdAt, {
    String role = 'user',
  }) {
    return ConversationMessageModel(
      id: id,
      conversationId: cid,
      role: role,
      content: content,
      createdAt: createdAt,
    );
  }

  ChatController ctrl() => container.read(
        chatControllerProvider(projectId: pid, conversationId: cid).notifier,
      );

  Future<void> waitIdle() async {
    const step = Duration(milliseconds: 2);
    const timeout = Duration(seconds: 2);
    final sw = Stopwatch()..start();
    while (sw.elapsed < timeout) {
      final st = container.read(
        chatControllerProvider(projectId: pid, conversationId: cid),
      );
      if (st.hasError) {
        return;
      }
      if (st.hasValue) {
        final v = st.requireValue;
        if (!v.isLoadingInitial && !v.isLoadingOlder) {
          return;
        }
      }
      await Future<void>.delayed(step);
    }
    fail('timeout waitIdle');
  }

  int offsetFromInvocation(Invocation inv) {
    final o = inv.namedArguments[#offset];
    if (o is int) {
      return o;
    }
    return 0;
  }

  setUp(() {
    mockRepo = MockConversationRepository();
    container = ProviderContainer(
      overrides: [
        conversationRepositoryProvider.overrideWithValue(mockRepo),
      ],
    );
    addTearDown(container.dispose);
  });

  group('ChatController', () {
    test('empty projectId yields AsyncError with ArgumentError', () {
      final c = ProviderContainer(
        overrides: [
          conversationRepositoryProvider.overrideWithValue(mockRepo),
        ],
      );
      addTearDown(c.dispose);
      final st = c.read(chatControllerProvider(projectId: '', conversationId: cid));
      expect(st.hasError, isTrue);
      expect(st.error, isA<ArgumentError>());
    });

    test('first load: canonical order is ASC (inverse of API newest-first page)',
        () async {
      when(
        mockRepo.getConversation(cid, cancelToken: anyNamed('cancelToken')),
      ).thenAnswer((_) async => conv());
      when(
        mockRepo.getMessages(
          cid,
          limit: anyNamed('limit'),
          offset: anyNamed('offset'),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenAnswer(
        (_) async => MessageListResponse(
          messages: [
            msg('bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb', 'new',
                DateTime.utc(2026, 1, 1, 12)),
            msg('aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'old',
                DateTime.utc(2026, 1, 1, 11)),
          ],
          hasNext: false,
          limit: 20,
          offset: 0,
        ),
      );

      ctrl();
      await waitIdle();

      final st = container.read(
        chatControllerProvider(projectId: pid, conversationId: cid),
      );
      final messages = st.requireValue.messages;
      expect(messages.map((m) => m.id).toList(), [
        'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa',
        'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb',
      ]);
    });

    test('loadOlder merges without duplicate ids', () async {
      when(
        mockRepo.getConversation(cid, cancelToken: anyNamed('cancelToken')),
      ).thenAnswer((_) async => conv());
      when(
        mockRepo.getMessages(
          cid,
          limit: anyNamed('limit'),
          offset: anyNamed('offset'),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenAnswer((inv) async {
        switch (offsetFromInvocation(inv)) {
          case 0:
            return MessageListResponse(
              messages: [
                msg('22222222-2222-2222-2222-222222222222', 'b',
                    DateTime.utc(2026, 1, 2)),
                msg('11111111-1111-1111-1111-111111111111', 'a',
                    DateTime.utc(2026, 1, 1)),
              ],
              hasNext: true,
              limit: 2,
              offset: 0,
            );
          case 2:
            return MessageListResponse(
              messages: [
                msg('11111111-1111-1111-1111-111111111111', 'a',
                    DateTime.utc(2026, 1, 1)),
                msg('00000000-0000-0000-0000-000000000000', 'z',
                    DateTime.utc(2026, 1, 0)),
              ],
              hasNext: false,
              limit: 2,
              offset: 2,
            );
          default:
            return const MessageListResponse();
        }
      });

      ctrl();
      await waitIdle();
      await ctrl().loadOlder();
      await waitIdle();

      final ids = container
          .read(chatControllerProvider(projectId: pid, conversationId: cid))
          .requireValue
          .messages
          .map((m) => m.id)
          .toList();
      expect(ids.toSet().length, ids.length);
      expect(
        ids,
        containsAll(<String>[
          '00000000-0000-0000-0000-000000000000',
          '11111111-1111-1111-1111-111111111111',
          '22222222-2222-2222-2222-222222222222',
        ]),
      );
    });

    test(
      'ConversationApiException on loadOlder does not replace chat with AsyncError',
      () async {
        when(
          mockRepo.getConversation(cid, cancelToken: anyNamed('cancelToken')),
        ).thenAnswer((_) async => conv());
        when(
          mockRepo.getMessages(
            cid,
            limit: anyNamed('limit'),
            offset: anyNamed('offset'),
            cancelToken: anyNamed('cancelToken'),
          ),
        ).thenAnswer((inv) async {
          switch (offsetFromInvocation(inv)) {
            case 0:
              return MessageListResponse(
                messages: [
                  msg(
                    '99999999-9999-9999-9999-999999999999',
                    'first',
                    DateTime.utc(2026, 2, 1),
                  ),
                ],
                hasNext: true,
                limit: 20,
                offset: 0,
              );
            case 1:
              throw ConversationApiException(
                'bad gateway',
                statusCode: 502,
              );
            default:
              return const MessageListResponse();
          }
        });

        ctrl();
        await waitIdle();

        await expectLater(
          () => ctrl().loadOlder(),
          throwsA(isA<ConversationApiException>()),
        );
        await waitIdle();

        final st = container.read(
          chatControllerProvider(projectId: pid, conversationId: cid),
        );
        expect(st.hasError, isFalse);
        final v = st.requireValue;
        expect(v.isLoadingOlder, isFalse);
        expect(
          v.messages.map((m) => m.id),
          contains('99999999-9999-9999-9999-999999999999'),
        );
      },
    );

    test('sendMessage receives valid X-Client-Message-ID (UUID v4 format)',
        () async {
      when(
        mockRepo.getConversation(cid, cancelToken: anyNamed('cancelToken')),
      ).thenAnswer((_) async => conv());
      when(
        mockRepo.getMessages(
          cid,
          limit: anyNamed('limit'),
          offset: anyNamed('offset'),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenAnswer((_) async => const MessageListResponse());
      when(
        mockRepo.sendMessage(
          cid,
          any,
          clientMessageId: anyNamed('clientMessageId'),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenAnswer(
        (inv) async {
          final id = inv.namedArguments[#clientMessageId]! as String;
          expect(isValidUuid(id), isTrue);
          return SendMessageResult(
            message: msg(
              '33333333-3333-3333-3333-333333333333',
              'x',
              DateTime.utc(2026, 2, 1),
            ),
            status: MessageSendStatus.created,
          );
        },
      );

      ctrl();
      await waitIdle();
      await ctrl().send('hello');

      verify(
        mockRepo.sendMessage(
          cid,
          const SendMessageRequest(content: 'hello'),
          clientMessageId: argThat(predicate(isValidUuid), named: 'clientMessageId'),
          cancelToken: null,
        ),
      ).called(1);
    });

    test('created and duplicate upsert to same single message by id', () async {
      when(
        mockRepo.getConversation(cid, cancelToken: anyNamed('cancelToken')),
      ).thenAnswer((_) async => conv());
      when(
        mockRepo.getMessages(
          cid,
          limit: anyNamed('limit'),
          offset: anyNamed('offset'),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenAnswer((_) async => const MessageListResponse());
      final m = msg(
        '44444444-4444-4444-4444-444444444444',
        'c',
        DateTime.utc(2026, 3, 1),
      );
      when(
        mockRepo.sendMessage(
          cid,
          any,
          clientMessageId: anyNamed('clientMessageId'),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenAnswer(
        (_) async => SendMessageResult(
          message: m,
          status: MessageSendStatus.created,
        ),
      );

      ctrl();
      await waitIdle();
      await ctrl().send('one');
      when(
        mockRepo.sendMessage(
          cid,
          any,
          clientMessageId: anyNamed('clientMessageId'),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenAnswer(
        (_) async => SendMessageResult(
          message: m,
          status: MessageSendStatus.duplicate,
        ),
      );
      await ctrl().send('two');

      final list = container
          .read(chatControllerProvider(projectId: pid, conversationId: cid))
          .requireValue
          .messages
          .where((x) => x.id == m.id)
          .toList();
      expect(list, hasLength(1));
    });

    test('ConversationNotFoundException on refresh yields AsyncError', () async {
      when(
        mockRepo.getConversation(cid, cancelToken: anyNamed('cancelToken')),
      ).thenAnswer((_) async => conv());
      when(
        mockRepo.getMessages(
          cid,
          limit: anyNamed('limit'),
          offset: anyNamed('offset'),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenAnswer((_) async => const MessageListResponse());

      ctrl();
      await waitIdle();

      when(
        mockRepo.getConversation(cid, cancelToken: anyNamed('cancelToken')),
      ).thenThrow(ConversationNotFoundException('gone'));

      await ctrl().refresh();
      await waitIdle();

      final st = container.read(
        chatControllerProvider(projectId: pid, conversationId: cid),
      );
      expect(st.hasError, isTrue);
      expect(st.error, isA<ConversationNotFoundException>());
    });

    test(
      'refresh keeps prior conversation until metadata reload completes',
      () async {
        final completer = Completer<ConversationModel>();
        var getConversationCalls = 0;
        when(
          mockRepo.getConversation(cid, cancelToken: anyNamed('cancelToken')),
        ).thenAnswer((_) async {
          getConversationCalls++;
          if (getConversationCalls == 1) {
            return conv();
          }
          return completer.future;
        });
        when(
          mockRepo.getMessages(
            cid,
            limit: anyNamed('limit'),
            offset: anyNamed('offset'),
            cancelToken: anyNamed('cancelToken'),
          ),
        ).thenAnswer((_) async => const MessageListResponse());

        ctrl();
        await waitIdle();

        final fut = ctrl().refresh();
        await Future<void>.delayed(Duration.zero);
        final mid = container
            .read(chatControllerProvider(projectId: pid, conversationId: cid))
            .requireValue;
        expect(mid.conversation, isNotNull);
        expect(mid.conversation!.title, 'T');
        expect(mid.isLoadingInitial, isTrue);

        completer.complete(conv());
        await fut;
        await waitIdle();

        final done = container
            .read(chatControllerProvider(projectId: pid, conversationId: cid))
            .requireValue;
        expect(done.conversation!.title, 'T');
      },
    );

    test('empty page with has_next true stops hasMoreOlder (anti-loop)',
        () async {
      when(
        mockRepo.getConversation(cid, cancelToken: anyNamed('cancelToken')),
      ).thenAnswer((_) async => conv());
      when(
        mockRepo.getMessages(
          cid,
          limit: anyNamed('limit'),
          offset: anyNamed('offset'),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenAnswer(
        (_) async => const MessageListResponse(
          messages: [],
          hasNext: true,
        ),
      );

      ctrl();
      await waitIdle();

      final s = container
          .read(chatControllerProvider(projectId: pid, conversationId: cid))
          .requireValue;
      expect(s.hasMoreOlder, isFalse);

      clearInteractions(mockRepo);
      await ctrl().loadOlder();
      await waitIdle();

      verifyNever(
        mockRepo.getMessages(
          cid,
          limit: anyNamed('limit'),
          offset: anyNamed('offset'),
          cancelToken: anyNamed('cancelToken'),
        ),
      );
    });

    test('transient send failure then retrySend uses same clientMessageId',
        () async {
      when(
        mockRepo.getConversation(cid, cancelToken: anyNamed('cancelToken')),
      ).thenAnswer((_) async => conv());
      when(
        mockRepo.getMessages(
          cid,
          limit: anyNamed('limit'),
          offset: anyNamed('offset'),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenAnswer((_) async => const MessageListResponse());

      var capturedId = '';
      when(
        mockRepo.sendMessage(
          cid,
          any,
          clientMessageId: anyNamed('clientMessageId'),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenAnswer((inv) async {
        capturedId = inv.namedArguments[#clientMessageId]! as String;
        throw ConversationApiException(
          'bad gateway',
          statusCode: 502,
        );
      });

      ctrl();
      await waitIdle();
      await ctrl().send('x');
      expect(capturedId, isNotEmpty);

      when(
        mockRepo.sendMessage(
          cid,
          any,
          clientMessageId: anyNamed('clientMessageId'),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenAnswer(
        (inv) async {
          expect(
            inv.namedArguments[#clientMessageId]! as String,
            capturedId,
          );
          return SendMessageResult(
            message: msg(
              '55555555-5555-5555-5555-555555555555',
              'x',
              DateTime.utc(2026, 4, 1),
            ),
            status: MessageSendStatus.created,
          );
        },
      );

      await ctrl().retrySend(capturedId);
      await waitIdle();

      // Первая попытка (502) и retry — один и тот же clientMessageId.
      verify(
        mockRepo.sendMessage(
          cid,
          const SendMessageRequest(content: 'x'),
          clientMessageId: capturedId,
          cancelToken: null,
        ),
      ).called(2);
    });

    test('two synchronous loadOlder calls perform one getMessages for older',
        () async {
      when(
        mockRepo.getConversation(cid, cancelToken: anyNamed('cancelToken')),
      ).thenAnswer((_) async => conv());
      when(
        mockRepo.getMessages(
          cid,
          limit: anyNamed('limit'),
          offset: anyNamed('offset'),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenAnswer((inv) async {
        switch (offsetFromInvocation(inv)) {
          case 0:
            return MessageListResponse(
              messages: [
                msg('aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'n',
                    DateTime.utc(2026, 5, 1)),
              ],
              hasNext: true,
              limit: 1,
              offset: 0,
            );
          case 1:
            return Future<MessageListResponse>.delayed(
              const Duration(milliseconds: 30),
              () => MessageListResponse(
                messages: [
                  msg('bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb', 'o',
                      DateTime.utc(2026, 4, 1)),
                ],
                hasNext: false,
                limit: 1,
                offset: 1,
              ),
            );
          default:
            return const MessageListResponse();
        }
      });

      ctrl();
      await waitIdle();

      final c = ctrl();
      final a = c.loadOlder();
      final b = c.loadOlder();
      await Future.wait<void>([a, b]);
      await waitIdle();

      verify(
        mockRepo.getMessages(
          cid,
          limit: anyNamed('limit'),
          offset: 1,
          cancelToken: anyNamed('cancelToken'),
        ),
      ).called(1);
    });

    test('upsert inserts message with created_at between existing (sort)',
        () async {
      when(
        mockRepo.getConversation(cid, cancelToken: anyNamed('cancelToken')),
      ).thenAnswer((_) async => conv());
      when(
        mockRepo.getMessages(
          cid,
          limit: anyNamed('limit'),
          offset: anyNamed('offset'),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenAnswer(
        (_) async => MessageListResponse(
          messages: [
            msg('cccccccc-cccc-cccc-cccc-cccccccccccc', 'hi',
                DateTime.utc(2026, 6, 2)),
            msg('dddddddd-dddd-dddd-dddd-dddddddddddd', 'lo',
                DateTime.utc(2026, 6, 1)),
          ],
          hasNext: false,
        ),
      );

      ctrl();
      await waitIdle();

      ctrl().applyIncomingMessage(
        msg('eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee', 'between',
            DateTime.utc(2026, 6, 1, 12)),
      );

      final ordered = container
          .read(chatControllerProvider(projectId: pid, conversationId: cid))
          .requireValue
          .messages;
      expect(ordered.map((m) => m.id).toList(), [
        'dddddddd-dddd-dddd-dddd-dddddddddddd',
        'eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee',
        'cccccccc-cccc-cccc-cccc-cccccccccccc',
      ]);
    });

    test('applyIncomingMessage ignores other conversationId', () async {
      when(
        mockRepo.getConversation(cid, cancelToken: anyNamed('cancelToken')),
      ).thenAnswer((_) async => conv());
      when(
        mockRepo.getMessages(
          cid,
          limit: anyNamed('limit'),
          offset: anyNamed('offset'),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenAnswer(
        (_) async => MessageListResponse(
          messages: [
            msg('ffffffff-ffff-ffff-ffff-ffffffffffff', 'x',
                DateTime.utc(2026, 7, 1)),
          ],
          hasNext: false,
        ),
      );

      ctrl();
      await waitIdle();

      final before = container
          .read(chatControllerProvider(projectId: pid, conversationId: cid))
          .requireValue
          .messages
          .length;

      ctrl().applyIncomingMessage(
        ConversationMessageModel(
          id: '12121212-1212-1212-1212-121212121212',
          conversationId: 'abababab-abab-abab-abab-abababababab',
          role: 'user',
          content: 'ghost',
          createdAt: DateTime.utc(2026, 8, 1),
        ),
      );

      final after = container
          .read(chatControllerProvider(projectId: pid, conversationId: cid))
          .requireValue
          .messages
          .length;
      expect(after, before);
    });

    test('refresh during loadOlder: stale page merge ignored after epoch bump',
        () async {
      when(
        mockRepo.getConversation(cid, cancelToken: anyNamed('cancelToken')),
      ).thenAnswer((_) async => conv());

      var offset0Calls = 0;
      final olderCompleter = Completer<MessageListResponse>();
      when(
        mockRepo.getMessages(
          cid,
          limit: anyNamed('limit'),
          offset: anyNamed('offset'),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenAnswer((inv) {
        final off = offsetFromInvocation(inv);
        if (off == 1) {
          return olderCompleter.future;
        }
        if (off == 0) {
          offset0Calls++;
          if (offset0Calls == 1) {
            return Future.value(
              MessageListResponse(
                messages: [
                  msg('13131313-1313-1313-1313-131313131313', 'f',
                      DateTime.utc(2026, 9, 1)),
                ],
                hasNext: true,
                limit: 1,
                offset: 0,
              ),
            );
          }
          return Future.value(
            MessageListResponse(
              messages: [
                msg('14141414-1414-1414-1414-141414141414', 'r',
                    DateTime.utc(2026, 9, 2)),
              ],
              hasNext: false,
              limit: 20,
              offset: 0,
            ),
          );
        }
        return Future.value(const MessageListResponse());
      });

      ctrl();
      await waitIdle();

      final c = ctrl();
      unawaited(c.loadOlder());
      await Future<void>.delayed(const Duration(milliseconds: 5));

      await c.refresh();
      await waitIdle();

      olderCompleter.complete(
        MessageListResponse(
          messages: [
            msg('15151515-1515-1515-1515-151515151515', 's',
                DateTime.utc(2026, 1, 1)),
          ],
          hasNext: false,
          limit: 1,
          offset: 1,
        ),
      );
      await waitIdle();

      final ids = container
          .read(chatControllerProvider(projectId: pid, conversationId: cid))
          .requireValue
          .messages
          .map((m) => m.id)
          .toSet();
      expect(ids, contains('14141414-1414-1414-1414-141414141414'));
      expect(ids, isNot(contains('15151515-1515-1515-1515-151515151515')));
    });

    test(
        'inflight sendMessage survives refresh; success merges into refreshed state',
        () async {
      when(
        mockRepo.getConversation(cid, cancelToken: anyNamed('cancelToken')),
      ).thenAnswer((_) async => conv());

      var getMessagesCalls = 0;
      when(
        mockRepo.getMessages(
          cid,
          limit: anyNamed('limit'),
          offset: anyNamed('offset'),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenAnswer((_) async {
        getMessagesCalls++;
        if (getMessagesCalls == 1) {
          return const MessageListResponse();
        }
        return MessageListResponse(
          messages: [
            msg('16161616-1616-1616-1616-161616161616', 'r',
                DateTime.utc(2026, 10, 1)),
          ],
          hasNext: false,
        );
      });

      Completer<SendMessageResult>? sendCompleter;
      when(
        mockRepo.sendMessage(
          cid,
          any,
          clientMessageId: anyNamed('clientMessageId'),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenAnswer((_) {
        sendCompleter = Completer<SendMessageResult>();
        return sendCompleter!.future;
      });

      ctrl();
      await waitIdle();

      final c = ctrl();
      unawaited(c.send('async'));

      await c.refresh();
      await waitIdle();

      sendCompleter!.complete(
        SendMessageResult(
          message: msg(
            '17171717-1717-1717-1717-171717171717',
            'async',
            DateTime.utc(2026, 10, 2),
          ),
          status: MessageSendStatus.created,
        ),
      );
      await Future<void>.delayed(const Duration(milliseconds: 20));
      await waitIdle();

      final ids = container
          .read(chatControllerProvider(projectId: pid, conversationId: cid))
          .requireValue
          .messages
          .map((m) => m.id)
          .toSet();
      expect(ids, contains('16161616-1616-1616-1616-161616161616'));
      expect(ids, contains('17171717-1717-1717-1717-171717171717'));
    });
  });

  group('chatErrorTitle', () {
    test('maps UnauthorizedException to l10n', () {
      final l10n = AppLocalizationsEn();
      expect(
        chatErrorTitle(l10n, UnauthorizedException('x')),
        l10n.errorUnauthorized,
      );
    });

    test('maps ConversationApiException with transport flag to errorNetwork',
        () {
      final l10n = AppLocalizationsEn();
      expect(
        chatErrorTitle(
          l10n,
          ConversationApiException(
            'any diagnostic text',
            isNetworkTransportError: true,
          ),
        ),
        l10n.errorNetwork,
      );
    });
  });

  group('chatErrorDetail', () {
    test('truncates without stacking ellipsis on slice already ending in …',
        () {
      final pad = 'a' * 198;
      final err = ConversationApiException('$pad${'\u2026' * 3}');
      final d = chatErrorDetail(err)!;
      expect(d.endsWith('…'), isTrue);
      expect('……'.allMatches(d).length, lessThan(2));
    });
  });
}
